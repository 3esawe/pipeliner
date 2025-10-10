package server

import (
	"fmt"
	"os"
	"pipeliner/api/routes"
	"pipeliner/internal/config"
	"pipeliner/internal/database"

	"github.com/spf13/cobra"
)

type ServerOpts struct {
	Port int
	Ip   string
}

func NewServerCommand() *cobra.Command {
	ServerConfig := &ServerOpts{}

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start the Pipeliner server",
		Long:  `Start the Pipeliner server to manage and run pipelines via a web interface`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.SilenceUsage = true
			cfg := config.LoadConfig()
			db, err := database.InitDB(cfg)
			if err != nil {
				cmd.PrintErrf("failed to initialize database: %v\n", err)
				os.Exit(1)
			}
			router := routes.InitRouter(db)
			router.Run(fmt.Sprintf(":%d", ServerConfig.Port))
		},
	}

	serverCmd.Flags().IntVarP(&ServerConfig.Port, "port", "p", 8080, "Port to run the server on")
	serverCmd.Flags().StringVarP(&ServerConfig.Ip, "ip", "i", "localhost", "IP address to bind the server to")

	return serverCmd
}
