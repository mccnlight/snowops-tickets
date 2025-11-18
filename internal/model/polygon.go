package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Polygon struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Name         string    `gorm:"type:varchar(255)" json:"name"`
	GeometryWKT  string    `gorm:"type:text" json:"geometry_wkt"`
	CentroidLat  *float64  `json:"centroid_lat"`
	CentroidLong *float64  `json:"centroid_long"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Polygon) TableName() string {
	return "polygons"
}

func (p *Polygon) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
