package routes

import (
	"pipeliner/internal/dao"
	"pipeliner/internal/handlers/web"
	"pipeliner/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func InitRouter(db *gorm.DB) *gin.Engine {
	router := gin.Default()
	router.Static("/static", "./static")

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
		web.GET("/scans", scanWebHandler.ScansPage)
		web.GET("/scans/:id", scanWebHandler.ScanDetailPage)
	}

	return router
}
