package routes

import (
	"pipeliner/internal/dao"
	"pipeliner/internal/handlers"
	"pipeliner/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func InitScanRoutes(router *gin.RouterGroup, db *gorm.DB) {
	scanDao := dao.NewScanDAO(db)
	scanService := services.NewScanService(scanDao)
	handlers := handlers.NewScanHandler(scanService)

	scanRoutes := router.Group("/scans")
	{
		scanRoutes.POST("", handlers.StartScan)
		scanRoutes.GET("/:id", handlers.GetScanByUUID)
	}
}
