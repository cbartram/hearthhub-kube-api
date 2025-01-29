package server

import (
	"context"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"log"
	"net/http"
	"os"
)

// NewRouter Create a new gin router and instantiates the routes and route handlers for the entire API.
func NewRouter(ctx context.Context) *gin.Engine {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: false,
	})

	logLevel, err := logrus.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = logrus.InfoLevel
	}

	log.SetOutput(os.Stdout)
	logrus.SetLevel(logLevel)

	r := gin.New()

	gin.DefaultWriter = logger.Writer()
	gin.DefaultErrorWriter = logger.Writer()
	gin.SetMode(gin.ReleaseMode)

	kubeService := service.MakeKubernetesService()
	cognitoService := service.MakeCognitoService()

	apiGroup := r.Group("/api/v1")
	serverGroup := apiGroup.Group("/server")
	modGroup := apiGroup.Group("/file")

	// Setup middleware
	r.Use(LogrusMiddleware(logger))
	serverGroup.Use(AuthMiddleware(cognitoService))
	modGroup.Use(AuthMiddleware(cognitoService))

	apiGroup.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "OK",
		})
	})

	modGroup.POST("/install", func(c *gin.Context) {
		handler := InstallFileHandler{}
		handler.HandleRequest(c, kubeService, ctx)
	})

	serverGroup.POST("/create", func(c *gin.Context) {
		handler := CreateServerHandler{}
		handler.HandleRequest(c, kubeService, ctx)
	})

	serverGroup.PUT("/scale", func(c *gin.Context) {
		handler := ScaleServerHandler{}
		handler.HandleRequest(c, kubeService, ctx)
	})

	return r
}
