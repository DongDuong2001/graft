package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/DongDuong2001/graft/internal/app"
	"github.com/DongDuong2001/graft/internal/version"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "graft",
		Short: "Graft is a lightweight self-hosted webhook bridge",
		Run: func(cmd *cobra.Command, args []string) {
			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			slog.SetDefault(logger)

			slog.Info("Starting Graft Webhook Bridge...", "version", version.Version)
			if err := app.Run(); err != nil {
				slog.Error("Application failed", "error", err)
				os.Exit(1)
			}
		},
	}

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the current version of Graft",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Graft CLI    %s\n", version.Version)
			fmt.Printf("Commit Hash: %s\n", version.Commit)
			fmt.Printf("Build Date:  %s\n", version.BuildDate)
		},
	}

	var docsCmd = &cobra.Command{
		Use:   "docs [output-dir]",
		Short: "Generate Markdown documentation for the CLI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			if err := doc.GenMarkdownTree(rootCmd, dir); err != nil {
				return err
			}
			fmt.Printf("Documentation generated in: %s\n", dir)
			return nil
		},
	}

	rootCmd.AddCommand(versionCmd, docsCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
