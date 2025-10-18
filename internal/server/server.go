package server

import (
	"fmt"
	"net/http"
	"time"
)

type HdaAndApi struct{}

func HttpServer(port int) *http.Server {
	hdaAndApi := &HdaAndApi{}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      hdaAndApi.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
