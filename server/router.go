package server

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("could not create in cluster config. Attempting to load local kube config: %v", err.Error())
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			log.Fatalf("could not load local kubernetes config: %v", err.Error())
		}
		log.Printf("local kube config loaded successfully")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating kubernetes client: %v", err)
	}

	apiGroup := r.Group("/api/v1")
	serverGroup := apiGroup.Group("/server")
	modGroup := apiGroup.Group("/file")

	apiGroup.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "OK",
		})
	})

	modGroup.POST("/install", func(c *gin.Context) {
		handler := InstallFileHandler{}
		handler.HandleRequest(c, clientset, ctx)
	})

	serverGroup.POST("/create", func(c *gin.Context) {
		handler := CreateServerHandler{}
		handler.HandleRequest(c, clientset, ctx)
	})

	serverGroup.PUT("/scale", func(c *gin.Context) {
		handler := ScaleServerHandler{}
		handler.HandleRequest(c, clientset, ctx)
	})

	return r
}
