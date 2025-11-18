package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ticket-service/internal/model"
)

type AppealRepository struct {
	db *gorm.DB
}

func NewAppealRepository(db *gorm.DB) *AppealRepository {
	return &AppealRepository{db: db}
}

func (r *AppealRepository) Create(ctx context.Context, appeal *model.Appeal) error {
	return r.db.WithContext(ctx).Create(appeal).Error
}

func (r *AppealRepository) GetByID(ctx context.Context, id string) (*model.Appeal, error) {
	var appeal model.Appeal
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&appeal).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &appeal, nil
}

func (r *AppealRepository) Update(ctx context.Context, appeal *model.Appeal) error {
	return r.db.WithContext(ctx).Save(appeal).Error
}

func (r *AppealRepository) ListByTicketID(ctx context.Context, ticketID uuid.UUID) ([]model.Appeal, error) {
	var appeals []model.Appeal
	err := r.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("created_at DESC").
		Find(&appeals).Error
	return appeals, err
}

func (r *AppealRepository) ListByTripID(ctx context.Context, tripID uuid.UUID) ([]model.Appeal, error) {
	var appeals []model.Appeal
	err := r.db.WithContext(ctx).
		Where("trip_id = ?", tripID).
		Order("created_at DESC").
		Find(&appeals).Error
	return appeals, err
}

func (r *AppealRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.Appeal, error) {
	var appeals []model.Appeal
	err := r.db.WithContext(ctx).
		Where("created_by_user_id = ?", userID).
		Order("created_at DESC").
		Find(&appeals).Error
	return appeals, err
}

func (r *AppealRepository) GetCommentsByAppealID(ctx context.Context, appealID uuid.UUID) ([]model.AppealComment, error) {
	var comments []model.AppealComment
	err := r.db.WithContext(ctx).
		Where("appeal_id = ?", appealID).
		Order("created_at ASC").
		Find(&comments).Error
	return comments, err
}

func (r *AppealRepository) AddComment(ctx context.Context, comment *model.AppealComment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

