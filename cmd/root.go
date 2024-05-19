package cmd

import (
	"log/slog"
	"os"

	"github.com/prathoss/hw/internal"
	"github.com/prathoss/hw/pkg"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "hw",
	Short: "Product, availability, booking server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := internal.NewConfigFromEnv()
		if err != nil {
			slog.Error("loading config failed", pkg.Err(err))
			return err
		}
		s, err := internal.NewServer(cfg)
		if err != nil {
			slog.Error("creating server failed", pkg.Err(err))
			return err
		}
		if err := s.Run(); err != nil {
			slog.Error("running server failed", pkg.Err(err))
			return err
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
