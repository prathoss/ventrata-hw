package pkg

import (
	"fmt"
	"os"
)

func ReadRequiredEnv(envName string) (string, error) {
	val := os.Getenv(envName)
	if val == "" {
		return "", MissingRequiredEnvError{EnvName: envName}
	}
	return val, nil
}

var _ error = MissingRequiredEnvError{}

type MissingRequiredEnvError struct {
	EnvName string
}

func (m MissingRequiredEnvError) Error() string {
	return fmt.Sprintf("Required environment variable %s is missing", m.EnvName)
}
