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
func NewRouter(ctx context.Context, kubeService service.KubernetesService, cognitoService service.CognitoService) (*gin.Engine, *WebSocketManager) {
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

	r.Use(CORSMiddleware(), LogrusMiddleware(logger))
	apiGroup := r.Group("/api/v1")
	serverGroup := apiGroup.Group("/server", CORSMiddleware(), AuthMiddleware(cognitoService))
	modGroup := apiGroup.Group("/file", CORSMiddleware(), AuthMiddleware(cognitoService))

	// The connection to RabbitMQ and exchange declaration occurs here.
	wsManager, err := NewWebSocketManager()
	if err != nil {
		logrus.Errorf("error creating websocket manager: %v", err)
	} else {
		// This starts listening to client connect and disconnect go routine channels
		go wsManager.Run()
	}

	r.GET("/ws", func(c *gin.Context) {
		logrus.Infof("receive new websocket connection")
		// When a user connects they get their own QueueBind and start sending events to the
		// channels listened to in Run() and listening for messages on their queue.
		wsManager.HandleWebSocket(c)
	})

	// The health route returns the latest versions for the valheim server and sidecar so users
	// can be alerted when to delete and re-create their servers.
	apiGroup.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"api-version":             os.Getenv("API_VERSION"),
			"valheim-server-version":  os.Getenv("VALHEIM_IMAGE_VERSION"),
			"valheim-sidecar-version": os.Getenv("BACKUP_MANAGER_IMAGE_VERSION"),
			"file-installer-version":  os.Getenv("FILE_MANAGER_IMAGE_VERSION"),
		})
	})

	modGroup.POST("/install", func(c *gin.Context) {
		handler := InstallFileHandler{}
		handler.HandleRequest(c, kubeService)
	})

	serverGroup.GET("/", func(c *gin.Context) {
		handler := GetServerHandler{}
		handler.HandleRequest(c, cognitoService, ctx)
	})

	serverGroup.POST("/create", func(c *gin.Context) {
		handler := CreateServerHandler{}
		handler.HandleRequest(c, kubeService, cognitoService, ctx)
	})

	serverGroup.DELETE("/delete", func(c *gin.Context) {
		handler := DeleteServerHandler{}
		handler.HandleRequest(c, kubeService, cognitoService, ctx)
	})

	serverGroup.PUT("/update", func(c *gin.Context) {
		handler := PatchServerHandler{}
		handler.HandleRequest(c, kubeService, cognitoService, ctx)
	})

	serverGroup.PUT("/scale", func(c *gin.Context) {
		handler := ScaleServerHandler{}
		handler.HandleRequest(c, kubeService, cognitoService, ctx)
	})

	return r, wsManager
}
