package routes

import (
	"pipeliner/internal/handlers"
	"pipeliner/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func InitConfigRoutes(router *gin.RouterGroup, db *gorm.DB) {
	configService := services.NewConfigService()
	handlers := handlers.NewConfigHandler(configService)

	configRoutes := router.Group("/config")
	{
		configRoutes.GET("", handlers.GetScanModules)
	}
}
