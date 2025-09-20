package server

import (
	"pipeliner/api/routes"
	"pipeliner/internal/config"
	"pipeliner/internal/database"

	"github.com/spf13/cobra"
)

func NewServerCommand() *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start the Pipeliner server",
		Long:  `Start the Pipeliner server to manage and run pipelines via a web interface`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.SilenceUsage = true
			cfg := config.LoadConfig()
			database.InitDB(cfg)
			router := routes.InitRouter(database.DB)
			router.Run()
		},
	}

	return serverCmd
}
