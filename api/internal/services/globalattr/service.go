package globalattr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
)

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

var (
	// ErrNotFound is returned when a global attribute does not exist.
	ErrNotFound = errors.New("global attribute not found")

	// ErrFieldNotFound is returned when a metadata field does not exist.
	ErrFieldNotFound = errors.New("metadata field not found")

	// ErrOptionNotFound is returned when a global attribute option does not exist.
	ErrOptionNotFound = errors.New("global option not found")

	// ErrLinkNotFound is returned when a product-global-attribute link does not exist.
	ErrLinkNotFound = errors.New("product global attribute link not found")

	// ErrNameRequired is returned when a global attribute name is empty.
	ErrNameRequired = errors.New("global attribute name is required")

	// ErrValueRequired is returned when an option value is empty.
	ErrValueRequired = errors.New("option value is required")

	// ErrFieldNameRequired is returned when a metadata field name is empty.
	ErrFieldNameRequired = errors.New("field name is required")

	// ErrRoleNameRequired is returned when a link role name is empty.
	ErrRoleNameRequired = errors.New("role name is required")

	// ErrInUse is returned when attempting to delete an attribute that is linked to products.
	ErrInUse = errors.New("global attribute is in use by products and cannot be deleted")
)

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// Service provides business logic for global attribute CRUD operations.
type Service struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *slog.Logger
}

// NewService creates a new global attribute service.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		pool:    pool,
		queries: db.New(pool),
		logger:  logger,
	}
}

// ---------------------------------------------------------------------------
// Param structs
// ---------------------------------------------------------------------------

// CreateParams contains the input fields for creating a global attribute.
type CreateParams struct {
	Name          string
	DisplayName   string
	Description   *string
	AttributeType string // select, color_swatch, button_group, image_swatch
	Category      *string
	Position      int32
	IsActive      bool
}

// UpdateParams contains the input fields for updating a global attribute.
type UpdateParams struct {
	Name          string
	DisplayName   string
	Description   *string
	AttributeType string
	Category      *string
	Position      int32
	IsActive      bool
}

// CreateFieldParams contains input for creating a metadata field.
type CreateFieldParams struct {
	GlobalAttributeID uuid.UUID
	FieldName         string
	DisplayName       string
	FieldType         string // text, number, boolean, select, url
	IsRequired        bool
	DefaultValue      *string
	SelectOptions     []string
	HelpText          *string
	Position          int32
}

// CreateOptionParams contains the input fields for creating a global attribute option.
// Metadata accepts json.RawMessage, map[string]string, or any JSON-serializable value.
type CreateOptionParams struct {
	GlobalAttributeID uuid.UUID
	Value             string
	DisplayValue      string
	ColorHex          *string
	ImageURL          *string
	Metadata          interface{}
	Position          int32
	IsActive          bool
}

// UpdateOptionParams contains the input fields for updating a global attribute option.
// Metadata accepts json.RawMessage, map[string]string, or any JSON-serializable value.
type UpdateOptionParams struct {
	Value        string
	DisplayValue string
	ColorHex     *string
	ImageURL     *string
	Metadata     interface{}
	Position     int32
	IsActive     bool
}

// CreateLinkParams contains input for linking a product to a global attribute.
type CreateLinkParams struct {
	ProductID           uuid.UUID
	GlobalAttributeID   uuid.UUID
	RoleName            string
	RoleDisplayName     string
	Position            int32
	AffectsPricing      bool
	AffectsShipping     bool
	PriceModifierField  *string
	WeightModifierField *string
}

// UpdateLinkParams contains input for updating a product-global-attribute link.
type UpdateLinkParams struct {
	RoleName            string
	RoleDisplayName     string
	Position            int32
	AffectsPricing      bool
	AffectsShipping     bool
	PriceModifierField  *string
	WeightModifierField *string
}

// SelectionInput contains input for a single option selection entry.
type SelectionInput struct {
	GlobalOptionID      uuid.UUID
	PriceModifier       pgtype.Numeric
	WeightModifierGrams *int32
	PositionOverride    *int32
}

// ---------------------------------------------------------------------------
// Global Attributes
// ---------------------------------------------------------------------------

