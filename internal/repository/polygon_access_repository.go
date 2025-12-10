package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PolygonAccessRepository struct {
	db *gorm.DB
}

func NewPolygonAccessRepository(db *gorm.DB) *PolygonAccessRepository {
	return &PolygonAccessRepository{db: db}
}

// Grant grants access to a polygon for a contractor
// If access already exists, it updates the source and reactivates it (sets revoked_at to NULL)
func (r *PolygonAccessRepository) Grant(ctx context.Context, polygonID, contractorID uuid.UUID, source string) error {
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO polygon_access (polygon_id, contractor_id, source, revoked_at)
		VALUES (?, ?, ?, NULL)
		ON CONFLICT (polygon_id, contractor_id)
		DO UPDATE SET
			source = EXCLUDED.source,
			revoked_at = NULL,
			updated_at = NOW()
	`, polygonID, contractorID, source).Error
}
