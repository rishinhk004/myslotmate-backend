package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"myslotmate-backend/internal/config"
	"myslotmate-backend/internal/controller"
	"myslotmate-backend/internal/db"
	cfgfirebase "myslotmate-backend/internal/firebase"
	"myslotmate-backend/internal/lib/event"
	"myslotmate-backend/internal/lib/identity"
	"myslotmate-backend/internal/lib/payment"
	"myslotmate-backend/internal/lib/payout"
	"myslotmate-backend/internal/lib/realtime"
	"myslotmate-backend/internal/lib/storage"
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

	// Upload service backed by AWS S3 (nil-safe if bucket not configured)
	var uploadService *storage.UploadService
	if cfg.S3.Bucket != "" && cfg.S3.AccessKey != "" && cfg.S3.SecretKey != "" {
		awsCfg, err := awscfg.LoadDefaultConfig(ctx,
			awscfg.WithRegion(cfg.S3.Region),
			awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.S3.AccessKey, cfg.S3.SecretKey, "",
			)),
		)
		if err != nil {
			log.Fatalf("failed to load AWS config: %v", err)
		}
		s3Client := s3.NewFromConfig(awsCfg)
		uploadService = storage.NewUploadService(s3Client, cfg.S3.Bucket, cfg.S3.Region)
		log.Printf("✓ AWS S3 upload service enabled: bucket=%s, region=%s", cfg.S3.Bucket, cfg.S3.Region)
	} else {
		log.Println("✗ WARNING: AWS S3 not fully configured — file uploads DISABLED")
		log.Println("  Set these environment variables to enable uploads:")
		log.Println("    AWS_S3_BUCKET=your-bucket-name")
		log.Println("    AWS_S3_REGION=ap-south-1")
		log.Println("    AWS_ACCESS_KEY_ID=your-access-key")
		log.Println("    AWS_SECRET_ACCESS_KEY=your-secret-key")
	}

	// Repository Pattern: Data Layer
	userRepo := repository.NewUserRepository(dbConn)
	hostRepo := repository.NewHostRepository(dbConn)
	eventRepo := repository.NewEventRepository(dbConn)
	bookingRepo := repository.NewBookingRepository(dbConn)
	reviewRepo := repository.NewReviewRepository(dbConn)
	inboxRepo := repository.NewInboxRepository(dbConn)
	accountRepo := repository.NewAccountRepository(dbConn)
	paymentRepo := repository.NewPaymentRepository(dbConn)
	payoutRepo := repository.NewPayoutRepository(dbConn)
	supportRepo := repository.NewSupportRepository(dbConn)
	savedExpRepo := repository.NewSavedExperienceRepository(dbConn)

	// Strategy Pattern: Identity Provider
	aadharProvider := identity.NewSetuAadharProvider(identity.SetuConfig{
		BaseURL:           cfg.Setu.BaseURL,
		ClientID:          cfg.Setu.ClientID,
		ClientSecret:      cfg.Setu.ClientSecret,
		ProductInstanceID: cfg.Setu.ProductInstanceID,
	})
	log.Println("Using Setu Aadhar Provider")

	// Strategy Pattern: Payout Provider (Cashfree)
	cfClientID := cfg.Cashfree.ClientID
	cfClientSecret := cfg.Cashfree.ClientSecret
	if cfClientID == "" || cfClientSecret == "" {
		log.Println("Warning: Cashfree credentials not configured — payouts disabled (using dummy keys)")
		cfClientID = "cf_dummy_client_id"
		cfClientSecret = "cf_dummy_client_secret"
	}
	payoutProvider := payout.NewCashfreeProvider(payout.CashfreeConfig{
		BaseURL:       cfg.Cashfree.BaseURL,
		ClientID:      cfClientID,
		ClientSecret:  cfClientSecret,
		WebhookSecret: cfg.Cashfree.WebhookSecret,
		APIVersion:    cfg.Cashfree.APIVersion,
	})
	log.Println("Using Cashfree Payout Provider")

	// Strategy Pattern: Payment Collection Provider (Razorpay Standard)
	rzpKeyID := cfg.Razorpay.KeyID
	rzpKeySecret := cfg.Razorpay.KeySecret
	if rzpKeyID == "" || rzpKeySecret == "" {
		log.Println("Warning: Razorpay credentials not configured — payment collection disabled (using dummy keys)")
		rzpKeyID = "rzp_test_dummy"
		rzpKeySecret = "dummy_secret"
	}
	paymentWebhookSecret := cfg.Razorpay.PaymentWebhookSecret
	if paymentWebhookSecret == "" {
		paymentWebhookSecret = cfg.Razorpay.WebhookSecret // fallback to shared secret
	}
	paymentProvider := payment.NewRazorpayProvider(payment.RazorpayConfig{
		KeyID:         rzpKeyID,
		KeySecret:     rzpKeySecret,
		WebhookSecret: paymentWebhookSecret,
	})
	log.Println("Using Razorpay Payment Collection Provider")

	userService := service.NewUserService(userRepo, hostRepo, savedExpRepo, accountRepo, paymentRepo, workerPool, dispatcher, aadharProvider, paymentProvider)
	hostService := service.NewHostService(hostRepo, userRepo, eventRepo, bookingRepo, reviewRepo, payoutRepo, accountRepo, dispatcher)
	eventService := service.NewEventService(eventRepo, bookingRepo, dispatcher)
	bookingService := service.NewBookingService(bookingRepo, eventRepo, accountRepo, paymentRepo, payoutRepo, hostRepo, dispatcher)
	reviewService := service.NewReviewService(reviewRepo, eventRepo, dispatcher)
	inboxService := service.NewInboxService(inboxRepo, eventRepo, socketService)
	supportService := service.NewSupportService(supportRepo)
	payoutService := service.NewPayoutService(payoutRepo, accountRepo, paymentRepo, hostRepo, payoutProvider, dispatcher)

	userController := controller.NewUserController(userService)
	hostController := controller.NewHostController(hostService)
	eventController := controller.NewEventController(eventService)
	bookingController := controller.NewBookingController(bookingService)
	reviewController := controller.NewReviewController(reviewService)
	inboxController := controller.NewInboxController(inboxService)
	payoutController := controller.NewPayoutController(payoutService)
	webhookController := controller.NewWebhookController(payoutService, userService, payoutProvider, paymentProvider)
	supportController := controller.NewSupportController(supportService, uploadService)
	uploadController := controller.NewUploadController(uploadService)
	adminController := controller.NewAdminController(hostService, fbApp.Auth, cfg.AdminEmail)

	router := server.NewRouter(
		fbApp,
		socketService,
		userController,
		hostController,
		eventController,
		bookingController,
		reviewController,
		inboxController,
		payoutController,
		webhookController,
		supportController,
		uploadController,
		adminController,
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

	// Self-ping keep-alive: prevents Render free-tier from spinning down.
	if cfg.RenderExternalURL != "" {
		go keepAlive(ctx, cfg.RenderExternalURL+"/health")
	}

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

// keepAlive pings the given URL every 10 minutes to prevent
// Render free-tier instances from spinning down due to inactivity.
func keepAlive(ctx context.Context, url string) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	client := &http.Client{Timeout: 15 * time.Second}
	log.Printf("Keep-alive enabled: pinging %s every 10m", url)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resp, err := client.Get(url)
			if err != nil {
				log.Printf("keep-alive ping failed: %v", err)
				continue
			}
			resp.Body.Close()
			log.Printf("keep-alive ping: %s", resp.Status)
		}
	}
}
