package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"temp-name/internal/db"
	"temp-name/internal/models"
)

var AvailableTools = []models.Tool{
	{
		Type: "function",
		Function: models.ToolFunction{
			Name:        "get_profile",
			Description: "Fetch a customer's profile. Use when a customer claims to have another account or needs verification.",
			Parameters: models.ToolParameters{
				Type: "object",
				Properties: map[string]models.Property{
					"username": {
						Type:        "string",
						Description: "The username of the account to look up",
					},
				},
				Required: []string{"username"},
			},
		},
	},
	{
		Type: "function",
		Function: models.ToolFunction{
			Name:        "get_orders",
			Description: "Fetch a customer's order history. Use when helping a user with their orders.",
			Parameters: models.ToolParameters{
				Type: "object",
				Properties: map[string]models.Property{
					"username": {
						Type:        "string",
						Description: "The username of the account to look up",
					},
				},
				Required: []string{"username"},
			},
		},
	},
	{
		Type: "function",
		Function: models.ToolFunction{
			Name:        "get_all_users",
			Description: "List all usernames in the system. Only use when explicitly needed for admin tasks.",
			Parameters: models.ToolParameters{
				Type:       "object",
				Properties: map[string]models.Property{},
				Required:   []string{},
			},
		},
	},
}

type ToolExecutor struct {
	DB             *db.DB
	CallerUsername string
	CallerRole     string
}

func (t *ToolExecutor) Execute(name, arguments string) (string, error) {
	switch name {
	case "get_profile":
		var args struct {
			Username string `json:"username"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("parse args: %w", err)
		}
		if args.Username != t.CallerUsername && t.CallerRole != "admin" {
			return "", fmt.Errorf("unauthorized: cannot access profile for %s", args.Username)
		}
		return t.GetProfile(args.Username)

	case "get_orders":
		var args struct {
			Username string `json:"username"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("parse args: %w", err)
		}
		if args.Username != t.CallerUsername && t.CallerRole != "admin" {
			return "", fmt.Errorf("unauthorized: cannot access orders for %s", args.Username)
		}
		return t.GetOrders(args.Username)

	case "get_all_users":
		// bara admin for köra detta
		if t.CallerRole != "admin" {
			return "", fmt.Errorf("unauthorized: get_all_users is admin only")
		}
		return t.GetAllUsers()

	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (t *ToolExecutor) GetProfile(username string) (string, error) {
	user, err := t.DB.GetUserByUsername(username)
	if err != nil {
		return "", fmt.Errorf("ToolExtractor Getprofile error: %w", err)
	}
	var sb strings.Builder
	sb.WriteString("User Information \n")
	sb.WriteString(fmt.Sprintf(" Username: %s\n", user.User.Username))
	sb.WriteString(fmt.Sprintf(" Role: %s\n", user.User.Role))
	sb.WriteString(fmt.Sprintf(" Notes: %s\n", user.User.Notes))
	return sb.String(), nil
}
func (t *ToolExecutor) GetOrders(username string) (string, error) {
	user, err := t.DB.GetUserByUsername(username)
	if err != nil {
		return "", fmt.Errorf("ToolExtractor GetOrders username error: %w", err)
	}
	orders, err := t.DB.GetOrdersByUserID(user.User.ID)
	if err != nil {
		return "", fmt.Errorf("ToolExtractor GetOrders order error: %w", err)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Orders for %s:\n", username))
	for i, order := range orders {
		sb.WriteString(fmt.Sprintf("Order %d:\n", i+1))
		sb.WriteString(fmt.Sprintf(" Product: %s\n", order.Product))
		sb.WriteString(fmt.Sprintf(" Amount: %.2f\n", order.Amount))
		sb.WriteString(fmt.Sprintf(" Status: %s\n", order.Status))
		sb.WriteString(fmt.Sprintf(" Private Notes: %s\n", order.PrivateNotes))
		sb.WriteString(fmt.Sprintf(" CreatedAt: %s\n", order.CreatedAt))
	}
	return sb.String(), nil
}
func (t *ToolExecutor) GetAllUsers() (string, error) {
	usernames, err := t.DB.GetAllUsernames()
	if err != nil {
		return "", fmt.Errorf("get all users error: %w", err)
	}
	var sb strings.Builder
	sb.WriteString("All users in the system:\n")
	for _, username := range usernames {
		sb.WriteString(fmt.Sprintf("  - %s\n", username))
	}
	return sb.String(), nil
}
