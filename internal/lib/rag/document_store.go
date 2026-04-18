package rag

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DocumentStore handles document storage operations
type DocumentStore struct {
	db *sql.DB
}

// NewDocumentStore creates a new document store
func NewDocumentStore(db *sql.DB) *DocumentStore {
	return &DocumentStore{db: db}
}

// RAGDocument represents a document in the database
type RAGDocument struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`    // 'pdf', 'txt', 'docx', 'manual'
	FileType  string    `json:"file_type"` // 'application/pdf', 'text/plain', etc.
	FileName  string    `json:"file_name"`
	FileSize  int64     `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StoreDocument stores a document in the database
func (ds *DocumentStore) StoreDocument(ctx context.Context, title, content, source, fileType, fileName string, fileSize int64) (string, error) {
	var id string

	err := ds.db.QueryRowContext(ctx, `
		INSERT INTO rag_documents (title, content, source, file_type, file_name, file_size, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id
	`, title, content, source, fileType, fileName, fileSize).Scan(&id)

	if err != nil {
		return "", fmt.Errorf("failed to store document: %w", err)
	}

	return id, nil
}

// GetDocument retrieves a document by ID
func (ds *DocumentStore) GetDocument(ctx context.Context, docID string) (*RAGDocument, error) {
	var doc RAGDocument

	err := ds.db.QueryRowContext(ctx, `
		SELECT id, title, content, source, file_type, file_name, file_size, created_at, updated_at
		FROM rag_documents
		WHERE id = $1
	`, docID).Scan(
		&doc.ID, &doc.Title, &doc.Content, &doc.Source,
		&doc.FileType, &doc.FileName, &doc.FileSize,
		&doc.CreatedAt, &doc.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	return &doc, nil
}

// ListDocuments lists all documents with pagination
func (ds *DocumentStore) ListDocuments(ctx context.Context, limit int, offset int) ([]RAGDocument, error) {
	var documents []RAGDocument

	rows, err := ds.db.QueryContext(ctx, `
		SELECT id, title, content, source, file_type, file_name, file_size, created_at, updated_at
		FROM rag_documents
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var doc RAGDocument
		err := rows.Scan(
			&doc.ID, &doc.Title, &doc.Content, &doc.Source,
			&doc.FileType, &doc.FileName, &doc.FileSize,
			&doc.CreatedAt, &doc.UpdatedAt,
		)
		if err != nil {
			continue
		}
		documents = append(documents, doc)
	}

	return documents, rows.Err()
}

// DeleteDocument deletes a document by ID
func (ds *DocumentStore) DeleteDocument(ctx context.Context, docID string) error {
	result, err := ds.db.ExecContext(ctx, `
		DELETE FROM rag_documents
		WHERE id = $1
	`, docID)

	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("document not found")
	}

	return nil
}

// GetAllDocumentsForIngestion retrieves all documents for the ingestion pipeline
func (ds *DocumentStore) GetAllDocumentsForIngestion(ctx context.Context) ([]RAGDocument, error) {
	var documents []RAGDocument

	rows, err := ds.db.QueryContext(ctx, `
		SELECT id, title, content, source, file_type, file_name, file_size, created_at, updated_at
		FROM rag_documents
		ORDER BY created_at ASC
	`)

	if err != nil {
		return nil, fmt.Errorf("failed to query documents for ingestion: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var doc RAGDocument
		err := rows.Scan(
			&doc.ID, &doc.Title, &doc.Content, &doc.Source,
			&doc.FileType, &doc.FileName, &doc.FileSize,
			&doc.CreatedAt, &doc.UpdatedAt,
		)
		if err != nil {
			continue
		}
		documents = append(documents, doc)
	}

	return documents, rows.Err()
}

// GetDocumentCount returns the total number of documents
func (ds *DocumentStore) GetDocumentCount(ctx context.Context) (int, error) {
	var count int

	err := ds.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM rag_documents`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}

	return count, nil
}
