package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"myslotmate-backend/internal/controller"
	"myslotmate-backend/internal/firebase"
	"myslotmate-backend/internal/lib/realtime"
)

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

	return r
}