// ListAll returns all global attributes, ordered by position then name.
func (s *Service) ListAll(ctx context.Context) ([]db.GlobalAttribute, error) {
	attrs, err := s.queries.ListGlobalAttributes(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing global attributes: %w", err)
	}
	return attrs, nil
}

// ListByCategory returns global attributes filtered by category.
func (s *Service) ListByCategory(ctx context.Context, category string) ([]db.GlobalAttribute, error) {
	cat := category
	attrs, err := s.queries.ListGlobalAttributesByCategory(ctx, &cat)
	if err != nil {
		return nil, fmt.Errorf("listing global attributes by category %q: %w", category, err)
	}
	return attrs, nil
}

// Get returns a single global attribute by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.GlobalAttribute, error) {
	attr, err := s.queries.GetGlobalAttribute(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.GlobalAttribute{}, ErrNotFound
		}
		return db.GlobalAttribute{}, fmt.Errorf("getting global attribute %s: %w", id, err)
	}
	return attr, nil
}

// GetByName returns a single global attribute by its unique name.
func (s *Service) GetByName(ctx context.Context, name string) (db.GlobalAttribute, error) {
	attr, err := s.queries.GetGlobalAttributeByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.GlobalAttribute{}, ErrNotFound
		}
		return db.GlobalAttribute{}, fmt.Errorf("getting global attribute by name %q: %w", name, err)
	}
	return attr, nil
}

// Create creates a new global attribute with an auto-generated UUID.
func (s *Service) Create(ctx context.Context, params CreateParams) (db.GlobalAttribute, error) {
	if params.Name == "" {
		return db.GlobalAttribute{}, ErrNameRequired
	}

	if params.AttributeType == "" {
		params.AttributeType = "select"
	}

	attr, err := s.queries.CreateGlobalAttribute(ctx, db.CreateGlobalAttributeParams{
		ID:            uuid.New(),
		Name:          params.Name,
		DisplayName:   params.DisplayName,
		Description:   params.Description,
		AttributeType: params.AttributeType,
		Category:      params.Category,
		Position:      params.Position,
		IsActive:      params.IsActive,
	})
	if err != nil {
		if isDuplicateKeyError(err) {
			return db.GlobalAttribute{}, fmt.Errorf("global attribute name %q already exists: %w", params.Name, err)
		}
		return db.GlobalAttribute{}, fmt.Errorf("creating global attribute: %w", err)
	}

	s.logger.Info("global attribute created",
		slog.String("id", attr.ID.String()),
		slog.String("name", attr.Name),
	)

	return attr, nil
}

// Update updates an existing global attribute.
func (s *Service) Update(ctx context.Context, id uuid.UUID, params UpdateParams) (db.GlobalAttribute, error) {
	if params.Name == "" {
		return db.GlobalAttribute{}, ErrNameRequired
	}

	// Verify the attribute exists before updating.
	_, err := s.queries.GetGlobalAttribute(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.GlobalAttribute{}, ErrNotFound
		}
		return db.GlobalAttribute{}, fmt.Errorf("fetching global attribute for update: %w", err)
	}

	attr, err := s.queries.UpdateGlobalAttribute(ctx, db.UpdateGlobalAttributeParams{
		ID:            id,
		Name:          params.Name,
		DisplayName:   params.DisplayName,
		Description:   params.Description,
		AttributeType: params.AttributeType,
		Category:      params.Category,
		Position:      params.Position,
		IsActive:      params.IsActive,
	})
	if err != nil {
		if isDuplicateKeyError(err) {
			return db.GlobalAttribute{}, fmt.Errorf("global attribute name %q already exists: %w", params.Name, err)
		}
		return db.GlobalAttribute{}, fmt.Errorf("updating global attribute %s: %w", id, err)
	}

	s.logger.Info("global attribute updated",
		slog.String("id", attr.ID.String()),
		slog.String("name", attr.Name),
	)

	return attr, nil
}

