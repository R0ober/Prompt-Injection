package auth

import (
	"context"
	"fmt"

	//"log"
	"net/http"
	"os"
	"strings"
	"temp-name/internal/db"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const UserKey contextKey = "user"

// jwt claims
type Claims struct {
	UserID   int
	Username string
	Role     string
	jwt.RegisteredClaims
}

func jwtSecret() []byte {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		s = "dev-secret-change-in-prod"
	}
	return []byte(s)
}

func Login(database *db.DB, username string, password string) (string, error) {
	user, err := database.GetUserByUsername(username)
	if err != nil {
		// vid fel login
		// TODO: se till att hantera sql.ErrNoRows (ingen användare hittades) från riktiga db errors
		// låter de vara så här nu
		//log.Printf("DEBUG: GetUserByUsername error: %v", err)
		return "", fmt.Errorf("invalid credentials")
	}
	//log.Printf("DEBUG: found user: %s hash: %s", user.Username, user.PasswordHash)
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	//log.Printf("DEBUG: bcrypt result: %v", err)
	if err != nil {
		// fel lösen
		return "", fmt.Errorf("invalid credentials")
	}

	claim := Claims{
		UserID:   user.User.ID,
		Username: user.User.Username,
		Role:     user.User.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	tokenString, err := token.SignedString(jwtSecret())
	if err != nil {
		return "", fmt.Errorf("login error: token convertion error")
	}
	return tokenString, nil
}

func tokenFromRequest(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	cookie, err := r.Cookie("token")
	if err != nil {
		return ""
	}
	return cookie.Value

}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := tokenFromRequest(r)
		if tokenStr == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		output_claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr,
			output_claims,
			func(token *jwt.Token) (interface{}, error) {
				// verifiera att token algorithmen är korrekt
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return jwtSecret(), nil
			})
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !token.Valid {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserKey, output_claims)
		next.ServeHTTP(w, r.WithContext(ctx))

	})
}

func GetClaims(r *http.Request) *Claims {
	claims, ok := r.Context().Value(UserKey).(*Claims) // casta till claims då är typ interface
	if !ok {
		return nil
	}
	return claims
}
