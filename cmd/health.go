package cmd

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Checks health of server",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := http.DefaultClient
		resp, err := client.Get("http://localhost:8080/api/v1/health")
		if err != nil {
			return err
		}

		if resp.StatusCode > 299 {
			return fmt.Errorf("server returned not successfull status code %s", resp.Status)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