// Delete deletes a global attribute by ID.
// Returns ErrNotFound if the attribute does not exist, or ErrInUse if products
// are linked to it.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// Verify existence first so callers get a clear ErrNotFound.
	_, err := s.queries.GetGlobalAttribute(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("fetching global attribute for delete: %w", err)
	}

	// Check whether any products are using this attribute.
	count, err := s.queries.CountGlobalAttributeUsage(ctx, id)
	if err != nil {
		return fmt.Errorf("counting usage for global attribute %s: %w", id, err)
	}
	if count > 0 {
		return ErrInUse
	}

	if err := s.queries.DeleteGlobalAttribute(ctx, id); err != nil {
		return fmt.Errorf("deleting global attribute %s: %w", id, err)
	}

	s.logger.Info("global attribute deleted", slog.String("id", id.String()))
	return nil
}

// CountUsage returns the number of products linked to a global attribute.
func (s *Service) CountUsage(ctx context.Context, id uuid.UUID) (int64, error) {
	count, err := s.queries.CountGlobalAttributeUsage(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("counting usage for global attribute %s: %w", id, err)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Metadata Fields
// ---------------------------------------------------------------------------

// ListFields returns all metadata field definitions for a global attribute.
func (s *Service) ListFields(ctx context.Context, globalAttributeID uuid.UUID) ([]db.GlobalAttributeMetadataField, error) {
	fields, err := s.queries.ListMetadataFields(ctx, globalAttributeID)
	if err != nil {
		return nil, fmt.Errorf("listing metadata fields for global attribute %s: %w", globalAttributeID, err)
	}
	return fields, nil
}

// CreateField creates a new metadata field definition.
func (s *Service) CreateField(ctx context.Context, params CreateFieldParams) (db.GlobalAttributeMetadataField, error) {
	if params.FieldName == "" {
		return db.GlobalAttributeMetadataField{}, ErrFieldNameRequired
	}

	if params.FieldType == "" {
		params.FieldType = "text"
	}

	field, err := s.queries.CreateMetadataField(ctx, db.CreateMetadataFieldParams{
		ID:                uuid.New(),
		GlobalAttributeID: params.GlobalAttributeID,
		FieldName:         params.FieldName,
		DisplayName:       params.DisplayName,
		FieldType:         params.FieldType,
		IsRequired:        params.IsRequired,
		DefaultValue:      params.DefaultValue,
		SelectOptions:     params.SelectOptions,
		HelpText:          params.HelpText,
		Position:          params.Position,
	})
	if err != nil {
		return db.GlobalAttributeMetadataField{}, fmt.Errorf("creating metadata field: %w", err)
	}

	s.logger.Info("metadata field created",
		slog.String("id", field.ID.String()),
		slog.String("field_name", field.FieldName),
		slog.String("global_attribute_id", field.GlobalAttributeID.String()),
	)

	return field, nil
}

// DeleteField deletes a metadata field by ID.
func (s *Service) DeleteField(ctx context.Context, id uuid.UUID) error {
	_, err := s.queries.GetMetadataField(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrFieldNotFound
		}
		return fmt.Errorf("fetching metadata field for delete: %w", err)
	}

	if err := s.queries.DeleteMetadataField(ctx, id); err != nil {
		return fmt.Errorf("deleting metadata field %s: %w", id, err)
	}

	s.logger.Info("metadata field deleted", slog.String("id", id.String()))
	return nil
}

// ---------------------------------------------------------------------------
// Global Attribute Options
// ---------------------------------------------------------------------------

// ListOptions returns all options for a global attribute, ordered by position.
func (s *Service) ListOptions(ctx context.Context, globalAttributeID uuid.UUID) ([]db.GlobalAttributeOption, error) {
	options, err := s.queries.ListGlobalAttributeOptions(ctx, globalAttributeID)
	if err != nil {
		return nil, fmt.Errorf("listing options for global attribute %s: %w", globalAttributeID, err)
	}
	return options, nil
}

// GetOption returns a single global attribute option by ID.
func (s *Service) GetOption(ctx context.Context, id uuid.UUID) (db.GlobalAttributeOption, error) {
	opt, err := s.queries.GetGlobalAttributeOption(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.GlobalAttributeOption{}, ErrOptionNotFound
		}
		return db.GlobalAttributeOption{}, fmt.Errorf("getting global attribute option %s: %w", id, err)
	}
	return opt, nil
}

