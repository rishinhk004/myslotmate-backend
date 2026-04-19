package rag

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// DocumentExtractor extracts text from various file formats
type DocumentExtractor struct{}

// NewDocumentExtractor creates a new document extractor
func NewDocumentExtractor() *DocumentExtractor {
	return &DocumentExtractor{}
}

// ExtractText extracts text from different file types
func (de *DocumentExtractor) ExtractText(fileData []byte, mediaType string) (string, error) {
	switch {
	case strings.Contains(mediaType, "application/pdf"):
		return de.extractPDF(fileData)
	case strings.Contains(mediaType, "text/plain"):
		return de.extractText(fileData)
	case strings.Contains(mediaType, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"):
		return de.extractDOCX(fileData)
	default:
		// Try to treat as text
		return de.extractText(fileData)
	}
}

// extractPDF extracts text from PDF files
func (de *DocumentExtractor) extractPDF(fileData []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return "", fmt.Errorf("failed to read PDF: %w", err)
	}

	var text strings.Builder

	for i := 1; i <= reader.NumPage(); i++ {
		p := reader.Page(i)

		// GetPlainText requires a font map; pass empty map
		textContent, err := p.GetPlainText(nil)
		if err != nil {
			// Continue on individual page errors
			continue
		}

		text.WriteString(textContent)
		text.WriteString("\n---PAGE BREAK---\n")
	}

	if text.Len() == 0 {
		return "", fmt.Errorf("no text extracted from PDF")
	}

	return text.String(), nil
}

// extractText extracts text from plain text files
func (de *DocumentExtractor) extractText(fileData []byte) (string, error) {
	text := string(fileData)

	// Validate it's readable
	if len(strings.TrimSpace(text)) == 0 {
		return "", fmt.Errorf("file is empty or not readable as text")
	}

	return text, nil
}

// extractDOCX extracts text from DOCX files
// Note: For full DOCX support, you'd need github.com/go-ole/go-ole or similar
// For now, this is a placeholder - DOCX is complex to parse in Go
func (de *DocumentExtractor) extractDOCX(fileData []byte) (string, error) {
	// DOCX files are ZIP archives with XML inside
	// This is a simplified implementation
	// For production, consider using a library like:
	// github.com/unidoc/unioffice or github.com/araddon/docx

	// Try to extract as text where possible
	text := string(fileData)

	// Remove binary data
	text = strings.Map(func(r rune) rune {
		if r < 32 && r != 9 && r != 10 && r != 13 {
			return -1
		}
		return r
	}, text)

	if len(strings.TrimSpace(text)) == 0 {
		return "", fmt.Errorf("no text could be extracted from DOCX file. Use .txt or .pdf instead")
	}

	return text, nil
}

// ValidateFileSize checks if file size is within limits
func (de *DocumentExtractor) ValidateFileSize(fileSize int64, maxSizeMB int64) error {
	maxBytes := maxSizeMB * 1024 * 1024
	if fileSize > maxBytes {
		return fmt.Errorf("file size (%dMB) exceeds maximum (%dMB)", fileSize/1024/1024, maxSizeMB)
	}
	return nil
}

// ValidateFileType checks if file type is supported
func (de *DocumentExtractor) ValidateFileType(mediaType string) error {
	supportedTypes := map[string]bool{
		"application/pdf": true,
		"text/plain":      true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"application/msword": true,
	}

	if !supportedTypes[mediaType] {
		return fmt.Errorf("unsupported file type: %s. Supported: PDF, TXT, DOCX", mediaType)
	}

	return nil
}
