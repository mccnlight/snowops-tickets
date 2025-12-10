package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CleaningAreaAccessRepository struct {
	db *gorm.DB
}

func NewCleaningAreaAccessRepository(db *gorm.DB) *CleaningAreaAccessRepository {
	return &CleaningAreaAccessRepository{db: db}
}

// Grant grants access to a cleaning area for a contractor
// If access already exists, it updates the source and reactivates it (sets revoked_at to NULL)
func (r *CleaningAreaAccessRepository) Grant(ctx context.Context, areaID, contractorID uuid.UUID, source string) error {
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO cleaning_area_access (cleaning_area_id, contractor_id, source, revoked_at)
		VALUES (?, ?, ?, NULL)
		ON CONFLICT (cleaning_area_id, contractor_id)
		DO UPDATE SET
			source = EXCLUDED.source,
			revoked_at = NULL,
			updated_at = NOW()
	`, areaID, contractorID, source).Error
}
