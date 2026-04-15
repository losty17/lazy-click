package auth

import (
	"errors"
	"os"
)

type SecretStore interface {
	Get(service string, account string) (string, error)
	Set(service string, account string, value string) error
}

type EnvStore struct{}

func (s EnvStore) Get(service string, account string) (string, error) {
	_ = service
	value := os.Getenv(account)
	if value == "" {
		return "", errors.New("secret not found in environment")
	}
	return value, nil
}

func (s EnvStore) Set(service string, account string, value string) error {
	_ = service
	return os.Setenv(account, value)
}
