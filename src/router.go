package src

import (
	"context"
	"github.com/cbartram/hearthhub-mod-api/src/handler"
	"github.com/cbartram/hearthhub-mod-api/src/handler/cognito"
	"github.com/cbartram/hearthhub-mod-api/src/handler/file"
	"github.com/cbartram/hearthhub-mod-api/src/handler/server"
	"github.com/cbartram/hearthhub-mod-api/src/handler/stripe_handlers"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"log"
	"net/http"
	"os"
)

// NewRouter Create a new gin router and instantiates the routes and route handlers for the entire API.
func NewRouter(ctx context.Context, wrapper *service.Wrapper) (*gin.Engine, *WebSocketManager) {
	logger := logrus.New()
	fmt := &logrus.TextFormatter{
		FullTimestamp: false,
		ForceColors:   true,
	}
	logger.SetFormatter(fmt)
	logrus.SetFormatter(fmt)

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
	r.MaxMultipartMemory = 32 << 20 // 32 MB

	r.Use(CORSMiddleware(), LogrusMiddleware(logger))
	apiGroup := r.Group("/api/v1")
	serverGroup := apiGroup.Group("/server", CORSMiddleware(), AuthMiddleware(wrapper.CognitoService, wrapper.HearthhubDb))
	modGroup := apiGroup.Group("/file", CORSMiddleware(), AuthMiddleware(wrapper.CognitoService, wrapper.HearthhubDb))
	cognitoGroup := apiGroup.Group("/cognito", CORSMiddleware())

	// The connection to RabbitMQ and exchange declaration occurs here.
	wsManager, err := NewWebSocketManager()
	if err != nil {
		logrus.Errorf("error creating websocket manager: %v", err)
	} else {
		// This starts listening to client connect and disconnect go routine channels
		go wsManager.Run()
	}

	r.GET("/api/v1/ws", func(c *gin.Context) {
		logrus.Infof("receive new websocket connection")
		// When a user connects they get their own QueueBind and start sending events to the
		// channels listened to in Run() and listening for messages on their queue.
		wsManager.HandleWebSocket(c)
	})

	apiGroup.GET("/stripe/create-checkout-session", AuthMiddleware(wrapper.CognitoService, wrapper.HearthhubDb), func(c *gin.Context) {
		h := stripe_handlers.CheckoutSessionHandler{}
		h.HandleRequest(c, wrapper.CognitoService)
	})

	apiGroup.GET("/stripe/create-billing-session", AuthMiddleware(wrapper.CognitoService, wrapper.HearthhubDb), func(c *gin.Context) {
		h := stripe_handlers.BillingSessionHandler{}
		h.HandleRequest(c)
	})

	apiGroup.POST("/stripe/webhook", func(c *gin.Context) {
		h := stripe_handlers.WebhookHandler{}
		h.HandleRequest(c, wrapper.RabbitMQService)
	})

	apiGroup.GET("/stripe/subscription", AuthMiddleware(wrapper.CognitoService, wrapper.HearthhubDb), func(c *gin.Context) {
		h := stripe_handlers.GetSubscriptionHandler{}
		h.HandleRequest(c)
	})

	// The health route returns the latest versions for the valheim src and sidecar so users
	// can be alerted when to delete and re-create their servers.
	apiGroup.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"api-version":             os.Getenv("API_VERSION"),
			"valheim-src-version":     os.Getenv("VALHEIM_IMAGE_VERSION"),
			"valheim-sidecar-version": os.Getenv("BACKUP_MANAGER_IMAGE_VERSION"),
			"file-installer-version":  os.Getenv("FILE_MANAGER_IMAGE_VERSION"),
		})
	})

	// The following 2 routes are the only routes that do not require Authorization in the form of a discord id
	// and OAuth refresh token to access.
	apiGroup.POST("/discord/oauth", func(c *gin.Context) {
		h := handler.DiscordRequestHandler{}
		h.HandleRequest(c, wrapper.DiscordService)
	})

	cognitoGroup.POST("/create-user", func(c *gin.Context) {
		h := cognito.CreateUserRequestHandler{}
		h.HandleRequest(c, ctx, wrapper.CognitoService, wrapper.HearthhubDb)
	})

	apiGroup.POST("/support/send-message", AuthMiddleware(wrapper.CognitoService, wrapper.HearthhubDb), func(c *gin.Context) {
		h := handler.SupportHandler{}
		h.HandleRequest(c)
	})

	//  Authorized routes below
	apiGroup.GET("/file", AuthMiddleware(wrapper.CognitoService, wrapper.HearthhubDb), func(c *gin.Context) {
		h := file.FileHandler{}
		h.HandleRequest(c, wrapper.S3Service)
	})

	apiGroup.POST("/file/generate-signed-url", AuthMiddleware(wrapper.CognitoService, wrapper.HearthhubDb), func(c *gin.Context) {
		h := file.UploadFileHandler{}
		h.HandleRequest(c, wrapper.S3Service, wrapper.StripeService)
	})

	cognitoGroup.POST("/auth", AuthMiddleware(wrapper.CognitoService, wrapper.HearthhubDb), func(c *gin.Context) {
		h := cognito.AuthHandler{}
		h.HandleRequest(c, ctx, wrapper)
	})

	cognitoGroup.POST("/refresh-session", AuthMiddleware(wrapper.CognitoService, wrapper.HearthhubDb), func(c *gin.Context) {
		h := cognito.RefreshSessionHandler{}
		h.HandleRequest(c, ctx, wrapper.CognitoService)
	})

	modGroup.POST("/install", func(c *gin.Context) {
		h := file.InstallFileHandler{}
		h.HandleRequest(c, wrapper.KubeService, wrapper.S3Service)
	})

	serverGroup.GET("/", func(c *gin.Context) {
		h := server.GetServerHandler{}
		h.HandleRequest(c, wrapper.HearthhubDb)
	})

	serverGroup.POST("/create", func(c *gin.Context) {
		h := server.CreateServerHandler{}
		h.HandleRequest(c, ctx, wrapper)
	})

	serverGroup.DELETE("/delete", func(c *gin.Context) {
		h := server.DeleteServerHandler{}
		h.HandleRequest(c, wrapper)
	})

	serverGroup.PUT("/update", func(c *gin.Context) {
		h := server.PatchServerHandler{}
		h.HandleRequest(c, ctx, wrapper)
	})

	serverGroup.PUT("/scale", func(c *gin.Context) {
		h := server.ScaleServerHandler{}
		h.HandleRequest(c, wrapper)
	})

	return r, wsManager
}
