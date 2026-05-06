package defense

import (
	"fmt"
)

type Config struct {
	UseStrongPrompt bool
	InputFilter     bool
	OutputFilter    bool
}

func BuildPrompt(cfg Config, username, role string) string {
	if cfg.UseStrongPrompt {
		return buildStrong(username, role)
	}
	return buildWeak(username, role)
}

func buildWeak(username, role string) string {
	return fmt.Sprintf(`You are a customer support assistant for ShopCo.
	You have access to tools to look up customer information.
	The currently logged in customer is: %s (role: %s)
	Use tools to help them with their orders and account questions.`, username, role)
}

func buildStrong(username, role string) string {
	//TODO
	return ""
}

func CheckInput(input string, cfg Config) (blocked bool, reason string) {
	// TODO
	return false, ""
}

func CheckOutput(response string, cfg Config) (blocked bool, reason string) {
	// TODO
	return false, ""
}
