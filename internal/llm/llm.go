package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"temp-name/internal/models"
	"time"
)

const (
	ModelGemma = "google/gemma-3-12b-it"
	ModelLlama = "meta-llama/llama-3.3-70b-instruct"
	ModelQwen  = "qwen/qwen3-8b"
)

type ChatRequest struct {
	Model    string           `json:"model"`
	Messages []models.Message `json:"messages"`
	Tools    []models.Tool    `json:"tools,omitempty"`
}

type ChatResponse struct {
	Choices []struct {
		Message models.Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func Chat(model string, messages []models.Message, availableTools []models.Tool) (models.Message, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return models.Message{}, fmt.Errorf("OPENROUTER_API_KEY not set")
	}

	payload := ChatRequest{
		Model:    model,
		Messages: messages,
		Tools:    availableTools,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return models.Message{}, fmt.Errorf("encode error: %w", err)
	}

	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return models.Message{}, fmt.Errorf("request error: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "http://localhost:8080")
	req.Header.Set("X-Title", "Injection Lab")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return models.Message{}, fmt.Errorf("response error: %w", err)
	}
	defer resp.Body.Close()

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return models.Message{}, fmt.Errorf("decode error: %w", err)
	}

	if result.Error != nil {
		return models.Message{}, fmt.Errorf("openrouter error: %s", result.Error.Message)
	}

	// fick vi någon response?
	if len(result.Choices) == 0 {
		return models.Message{}, fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message, nil
}
