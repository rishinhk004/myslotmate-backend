package realtime

import (
	"log"

	socketio "github.com/googollee/go-socket.io"
)

// SocketService encapsulates the socket server logic (Singleton/Facade)
type SocketService struct {
	server *socketio.Server
}

// NewSocketService initializes the socket.io server
func NewSocketService() (*SocketService, error) {
	server := socketio.NewServer(nil)

	// Event: Connection
	server.OnConnect("/", func(s socketio.Conn) error {
		s.SetContext("")
		log.Println("connected:", s.ID())
		return nil
	})

	// Event: Join Room (e.g., "event_123" or "user_456")
	server.OnEvent("/", "join_room", func(s socketio.Conn, room string) {
		s.Join(room)
		log.Printf("Socket %s joined room %s", s.ID(), room)
	})

	// Event: Disconnect
	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		log.Println("closed", reason)
	})

	// Start server in background? No, usually started by ServeHTTP or run method.
	// We'll run server.Serve() in a goroutine in main.

	return &SocketService{server: server}, nil
}

func (s *SocketService) GetServer() *socketio.Server {
	return s.server
}

// BroadcastToRoom sends an event with data to a specific room
func (s *SocketService) BroadcastToRoom(room, eventName string, data interface{}) {
	if s.server != nil {
		s.server.BroadcastToRoom("/", room, eventName, data)
	}
}

// Start runs the socket server processing loop
func (s *SocketService) Start() {
	go func() {
		if err := s.server.Serve(); err != nil {
			log.Fatalf("socketio listen error: %s\n", err)
		}
	}()
}

// Close stops the server
func (s *SocketService) Close() {
	if s.server != nil {
		s.server.Close()
	}
}
