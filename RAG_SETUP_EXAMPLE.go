package main

// ============================================================================
// EXAMPLE: How to Initialize RAG Engine in main.go
// ============================================================================
// This file shows the setup pattern for integrating the Go RAG chatbot
// into your existing MySlotMate backend.

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"myslotmate-backend/internal/controller"
	"myslotmate-backend/internal/lib/rag"
	"myslotmate-backend/internal/server"

	"github.com/jackc/pgx/v5"
)

// Example function to initialize RAG chatbot
func setupRAGEngine(dbConn *pgx.Conn) *controller.RAGChatController {
	// Get credentials from environment
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		log.Fatal("OPENAI_API_KEY not set in environment")
	}

	pineconeKey := os.Getenv("PINECONE_API_KEY")
	if pineconeKey == "" {
		log.Fatal("PINECONE_API_KEY not set in environment")
	}

	indexName := os.Getenv("PINECONE_INDEX_NAME")
	if indexName == "" {
		indexName = "myslotmate-rag"
	}

	// Initialize RAG engine
	ragEngine := rag.NewRAGEngine(
		dbConn,
		openaiKey,   // OpenAI API key
		pineconeKey, // Pinecone API key
		indexName,   // Pinecone index name
		"us-east-1", // Pinecone environment
		4,           // topK - number of chunks to retrieve
		10,          // maxMemory - conversation history size
	)

	// Return controller to be used in router
	return controller.NewRAGChatController(ragEngine)
}

// Example: How to use in your main() function
func exampleMain() {
	// Assuming you already have:
	// - Database connection: dbConn (*pgx.Conn)
	// - Firebase app: fbApp (*firebase.App)
	// - Other controllers already initialized

	// Near the end of server setup, add:
	ragChatCtrl := setupRAGEngine(dbConn)

	// Then pass to NewRouter (along with other existing controllers):
	router := server.NewRouter(
		fbApp,
		socketService,
		userCtrl,
		hostCtrl,
		eventCtrl,
		bookingCtrl,
		reviewCtrl,
		inboxCtrl,
		payoutCtrl,
		webhookCtrl,
		supportCtrl,
		uploadCtrl,
		adminCtrl,
		blogCtrl,
		ragChatCtrl, // ← Add this line
	)

	// Start server
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "5000"
	}

	log.Printf("🚀 Server starting on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

// ============================================================================
// INITIALIZATION CHECKLIST
// ============================================================================
/*

Before running, ensure:

1. ✓ Environment Variables Set
   - OPENAI_API_KEY=sk-...
   - PINECONE_API_KEY=pcsk_...
   - PINECONE_INDEX_NAME=myslotmate-rag
   - DATABASE_URL=postgresql://...

2. ✓ Go Dependencies Installed
   - Run: go get github.com/jackc/pgx/v5
   - Dependencies already in go.mod for other packages

3. ✓ Pinecone Index Created
   - Go to https://console.pinecone.io
   - Create index: myslotmate-rag
   - Dimension: 1536 (OpenAI text-embedding-3-small)
   - Metric: cosine
   - Region: us-east-1 (or your preferred)

4. ✓ Data Ingested
   - Call POST /api/admin/rag/ingest
   - OR run manually at startup:
     ragEngine := setupRAGEngine(dbConn)
     stats, err := ragEngine.IngestData(context.Background())
     if err != nil {
       log.Fatal(err)
     }
     log.Printf("Ingestion stats: %+v\n", stats)

5. ✓ Router Updated
   - NewRouter now accepts ragChatCtrl parameter
   - Routes automatically registered:
     - POST /api/chat/rag
     - POST /api/chat/rag/clear
     - POST /api/chat/rag/delete
     - POST /api/admin/rag/ingest

*/

// ============================================================================
// OPTIONAL: Manual Data Ingestion at Startup
// ============================================================================
func ingestDataAtStartup(ragEngine *rag.RAGEngine) {
	log.Println("Starting RAG data ingestion...")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second) // 5 min timeout
	defer cancel()

	stats, err := ragEngine.IngestData(ctx)
	if err != nil {
		log.Printf("⚠️  Ingestion error: %v\n", err)
		// Note: Don't fatal here - server can still work
		return
	}

	log.Printf("✅ Ingestion complete:\n%+v\n", stats)
}

// ============================================================================
// API USAGE EXAMPLES
// ============================================================================
/*

1. Ask a Question
   POST /api/chat/rag
   {
     "question": "What events are available in Brooklyn?",
     "user_id": "user_123"
   }

   Response:
   {
     "answer": "...",
     "sources": ["events", "blogs"],
     "conversation_id": "user_123",
     "success": true
   }

2. Clear Conversation
   POST /api/chat/rag/clear
   {"user_id": "user_123"}

3. Delete Conversation
   POST /api/chat/rag/delete
   {"user_id": "user_123"}

4. Re-ingest Data (Admin)
   POST /api/admin/rag/ingest

5. Test Chat
   curl -X POST http://localhost:5000/api/chat/rag \
     -H "Content-Type: application/json" \
     -d '{"question": "Hello!", "user_id": "test"}'

*/

// ============================================================================
// TROUBLESHOOTING
// ============================================================================
/*

Q: "OpenAI API error 401: invalid api key"
A: Check OPENAI_API_KEY in .env, make sure it's valid at https://platform.openai.com/api-keys

Q: "Pinecone API error 401: invalid api key"
A: Check PINECONE_API_KEY in .env, make sure it matches your Pinecone account

Q: "failed to connect to database"
A: Check DATABASE_URL in .env, ensure PostgreSQL/Neon is accessible

Q: "no embedding returned from OpenAI"
A: Check question length - try with a longer, more detailed question

Q: "vectors_stored: 0" after ingestion
A: Check that database tables have data:
   SELECT COUNT(*) FROM events;
   SELECT COUNT(*) FROM users;
   SELECT COUNT(*) FROM blogs;

*/
