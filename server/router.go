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

func LogrusMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path != "/api/v1/health" {
			logger.WithFields(logrus.Fields{
				"user-agent": c.Request.UserAgent(),
				"error":      c.Errors.ByType(gin.ErrorTypePrivate).String(),
			}).Infof("[%s] %s: ", c.Request.Method, c.Request.URL.Path)
		}
		c.Next()
	}
}

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

	r.Use(LogrusMiddleware(logger))

	kubeService := service.MakeKubernetesService()

	basicAuth := gin.BasicAuth(gin.Accounts{
		"hearthhub": os.Getenv("BASIC_AUTH_PASSWORD"),
	})

	apiGroup := r.Group("/api/v1")
	serverGroup := apiGroup.Group("/server", basicAuth)
	modGroup := apiGroup.Group("/file", basicAuth)

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
