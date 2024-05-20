package cmd

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/prathoss/hw/internal"
	"github.com/prathoss/hw/pkg"
	"github.com/spf13/cobra"
)

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Checks health of server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := internal.NewConfigFromEnv()
		logger := slog.With("component", "health")
		if err != nil {
			logger.Error("could not initialize config", pkg.Err(err))
			return err
		}
		client := http.DefaultClient
		resp, err := client.Get(fmt.Sprintf("http://%s/api/v1/health", cfg.ServerAddress))
		if err != nil {
			logger.Error("could not connect to server", pkg.Err(err))
			return err
		}

		if resp.StatusCode > 299 {
			err := fmt.Errorf("server returned not successful status code %s", resp.Status)
			logger.Error("did not receive successful status code", pkg.Err(err))
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
