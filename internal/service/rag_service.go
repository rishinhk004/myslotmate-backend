package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RAGRequest represents a chat request to the RAG service
type RAGRequest struct {
	Question       string `json:"question"`
	ConversationID string `json:"conversation_id"`
}

// RAGResponse represents a response from the RAG service
type RAGResponse struct {
	Answer         string   `json:"answer"`
	Sources        []string `json:"sources"`
	Success        bool     `json:"success"`
	ConversationID string   `json:"conversation_id"`
	Error          string   `json:"error,omitempty"`
}

// RAGService handles communication with the RAG chatbot service
type RAGService struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewRAGService creates a new RAG service client
func NewRAGService(baseURL string) *RAGService {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:5001"
	}

	return &RAGService{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Chat sends a question to the RAG chatbot and returns the answer
func (s *RAGService) Chat(question, conversationID string) (*RAGResponse, error) {
	req := RAGRequest{
		Question:       question,
		ConversationID: conversationID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := s.HTTPClient.Post(
		s.BaseURL+"/chat",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call RAG service: %w", err)
	}
	defer resp.Body.Close()

	var ragResp RAGResponse
	if err := json.NewDecoder(resp.Body).Decode(&ragResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !ragResp.Success {
		return nil, fmt.Errorf("RAG service error: %s", ragResp.Error)
	}

	return &ragResp, nil
}

// ClearConversation clears the chat history for a conversation
func (s *RAGService) ClearConversation(conversationID string) error {
	req := map[string]string{
		"conversation_id": conversationID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := s.HTTPClient.Post(
		s.BaseURL+"/chat/clear",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("failed to call RAG service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("RAG service error: %s", string(body))
	}

	return nil
}

// DeleteConversation deletes a conversation entirely
func (s *RAGService) DeleteConversation(conversationID string) error {
	req := map[string]string{
		"conversation_id": conversationID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := s.HTTPClient.Post(
		s.BaseURL+"/chat/delete",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("failed to call RAG service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("RAG service error: %s", string(body))
	}

	return nil
}

// Health checks if the RAG service is running
func (s *RAGService) Health() error {
	resp, err := s.HTTPClient.Get(s.BaseURL + "/health")
	if err != nil {
		return fmt.Errorf("RAG service not responding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("RAG service unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// IngestData triggers data ingestion on the RAG service (admin only)
func (s *RAGService) IngestData() error {
	resp, err := s.HTTPClient.Post(
		s.BaseURL+"/ingest",
		"application/json",
		bytes.NewBuffer([]byte{}),
	)
	if err != nil {
		return fmt.Errorf("failed to call RAG service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("RAG service error: %s", string(body))
	}

	return nil
}
