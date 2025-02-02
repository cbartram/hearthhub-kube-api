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
func NewRouter(ctx context.Context, kubeService service.KubernetesService, cognitoService service.CognitoService) *gin.Engine {
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

	apiGroup := r.Group("/api/v1")
	serverGroup := apiGroup.Group("/server")
	modGroup := apiGroup.Group("/file")

	// Setup middleware
	r.Use(LogrusMiddleware(logger))
	serverGroup.Use(AuthMiddleware(cognitoService))
	modGroup.Use(AuthMiddleware(cognitoService))

	wsManager := NewWebSocketManager()

	go wsManager.Run()
	go wsManager.ConsumeRabbitMQ()

	r.GET("/ws", func(c *gin.Context) {
		tmp, exists := c.Get("user")
		if !exists {
			logrus.Errorf("user not found in context")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
			return
		}

		user := tmp.(*service.CognitoUser)
		wsManager.HandleWebSocket(user, c.Writer, c.Request)
	}, AuthMiddleware(cognitoService))

	// The health route returns the latest versions for the valheim server and sidecar so users
	// can be alerted when to delete and re-create their servers.
	apiGroup.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":                  "OK",
			"valheim-server-version":  os.Getenv("VALHEIM_IMAGE_VERSION"),
			"valheim-sidecar-version": os.Getenv("BACKUP_MANAGER_IMAGE_VERSION"),
		})
	})

	modGroup.POST("/install", func(c *gin.Context) {
		handler := InstallFileHandler{}
		handler.HandleRequest(c, kubeService)
	})

	serverGroup.POST("/create", func(c *gin.Context) {
		handler := CreateServerHandler{}
		handler.HandleRequest(c, kubeService, cognitoService, ctx)
	})

	serverGroup.DELETE("/delete", func(c *gin.Context) {
		handler := DeleteServerHandler{}
		handler.HandleRequest(c, kubeService, cognitoService, ctx)
	})

	serverGroup.PUT("/scale", func(c *gin.Context) {
		handler := ScaleServerHandler{}
		handler.HandleRequest(c, kubeService, cognitoService, ctx)
	})

	return r
}
