package rag

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DataIngestionEngine handles loading data from PostgreSQL
type DataIngestionEngine struct {
	db           *sql.DB
	gemini       *GeminiClient
	pinecone     *PineconeClient
	chunkSize    int
	chunkOverlap int
}

// NewDataIngestionEngine creates a new data ingestion engine
func NewDataIngestionEngine(db *sql.DB, gemini *GeminiClient, pinecone *PineconeClient, chunkSize, chunkOverlap int) *DataIngestionEngine {
	return &DataIngestionEngine{
		db:           db,
		gemini:       gemini,
		pinecone:     pinecone,
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
	}
}

// IngestData loads all data from database and stores in Pinecone
func (d *DataIngestionEngine) IngestData(ctx context.Context) (*IngestionStats, error) {
	stats := &IngestionStats{}
	startTime := time.Now()

	events, err := d.loadEvents(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to load events: %v\n", err)
	} else {
		stats.TotalDocuments += len(events)
		fmt.Printf("Loaded %d events\n", len(events))
	}

	hosts, err := d.loadHosts(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to load hosts: %v\n", err)
	} else {
		stats.TotalDocuments += len(hosts)
		fmt.Printf("Loaded %d hosts\n", len(hosts))
	}

	users, err := d.loadUsers(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to load users: %v\n", err)
	} else {
		stats.TotalDocuments += len(users)
		fmt.Printf("Loaded %d users\n", len(users))
	}

	blogs, err := d.loadBlogs(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to load blogs: %v\n", err)
	} else {
		stats.TotalDocuments += len(blogs)
		fmt.Printf("Loaded %d blogs\n", len(blogs))
	}

	documents, err := d.loadDocuments(ctx)
	if err != nil {
		fmt.Printf("Warning: failed to load uploaded documents: %v\n", err)
	} else {
		stats.TotalDocuments += len(documents)
		fmt.Printf("Loaded %d uploaded documents\n", len(documents))
	}

	allDocuments := append(events, hosts...)
	allDocuments = append(allDocuments, users...)
	allDocuments = append(allDocuments, blogs...)
	allDocuments = append(allDocuments, documents...)

	fmt.Printf("\nChunking %d documents...\n", len(allDocuments))

	chunks := d.chunkDocuments(allDocuments)
	stats.Chunks = len(chunks)
	fmt.Printf("Created %d chunks\n", len(chunks))

	fmt.Printf("\nCreating embeddings and storing in Pinecone...\n")
	vectorsToStore := []map[string]interface{}{}

	for i, chunk := range chunks {
		embedding, err := d.gemini.GetDocumentEmbedding(ctx, chunk.Content, chunk.Metadata["title"])
		if err != nil {
			stats.Errors++
			fmt.Printf("Warning: failed to get embedding for chunk %d: %v\n", i, err)
			continue
		}

		vectorID := fmt.Sprintf("%s_%d", uuid.New().String(), i)
		metadata := chunk.Metadata
		metadata["content"] = chunk.Content

		vector := map[string]interface{}{
			"id":       vectorID,
			"values":   embedding,
			"metadata": metadata,
		}
		vectorsToStore = append(vectorsToStore, vector)

		if (i+1)%50 == 0 {
			fmt.Printf("  Processed %d chunks...\n", i+1)
		}
	}

	batchSize := 100
	for i := 0; i < len(vectorsToStore); i += batchSize {
		end := i + batchSize
		if end > len(vectorsToStore) {
			end = len(vectorsToStore)
		}

		batch := vectorsToStore[i:end]
		if err := d.pinecone.Upsert(ctx, batch); err != nil {
			stats.Errors++
			fmt.Printf("Warning: failed to upsert batch %d: %v\n", i/batchSize+1, err)
		} else {
			stats.VectorsStored += len(batch)
		}

		fmt.Printf("  Stored batch %d/%d\n", i/batchSize+1, (len(vectorsToStore)+batchSize-1)/batchSize)
	}

	stats.Duration = time.Since(startTime).String()
	fmt.Printf("\nIngestion complete in %s\n", stats.Duration)
	fmt.Printf("  - Documents: %d\n  - Chunks: %d\n  - Vectors stored: %d\n  - Errors: %d\n",
		stats.TotalDocuments, stats.Chunks, stats.VectorsStored, stats.Errors)

	return stats, nil
}

