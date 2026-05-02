package postgres

import (
	"context"
	"errors"
	"fmt"
	"subscriptionServices/internal/domain"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SubscriptionStorage struct {
	pool *pgxpool.Pool
}

func NewSubscriptionStorage(pool *pgxpool.Pool) domain.SubscriptionRepository {
	return &SubscriptionStorage{pool: pool}
}

type dbModel struct {
	ID          uuid.UUID
	ServiceName string
	Price       int
	UserID      uuid.UUID
	StartDate   string
	EndDate     *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func mapToDB(sub *domain.Subscription) *dbModel {
	return &dbModel{
		ID:          sub.ID,
		ServiceName: sub.ServiceName,
		Price:       sub.Price,
		UserID:      sub.UserID,
		StartDate:   sub.StartDate,
		EndDate:     sub.EndDate,
		CreatedAt:   sub.CreatedAt,
		UpdatedAt:   sub.UpdatedAt,
	}
}

func mapToDomain(db *dbModel) *domain.Subscription {
	return &domain.Subscription{
		ID:          db.ID,
		ServiceName: db.ServiceName,
		Price:       db.Price,
		UserID:      db.UserID,
		StartDate:   db.StartDate,
		EndDate:     db.EndDate,
		CreatedAt:   db.CreatedAt,
		UpdatedAt:   db.UpdatedAt,
	}
}

func (s *SubscriptionStorage) Save(ctx context.Context, sub *domain.Subscription) error {
	model := mapToDB(sub)

	query := `
		INSERT INTO subscriptions (id, service_name, price, user_id, start_date, end_date, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) 
		ON CONFLICT (id) DO UPDATE SET 
			service_name = EXCLUDED.service_name, 
			price = EXCLUDED.price, 
			start_date = EXCLUDED.start_date, 
			end_date = EXCLUDED.end_date,
			updated_at = EXCLUDED.updated_at;`

	_, err := s.pool.Exec(ctx, query,
		model.ID, model.ServiceName, model.Price, model.UserID,
		model.StartDate, model.EndDate, model.CreatedAt, model.UpdatedAt)

	return err
}

func (s *SubscriptionStorage) GetByID(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	var model dbModel
	query := `
		SELECT id, service_name, price, user_id, start_date, end_date, created_at, updated_at 
		FROM subscriptions WHERE id = $1;`

	err := s.pool.QueryRow(ctx, query, id).Scan(
		&model.ID, &model.ServiceName, &model.Price, &model.UserID,
		&model.StartDate, &model.EndDate, &model.CreatedAt, &model.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("subscription not found")
		}
		return nil, err
	}

	return mapToDomain(&model), nil
}

func (s *SubscriptionStorage) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM subscriptions WHERE id = $1;`
	_, err := s.pool.Exec(ctx, query, id)
	return err
}

func (s *SubscriptionStorage) List(ctx context.Context, limit, offset int) ([]*domain.Subscription, error) {
	query := `
		SELECT id, service_name, price, user_id, start_date, end_date, created_at, updated_at 
		FROM subscriptions 
		ORDER BY created_at DESC LIMIT $1 OFFSET $2;`

	rows, err := s.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*domain.Subscription
	for rows.Next() {
		var model dbModel
		if err := rows.Scan(
			&model.ID, &model.ServiceName, &model.Price, &model.UserID,
			&model.StartDate, &model.EndDate, &model.CreatedAt, &model.UpdatedAt,
		); err != nil {
			return nil, err
		}
		subs = append(subs, mapToDomain(&model))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return subs, nil
}

func (s *SubscriptionStorage) GetSubscriptionsForCalculation(ctx context.Context, userID uuid.UUID, serviceName *string, startPeriod string, endPeriod string) ([]*domain.Subscription, error) {
	query := `
		SELECT id, service_name, price, user_id, start_date, end_date, created_at, updated_at 
		FROM subscriptions 
		WHERE user_id = $1
		  AND TO_DATE(start_date, 'MM-YYYY') <= TO_DATE($2, 'MM-YYYY')
		  AND (end_date IS NULL OR TO_DATE(end_date, 'MM-YYYY') >= TO_DATE($3, 'MM-YYYY'))`

	var args []interface{}
	args = append(args, userID, endPeriod, startPeriod)

	if serviceName != nil && *serviceName != "" {
		args = append(args, *serviceName)
		query += fmt.Sprintf(` AND service_name = $%d`, len(args))
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*domain.Subscription
	for rows.Next() {
		var model dbModel
		if err := rows.Scan(
			&model.ID, &model.ServiceName, &model.Price, &model.UserID,
			&model.StartDate, &model.EndDate, &model.CreatedAt, &model.UpdatedAt,
		); err != nil {
			return nil, err
		}
		subs = append(subs, mapToDomain(&model))
	}

	return subs, nil
}
