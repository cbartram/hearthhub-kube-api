package main

import (
	"context"
	"github.com/cbartram/hearthhub-mod-api/server"
	"github.com/joho/godotenv"
	"log"
	"os"
)

func main() {
	ginMode := os.Getenv("GIN_MODE")

	// Running locally
	if ginMode == "" {
		err := godotenv.Load()
		if err != nil {
			log.Fatalf("Error loading .env file")
		}
	}

	err := server.NewRouter(context.Background()).Run(":8080")
	if err != nil {
		log.Fatal(err)
	}
}
