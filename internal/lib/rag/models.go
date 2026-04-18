package rag

import "time"

// Message represents a chat message
type Message struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Conversation represents a conversation session
type Conversation struct {
	ID        string    `json:"id"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChatRequest represents a chat request
type ChatRequest struct {
	Question       string `json:"question"`
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id,omitempty"`
}

// ChatResponse represents a chat response
type ChatResponse struct {
	Answer         string   `json:"answer"`
	Sources        []string `json:"sources"`
	ConversationID string   `json:"conversation_id"`
	Success        bool     `json:"success"`
	Error          string   `json:"error,omitempty"`
}

// Document represents a document chunk
type Document struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata"`
	Embedding []float32         `json:"embedding,omitempty"`
}

// PineconeVector represents a vector in Pinecone
type PineconeVector struct {
	ID       string                 `json:"id"`
	Values   []float32              `json:"values"`
	Metadata map[string]interface{} `json:"metadata"`
}

// GeminiPart represents a Gemini content part.
type GeminiPart struct {
	Text string `json:"text,omitempty"`
}

// GeminiContent represents a Gemini content message.
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiEmbeddingRequest represents an embedding request to Gemini.
type GeminiEmbeddingRequest struct {
	Content              GeminiContent `json:"content"`
	TaskType             string        `json:"taskType,omitempty"`
	Title                string        `json:"title,omitempty"`
	OutputDimensionality int           `json:"outputDimensionality,omitempty"`
}

// GeminiEmbeddingResponse represents an embedding response from Gemini.
type GeminiEmbeddingResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

// GeminiGenerationConfig configures answer generation.
type GeminiGenerationConfig struct {
	Temperature     float32 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

// GeminiGenerateContentRequest represents a text generation request to Gemini.
type GeminiGenerateContentRequest struct {
	Contents         []GeminiContent         `json:"contents"`
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

// GeminiGenerateContentResponse represents a text generation response from Gemini.
type GeminiGenerateContentResponse struct {
	Candidates []struct {
		Content      GeminiContent `json:"content"`
		FinishReason string        `json:"finishReason,omitempty"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason,omitempty"`
	} `json:"promptFeedback,omitempty"`
}

// IngestionStats represents data ingestion statistics
type IngestionStats struct {
	TotalDocuments int    `json:"total_documents"`
	Chunks         int    `json:"chunks"`
	VectorsStored  int    `json:"vectors_stored"`
	Errors         int    `json:"errors"`
	Duration       string `json:"duration"`
}
