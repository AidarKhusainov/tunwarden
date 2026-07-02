package cli

func emptyAs(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
