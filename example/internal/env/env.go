package env

import "os"

// Or returns the value of the environment variable named by key,
// or fallback if the variable is empty or unset.
func Or(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
