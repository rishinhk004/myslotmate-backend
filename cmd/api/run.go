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
	"myslotmate-backend/internal/controller"
	"myslotmate-backend/internal/db"
	cfgfirebase "myslotmate-backend/internal/firebase"
	"myslotmate-backend/internal/lib/event"
	"myslotmate-backend/internal/lib/identity"
	"myslotmate-backend/internal/lib/realtime"
	"myslotmate-backend/internal/lib/worker"
	"myslotmate-backend/internal/repository"
	"myslotmate-backend/internal/server"
	"myslotmate-backend/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Singleton Pattern: Database Connection
	dbConn, err := db.Open(cfg.Database.URL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer dbConn.Close()

	if err := dbConn.PingContext(ctx); err != nil {
		log.Printf("Warning: failed to ping database (is it running?): %v", err)
	}

	// Observer Pattern: Event Dispatcher (Singleton)
	dispatcher := event.GetDispatcher()

	socketService, err := realtime.NewSocketService()
	if err != nil {
		log.Printf("Warning: failed to initialize socket service: %v", err)
	} else {
		socketService.Start()
		defer socketService.Close()
	}

	// Executor Pattern: Worker Pool
	workerPool := worker.NewWorkerPool(5, 100)
	workerPool.Start()
	defer workerPool.Stop(context.Background())

	fbApp, err := cfgfirebase.NewApp(ctx, cfgfirebase.Config{
		CredentialsFile: cfg.Firebase.CredentialsFile,
		ProjectID:       cfg.Firebase.ProjectID,
	})
	if err != nil {
		log.Fatalf("failed to initialize firebase app: %v", err)
	}

	// Repository Pattern: Data Layer
	userRepo := repository.NewUserRepository(dbConn)
	hostRepo := repository.NewHostRepository(dbConn)
	eventRepo := repository.NewEventRepository(dbConn)
	bookingRepo := repository.NewBookingRepository(dbConn)
	reviewRepo := repository.NewReviewRepository(dbConn)
	inboxRepo := repository.NewInboxRepository(dbConn)

	// Strategy Pattern: Identity Provider
	aadharProvider := identity.NewSetuAadharProvider(identity.SetuConfig{
		BaseURL:           cfg.Setu.BaseURL,
		ClientID:          cfg.Setu.ClientID,
		ClientSecret:      cfg.Setu.ClientSecret,
		ProductInstanceID: cfg.Setu.ProductInstanceID,
	})
	log.Println("Using Setu Aadhar Provider")

	userService := service.NewUserService(userRepo, workerPool, dispatcher, aadharProvider)
	hostService := service.NewHostService(hostRepo, userRepo, dispatcher)
	eventService := service.NewEventService(eventRepo, dispatcher)
	bookingService := service.NewBookingService(bookingRepo, eventRepo, dispatcher)
	reviewService := service.NewReviewService(reviewRepo, dispatcher)
	inboxService := service.NewInboxService(inboxRepo, eventRepo, socketService)

	userController := controller.NewUserController(userService)
	hostController := controller.NewHostController(hostService)
	eventController := controller.NewEventController(eventService)
	bookingController := controller.NewBookingController(bookingService)
	reviewController := controller.NewReviewController(reviewService)
	inboxController := controller.NewInboxController(inboxService)

	router := server.NewRouter(
		fbApp,
		socketService,
		userController,
		hostController,
		eventController,
		bookingController,
		reviewController,
		inboxController,
	)

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
