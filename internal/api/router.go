package api

import (
	"encoding/json"
	"net/http"
	"temp-name/internal/auth"
	"temp-name/internal/db"
)

func NewRouter(database *db.DB) http.Handler {
	mux := http.NewServeMux()
	//api route
	mux.Handle("POST /api/login", loginHandler(database))
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
