package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"myslotmate-backend/internal/auth"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/repository"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// BlogController handles HTTP requests for blog operations
type BlogController struct {
	blogRepo     repository.BlogRepository
	firebaseAuth *fbauth.Client
	adminEmail   string
}

// NewBlogController Factory for BlogController
func NewBlogController(br repository.BlogRepository, fa *fbauth.Client, adminEmail string) *BlogController {
	return &BlogController{
		blogRepo:     br,
		firebaseAuth: fa,
		adminEmail:   adminEmail,
	}
}

// RegisterRoutes registers routes for the blog controller on the provided router
func (c *BlogController) RegisterRoutes(r chi.Router) {
	r.Route("/blogs", func(r chi.Router) {
		// Admin-only routes (registered first for priority)
		r.With(auth.IsAdmin(c.firebaseAuth, c.adminEmail)).Post("/", c.CreateBlog)
		r.With(auth.IsAdmin(c.firebaseAuth, c.adminEmail)).Put("/{blogID}", c.UpdateBlog)
		r.With(auth.IsAdmin(c.firebaseAuth, c.adminEmail)).Delete("/{blogID}", c.DeleteBlog)
		r.With(auth.IsAdmin(c.firebaseAuth, c.adminEmail)).Post("/{blogID}/publish", c.PublishBlog)
		r.With(auth.IsAdmin(c.firebaseAuth, c.adminEmail)).Post("/{blogID}/unpublish", c.UnpublishBlog)

		// Public routes
		r.Get("/category/{category}", c.ListBlogsByCategory)
		r.Get("/", c.ListPublishedBlogs)
		r.Get("/{blogID}", c.GetBlog)
	})
}

// ── Request types ───────────────────────────────────────────────────────────

type CreateBlogRequest struct {
	Title           string  `json:"title" validate:"required"`
	Description     *string `json:"description,omitempty"`
	Category        string  `json:"category" validate:"required,oneof=Hosting Wellness Adventure"`
	Content         string  `json:"content" validate:"required"`
	CoverImageURL   *string `json:"cover_image_url,omitempty"`
	ReadTimeMinutes int     `json:"read_time_minutes,omitempty"`
}

type UpdateBlogRequest struct {
	Title           string  `json:"title"`
	Description     *string `json:"description,omitempty"`
	Category        string  `json:"category"`
	Content         string  `json:"content"`
	CoverImageURL   *string `json:"cover_image_url,omitempty"`
	ReadTimeMinutes int     `json:"read_time_minutes,omitempty"`
}

// ── Handlers ────────────────────────────────────────────────────────────────

// CreateBlog creates a new blog post (admin only)
func (c *BlogController) CreateBlog(w http.ResponseWriter, r *http.Request) {
	var req CreateBlogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Get admin email from context
	email, _ := r.Context().Value(auth.ContextKeyEmail).(string)

	// Set default read time if not provided
	if req.ReadTimeMinutes == 0 {
		req.ReadTimeMinutes = 5
	}

	blog := &models.Blog{
		Title:           req.Title,
		Description:     req.Description,
		Category:        req.Category,
		Content:         req.Content,
		CoverImageURL:   req.CoverImageURL,
		ReadTimeMinutes: req.ReadTimeMinutes,
		AuthorName:      email,
		// AuthorID will be set after we fetch the admin user - for now we'll use a placeholder
		// In production, you might want to fetch this from Firebase or have a separate admin user record
	}

	if err := c.blogRepo.Create(r.Context(), blog); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, blog)
}

// GetBlog retrieves a single blog post by ID
func (c *BlogController) GetBlog(w http.ResponseWriter, r *http.Request) {
	blogIDStr := chi.URLParam(r, "blogID")
	blogID, err := uuid.Parse(blogIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	blog, err := c.blogRepo.GetByID(r.Context(), blogID)
	if err != nil {
		RespondError(w, http.StatusNotFound, "Blog not found")
		return
	}

	RespondSuccess(w, http.StatusOK, blog)
}

// ListPublishedBlogs retrieves all published blogs with pagination
func (c *BlogController) ListPublishedBlogs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	blogs, err := c.blogRepo.ListPublished(r.Context(), limit, offset)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if blogs == nil {
		blogs = []*models.Blog{}
	}

	RespondSuccess(w, http.StatusOK, blogs)
}

// ListBlogsByCategory retrieves published blogs filtered by category
func (c *BlogController) ListBlogsByCategory(w http.ResponseWriter, r *http.Request) {
	category := chi.URLParam(r, "category")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	blogs, err := c.blogRepo.ListPublishedByCategory(r.Context(), category, limit, offset)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if blogs == nil {
		blogs = []*models.Blog{}
	}

	RespondSuccess(w, http.StatusOK, blogs)
}

// UpdateBlog updates an existing blog post (admin only)
func (c *BlogController) UpdateBlog(w http.ResponseWriter, r *http.Request) {
	blogIDStr := chi.URLParam(r, "blogID")
	blogID, err := uuid.Parse(blogIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	var req UpdateBlogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	blog, err := c.blogRepo.GetByID(r.Context(), blogID)
	if err != nil {
		RespondError(w, http.StatusNotFound, "Blog not found")
		return
	}

	// Update fields that were provided
	if req.Title != "" {
		blog.Title = req.Title
	}
	if req.Description != nil {
		blog.Description = req.Description
	}
	if req.Category != "" {
		blog.Category = req.Category
	}
	if req.Content != "" {
		blog.Content = req.Content
	}
	if req.CoverImageURL != nil {
		blog.CoverImageURL = req.CoverImageURL
	}
	if req.ReadTimeMinutes > 0 {
		blog.ReadTimeMinutes = req.ReadTimeMinutes
	}

	if err := c.blogRepo.Update(r.Context(), blog); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, blog)
}

// DeleteBlog deletes a blog post (admin only)
func (c *BlogController) DeleteBlog(w http.ResponseWriter, r *http.Request) {
	blogIDStr := chi.URLParam(r, "blogID")
	blogID, err := uuid.Parse(blogIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	if err := c.blogRepo.Delete(r.Context(), blogID); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]string{"message": "Blog deleted successfully"})
}

// PublishBlog publishes a blog post (admin only)
func (c *BlogController) PublishBlog(w http.ResponseWriter, r *http.Request) {
	blogIDStr := chi.URLParam(r, "blogID")
	blogID, err := uuid.Parse(blogIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	if err := c.blogRepo.Publish(r.Context(), blogID); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	blog, _ := c.blogRepo.GetByID(r.Context(), blogID)
	RespondSuccess(w, http.StatusOK, blog)
}

// UnpublishBlog unpublishes a blog post (admin only)
func (c *BlogController) UnpublishBlog(w http.ResponseWriter, r *http.Request) {
	blogIDStr := chi.URLParam(r, "blogID")
	blogID, err := uuid.Parse(blogIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid blog ID")
		return
	}

	if err := c.blogRepo.Unpublish(r.Context(), blogID); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	blog, _ := c.blogRepo.GetByID(r.Context(), blogID)
	RespondSuccess(w, http.StatusOK, blog)
}
