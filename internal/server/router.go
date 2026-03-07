package server

import (
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"myslotmate-backend/internal/controller"
	"myslotmate-backend/internal/firebase"
	"myslotmate-backend/internal/lib/realtime"
)

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8"/>
  <title>MySlotMate API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"/>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url:"/swagger.yaml",
      dom_id:"#swagger-ui",
      presets:[SwaggerUIBundle.presets.apis,SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout:"BaseLayout"
    });
  </script>
</body>
</html>`

// NewRouter builds the HTTP router with all routes and middleware wired.
func NewRouter(
	fbApp *firebase.App,
	socketService *realtime.SocketService,
	userCtrl *controller.UserController,
	hostCtrl *controller.HostController,
	eventCtrl *controller.EventController,
	bookingCtrl *controller.BookingController,
	reviewCtrl *controller.ReviewController,
	inboxCtrl *controller.InboxController,
	payoutCtrl *controller.PayoutController,
	webhookCtrl *controller.WebhookController,
	supportCtrl *controller.SupportController,
	uploadCtrl *controller.UploadController,
	adminCtrl *controller.AdminController,
) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Swagger UI & spec
	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(swaggerHTML))
	})
	r.Get("/swagger.yaml", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile("swagger.yaml")
		if err != nil {
			http.Error(w, "swagger.yaml not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		_, _ = w.Write(data)
	})

	if socketService != nil {
		r.Handle("/socket.io/*", socketService.GetServer())
	}

	if userCtrl != nil {
		userCtrl.RegisterRoutes(r)
	}

	if hostCtrl != nil {
		hostCtrl.RegisterRoutes(r)
	}

	if eventCtrl != nil {
		eventCtrl.RegisterRoutes(r)
	}

	if bookingCtrl != nil {
		bookingCtrl.RegisterRoutes(r)
	}

	if reviewCtrl != nil {
		reviewCtrl.RegisterRoutes(r)
	}

	if inboxCtrl != nil {
		inboxCtrl.RegisterRoutes(r)
	}

	if payoutCtrl != nil {
		payoutCtrl.RegisterRoutes(r)
	}

	if webhookCtrl != nil {
		webhookCtrl.RegisterRoutes(r)
	}

	if supportCtrl != nil {
		supportCtrl.RegisterRoutes(r)
	}

	if uploadCtrl != nil {
		uploadCtrl.RegisterRoutes(r)
	}

	if adminCtrl != nil {
		adminCtrl.RegisterRoutes(r)
	}

	return r
}
