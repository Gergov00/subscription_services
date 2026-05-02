package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"subscriptionServices/internal/domain"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SubscriptionStorage struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewSubscriptionStorage(pool *pgxpool.Pool, log *slog.Logger) domain.SubscriptionRepository {
	return &SubscriptionStorage{pool: pool, log: log.With(slog.String("layer", "repository"))}
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

	start := time.Now()
	_, err := s.pool.Exec(ctx, query,
		model.ID, model.ServiceName, model.Price, model.UserID,
		model.StartDate, model.EndDate, model.CreatedAt, model.UpdatedAt)

	if err != nil {
		s.log.Error("save subscription failed", slog.String("id", sub.ID.String()), slog.Any("error", err))
		return err
	}

	s.log.Info("save subscription", slog.String("id", sub.ID.String()), slog.Duration("latency", time.Since(start)))
	return nil
}

func (s *SubscriptionStorage) GetByID(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	var model dbModel
	query := `
		SELECT id, service_name, price, user_id, start_date, end_date, created_at, updated_at 
		FROM subscriptions WHERE id = $1;`

	start := time.Now()
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&model.ID, &model.ServiceName, &model.Price, &model.UserID,
		&model.StartDate, &model.EndDate, &model.CreatedAt, &model.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.log.Warn("subscription not found", slog.String("id", id.String()))
			return nil, errors.New("subscription not found")
		}
		s.log.Error("get subscription failed", slog.String("id", id.String()), slog.Any("error", err))
		return nil, err
	}

	s.log.Info("get subscription", slog.String("id", id.String()), slog.Duration("latency", time.Since(start)))
	return mapToDomain(&model), nil
}

func (s *SubscriptionStorage) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM subscriptions WHERE id = $1;`
	start := time.Now()
	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		s.log.Error("delete subscription failed", slog.String("id", id.String()), slog.Any("error", err))
		return err
	}
	s.log.Info("delete subscription", slog.String("id", id.String()), slog.Duration("latency", time.Since(start)))
	return nil
}

func (s *SubscriptionStorage) List(ctx context.Context, limit, offset int) ([]*domain.Subscription, error) {
	query := `
		SELECT id, service_name, price, user_id, start_date, end_date, created_at, updated_at 
		FROM subscriptions 
		ORDER BY created_at DESC LIMIT $1 OFFSET $2;`

	start := time.Now()
	rows, err := s.pool.Query(ctx, query, limit, offset)
	if err != nil {
		s.log.Error("list subscriptions failed", slog.Any("error", err))
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

	s.log.Info("list subscriptions", slog.Int("count", len(subs)), slog.Duration("latency", time.Since(start)))
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