// CreateOption creates a new global attribute option with an auto-generated UUID.
func (s *Service) CreateOption(ctx context.Context, params CreateOptionParams) (db.GlobalAttributeOption, error) {
	if params.Value == "" {
		return db.GlobalAttributeOption{}, ErrValueRequired
	}

	metadataJSON := marshalMetadata(params.Metadata)

	opt, err := s.queries.CreateGlobalAttributeOption(ctx, db.CreateGlobalAttributeOptionParams{
		ID:                uuid.New(),
		GlobalAttributeID: params.GlobalAttributeID,
		Value:             params.Value,
		DisplayValue:      params.DisplayValue,
		ColorHex:          params.ColorHex,
		ImageUrl:          params.ImageURL,
		Metadata:          metadataJSON,
		Position:          params.Position,
		IsActive:          params.IsActive,
	})
	if err != nil {
		return db.GlobalAttributeOption{}, fmt.Errorf("creating global attribute option: %w", err)
	}

	s.logger.Info("global attribute option created",
		slog.String("id", opt.ID.String()),
		slog.String("global_attribute_id", opt.GlobalAttributeID.String()),
		slog.String("value", opt.Value),
	)

	return opt, nil
}

// UpdateOption updates an existing global attribute option.
func (s *Service) UpdateOption(ctx context.Context, id uuid.UUID, params UpdateOptionParams) (db.GlobalAttributeOption, error) {
	if params.Value == "" {
		return db.GlobalAttributeOption{}, ErrValueRequired
	}

	metadataJSON := marshalMetadata(params.Metadata)

	// Verify the option exists before updating.
	_, err := s.queries.GetGlobalAttributeOption(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.GlobalAttributeOption{}, ErrOptionNotFound
		}
		return db.GlobalAttributeOption{}, fmt.Errorf("fetching global attribute option for update: %w", err)
	}

	opt, err := s.queries.UpdateGlobalAttributeOption(ctx, db.UpdateGlobalAttributeOptionParams{
		ID:           id,
		Value:        params.Value,
		DisplayValue: params.DisplayValue,
		ColorHex:     params.ColorHex,
		ImageUrl:     params.ImageURL,
		Metadata:     metadataJSON,
		Position:     params.Position,
		IsActive:     params.IsActive,
	})
	if err != nil {
		return db.GlobalAttributeOption{}, fmt.Errorf("updating global attribute option %s: %w", id, err)
	}

	s.logger.Info("global attribute option updated",
		slog.String("id", opt.ID.String()),
		slog.String("value", opt.Value),
	)

	return opt, nil
}

// DeleteOption deletes a global attribute option by ID.
func (s *Service) DeleteOption(ctx context.Context, id uuid.UUID) error {
	_, err := s.queries.GetGlobalAttributeOption(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOptionNotFound
		}
		return fmt.Errorf("fetching global attribute option for delete: %w", err)
	}

	if err := s.queries.DeleteGlobalAttributeOption(ctx, id); err != nil {
		return fmt.Errorf("deleting global attribute option %s: %w", id, err)
	}

	s.logger.Info("global attribute option deleted", slog.String("id", id.String()))
	return nil
}

// ---------------------------------------------------------------------------
// Product Global Attribute Links
// ---------------------------------------------------------------------------

// ListLinks returns all global attribute links for a product, ordered by position.
func (s *Service) ListLinks(ctx context.Context, productID uuid.UUID) ([]db.ProductGlobalAttributeLink, error) {
	links, err := s.queries.ListProductGlobalAttributeLinks(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("listing product global attribute links for product %s: %w", productID, err)
	}
	return links, nil
}

// GetLink returns a single product-global-attribute link by ID.
func (s *Service) GetLink(ctx context.Context, id uuid.UUID) (db.ProductGlobalAttributeLink, error) {
	link, err := s.queries.GetProductGlobalAttributeLink(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductGlobalAttributeLink{}, ErrLinkNotFound
		}
		return db.ProductGlobalAttributeLink{}, fmt.Errorf("getting product global attribute link %s: %w", id, err)
	}
	return link, nil
}

