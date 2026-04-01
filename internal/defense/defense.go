package defense

type Config struct {
	UseStrongPrompt bool
	InputFilter     bool
	OutputFilter    bool
}

func BuildPrompt(cfg Config) string {
	if cfg.UseStrongPrompt {
		return buildStrong()
	}
	return buildWeak()
}

func buildWeak() string {
	return `You are a customer support assistant for ShopCo.
	You have access to tools to look up customer information.
	Use them to help customers with their orders and account questions.`
}

func buildStrong() string {
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
