package main

import (
	"log"
	"os"
	"strconv"

	"github.com/effiware/goth-template/internal/server"

	_ "github.com/joho/godotenv/autoload"
)

// main is the entry point of the application.
func main() {
	port, err := strconv.Atoi(os.Getenv("APP_PORT"))
	if err != nil {
		log.Fatal("APP_PORT env variable not set correctly")
	}

	httpServer := server.HttpServer(port)
	log.Printf("Running server on %s", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
