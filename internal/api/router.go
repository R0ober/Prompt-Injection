package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"temp-name/internal/auth"
	"temp-name/internal/db"
	"temp-name/internal/defense"
	"temp-name/internal/llm"
	"temp-name/internal/models"
	"temp-name/internal/tools"
)

type ChatRequestBody struct {
	Message        string `json:"message"`
	Model          string `json:"model"`
	ConversationID string `json:"conversation_id"`
}

type ChatResponse struct {
	Response    string `json:"response"`
	Blocked     bool   `json:"blocked"`
	BlockReason string `json:"block_reason,omitempty"`
}

func NewRouter(database *db.DB) http.Handler {
	mux := http.NewServeMux()
	//api route
	mux.Handle("POST /api/login", loginHandler(database))

	// skyddade routes
	mux.Handle("POST /api/chat", auth.Middleware(chatHandler(database)))
	mux.Handle("POST /api/upload", auth.Middleware(uploadHandler(database)))

	//redirect root till login
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/login.html", http.StatusFound)
			return
		}
		http.FileServer(http.Dir("frontend")).ServeHTTP(w, r)
	})

	return mux
}

func uploadHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetClaims(r)
		if claims == nil {
			http.Error(w, "unathorized", http.StatusUnauthorized)
			return
		}

		// parse med 10 mb storleks limit
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "file to large", http.StatusBadRequest)
			return
		}

		conversationID := r.FormValue("conversation_id")
		if conversationID == "" {
			http.Error(w, "missing conversation_id", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "no file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// convertera pdf eller txt fil till string
		text, err := extractText(file, header.Filename)
		if err != nil {
			http.Error(w, "could not read file", http.StatusBadRequest)
			return
		}
		if text == "" {
			http.Error(w, "empty file", http.StatusBadRequest)
			return
		}

		// lägg til fil i chat history
		msg := models.Message{
			Role: "user",
			Content: fmt.Sprintf(
				"I have uploaded a file named '%s'. Here is its contents:\n\n[FILE CONTENT START]\n%s\n[FILE CONTENT END]",
				header.Filename,
				text,
			),
		}
		database.SaveMessage(conversationID, claims.UserID, msg, "")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "uploaded"})

	}
}

func extractText(file multipart.File, filename string) (string, error) {
	buf, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	if strings.HasSuffix(strings.ToLower(filename), ".txt") {
		return string(buf), nil
	}

	if strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		// write to temp file — pdftotext needs a file path
		tmp, err := os.CreateTemp("", "upload-*.pdf")
		if err != nil {
			return "", fmt.Errorf("temp file: %w", err)
		}
		defer os.Remove(tmp.Name())
		defer tmp.Close()

		if _, err := tmp.Write(buf); err != nil {
			return "", fmt.Errorf("write temp: %w", err)
		}
		tmp.Close()

		// "-" means output to stdout
		out, err := exec.Command("pdftotext", tmp.Name(), "-").Output()
		if err != nil {
			return "", fmt.Errorf("pdftotext error: %w", err)
		}
		return string(out), nil
	}

	return "", fmt.Errorf("unsupported file type")
}

func loginHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var LoginBody struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		err := json.NewDecoder(r.Body).Decode(&LoginBody)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		tokenString, err := auth.Login(database, LoginBody.Username, LoginBody.Password)
		if err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		// browsers
		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    tokenString,
			Path:     "/",
			HttpOnly: true, // js kan inte läsa token xss protection
			SameSite: http.SameSiteStrictMode,
		})
		// mobile apps/ api
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"token": tokenString})

	}
}
func chatHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var chatRequest ChatRequestBody

		if err := json.NewDecoder(r.Body).Decode(&chatRequest); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		claims := auth.GetClaims(r)
		if claims == nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		var cfg = defense.Config{
			UseStrongPrompt: false,
			InputFilter:     false,
			OutputFilter:    false,
		}

		history, err := database.GetHistory(chatRequest.ConversationID, claims.UserID)
		if err != nil {
			http.Error(w, "history error", http.StatusInternalServerError)
			return
		}
		//input filter
		if blocked, reason := defense.CheckInput(chatRequest.Message, cfg); blocked {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ChatResponse{
				Blocked:     true,
				BlockReason: reason,
			})
			return
		}
		// nytt meddelande: system prompt + history + new user message
		systemPrompt := defense.BuildPrompt(cfg, claims.Username, claims.Role)
		messages := []models.Message{
			{Role: "system", Content: systemPrompt},
		}
		messages = append(messages, history...)
		messages = append(messages, models.Message{
			Role:    "user",
			Content: chatRequest.Message,
		})

		executor := &tools.ToolExecutor{DB: database,
			CallerUsername: claims.Username,
			CallerRole:     claims.Role}

		// tool calling loop, låter llm max köra 10 iterationer
		for i := 0; i < 10; i++ {
			response, err := llm.Chat(chatRequest.Model, messages, tools.AvailableTools)
			if err != nil {
				log.Printf("llm error: %v", err)
				http.Error(w, "llm error", http.StatusBadGateway)
				return
			}

			// inga tool calls — llm är klar returna response
			if len(response.ToolCalls) == 0 {
				database.SaveMessage(chatRequest.ConversationID, claims.UserID, models.Message{Role: "user", Content: chatRequest.Message}, chatRequest.Model)
				database.SaveMessage(chatRequest.ConversationID, claims.UserID, response, chatRequest.Model)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(ChatResponse{
					Response: response.Content,
					Blocked:  false,
				})
				return
			}

			// append llm message with tool calls to history
			messages = append(messages, response)

			// kör alla tool calls
			for _, toolCall := range response.ToolCalls {
				result, err := executor.Execute(toolCall.Function.Name, toolCall.Function.Arguments)
				if err != nil {
					result = fmt.Sprintf("error: %v", err)
				}
				log.Printf("tool call: %s(%s) => %s", toolCall.Function.Name, toolCall.Function.Arguments, result)

				// append tool result to messages
				messages = append(messages, models.Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: toolCall.ID,
				})
			}
		}
		// if we hit max iterations return error
		http.Error(w, "max tool iterations reached", http.StatusInternalServerError)
	}
}
