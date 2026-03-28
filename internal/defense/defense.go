package defense

type Config struct {
	UseStrongPrompt bool
	InputFilter     bool
	OutputFilter    bool
}

func BuildPrompt(dbContext string, cfg Config) string {
	if cfg.UseStrongPrompt {
		return buildStrong(dbContext)
	}
	return buildWeak(dbContext)
}

func buildWeak(dbContext string) string {
	return `You are a customer support assistant.
	The following is the current customer's data fetched from our database.
	Use this data to answer their questions directly,do not say you cannot access a database.

	` + dbContext
}

func buildStrong(dbContext string) string {
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
