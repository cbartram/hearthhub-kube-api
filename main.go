package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/cbartram/hearthhub-mod-api/server"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/joho/godotenv"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
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

	kubeService := service.MakeKubernetesService(kubeConfig)
	cognitoService := service.MakeCognitoService(cfg)

	err = server.NewRouter(context.Background(), kubeService, cognitoService).Run(fmt.Sprintf(":%v", *port))
	if err != nil {
		log.Fatal("failed to run http server: " + err.Error())
	}
	log.Printf("Server Listening on port %s", *port)
}
