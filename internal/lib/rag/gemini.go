package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultGeminiChatModel         = "gemini-2.5-flash"
	defaultGeminiEmbeddingModel    = "gemini-embedding-001"
	defaultGeminiEmbeddingDim      = 1536
	maxGeminiEmbeddingInputRunes   = 8000
	defaultGeminiMaxOutputTokens   = 1000
	defaultGeminiAnswerTemperature = 0.7
)

// GeminiClient handles Gemini API calls for retrieval and answer generation.
type GeminiClient struct {
	apiKey              string
	baseURL             string
	chatModel           string
	embeddingModel      string
	embeddingDimensions int
	client              *http.Client
}

// NewGeminiClient creates a new Gemini client with RAG-friendly defaults.
func NewGeminiClient(apiKey string, embeddingDimensions int) *GeminiClient {
	if embeddingDimensions <= 0 {
		embeddingDimensions = defaultGeminiEmbeddingDim
	}

	return &GeminiClient{
		apiKey:              apiKey,
		baseURL:             "https://generativelanguage.googleapis.com/v1beta/models",
		chatModel:           defaultGeminiChatModel,
		embeddingModel:      defaultGeminiEmbeddingModel,
		embeddingDimensions: embeddingDimensions,
		client:              &http.Client{Timeout: 30 * time.Second},
	}
}

// GetQueryEmbedding converts a search query into an embedding vector.
func (g *GeminiClient) GetQueryEmbedding(ctx context.Context, text string) ([]float32, error) {
	return g.getEmbedding(ctx, text, "RETRIEVAL_QUERY", "")
}

// GetDocumentEmbedding converts a knowledge-base document into an embedding vector.
func (g *GeminiClient) GetDocumentEmbedding(ctx context.Context, text, title string) ([]float32, error) {
	return g.getEmbedding(ctx, text, "RETRIEVAL_DOCUMENT", title)
}

func (g *GeminiClient) getEmbedding(ctx context.Context, text, taskType, title string) ([]float32, error) {
	text = truncateRunes(text, maxGeminiEmbeddingInputRunes)

	req := GeminiEmbeddingRequest{
		Content: GeminiContent{
			Parts: []GeminiPart{{Text: text}},
		},
		TaskType:             taskType,
		OutputDimensionality: g.embeddingDimensions,
	}
	if title != "" {
		req.Title = title
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/%s:embedContent", g.baseURL, g.embeddingModel), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	g.setHeaders(httpReq)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call Gemini embeddings API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini embeddings API error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var embResp GeminiEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embResp.Embedding.Values) == 0 {
		return nil, fmt.Errorf("no embedding returned from Gemini")
	}

	return embResp.Embedding.Values, nil
}

// GenerateAnswer creates a response grounded in retrieved context.
func (g *GeminiClient) GenerateAnswer(ctx context.Context, question, contextText string, conversationHistory []Message) (string, error) {
	systemPrompt := `You are a helpful assistant for MySlotMate, a platform for booking and hosting experiences.

Use the provided knowledge base context to answer questions accurately. If the context does not contain enough relevant information, say so politely.

Be friendly, concise, and helpful. Focus on the user's specific question.`

	if strings.TrimSpace(contextText) == "" {
		contextText = "No relevant knowledge base context was retrieved."
	}

	contents := []GeminiContent{
		{
			Role:  "user",
			Parts: []GeminiPart{{Text: systemPrompt}},
		},
	}

	historyLimit := 4
	if len(conversationHistory) < historyLimit {
		historyLimit = len(conversationHistory)
	}
	startIdx := len(conversationHistory) - historyLimit
	for i := startIdx; i < len(conversationHistory); i++ {
		contents = append(contents, GeminiContent{
			Role:  toGeminiRole(conversationHistory[i].Role),
			Parts: []GeminiPart{{Text: conversationHistory[i].Content}},
		})
	}

	finalPrompt := fmt.Sprintf(`Knowledge base context:
%s

Current user question:
%s

Answer using the knowledge base context when it is relevant. If the answer is not available in the context, say that clearly instead of guessing.`, contextText, question)

	req := GeminiGenerateContentRequest{
		Contents: append(contents, GeminiContent{
			Role:  "user",
			Parts: []GeminiPart{{Text: finalPrompt}},
		}),
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     defaultGeminiAnswerTemperature,
			MaxOutputTokens: defaultGeminiMaxOutputTokens,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/%s:generateContent", g.baseURL, g.chatModel), bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	g.setHeaders(httpReq)

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to call Gemini generateContent API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Gemini generateContent API error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var genResp GeminiGenerateContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(genResp.Candidates) == 0 {
		if genResp.PromptFeedback.BlockReason != "" {
			return "", fmt.Errorf("Gemini blocked the prompt: %s", genResp.PromptFeedback.BlockReason)
		}
		return "", fmt.Errorf("no completion returned from Gemini")
	}

	answer := extractGeminiText(genResp.Candidates[0].Content.Parts)
	if answer == "" {
		return "", fmt.Errorf("Gemini returned an empty answer")
	}

	return strings.TrimSpace(answer), nil
}

func (g *GeminiClient) setHeaders(req *http.Request) {
	req.Header.Set("x-goog-api-key", g.apiKey)
	req.Header.Set("Content-Type", "application/json")
}

func truncateRunes(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes])
}

func toGeminiRole(role string) string {
	switch role {
	case "assistant", "model":
		return "model"
	default:
		return "user"
	}
}

func extractGeminiText(parts []GeminiPart) string {
	var builder strings.Builder
	for _, part := range parts {
		if strings.TrimSpace(part.Text) == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(part.Text)
	}
	return builder.String()
}
