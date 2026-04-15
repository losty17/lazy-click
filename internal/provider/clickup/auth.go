package clickup

import (
	"errors"
	"os"
)

const TokenEnvVar = "CLICKUP_API_TOKEN"

func TokenFromEnv() (string, error) {
	token := os.Getenv(TokenEnvVar)
	if token == "" {
		return "", errors.New("CLICKUP_API_TOKEN is not set")
	}
	return token, nil
}
