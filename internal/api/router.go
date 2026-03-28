package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"temp-name/internal/auth"
	"temp-name/internal/db"
	"temp-name/internal/defense"
	"temp-name/internal/llm"
)

type ChatRequestBody struct {
	Message string `json:"message"`
	Model   string `json:"model"`
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
		err := json.NewDecoder(r.Body).Decode(&chatRequest)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		claims := auth.GetClaims(r)
		if claims == nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		llmContext, err := buildDBContext(database, claims.Username)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var cfg = defense.Config{
			UseStrongPrompt: false,
			InputFilter:     false,
			OutputFilter:    false,
		}
		systemPrompt := defense.BuildPrompt(llmContext, cfg)

		response, err := llm.Chat(chatRequest.Model, systemPrompt, chatRequest.Message)
		if err != nil {
			log.Printf("DEBUG llm error: %v", err)
			http.Error(w, "llm error", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ChatResponse{
			Response: response,
			Blocked:  false,
		})

	}
}

func buildDBContext(database *db.DB, username string) (string, error) {
	var sb strings.Builder

	user, err := database.GetUserByUsername(username)
	if err != nil {
		return "", fmt.Errorf("user fetch error: %w", err)
	}

	sb.WriteString("User information:\n")
	sb.WriteString(fmt.Sprintf("  Username: %s\n", user.User.Username))
	sb.WriteString(fmt.Sprintf("  Role: %s\n", user.User.Role))
	sb.WriteString(fmt.Sprintf("  Notes: %s\n", user.User.Notes))

	orders, err := database.GetOrdersByUserID(user.User.ID)
	if err != nil {
		return "", fmt.Errorf("order fetch error: %w", err)
	}

	sb.WriteString("\nOrder information:\n")
	for i, order := range orders {
		sb.WriteString(fmt.Sprintf("  Order %d:\n", i+1))
		sb.WriteString(fmt.Sprintf("    Product: %s\n", order.Product))
		sb.WriteString(fmt.Sprintf("    Amount: %.2f\n", order.Amount))
		sb.WriteString(fmt.Sprintf("    Status: %s\n", order.Status))
		sb.WriteString(fmt.Sprintf("    Notes: %s\n", order.PrivateNotes))
	}

	return sb.String(), nil
}
