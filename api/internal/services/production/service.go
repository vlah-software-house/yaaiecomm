package production

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/forgecommerce/api/internal/database/gen"
)

var (
	// ErrNotFound is returned when a production batch does not exist.
	ErrNotFound = errors.New("production batch not found")

	// ErrInvalidStatus is returned when a status transition is not allowed.
	ErrInvalidStatus = errors.New("invalid status transition")
)

// Service provides business logic for production batch operations.
type Service struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

// NewService creates a new production batch service.
func NewService(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		queries: db.New(pool),
		pool:    pool,
		logger:  logger,
	}
}

// CreateBatchParams contains the input fields for creating a production batch.
type CreateBatchParams struct {
	ProductID     uuid.UUID
	VariantID     pgtype.UUID
	PlannedQty    int
	ScheduledDate string // "2006-01-02" format, optional
	Notes         string
	CreatedBy     pgtype.UUID
}

// CreateBatch creates a new production batch with an auto-generated batch number.
func (s *Service) CreateBatch(ctx context.Context, params CreateBatchParams) (db.ProductionBatch, error) {
	nextNum, err := s.queries.NextBatchNumber(ctx)
	if err != nil {
		return db.ProductionBatch{}, fmt.Errorf("generating batch number: %w", err)
	}

	batchNumber := fmt.Sprintf("PB-%04d", nextNum)

	var schedDate pgtype.Date
	if params.ScheduledDate != "" {
		t, err := time.Parse("2006-01-02", params.ScheduledDate)
		if err == nil {
			schedDate = pgtype.Date{Time: t, Valid: true}
		}
	}

	var notes *string
	if params.Notes != "" {
		notes = &params.Notes
	}

	batch, err := s.queries.CreateProductionBatch(ctx, db.CreateProductionBatchParams{
		BatchNumber:     batchNumber,
		ProductID:       params.ProductID,
		VariantID:       params.VariantID,
		PlannedQuantity: int32(params.PlannedQty),
		Status:          db.ProductionBatchStatusDraft,
		ScheduledDate:   schedDate,
		Notes:           notes,
		CreatedBy:       params.CreatedBy,
	})
	if err != nil {
		return db.ProductionBatch{}, fmt.Errorf("creating production batch: %w", err)
	}

	s.logger.Info("production batch created",
		slog.String("batch_id", batch.ID.String()),
		slog.String("batch_number", batch.BatchNumber),
		slog.String("product_id", batch.ProductID.String()),
	)

	return batch, nil
}

// Get returns a single production batch by ID, including joined product and variant info.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (db.GetProductionBatchRow, error) {
	batch, err := s.queries.GetProductionBatch(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.GetProductionBatchRow{}, ErrNotFound
		}
		return db.GetProductionBatchRow{}, fmt.Errorf("getting production batch %s: %w", id, err)
	}
	return batch, nil
}

// List returns paginated production batches.
func (s *Service) List(ctx context.Context, limit, offset int) ([]db.ListProductionBatchesRow, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 250 {
		limit = 250
	}

	batches, err := s.queries.ListProductionBatches(ctx, db.ListProductionBatchesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("listing production batches: %w", err)
	}
	return batches, nil
}

// ListByStatus returns production batches filtered by status.
func (s *Service) ListByStatus(ctx context.Context, status db.ProductionBatchStatus) ([]db.ListProductionBatchesByStatusRow, error) {
	batches, err := s.queries.ListProductionBatchesByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("listing production batches by status %s: %w", status, err)
	}
	return batches, nil
}

// Start transitions a batch from draft/scheduled to in_progress.
func (s *Service) Start(ctx context.Context, id uuid.UUID) (db.ProductionBatch, error) {
	batch, err := s.queries.StartProductionBatch(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductionBatch{}, ErrInvalidStatus
		}
		return db.ProductionBatch{}, fmt.Errorf("starting production batch %s: %w", id, err)
	}

	s.logger.Info("production batch started",
		slog.String("batch_id", id.String()),
	)

	return batch, nil
}

// Complete transitions a batch from in_progress to completed.
func (s *Service) Complete(ctx context.Context, id uuid.UUID, actualQty int, costTotal pgtype.Numeric) (db.ProductionBatch, error) {
	qty := int32(actualQty)
	batch, err := s.queries.CompleteProductionBatch(ctx, db.CompleteProductionBatchParams{
		ID:             id,
		ActualQuantity: &qty,
		CostTotal:      costTotal,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductionBatch{}, ErrInvalidStatus
		}
		return db.ProductionBatch{}, fmt.Errorf("completing production batch %s: %w", id, err)
	}

	s.logger.Info("production batch completed",
		slog.String("batch_id", id.String()),
		slog.Int("actual_quantity", actualQty),
	)

	return batch, nil
}

// Cancel transitions a batch from draft/scheduled to cancelled.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID) (db.ProductionBatch, error) {
	batch, err := s.queries.CancelProductionBatch(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductionBatch{}, ErrInvalidStatus
		}
		return db.ProductionBatch{}, fmt.Errorf("cancelling production batch %s: %w", id, err)
	}

	s.logger.Info("production batch cancelled",
		slog.String("batch_id", id.String()),
	)

	return batch, nil
}

// AddMaterial adds a raw material requirement to a production batch.
func (s *Service) AddMaterial(ctx context.Context, batchID, materialID uuid.UUID, requiredQty, unitCost pgtype.Numeric) (db.ProductionBatchMaterial, error) {
	material, err := s.queries.CreateBatchMaterial(ctx, db.CreateBatchMaterialParams{
		BatchID:          batchID,
		RawMaterialID:    materialID,
		RequiredQuantity: requiredQty,
		UnitCost:         unitCost,
	})
	if err != nil {
		return db.ProductionBatchMaterial{}, fmt.Errorf("adding material to batch %s: %w", batchID, err)
	}
	return material, nil
}

// ListMaterials returns all materials for a given production batch.
func (s *Service) ListMaterials(ctx context.Context, batchID uuid.UUID) ([]db.ListBatchMaterialsRow, error) {
	materials, err := s.queries.ListBatchMaterials(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("listing materials for batch %s: %w", batchID, err)
	}
	return materials, nil
}

// CountByStatus returns the count of production batches with a given status.
func (s *Service) CountByStatus(ctx context.Context, status db.ProductionBatchStatus) (int64, error) {
	count, err := s.queries.CountProductionBatchesByStatus(ctx, status)
	if err != nil {
		return 0, fmt.Errorf("counting batches by status %s: %w", status, err)
	}
	return count, nil
}
