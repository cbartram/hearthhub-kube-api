package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/cbartram/hearthhub-common/model"
	"github.com/cbartram/hearthhub-mod-api/src"
	"github.com/cbartram/hearthhub-mod-api/src/handler/stripe_handlers"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"time"
)

func main() {
	ginMode := os.Getenv("GIN_MODE")

	port := flag.String("port", "8080", "port to listen on")
	flag.Parse()

	// Running locally
	if ginMode == "" {
		err := godotenv.Load()
		if err != nil {
			log.Fatalf(fmt.Sprintf("error loading .env file: %v", err))
		}
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("error loading default aws config: %s", err)
	}

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("could not create in cluster config. Attempting to load local kube config: %v", err.Error())
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			log.Fatalf("could not load local kubernetes config: %v", err.Error())
		}
		log.Printf("local kube config loaded successfully")
	}

	db := model.Connect()
	stripeService := service.MakeStripeService()
	kubeService := service.MakeKubernetesService(kubeConfig)
	cognitoService := service.MakeCognitoService(cfg)
	discordService, err := service.MakeDiscordService()
	if err != nil {
		log.Fatalf("failed to make discord service: %v", err)
	}
	s3Service, err := service.MakeS3Service("us-east-1")
	if err != nil {
		logrus.Fatalf("failed to create S3 service: %v", err)
	}
	rabbitMqService, err := service.MakeRabbitMQService("stripe-webhooks", "stripe-webhooks")
	if err != nil {
		logrus.Fatalf("failed to make rabbitmq service: %v", err)
	}

	// Registers a new go routine listening to the stripe-webhooks channel. New messages are enqueued when the /api/v1/stripe/webhook
	// endpoint is called and this function consumes the messages with a 5-second delay in between each message resolving eventual consistency
	// issues with both cognito and stripe when many events are sent at checkout.
	err = rabbitMqService.RegisterConsumer(stripe_handlers.ConsumeMessageWithDelay, 3*time.Second, db)
	if err != nil {
		logrus.Errorf("failed to register stripe webhook message consumer: %v", err)
	}

	router, wsManager := src.NewRouter(context.Background(), &service.Wrapper{
		DiscordService:  discordService,
		S3Service:       s3Service,
		CognitoService:  cognitoService,
		KubeService:     kubeService,
		StripeService:   stripeService,
		RabbitMQService: rabbitMqService,
		HearthhubDb:     db,
	})

	defer func() {
		logrus.Infof("Closing websocket connection and channel")
		wsManager.Connection.Close()
		wsManager.Channel.Close()
		rabbitMqService.Close()
	}()

	log.Printf("Server Listening on port %s", *port)
	err = router.Run(fmt.Sprintf(":%v", *port))
	if err != nil {
		log.Fatal("failed to run http server: " + err.Error())
	}
}
