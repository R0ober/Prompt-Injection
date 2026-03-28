package main

import (
	"log"
	"net/http"
	"os"
	"temp-name/internal/api"
	"temp-name/internal/db"
	"time"

	"github.com/joho/godotenv"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {

	// Load .env file into environment variables
	// WARNING: never use godotenv in production — set real env vars on the server instead
	// .env file should be in .gitignore to avoid leaking secrets
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from environment directly")
	}
	log.Printf("DEBUG api key starts with: %s", os.Getenv("OPENROUTER_API_KEY")[:8])
	port := ":" + getEnv("PORT", "8080")
	dbURL := getEnv("DATABASEURL", "postgres://postgres:postgres@localhost:5432/injectionlab?sslmode=disable")
	var database *db.DB
	var err error
	backoff := 0
	for {
		database, err = db.Connect(dbURL)
		if err == nil {
			defer database.Close()
			break
		}
		if backoff >= 4 {
			log.Fatalf("could not connect after 5 attempts: %v", err)
		}
		wait := time.Duration(1<<backoff) * time.Second
		log.Printf("database not ready, retry in %v...(%v)", wait, err)
		time.Sleep(wait)

		backoff++
	}
	err = database.Migrate()
	if err != nil {
		log.Fatalf("database migrate error: %v", err)
	}
	err = database.Seed()
	if err != nil {
		log.Fatalf("seed error: %v", err)
	}

	router := api.NewRouter(database)
	log.Printf("Sucess: running on: %v with db: %v", port, dbURL)
	log.Fatal(http.ListenAndServe(port, router))

}