// loadEvents loads events from database and enriches them with host details.
func (d *DataIngestionEngine) loadEvents(ctx context.Context) ([]Document, error) {
	var documents []Document

	rows, err := d.db.QueryContext(ctx, `
		SELECT
			e.id,
			COALESCE(e.title, ''),
			COALESCE(e.description, ''),
			COALESCE(e.category::text, ''),
			COALESCE(e.location, ''),
			e.created_at,
			COALESCE(e.status::text, ''),
			COALESCE(h.id::text, ''),
			COALESCE(h.first_name, ''),
			COALESCE(h.last_name, ''),
			COALESCE(h.tagline, ''),
			COALESCE(h.city, '')
		FROM events e
		LEFT JOIN hosts h ON h.id = e.host_id
		LIMIT 1000
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, description, category, location, status string
		var hostID, hostFirstName, hostLastName, hostTagline, hostCity string
		var createdAt time.Time

		if err := rows.Scan(
			&id,
			&title,
			&description,
			&category,
			&location,
			&createdAt,
			&status,
			&hostID,
			&hostFirstName,
			&hostLastName,
			&hostTagline,
			&hostCity,
		); err != nil {
			continue
		}

		hostName := strings.TrimSpace(hostFirstName + " " + hostLastName)
		contentLines := []string{
			fmt.Sprintf("Event: %s", title),
			fmt.Sprintf("Category: %s", category),
			fmt.Sprintf("Status: %s", status),
		}
		if hostName != "" {
			contentLines = append(contentLines, fmt.Sprintf("Host: %s", hostName))
		}
		if hostTagline != "" {
			contentLines = append(contentLines, fmt.Sprintf("Host Tagline: %s", hostTagline))
		}
		if hostCity != "" {
			contentLines = append(contentLines, fmt.Sprintf("Host City: %s", hostCity))
		}
		if location != "" {
			contentLines = append(contentLines, fmt.Sprintf("Location: %s", location))
		}
		if description != "" {
			contentLines = append(contentLines, fmt.Sprintf("Description: %s", description))
		}
		contentLines = append(contentLines, fmt.Sprintf("Created: %s", createdAt.Format("2006-01-02")))

		doc := Document{
			ID:      id,
			Content: strings.Join(contentLines, "\n"),
			Metadata: map[string]string{
				"source":   "events",
				"event_id": id,
				"title":    title,
				"category": category,
				"status":   status,
			},
		}
		if hostID != "" {
			doc.Metadata["host_id"] = hostID
		}
		if hostName != "" {
			doc.Metadata["host_name"] = hostName
		}

		documents = append(documents, doc)
	}

	return documents, rows.Err()
}

// loadHosts loads approved host profiles from database.
func (d *DataIngestionEngine) loadHosts(ctx context.Context) ([]Document, error) {
	var documents []Document

	rows, err := d.db.QueryContext(ctx, `
		SELECT
			id,
			user_id,
			COALESCE(first_name, ''),
			COALESCE(last_name, ''),
			COALESCE(city, ''),
			COALESCE(tagline, ''),
			COALESCE(bio, ''),
			COALESCE(experience_desc, ''),
			COALESCE(description, ''),
			COALESCE(array_to_string(expertise_tags, ', '), ''),
			COALESCE(application_status::text, ''),
			COALESCE(avg_rating, 0),
			COALESCE(total_reviews, 0),
			is_identity_verified,
			is_super_host,
			created_at
		FROM hosts
		WHERE application_status = 'approved'
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, userID, firstName, lastName, city, tagline, bio, experienceDesc, description, expertiseTags, applicationStatus string
		var avgRating float64
		var totalReviews int
		var isIdentityVerified, isSuperHost bool
		var createdAt time.Time

		if err := rows.Scan(
			&id,
			&userID,
			&firstName,
			&lastName,
			&city,
			&tagline,
			&bio,
			&experienceDesc,
			&description,
			&expertiseTags,
			&applicationStatus,
			&avgRating,
			&totalReviews,
			&isIdentityVerified,
			&isSuperHost,
			&createdAt,
		); err != nil {
			continue
		}

		hostName := strings.TrimSpace(firstName + " " + lastName)
		contentLines := []string{
			fmt.Sprintf("Host: %s", hostName),
			fmt.Sprintf("Application Status: %s", applicationStatus),
			fmt.Sprintf("City: %s", city),
		}
		if tagline != "" {
			contentLines = append(contentLines, fmt.Sprintf("Tagline: %s", tagline))
		}
		if bio != "" {
			contentLines = append(contentLines, fmt.Sprintf("Bio: %s", bio))
		}
		if experienceDesc != "" {
			contentLines = append(contentLines, fmt.Sprintf("Experience Focus: %s", experienceDesc))
		}
		if description != "" {
			contentLines = append(contentLines, fmt.Sprintf("Description: %s", description))
		}
		if expertiseTags != "" {
			contentLines = append(contentLines, fmt.Sprintf("Expertise Tags: %s", expertiseTags))
		}
		contentLines = append(contentLines,
			fmt.Sprintf("Average Rating: %.2f", avgRating),
			fmt.Sprintf("Total Reviews: %d", totalReviews),
			fmt.Sprintf("Identity Verified: %t", isIdentityVerified),
			fmt.Sprintf("Super Host: %t", isSuperHost),
			fmt.Sprintf("Joined: %s", createdAt.Format("2006-01-02")),
		)

		documents = append(documents, Document{
			ID:      id,
			Content: strings.Join(contentLines, "\n"),
			Metadata: map[string]string{
				"source":             "hosts",
				"host_id":            id,
				"user_id":            userID,
				"title":              hostName,
				"application_status": applicationStatus,
			},
		})
	}

	return documents, rows.Err()
}

