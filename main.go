package main

import (
	"context"
	"github.com/cbartram/hearthhub-mod-api/server"
	"log"
)

func main() {
	err := server.NewRouter(context.Background()).Run(":8080")
	if err != nil {
		log.Fatal(err)
	}
}