// CreateLink creates a new link between a product and a global attribute.
func (s *Service) CreateLink(ctx context.Context, params CreateLinkParams) (db.ProductGlobalAttributeLink, error) {
	if params.RoleName == "" {
		return db.ProductGlobalAttributeLink{}, ErrRoleNameRequired
	}

	link, err := s.queries.CreateProductGlobalAttributeLink(ctx, db.CreateProductGlobalAttributeLinkParams{
		ID:                  uuid.New(),
		ProductID:           params.ProductID,
		GlobalAttributeID:   params.GlobalAttributeID,
		RoleName:            params.RoleName,
		RoleDisplayName:     params.RoleDisplayName,
		Position:            params.Position,
		AffectsPricing:      params.AffectsPricing,
		AffectsShipping:     params.AffectsShipping,
		PriceModifierField:  params.PriceModifierField,
		WeightModifierField: params.WeightModifierField,
	})
	if err != nil {
		return db.ProductGlobalAttributeLink{}, fmt.Errorf("creating product global attribute link: %w", err)
	}

	s.logger.Info("product global attribute link created",
		slog.String("id", link.ID.String()),
		slog.String("product_id", link.ProductID.String()),
		slog.String("global_attribute_id", link.GlobalAttributeID.String()),
		slog.String("role_name", link.RoleName),
	)

	return link, nil
}

// UpdateLink updates an existing product-global-attribute link.
func (s *Service) UpdateLink(ctx context.Context, id uuid.UUID, params UpdateLinkParams) (db.ProductGlobalAttributeLink, error) {
	if params.RoleName == "" {
		return db.ProductGlobalAttributeLink{}, ErrRoleNameRequired
	}

	// Verify the link exists.
	_, err := s.queries.GetProductGlobalAttributeLink(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductGlobalAttributeLink{}, ErrLinkNotFound
		}
		return db.ProductGlobalAttributeLink{}, fmt.Errorf("fetching product link for update: %w", err)
	}

	link, err := s.queries.UpdateProductGlobalAttributeLink(ctx, db.UpdateProductGlobalAttributeLinkParams{
		ID:                  id,
		RoleName:            params.RoleName,
		RoleDisplayName:     params.RoleDisplayName,
		Position:            params.Position,
		AffectsPricing:      params.AffectsPricing,
		AffectsShipping:     params.AffectsShipping,
		PriceModifierField:  params.PriceModifierField,
		WeightModifierField: params.WeightModifierField,
	})
	if err != nil {
		return db.ProductGlobalAttributeLink{}, fmt.Errorf("updating product link %s: %w", id, err)
	}

	s.logger.Info("product global attribute link updated",
		slog.String("id", link.ID.String()),
		slog.String("role_name", link.RoleName),
	)

	return link, nil
}

// DeleteLink deletes a product-global-attribute link by ID. Cascades to selections.
func (s *Service) DeleteLink(ctx context.Context, id uuid.UUID) error {
	_, err := s.queries.GetProductGlobalAttributeLink(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLinkNotFound
		}
		return fmt.Errorf("fetching product link for delete: %w", err)
	}

	if err := s.queries.DeleteProductGlobalAttributeLink(ctx, id); err != nil {
		return fmt.Errorf("deleting product link %s: %w", id, err)
	}

	s.logger.Info("product global attribute link deleted", slog.String("id", id.String()))
	return nil
}

// ListProductsUsing returns a summary of products using a given global attribute.
func (s *Service) ListProductsUsing(ctx context.Context, globalAttributeID uuid.UUID) ([]db.ListProductsUsingGlobalAttributeRow, error) {
	products, err := s.queries.ListProductsUsingGlobalAttribute(ctx, globalAttributeID)
	if err != nil {
		return nil, fmt.Errorf("listing products using global attribute %s: %w", globalAttributeID, err)
	}
	return products, nil
}

// ---------------------------------------------------------------------------
// Option Selections
// ---------------------------------------------------------------------------