// loadUsers loads user profiles from database
func (d *DataIngestionEngine) loadUsers(ctx context.Context) ([]Document, error) {
	var documents []Document

	rows, err := d.db.QueryContext(ctx, `
		SELECT id, bio, location, created_at
		FROM users
		WHERE bio IS NOT NULL
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, bio, location string
		var createdAt time.Time

		if err := rows.Scan(&id, &bio, &location, &createdAt); err != nil {
			continue
		}

		content := fmt.Sprintf(`User Profile
Bio: %s
Location: %s
Member since: %s`, bio, location, createdAt.Format("2006-01-02"))

		doc := Document{
			ID:      id,
			Content: content,
			Metadata: map[string]string{
				"source":  "user_profiles",
				"user_id": id,
			},
		}
		documents = append(documents, doc)
	}

	return documents, rows.Err()
}

// loadBlogs loads blogs from database
func (d *DataIngestionEngine) loadBlogs(ctx context.Context) ([]Document, error) {
	var documents []Document

	rows, err := d.db.QueryContext(ctx, `
		SELECT id, title, content, category
		FROM blogs
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, content, category string

		if err := rows.Scan(&id, &title, &content, &category); err != nil {
			continue
		}

		contentText := content
		if len(contentText) > 2000 {
			contentText = contentText[:2000]
		}

		docContent := fmt.Sprintf(`Blog: %s
Category: %s
Content:
%s`, title, category, contentText)

		doc := Document{
			ID:      id,
			Content: docContent,
			Metadata: map[string]string{
				"source":  "blogs",
				"blog_id": id,
				"title":   title,
			},
		}
		documents = append(documents, doc)
	}

	return documents, rows.Err()
}

// loadDocuments loads uploaded documents from the rag_documents table
func (d *DataIngestionEngine) loadDocuments(ctx context.Context) ([]Document, error) {
	var documents []Document

	rows, err := d.db.QueryContext(ctx, `
		SELECT id, title, content, source
		FROM rag_documents
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, content, source string

		if err := rows.Scan(&id, &title, &content, &source); err != nil {
			continue
		}

		contentText := content
		if len(contentText) > 5000 {
			contentText = contentText[:5000]
		}

		docContent := fmt.Sprintf(`Document: %s
Source: %s
Content:
%s`, title, source, contentText)

		doc := Document{
			ID:      id,
			Content: docContent,
			Metadata: map[string]string{
				"source":      "uploaded_documents",
				"document_id": id,
				"title":       title,
				"type":        source,
			},
		}
		documents = append(documents, doc)
	}

	return documents, rows.Err()
}

// chunkDocuments splits documents into chunks
func (d *DataIngestionEngine) chunkDocuments(documents []Document) []Document {
	var chunks []Document

	for _, doc := range documents {
		text := doc.Content

		for i := 0; i < len(text); i += d.chunkSize - d.chunkOverlap {
			end := i + d.chunkSize
			if end > len(text) {
				end = len(text)
			}

			chunk := Document{
				ID:       fmt.Sprintf("%s_chunk_%d", doc.ID, len(chunks)),
				Content:  strings.TrimSpace(text[i:end]),
				Metadata: doc.Metadata,
			}
			chunks = append(chunks, chunk)

			if end >= len(text) {
				break
			}
		}
	}

	return chunks
}
