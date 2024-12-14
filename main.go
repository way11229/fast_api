package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/way11229/fast_api/services"
)

const (
	USER_FILES_PATH = "/user_files"
	SERVER_PORT     = ":80"
)

func main() {
	server := services.NewUserFileServer(SERVER_PORT, USER_FILES_PATH)
	server.Start()

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// <-- Uncomment and import "os/signal" if you want to use signal handling
	<-quit // Block and wait for shutdown signal

	server.Stop()
}
