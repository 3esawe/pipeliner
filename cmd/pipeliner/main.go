package main

import (
	"context"
	"os"
	"os/signal"
	"pipeliner/internal/notification"
	"pipeliner/pkg/engine"
	hooks "pipeliner/pkg/hooks"
	tools "pipeliner/pkg/tools"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	module        string
	domain        string
	verbose       bool
	discordClient *notification.NotificationClient
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

		engine := engine.NewPiplinerEngine(ctx,
			engine.WithPeriodic(1),
			engine.WithNotificationClient(discordClient))

		// Set options directly
		engine.PrepareScan(&tools.Options{
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
				log.Errorf("Main error: %v", err)
				return err
			}
		case <-ctx.Done():
			err := <-errChan
			if err != nil {
				log.Errorf("Main error: %v", err)
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
	initHooks()
	// Initialize Discord client
	var discordClient *notification.NotificationClient
	if token := os.Getenv("DISCORD_TOKEN"); token != "" {
		var err error
		discordClient, err = notification.NewNotificationClient()
		if err != nil {
			log.Warnf("Failed to initialize Discord client: %v", err)
		} else {
			defer discordClient.Close()
			log.Info("Discord notifications enabled")
		}
	} else {
		log.Info("DISCORD_TOKEN not set - Discord notifications disabled")
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func initHooks() {
	tools.RegisterHookForStage(tools.StageSubdomain, &hooks.CombineOutput{})
	tools.RegisterHookForStage(tools.StageVuln, &hooks.NotifierHook{
		Config: hooks.NotifierHookConfig{
			Filename: "nuclei_output.txt",
		},
	})
}
