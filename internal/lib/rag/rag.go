package rag

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RAGEngine is the main RAG chatbot engine
type RAGEngine struct {
	gemini        *GeminiClient
	pinecone      *PineconeClient
	ingestion     *DataIngestionEngine
	conversations map[string]*Conversation
	mutex         sync.RWMutex
	topK          int32
	maxMemory     int
}

// NewRAGEngine creates a new RAG engine
func NewRAGEngine(db *sql.DB, geminiKey string, geminiEmbeddingDimensions int, pineconeKey, pineconeHost, indexName string, topK int32, maxMemory int) *RAGEngine {
	gemini := NewGeminiClient(geminiKey, geminiEmbeddingDimensions)
	pinecone := NewPineconeClient(pineconeKey, pineconeHost, indexName)
	ingestion := NewDataIngestionEngine(db, gemini, pinecone, 1000, 200)

	return &RAGEngine{
		gemini:        gemini,
		pinecone:      pinecone,
		ingestion:     ingestion,
		conversations: make(map[string]*Conversation),
		topK:          topK,
		maxMemory:     maxMemory,
	}
}

// Chat sends a question and gets an answer
func (r *RAGEngine) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Get or create conversation
	conv := r.getOrCreateConversation(req.ConversationID)

	// Encode question to embedding
	queryEmbedding, err := r.gemini.GetQueryEmbedding(ctx, req.Question)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %w", err)
	}

	// Search Pinecone for similar chunks
	results, err := r.pinecone.Query(ctx, queryEmbedding, r.topK, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query Pinecone: %w", err)
	}

	// Build context from results
	contextParts := []string{}
	sources := []string{}

	for _, result := range results {
		// Extract content and metadata
		metadata, ok := result["metadata"].(map[string]interface{})
		if !ok {
			continue
		}

		// Try to get content from metadata
		var content string
		if c, ok := metadata["content"].(string); ok {
			content = c
		}

		if content != "" {
			contextParts = append(contextParts, content)
		}

		// Collect sources
		if source, ok := metadata["source"].(string); ok {
			if !contains(sources, source) {
				sources = append(sources, source)
			}
		}
	}

	contextStr := ""
	if len(contextParts) > 0 {
		contextStr = fmt.Sprintf("Relevant information from knowledge base:\n%s", join(contextParts, "\n---\n"))
	}

	// Avoid vague model replies when retrieval returns nothing useful.
	if strings.TrimSpace(contextStr) == "" {
		answer := "I couldn't find any relevant information in the knowledge base for that question yet."

		conv.Messages = append(conv.Messages, Message{
			Role:      "user",
			Content:   req.Question,
			Timestamp: time.Now(),
		})
		conv.Messages = append(conv.Messages, Message{
			Role:      "assistant",
			Content:   answer,
			Timestamp: time.Now(),
		})

		if len(conv.Messages) > r.maxMemory {
			conv.Messages = conv.Messages[len(conv.Messages)-r.maxMemory:]
		}

		conv.UpdatedAt = time.Now()

		return &ChatResponse{
			Answer:         answer,
			Sources:        sources,
			ConversationID: req.ConversationID,
			Success:        true,
		}, nil
	}

	// Generate answer using Gemini
	answer, err := r.gemini.GenerateAnswer(ctx, req.Question, contextStr, conv.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate answer: %w", err)
	}

	// Add messages to conversation history
	conv.Messages = append(conv.Messages, Message{
		Role:      "user",
		Content:   req.Question,
		Timestamp: time.Now(),
	})
	conv.Messages = append(conv.Messages, Message{
		Role:      "assistant",
		Content:   answer,
		Timestamp: time.Now(),
	})

	// Trim conversation history to max memory
	if len(conv.Messages) > r.maxMemory {
		conv.Messages = conv.Messages[len(conv.Messages)-r.maxMemory:]
	}

	conv.UpdatedAt = time.Now()

	return &ChatResponse{
		Answer:         answer,
		Sources:        sources,
		ConversationID: req.ConversationID,
		Success:        true,
	}, nil
}

// ClearConversation clears the chat history
func (r *RAGEngine) ClearConversation(conversationID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if conv, ok := r.conversations[conversationID]; ok {
		conv.Messages = []Message{}
		conv.UpdatedAt = time.Now()
	}
}

// DeleteConversation deletes a conversation
func (r *RAGEngine) DeleteConversation(conversationID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.conversations, conversationID)
}

// IngestData triggers data ingestion
func (r *RAGEngine) IngestData(ctx context.Context) (*IngestionStats, error) {
	return r.ingestion.IngestData(ctx)
}

// getOrCreateConversation gets or creates a conversation
func (r *RAGEngine) getOrCreateConversation(conversationID string) *Conversation {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if conv, ok := r.conversations[conversationID]; ok {
		return conv
	}

	conv := &Conversation{
		ID:        conversationID,
		Messages:  []Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	r.conversations[conversationID] = conv

	return conv
}

// GetConversation gets a conversation
func (r *RAGEngine) GetConversation(conversationID string) *Conversation {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.conversations[conversationID]
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func join(slice []string, sep string) string {
	result := ""
	for i, v := range slice {
		if i > 0 {
			result += sep
		}
		result += v
	}
	return result
}
