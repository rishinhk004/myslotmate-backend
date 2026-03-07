package controller

import (
	"net/http"

	"myslotmate-backend/internal/lib/storage"

	"github.com/go-chi/chi/v5"
)

// UploadController provides a generic file upload endpoint.
// The frontend uploads files here first, gets back URLs, then includes those URLs
// in the JSON body when creating/updating events, reviews, support tickets, etc.
type UploadController struct {
	uploadService *storage.UploadService
}

func NewUploadController(us *storage.UploadService) *UploadController {
	return &UploadController{uploadService: us}
}

func (c *UploadController) RegisterRoutes(r chi.Router) {
	r.Route("/upload", func(r chi.Router) {
		r.Post("/", c.Upload) // generic: POST /upload?folder=events/covers
	})
}

// Upload handles multipart file uploads.
//
// Query params:
//
//	folder – GCS path prefix (default "general"). Examples:
//	  events/covers, events/gallery, reviews/photos, support/evidence, hosts/documents
//
// Form field:
//
//	files – one or more files (SVG, PNG, JPG, PDF ≤ 10 MB each)
//
// Response: { "data": [ { "file_name": "...", "url": "...", "size": 12345 }, ... ] }
func (c *UploadController) Upload(w http.ResponseWriter, r *http.Request) {
	if c.uploadService == nil {
		RespondError(w, http.StatusServiceUnavailable, "File upload is not configured")
		return
	}

	const maxBody = 60 << 20 // 60 MB total
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := r.ParseMultipartForm(maxBody); err != nil {
		RespondError(w, http.StatusBadRequest, "Request too large or invalid multipart form")
		return
	}

	folder := r.URL.Query().Get("folder")
	if folder == "" {
		folder = "general"
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		RespondError(w, http.StatusBadRequest, "No files provided (use form field 'files')")
		return
	}

	results, err := c.uploadService.UploadMultiple(r.Context(), folder, files)
	if err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, results)
}
