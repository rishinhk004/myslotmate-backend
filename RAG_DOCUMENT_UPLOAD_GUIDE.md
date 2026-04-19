# RAG Document Upload System - Implementation Summary

## ✅ Completed Implementation

### 1. **File Upload Endpoint**
- **Route**: `POST /api/upload/rag-document`
- **Handler**: `RAGDocumentController.UploadDocument()`
- **Features**:
  - Accepts multipart form with file and optional title
  - Validates file type (PDF, TXT, DOCX)
  - Validates file size (50MB max)
  - Extracts text from uploaded files
  - Stores document metadata and content in PostgreSQL
  - Returns document ID for tracking

### 2. **Document Management Endpoints**
- **List Documents**: `GET /api/admin/rag/documents?limit=20&offset=0`
  - Handler: `RAGDocumentController.ListDocuments()`
  - Returns paginated list of all uploaded documents
  - Includes total count for pagination

- **Get Document**: `GET /api/admin/rag/documents/{id}`
  - Handler: `RAGDocumentController.GetDocument()`
  - Retrieves full document content by ID

- **Delete Document**: `DELETE /api/admin/rag/documents/{id}`
  - Handler: `RAGDocumentController.DeleteDocument()`
  - Removes document, requires re-ingest to update index

### 3. **Auto-Inclusion in Ingestion Pipeline**
- New method: `DataIngestionEngine.loadDocuments(ctx)`
- Loads all documents from `rag_documents` table
- Automatically included when calling `/api/admin/rag/ingest`
- Documents are chunked, embedded, and stored in Pinecone vector database

### 4. **Database Schema**
- **Table**: `rag_documents`
- **Migration**: `migrations/20260418120000_create_rag_documents_table.sql`
- **Columns**:
  - `id` (UUID, Primary Key)
  - `title` (VARCHAR 255)
  - `content` (TEXT - full extracted text)
  - `source` (VARCHAR 50 - pdf/txt/docx)
  - `file_type` (VARCHAR 100 - MIME type)
  - `file_name` (VARCHAR 255 - original filename)
  - `file_size` (BIGINT - bytes)
  - `created_at`, `updated_at` (TIMESTAMP)
- **Indexes**: created_at, source, title for performance

### 5. **File Format Support**
- **PDF**: Full text extraction using ledongthuc/pdf library
- **TXT**: Plain text handling (UTF-8)
- **DOCX**: Basic DOCX parsing (supports simple word documents)

### 6. **Configuration**
Added to `internal/config/config.go`:
```go
type OpenAIConfig struct {
    APIKey string
}

type PineconeConfig struct {
    APIKey      string
    IndexName   string
    Environment string
}
```

Environment Variables Required:
- `OPENAI_API_KEY`
- `PINECONE_API_KEY`
- `PINECONE_INDEX_NAME` (default: "myslotmate-rag")
- `PINECONE_ENVIRONMENT` (default: "us-east-1")

### 7. **Router Integration**
Updated `internal/server/router.go`:
- Added `ragDocCtrl` parameter to `NewRouter()`
- Routes all document endpoints:
  - `POST /api/upload/rag-document`
  - `GET /api/admin/rag/documents`
  - `DELETE /api/admin/rag/documents/{id}`

### 8. **Main Function Integration**
Updated `cmd/api/run.go`:
- Initializes `RAGEngine` with OpenAI and Pinecone credentials
- Creates `DocumentStore` for document operations
- Initializes both `ragChatCtrl` and `ragDocCtrl`
- Passes both controllers to `NewRouter()`

## 🔄 Workflow

### Upload Document
1. User POST to `/api/upload/rag-document` with file
2. File validated (type & size)
3. Text extracted from file
4. Document stored in `rag_documents` table with metadata
5. Response includes `document_id`
6. Admin calls `POST /api/admin/rag/ingest` to index the document

### Automatic Ingestion
1. `/api/admin/rag/ingest` endpoint called
2. Loads from 4 sources:
   - Events table (1000 limit)
   - Users table (500 limit)
   - Blogs table (500 limit)
   - **uploaded documents (rag_documents)** ← NEW
3. All content chunked (1000 chars, 200 char overlap)
4. Embeddings created via OpenAI API
5. Vectors stored in Pinecone index

### Query Documents
1. User asks question via `/api/chat/rag`
2. RAGEngine:
   - Embeds question with OpenAI
   - Searches Pinecone for top-K similar chunks
   - Includes conversation history
   - Generates answer with GPT-4-turbo
   - Returns response with citations

## 📋 API Examples

### Upload a PDF
```bash
curl -X POST \
  -F "file=@document.pdf" \
  -F "title=My Custom Title" \
  http://localhost:8080/api/upload/rag-document
```

Response:
```json
{
  "success": true,
  "message": "Document uploaded successfully. Call /api/admin/rag/ingest to index it.",
  "document_id": "550e8400-e29b-41d4-a716-446655440000",
  "document": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "title": "My Custom Title",
    "source": "pdf",
    "file_name": "document.pdf",
    "file_size": 245000
  }
}
```

### List Documents
```bash
curl http://localhost:8080/api/admin/rag/documents?limit=10&offset=0
```

### Delete Document
```bash
curl -X DELETE \
  http://localhost:8080/api/admin/rag/documents/550e8400-e29b-41d4-a716-446655440000
```

### Re-index All Documents
```bash
curl -X POST \
  -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/admin/rag/ingest
```

## 🚀 Deployment Checklist

- [ ] Set `OPENAI_API_KEY` environment variable
- [ ] Set `PINECONE_API_KEY` environment variable  
- [ ] Set `PINECONE_INDEX_NAME` if not using default
- [ ] Run database migration: `20260418120000_create_rag_documents_table.sql`
- [ ] Restart Go backend server
- [ ] Test document upload with POST /api/upload/rag-document
- [ ] Test document listing with GET /api/admin/rag/documents
- [ ] Call POST /api/admin/rag/ingest to initialize vector index
- [ ] Test RAG chat with POST /api/chat/rag

## 📝 Notes

- All uploaded documents are **auto-included in ingestion** - no manual configuration needed
- Document content size limited to 5000 characters per document (can be increased in `loadDocuments` method)
- File size limit is 50MB per file
- DOCX support is basic; complex formatting may be lost
- Vector embeddings use `text-embedding-3-small` (1536 dimensions)
- Conversation history limited to 20 messages per user
- Supports concurrent document uploads
