package admin

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/internal/services/category"
	"github.com/forgecommerce/api/templates/admin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// CategoryHandler holds dependencies for category admin HTTP handlers.
type CategoryHandler struct {
	categories *category.Service
	logger     *slog.Logger
}

// NewCategoryHandler creates a new category handler.
func NewCategoryHandler(categories *category.Service, logger *slog.Logger) *CategoryHandler {
	return &CategoryHandler{
		categories: categories,
		logger:     logger,
	}
}

// RegisterRoutes registers all category admin routes on the given mux.
func (h *CategoryHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/categories", h.ListCategories)
	mux.HandleFunc("GET /admin/categories/new", h.ShowNewCategory)
	mux.HandleFunc("POST /admin/categories", h.CreateCategory)
	mux.HandleFunc("GET /admin/categories/{id}", h.ShowEditCategory)
	mux.HandleFunc("POST /admin/categories/{id}", h.UpdateCategory)
	mux.HandleFunc("POST /admin/categories/{id}/delete", h.DeleteCategory)
}

// ListCategories renders the category list page.
func (h *CategoryHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	cats, err := h.categories.List(r.Context(), false)
	if err != nil {
		h.logger.Error("failed to list categories", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Build a map of category ID -> name for resolving parent names.
	nameByID := make(map[string]string, len(cats))
	for _, c := range cats {
		nameByID[c.ID.String()] = c.Name
	}

	items := make([]admin.CategoryListItem, 0, len(cats))
	for _, c := range cats {
		productCount, err := h.categories.CountProducts(r.Context(), c.ID)
		if err != nil {
			h.logger.Error("failed to count products for category",
				"category_id", c.ID.String(),
				"error", err,
			)
			productCount = 0
		}

		parentName := ""
		if pid := pgtypeUUIDString(c.ParentID); pid != "" {
			parentName = nameByID[pid]
		}

		items = append(items, admin.CategoryListItem{
			ID:           c.ID.String(),
			Name:         c.Name,
			Slug:         c.Slug,
			ProductCount: int(productCount),
			IsActive:     c.IsActive,
			ParentName:   parentName,
			Position:     int(c.Position),
		})
	}

	admin.CategoryListPage(items, csrfToken).Render(r.Context(), w)
}

// ShowNewCategory renders an empty category form for creating a new category.
func (h *CategoryHandler) ShowNewCategory(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	parents, err := h.buildParentOptions(r, uuid.Nil)
	if err != nil {
		h.logger.Error("failed to load parent categories", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := admin.CategoryFormData{
		IsNew:     true,
		IsActive:  true,
		Position:  "0",
		CSRFToken: csrfToken,
		Parents:   parents,
	}

	admin.CategoryFormPage(data).Render(r.Context(), w)
}

// CreateCategory processes the create category form submission.
func (h *CategoryHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	params, formData, err := h.parseCategoryForm(r, uuid.Nil, true)
	if err != nil {
		formData.CSRFToken = csrfToken
		formData.Error = err.Error()
		parents, _ := h.buildParentOptions(r, uuid.Nil)
		formData.Parents = parents
		w.WriteHeader(http.StatusUnprocessableEntity)
		admin.CategoryFormPage(formData).Render(r.Context(), w)
		return
	}

	cat, err := h.categories.Create(r.Context(), params)
	if err != nil {
		h.logger.Error("failed to create category", "error", err)
		formData.CSRFToken = csrfToken
		formData.Error = "Failed to create category. Please try again."
		parents, _ := h.buildParentOptions(r, uuid.Nil)
		formData.Parents = parents
		w.WriteHeader(http.StatusInternalServerError)
		admin.CategoryFormPage(formData).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/categories/%s", cat.ID.String()), http.StatusSeeOther)
}

// ShowEditCategory renders the category edit form.
func (h *CategoryHandler) ShowEditCategory(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	cat, err := h.categories.Get(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get category", "id", id.String(), "error", err)
		http.Error(w, "Category not found", http.StatusNotFound)
		return
	}

	parents, err := h.buildParentOptions(r, id)
	if err != nil {
		h.logger.Error("failed to load parent categories", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := admin.CategoryFormData{
		ID:          cat.ID.String(),
		Name:        cat.Name,
		Slug:        cat.Slug,
		Description: derefStr(cat.Description),
		ParentID:    pgtypeUUIDString(cat.ParentID),
		Position:    strconv.Itoa(int(cat.Position)),
		SEOTitle:    derefStr(cat.SeoTitle),
		SEODesc:     derefStr(cat.SeoDescription),
		IsActive:    cat.IsActive,
		IsNew:       false,
		CSRFToken:   csrfToken,
		Parents:     parents,
	}

	admin.CategoryFormPage(data).Render(r.Context(), w)
}

// UpdateCategory processes the update category form submission.
func (h *CategoryHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	createParams, formData, err := h.parseCategoryForm(r, id, false)
	if err != nil {
		formData.CSRFToken = csrfToken
		formData.Error = err.Error()
		parents, _ := h.buildParentOptions(r, id)
		formData.Parents = parents
		w.WriteHeader(http.StatusUnprocessableEntity)
		admin.CategoryFormPage(formData).Render(r.Context(), w)
		return
	}

	updateParams := category.UpdateCategoryParams{
		Name:           createParams.Name,
		Slug:           createParams.Slug,
		Description:    createParams.Description,
		ParentID:       createParams.ParentID,
		Position:       createParams.Position,
		ImageUrl:       createParams.ImageUrl,
		SeoTitle:       createParams.SeoTitle,
		SeoDescription: createParams.SeoDescription,
		IsActive:       createParams.IsActive,
	}

	_, err = h.categories.Update(r.Context(), id, updateParams)
	if err != nil {
		h.logger.Error("failed to update category", "id", id.String(), "error", err)
		formData.CSRFToken = csrfToken
		formData.Error = "Failed to update category. Please try again."
		parents, _ := h.buildParentOptions(r, id)
		formData.Parents = parents
		w.WriteHeader(http.StatusInternalServerError)
		admin.CategoryFormPage(formData).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/categories/%s", id.String()), http.StatusSeeOther)
}

// DeleteCategory deletes a category and redirects to the list.
func (h *CategoryHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	if err := h.categories.Delete(r.Context(), id); err != nil {
		h.logger.Error("failed to delete category", "id", id.String(), "error", err)
		http.Error(w, "Failed to delete category", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/categories", http.StatusSeeOther)
}

// --- Helper methods ---

// parseCategoryForm extracts category fields from the form and returns both
// a CreateCategoryParams (used for both create and update) and a CategoryFormData
// (for re-rendering the form on validation errors). The currentID is used for
// form data population; pass uuid.Nil for new categories.
func (h *CategoryHandler) parseCategoryForm(r *http.Request, currentID uuid.UUID, isNew bool) (category.CreateCategoryParams, admin.CategoryFormData, error) {
	name := r.FormValue("name")
	slug := r.FormValue("slug")
	description := r.FormValue("description")
	parentIDStr := r.FormValue("parent_id")
	positionStr := r.FormValue("position")
	seoTitle := r.FormValue("seo_title")
	seoDesc := r.FormValue("seo_description")
	isActive := r.FormValue("is_active") != ""

	formData := admin.CategoryFormData{
		Name:        name,
		Slug:        slug,
		Description: description,
		ParentID:    parentIDStr,
		Position:    positionStr,
		SEOTitle:    seoTitle,
		SEODesc:     seoDesc,
		IsActive:    isActive,
		IsNew:       isNew,
	}
	if currentID != uuid.Nil {
		formData.ID = currentID.String()
	}

	if name == "" {
		return category.CreateCategoryParams{}, formData, fmt.Errorf("name is required")
	}

	position, err := strconv.Atoi(positionStr)
	if err != nil {
		position = 0
	}

	var parentID *uuid.UUID
	if parentIDStr != "" {
		pid, err := uuid.Parse(parentIDStr)
		if err != nil {
			return category.CreateCategoryParams{}, formData, fmt.Errorf("invalid parent category ID")
		}
		parentID = &pid
	}

	params := category.CreateCategoryParams{
		Name:     name,
		Slug:     slug,
		Position: int32(position),
		IsActive: isActive,
	}

	if description != "" {
		params.Description = &description
	}
	if seoTitle != "" {
		params.SeoTitle = &seoTitle
	}
	if seoDesc != "" {
		params.SeoDescription = &seoDesc
	}
	if parentID != nil {
		params.ParentID = parentID
	}

	return params, formData, nil
}

// buildParentOptions loads all categories and builds a list of CategoryOption
// values suitable for a parent-category dropdown. The category identified by
// excludeID is filtered out so a category cannot be its own parent.
func (h *CategoryHandler) buildParentOptions(r *http.Request, excludeID uuid.UUID) ([]admin.CategoryOption, error) {
	cats, err := h.categories.List(r.Context(), false)
	if err != nil {
		return nil, fmt.Errorf("listing categories for parent options: %w", err)
	}

	options := make([]admin.CategoryOption, 0, len(cats))
	for _, c := range cats {
		if excludeID != uuid.Nil && c.ID == excludeID {
			continue
		}
		options = append(options, admin.CategoryOption{
			ID:   c.ID.String(),
			Name: c.Name,
		})
	}

	return options, nil
}

// pgtypeUUIDString converts a pgtype.UUID to its string representation.
// Returns an empty string if the UUID is not valid (NULL).
func pgtypeUUIDString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

// derefStr safely dereferences a *string, returning an empty string for nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
