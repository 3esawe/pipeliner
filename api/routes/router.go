package routes

import (
	"pipeliner/internal/dao"
	"pipeliner/internal/handlers"
	"pipeliner/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func InitRouter(db *gorm.DB) *gin.Engine {
	router := gin.Default()
	router.Static("/static", "./static")

	scanDao := dao.NewScanDAO(db)
	scanService := services.NewScanService(scanDao)
	scanWebHandlers := handlers.NewScanHandler(scanService)
	configWebHandlers := handlers.NewConfigHandler(services.NewConfigService())

	// REST APIs
	api := router.Group("/api")
	{
		InitScanRoutes(api, db)
		InitConfigRoutes(api, db)
	}

	// web pages
	web := router.Group("/")
	{
		web.GET("/", scanWebHandlers.HomePage)
		web.GET("/config", configWebHandlers.ConfigPage)
	}

	return router
}
