package main

import (
	"context"
	"os"
	"os/signal"
	"pipeliner/pkg/engine"
	"pipeliner/pkg/tools"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	module  string
	domain  string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "pipeliner",
	Short: "A modular pipeline tool for security scanning",
	Long:  `Pipeliner is a modular tool for running security scanning pipelines with configurable parameters`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose {
			log.SetLevel(log.DebugLevel)
		} else {
			log.SetLevel(log.InfoLevel)
		}
	},
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan using specified pipeline module",
	Long:  `Scan using the specified pipeline module configuration`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle SIGINT and SIGTERM
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			log.Info("Shutting down")
			cancel()
		}()

		engine := engine.NewEngine(ctx)

		// Set options directly
		engine.SetOptions(&tools.Options{
			ScanType: module,
			Domain:   domain,
		})

		errChan := make(chan error, 1)
		go func() {
			errChan <- engine.Run()
		}()

		select {
		case err := <-errChan:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			err := <-errChan
			if err != nil {
				return err
			}
		}

		log.Info("All tools finished execution")
		return nil

	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	// Scan command flags
	scanCmd.Flags().StringVarP(&module, "module", "m", "", "Pipeline module to execute (required)")
	scanCmd.Flags().StringVarP(&domain, "domain", "d", "", "Target domain for scanning")

	// Mark required flags
	scanCmd.MarkFlagRequired("module")

	rootCmd.AddCommand(scanCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
