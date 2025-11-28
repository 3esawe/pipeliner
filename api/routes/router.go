package routes

import (
	"os"
	"path/filepath"
	"pipeliner/internal/dao"
	"pipeliner/internal/handlers/web"
	"pipeliner/internal/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func InitRouter(db *gorm.DB) *gin.Engine {
	router := gin.Default()
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://127.0.0.1:3000"}
	router.Use(cors.New(config))
	cwd, err := os.Getwd()
	if err != nil {
		panic("failed to get current working directory: " + err.Error())
	}

	staticDir := filepath.Join(cwd, "static")
	scansDir := filepath.Join(cwd, "scans")

	router.Static("/static", staticDir)
	router.Static("/scan-files", scansDir)

	indexWebHandlers := web.NewIndexHandler()
	scanDao := dao.NewScanDAO(db)
	scanService := services.NewScanService(scanDao)
	configService := services.NewConfigService()
	configWebHandlers := web.NewConfigWebHandler(configService)
	scanWebHandler := web.NewScanWebHandler(scanService, configService)

	// REST APIs
	api := router.Group("/api")
	{
		InitScanRoutes(api, db)
		InitConfigRoutes(api, db)
	}

	// web pages
	web := router.Group("/")
	{
		web.GET("/", indexWebHandlers.HomePage)
		web.GET("/config", configWebHandlers.ConfigPage)
		web.GET("/scan/new", scanWebHandler.StartScanPage)
		web.GET("/scans/:id/images", scanWebHandler.ScreenShotsPage)
		web.GET("/scans/:id/subdomains", scanWebHandler.SubdomainsPage)
		web.GET("/scans/:id", scanWebHandler.ScanDetailPage)
		web.GET("/scans", scanWebHandler.ScansPage)
	}

	return router
}
