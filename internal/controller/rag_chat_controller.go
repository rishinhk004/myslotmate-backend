package controller

import (
	"encoding/json"
	"myslotmate-backend/internal/lib/rag"
	"net/http"
)

// RAGChatRequest represents a chat request from the client
type RAGChatRequest struct {
	Question string `json:"question" validate:"required,min=3"`
	UserID   string `json:"user_id,omitempty"`
}

// RAGChatController handles RAG chatbot endpoints
type RAGChatController struct {
	ragEngine *rag.RAGEngine
}

// NewRAGChatController creates a new RAG chat controller
func NewRAGChatController(ragEngine *rag.RAGEngine) *RAGChatController {
	return &RAGChatController{
		ragEngine: ragEngine,
	}
}

// Chat handles POST /api/chat/rag
// User Query → RAG Engine → Answer
func (c *RAGChatController) Chat(w http.ResponseWriter, r *http.Request) {
	var req RAGChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Question == "" {
		http.Error(w, "Question is required", http.StatusBadRequest)
		return
	}

	// Use user ID as conversation ID (or a session ID)
	conversationID := req.UserID
	if conversationID == "" {
		conversationID = "anonymous"
	}

	// Create Chat request
	chatReq := &rag.ChatRequest{
		Question:       req.Question,
		ConversationID: conversationID,
		UserID:         req.UserID,
	}

	// Call RAG engine
	ragResp, err := c.ragEngine.Chat(r.Context(), chatReq)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Return response to client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ragResp)
}

// ClearChat handles POST /api/chat/rag/clear
func (c *RAGChatController) ClearChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	c.ragEngine.ClearConversation(req.UserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Conversation cleared",
		"status":  "success",
	})
}

// DeleteChat handles POST /api/chat/rag/delete
func (c *RAGChatController) DeleteChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	c.ragEngine.DeleteConversation(req.UserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Conversation deleted",
		"status":  "success",
	})
}

// IngestData handles POST /api/admin/rag/ingest
func (c *RAGChatController) IngestData(w http.ResponseWriter, r *http.Request) {
	// TODO: Add authentication here in production

	stats, err := c.ragEngine.IngestData(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"stats":   stats,
	})
}
