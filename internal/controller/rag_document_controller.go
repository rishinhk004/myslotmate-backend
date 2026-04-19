package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"myslotmate-backend/internal/lib/rag"
	"net/http"
	"strconv"
)

// RAGDocumentController handles document upload and management
type RAGDocumentController struct {
	docStore  *rag.DocumentStore
	extractor *rag.DocumentExtractor
}

// NewRAGDocumentController creates a new document controller
func NewRAGDocumentController(docStore *rag.DocumentStore) *RAGDocumentController {
	return &RAGDocumentController{
		docStore:  docStore,
		extractor: rag.NewDocumentExtractor(),
	}
}

// UploadDocument handles POST /api/rag/documents/upload
// Upload a PDF, TXT, or DOCX file
func (c *RAGDocumentController) UploadDocument(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	err := r.ParseMultipartForm(50 * 1024 * 1024) // 50MB max
	if err != nil {
		writeErrorJSON(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get file from form
	file, handler, err := r.FormFile("file")
	if err != nil {
		writeErrorJSON(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get title from form (optional)
	title := r.FormValue("title")
	if title == "" {
		title = handler.Filename
	}

	// Read file data
	fileData, err := io.ReadAll(file)
	if err != nil {
		writeErrorJSON(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Validate file size (50MB max)
	if err := c.extractor.ValidateFileSize(int64(len(fileData)), 50); err != nil {
		writeErrorJSON(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate file type
	mediaType := handler.Header.Get("Content-Type")
	if mediaType == "" {
		// Try to infer from extension
		mediaType = inferMediaType(handler.Filename)
	}

	if err := c.extractor.ValidateFileType(mediaType); err != nil {
		writeErrorJSON(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Extract text from file
	content, err := c.extractor.ExtractText(fileData, mediaType)
	if err != nil {
		writeErrorJSON(w, fmt.Sprintf("Failed to extract text: %v", err), http.StatusBadRequest)
		return
	}

	// Determine source
	source := "uploaded_document"
	if mediaType == "application/pdf" {
		source = "pdf"
	} else if mediaType == "text/plain" {
		source = "txt"
	} else if mediaType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		source = "docx"
	}

	// Store document in database
	docID, err := c.docStore.StoreDocument(
		r.Context(),
		title,
		content,
		source,
		mediaType,
		handler.Filename,
		handler.Size,
	)
	if err != nil {
		writeErrorJSON(w, fmt.Sprintf("Failed to store document: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"message":     "Document uploaded successfully. Call /api/admin/rag/ingest to index it.",
		"document_id": docID,
		"document": map[string]interface{}{
			"id":        docID,
			"title":     title,
			"source":    source,
			"file_name": handler.Filename,
			"file_size": handler.Size,
		},
	})
}

// ListDocuments handles GET /api/rag/documents
// List all uploaded documents with pagination
func (c *RAGDocumentController) ListDocuments(w http.ResponseWriter, r *http.Request) {
	// Parse pagination
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Fetch documents
	documents, err := c.docStore.ListDocuments(r.Context(), limit, offset)
	if err != nil {
		writeErrorJSON(w, fmt.Sprintf("Failed to list documents: %v", err), http.StatusInternalServerError)
		return
	}

	// Get total count
	count, err := c.docStore.GetDocumentCount(r.Context())
	if err != nil {
		writeErrorJSON(w, fmt.Sprintf("Failed to get count: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"documents": documents,
		"pagination": map[string]interface{}{
			"limit":  limit,
			"offset": offset,
			"total":  count,
		},
	})
}

// GetDocument handles GET /api/rag/documents/{id}
// Get a specific document
func (c *RAGDocumentController) GetDocument(w http.ResponseWriter, r *http.Request) {
	// Get ID from URL path
	docID := r.URL.Path[len("/api/rag/documents/"):]

	doc, err := c.docStore.GetDocument(r.Context(), docID)
	if err != nil {
		writeErrorJSON(w, "Document not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"document": doc,
	})
}

// DeleteDocument handles DELETE /api/rag/documents/{id}
// Delete a document (admin only)
func (c *RAGDocumentController) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	// Get ID from URL path
	docID := r.URL.Path[len("/api/rag/documents/"):]

	err := c.docStore.DeleteDocument(r.Context(), docID)
	if err != nil {
		writeErrorJSON(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Document deleted. Call /api/admin/rag/ingest to re-index.",
	})
}

// Helper functions

func inferMediaType(filename string) string {
	if len(filename) < 4 {
		return "text/plain"
	}

	ext := filename[len(filename)-4:]
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	default:
		return "text/plain"
	}
}

func writeErrorJSON(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
