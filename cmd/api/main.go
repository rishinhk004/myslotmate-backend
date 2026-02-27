package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"myslotmate-backend/internal/config"
	cfgfirebase "myslotmate-backend/internal/firebase"
	"myslotmate-backend/internal/server"
)

func main() {
	// Load configuration (env vars, etc.)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Root context for the application lifecycle.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Firebase Admin.
	fbApp, err := cfgfirebase.NewApp(ctx, cfgfirebase.Config{
		CredentialsFile: cfg.Firebase.CredentialsFile,
		ProjectID:       cfg.Firebase.ProjectID,
	})
	if err != nil {
		log.Fatalf("failed to initialize firebase app: %v", err)
	}

	// Build HTTP server with all routes and dependencies wired.
	router := server.NewRouter(fbApp)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background.
	go func() {
		log.Printf("HTTP server listening on :%s", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	} else {
		log.Println("server stopped cleanly")
	}
}

