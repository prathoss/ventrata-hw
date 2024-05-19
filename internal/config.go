package internal

import (
	"os"

	"github.com/prathoss/hw/pkg"
)

type Config struct {
	DatabaseDSN   string
	ServerAddress string
}

func NewConfigFromEnv() (Config, error) {
	serverAddress := os.Getenv("HW_SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = ":8080"
	}
	databaseDSN, err := pkg.ReadRequiredEnv("HW_DATABASE")
	if err != nil {
		return Config{}, err
	}
	return Config{
		DatabaseDSN:   databaseDSN,
		ServerAddress: serverAddress,
	}, nil
}
