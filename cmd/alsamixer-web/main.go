package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/user/alsamixer-web/internal/config"
	"github.com/user/alsamixer-web/internal/server"
	"github.com/user/alsamixer-web/internal/sse"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func main() {
	log.Println("alsamixer-web starting...")

	cfg, err := config.Load()
	if err != nil {
		log.Printf("failed to load config: %v", err)
		log.Printf("usage:\n%s", config.HelpText())
		os.Exit(2)
	}

	hub := sse.NewHub()
	go hub.Run()

	srv := server.NewServer(cfg, hub)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- srv.Start()
	}()

	select {
	case sig := <-sigCh:
		log.Printf("received signal %s, shutting down", sig)
	case err := <-serverErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Stop(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	hub.Stop()
	log.Println("alsamixer-web stopped")
}
