package main

import (
	"context"
	"pipeliner/cmd/pipeliner/scan"
	"pipeliner/cmd/pipeliner/server"

	"github.com/spf13/cobra"
)

func Execute() error {
	var rootCmd = &cobra.Command{
		Use:   "pipeliner",
		Short: "A modular pipeline tool for security scanning",
		Long:  `Pipeliner is a modular tool for running security scanning pipelines with configurable parameters`,
	}

	// Initialize hooks
	scan.InitHooks()

	// Add commands
	rootCmd.AddCommand(scan.NewScanCommand())
	rootCmd.AddCommand(scan.NewListConfigsCommand())
	rootCmd.AddCommand(scan.NewListHooksCommand())
	rootCmd.AddCommand(server.NewServerCommand())
	return rootCmd.ExecuteContext(context.Background())
}
