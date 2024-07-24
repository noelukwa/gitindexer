package api

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/noelukwa/indexer/internal/manager"
	"github.com/noelukwa/indexer/internal/manager/api/handlers"
)

func SetupRoutes(managerService *manager.Service, e *echo.Echo) *echo.Echo {

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "SAMEORIGIN",
		HSTSMaxAge:            31536000,
		HSTSExcludeSubdomains: true,
	}))

	intentHandler := handlers.NewIntentHandler(managerService)

	e.POST("/intents", intentHandler.CreateIntent)
	e.PUT("/intents/:id", intentHandler.UpdateIntent)
	e.GET("/intents/:id", intentHandler.FetchIntent)
	e.GET("/intents", intentHandler.FetchIntents)

	remoteRepoHandler := handlers.NewRemoteRepositoryHandler(managerService)
	e.GET("/repos/:name", remoteRepoHandler.FetchRepoInfo)
	e.GET("/repos/:name/committers", remoteRepoHandler.FetchTopCommitters)
	return e
}
