package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

	//static fil
	mux.Handle("/", http.FileServer(http.Dir("frontend")))

	return mux
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

		history, err := database.GetHistory(chatRequest.ConversationID)
		if err != nil {
			http.Error(w, "history error", http.StatusInternalServerError)
			return
		}

		// nytt meddelande: system prompt + history + new user message
		systemPrompt := defense.BuildPrompt(cfg)
		messages := []models.Message{
			{Role: "system", Content: systemPrompt},
		}
		messages = append(messages, history...)
		messages = append(messages, models.Message{
			Role:    "user",
			Content: chatRequest.Message,
		})

		executor := &tools.ToolExecutor{DB: database}

		// tool calling loop, låter llm max köra 5 iterationer
		for i := 0; i < 5; i++ {
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
