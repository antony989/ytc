package routes

import (
	"github.com/kurosaki/l1/internal/handlers"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func New() *echo.Echo {
	router := echo.New()
	router.Use(middleware.Logger())
	router.Use(middleware.Recover())
	router.POST("/api/comment", handlers.CommentGetById)
	// 다른거 테스트할 떄 까지 비활성화
	router.POST("/api/addJob", handlers.ResponseJob)
	return router
}