// ListSelections returns all option selections for a product link.
func (s *Service) ListSelections(ctx context.Context, linkID uuid.UUID) ([]db.ProductGlobalOptionSelection, error) {
	sels, err := s.queries.ListOptionSelections(ctx, linkID)
	if err != nil {
		return nil, fmt.Errorf("listing option selections for link %s: %w", linkID, err)
	}
	return sels, nil
}

// SetSelections replaces all option selections for a link in a single transaction.
// It deletes existing selections and inserts the new set.
func (s *Service) SetSelections(ctx context.Context, linkID uuid.UUID, selections []SelectionInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction for set option selections: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := s.queries.WithTx(tx)

	// Delete all existing selections for this link.
	if err := qtx.DeleteAllOptionSelections(ctx, linkID); err != nil {
		return fmt.Errorf("deleting existing option selections for link %s: %w", linkID, err)
	}

	// Insert new selections.
	for _, sel := range selections {
		_, err := qtx.CreateOptionSelection(ctx, db.CreateOptionSelectionParams{
			ID:                  uuid.New(),
			LinkID:              linkID,
			GlobalOptionID:      sel.GlobalOptionID,
			PriceModifier:       sel.PriceModifier,
			WeightModifierGrams: sel.WeightModifierGrams,
			PositionOverride:    sel.PositionOverride,
		})
		if err != nil {
			return fmt.Errorf("inserting option selection for link %s, option %s: %w", linkID, sel.GlobalOptionID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing option selections for link %s: %w", linkID, err)
	}

	s.logger.Info("option selections updated",
		slog.String("link_id", linkID.String()),
		slog.Int("count", len(selections)),
	)

	return nil
}

// DeleteAllSelections removes all option selections for a product link.
func (s *Service) DeleteAllSelections(ctx context.Context, linkID uuid.UUID) error {
	if err := s.queries.DeleteAllOptionSelections(ctx, linkID); err != nil {
		return fmt.Errorf("deleting all option selections for link %s: %w", linkID, err)
	}

	s.logger.Info("all option selections deleted", slog.String("link_id", linkID.String()))
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// marshalMetadata converts the metadata value to json.RawMessage.
// Accepts json.RawMessage, map[string]string, or any JSON-serializable value.
// Returns "{}" for nil input.
func marshalMetadata(v interface{}) json.RawMessage {
	if v == nil {
		return json.RawMessage("{}")
	}
	// If already json.RawMessage or []byte, use directly.
	switch m := v.(type) {
	case json.RawMessage:
		if len(m) == 0 {
			return json.RawMessage("{}")
		}
		return m
	case []byte:
		if len(m) == 0 {
			return json.RawMessage("{}")
		}
		return json.RawMessage(m)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return json.RawMessage("{}")
		}
		return data
	}
}

// isDuplicateKeyError checks if a PostgreSQL error is a unique constraint violation (23505).
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

// numericToString converts a pgtype.Numeric to a string representation.
func numericToString(n pgtype.Numeric) string {
	if !n.Valid {
		return ""
	}
	f, _ := n.Float64Value()
	if !f.Valid {
		return ""
	}
	return fmt.Sprintf("%.2f", f.Float64)
}

// ---------------------------------------------------------------------------
// Handler-compatible aliases and types
// ---------------------------------------------------------------------------
// These methods and types provide backward compatibility for admin handlers
// that use a slightly different naming convention. They delegate to the
// canonical methods defined above.

// ProductLink is a simplified representation of a product-global-attribute link
// used by admin handlers.
type ProductLink struct {
	ID                uuid.UUID
	GlobalAttributeID uuid.UUID
	Role              string
	AffectsPricing    bool
	AffectsShipping   bool
}

// OptionWithMetadata represents an option with parsed metadata for template rendering.
type OptionWithMetadata struct {
	ID           uuid.UUID
	Value        string
	DisplayValue string
	ColorHex     *string
	ImageURL     *string
	Metadata     map[string]string
	Position     int32
	IsActive     bool
}

// CreateAttributeParams is an alias for CreateParams (handler compatibility).
type CreateAttributeParams = CreateParams

// UpdateAttributeParams is an alias for UpdateParams (handler compatibility).
type UpdateAttributeParams = UpdateParams

// CreateMetadataFieldParams is an alias for CreateFieldParams (handler compatibility).
type CreateMetadataFieldParams = CreateFieldParams

// LinkToProductParams contains input for linking a global attribute to a product.
type LinkToProductParams struct {
	ProductID         uuid.UUID
	GlobalAttributeID uuid.UUID
	Role              string
	AffectsPricing    bool
	AffectsShipping   bool
}

// UpdateSelectionsParams contains input for updating option selections for a link.
type UpdateSelectionsParams struct {
	LinkID          uuid.UUID
	ProductID       uuid.UUID
	SelectedOptions []uuid.UUID
	PriceModifiers  map[uuid.UUID]string
}

// ListAttributes is an alias for ListAll (handler compatibility).
func (s *Service) ListAttributes(ctx context.Context) ([]db.GlobalAttribute, error) {
	return s.ListAll(ctx)
}

// GetAttribute is an alias for Get (handler compatibility).
func (s *Service) GetAttribute(ctx context.Context, id uuid.UUID) (db.GlobalAttribute, error) {
	return s.Get(ctx, id)
}

// CreateAttribute is an alias for Create (handler compatibility).
func (s *Service) CreateAttribute(ctx context.Context, params CreateParams) (db.GlobalAttribute, error) {
	return s.Create(ctx, params)
}

// UpdateAttribute is an alias for Update (handler compatibility).
func (s *Service) UpdateAttribute(ctx context.Context, id uuid.UUID, params UpdateParams) (db.GlobalAttribute, error) {
	return s.Update(ctx, id, params)
}

// DeleteAttribute is an alias for Delete (handler compatibility).
func (s *Service) DeleteAttribute(ctx context.Context, id uuid.UUID) error {
	return s.Delete(ctx, id)
}

// CountOptions returns the number of options for a global attribute.
func (s *Service) CountOptions(ctx context.Context, globalAttributeID uuid.UUID) (int64, error) {
	opts, err := s.queries.ListGlobalAttributeOptions(ctx, globalAttributeID)
	if err != nil {
		return 0, fmt.Errorf("counting options for global attribute %s: %w", globalAttributeID, err)
	}
	return int64(len(opts)), nil
}

// CountUsages is an alias for CountUsage (handler compatibility).
func (s *Service) CountUsages(ctx context.Context, id uuid.UUID) (int64, error) {
	return s.CountUsage(ctx, id)
}

// ListMetadataFields is an alias for ListFields (handler compatibility).
func (s *Service) ListMetadataFields(ctx context.Context, globalAttributeID uuid.UUID) ([]db.GlobalAttributeMetadataField, error) {
	return s.ListFields(ctx, globalAttributeID)
}

// CreateMetadataField is an alias for CreateField (handler compatibility).
func (s *Service) CreateMetadataField(ctx context.Context, params CreateFieldParams) (db.GlobalAttributeMetadataField, error) {
	return s.CreateField(ctx, params)
}

// DeleteMetadataField is an alias for DeleteField (handler compatibility).
func (s *Service) DeleteMetadataField(ctx context.Context, id uuid.UUID) error {
	return s.DeleteField(ctx, id)
}

// ListOptionsWithMetadata returns options with parsed metadata maps for template rendering.
func (s *Service) ListOptionsWithMetadata(ctx context.Context, globalAttributeID uuid.UUID) ([]OptionWithMetadata, error) {
	opts, err := s.queries.ListGlobalAttributeOptions(ctx, globalAttributeID)
	if err != nil {
		return nil, fmt.Errorf("listing options with metadata for global attribute %s: %w", globalAttributeID, err)
	}

	result := make([]OptionWithMetadata, 0, len(opts))
	for _, o := range opts {
		meta := make(map[string]string)
		if len(o.Metadata) > 0 {
			_ = json.Unmarshal(o.Metadata, &meta)
		}
		result = append(result, OptionWithMetadata{
			ID:           o.ID,
			Value:        o.Value,
			DisplayValue: o.DisplayValue,
			ColorHex:     o.ColorHex,
			ImageURL:     o.ImageUrl,
			Metadata:     meta,
			Position:     o.Position,
			IsActive:     o.IsActive,
		})
	}

	return result, nil
}

// ListProductLinks returns product-global-attribute links as simplified ProductLink values.
func (s *Service) ListProductLinks(ctx context.Context, productID uuid.UUID) ([]ProductLink, error) {
	links, err := s.queries.ListProductGlobalAttributeLinks(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("listing product links for product %s: %w", productID, err)
	}

	result := make([]ProductLink, 0, len(links))
	for _, l := range links {
		result = append(result, ProductLink{
			ID:                l.ID,
			GlobalAttributeID: l.GlobalAttributeID,
			Role:              l.RoleName,
			AffectsPricing:    l.AffectsPricing,
			AffectsShipping:   l.AffectsShipping,
		})
	}

	return result, nil
}

// GetProductLink returns a single product-global-attribute link as a ProductLink.
func (s *Service) GetProductLink(ctx context.Context, id uuid.UUID) (ProductLink, error) {
	link, err := s.queries.GetProductGlobalAttributeLink(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProductLink{}, ErrLinkNotFound
		}
		return ProductLink{}, fmt.Errorf("getting product link %s: %w", id, err)
	}
	return ProductLink{
		ID:                link.ID,
		GlobalAttributeID: link.GlobalAttributeID,
		Role:              link.RoleName,
		AffectsPricing:    link.AffectsPricing,
		AffectsShipping:   link.AffectsShipping,
	}, nil
}

// LinkToProduct creates a new link between a product and a global attribute using
// the simplified LinkToProductParams.
func (s *Service) LinkToProduct(ctx context.Context, params LinkToProductParams) (ProductLink, error) {
	link, err := s.CreateLink(ctx, CreateLinkParams{
		ProductID:         params.ProductID,
		GlobalAttributeID: params.GlobalAttributeID,
		RoleName:          params.Role,
		RoleDisplayName:   params.Role,
		AffectsPricing:    params.AffectsPricing,
		AffectsShipping:   params.AffectsShipping,
	})
	if err != nil {
		return ProductLink{}, err
	}
	return ProductLink{
		ID:                link.ID,
		GlobalAttributeID: link.GlobalAttributeID,
		Role:              link.RoleName,
		AffectsPricing:    link.AffectsPricing,
		AffectsShipping:   link.AffectsShipping,
	}, nil
}

// UnlinkFromProduct is an alias for DeleteLink (handler compatibility).
func (s *Service) UnlinkFromProduct(ctx context.Context, linkID uuid.UUID) error {
	return s.DeleteLink(ctx, linkID)
}

// GetSelectedOptions returns the selected option IDs and their price modifiers for a link.
func (s *Service) GetSelectedOptions(ctx context.Context, linkID uuid.UUID) ([]uuid.UUID, map[uuid.UUID]string, error) {
	sels, err := s.queries.ListOptionSelections(ctx, linkID)
	if err != nil {
		return nil, nil, fmt.Errorf("listing selections for link %s: %w", linkID, err)
	}

	ids := make([]uuid.UUID, 0, len(sels))
	modifiers := make(map[uuid.UUID]string)
	for _, sel := range sels {
		ids = append(ids, sel.GlobalOptionID)
		pm := numericToString(sel.PriceModifier)
		if pm != "" && pm != "0.00" {
			modifiers[sel.GlobalOptionID] = pm
		}
	}

	return ids, modifiers, nil
}

// UpdateProductOptionSelections replaces option selections for a link based on
// selected option IDs and optional price modifier strings.
func (s *Service) UpdateProductOptionSelections(ctx context.Context, params UpdateSelectionsParams) error {
	selections := make([]SelectionInput, 0, len(params.SelectedOptions))
	for i, optID := range params.SelectedOptions {
		var pm pgtype.Numeric
		if pmStr, ok := params.PriceModifiers[optID]; ok && pmStr != "" {
			_ = pm.Scan(pmStr)
		}
		pos := int32(i)
		selections = append(selections, SelectionInput{
			GlobalOptionID:   optID,
			PriceModifier:    pm,
			PositionOverride: &pos,
		})
	}
	return s.SetSelections(ctx, params.LinkID, selections)
}
