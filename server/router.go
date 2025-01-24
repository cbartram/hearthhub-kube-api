package server

import (
	"context"
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

	apiGroup := r.Group("/api/v1")
	serverGroup := apiGroup.Group("/server")

	apiGroup.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "OK",
		})
	})

	serverGroup.POST("/create", func(c *gin.Context) {
		handler := CreateServerHandler{}
		handler.HandleRequest(c, ctx)
	})

	logger.Infof("Server listening on port: 8080")
	return r
}
