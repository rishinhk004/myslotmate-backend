-- Create rag_documents table for storing uploaded documents for RAG
CREATE TABLE rag_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    source VARCHAR(50) NOT NULL,  -- 'pdf', 'txt', 'docx', 'manual'
    file_type VARCHAR(100),        -- MIME type: 'application/pdf', 'text/plain', etc.
    file_name VARCHAR(255),        -- Original filename
    file_size BIGINT,              -- Size in bytes
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for common queries
CREATE INDEX idx_rag_documents_created_at ON rag_documents(created_at DESC);
CREATE INDEX idx_rag_documents_source ON rag_documents(source);
CREATE INDEX idx_rag_documents_title ON rag_documents(title);
