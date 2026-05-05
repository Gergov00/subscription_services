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
	tag, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		s.log.Error("delete subscription failed", slog.String("id", id.String()), slog.Any("error", err))
		return err
	}
	if tag.RowsAffected() == 0 {
		s.log.Warn("subscription not found for delete", slog.String("id", id.String()))
		return errors.New("subscription not found")
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

// CalculateTotalCost вычисляет суммарную стоимость подписок за период целиком на стороне БД.
// Использует функцию month_abs() для перевода 'MM-YYYY' в абсолютный номер месяца,
// что делает запрос читаемым и позволяет PostgreSQL кэшировать вычисления (IMMUTABLE).
func (s *SubscriptionStorage) CalculateTotalCost(ctx context.Context, params domain.CalculateCostParams) (int, error) {
	query := `
		SELECT COALESCE(SUM(
			GREATEST(0,
				LEAST(month_abs($3), COALESCE(month_abs(end_date), month_abs($3))) -
				GREATEST(month_abs($2), month_abs(start_date)) + 1
			) * price
		), 0)
		FROM subscriptions
		WHERE user_id = $1
		  AND month_abs(start_date) <= month_abs($3)
		  AND (end_date IS NULL OR month_abs(end_date) >= month_abs($2))`

	args := []interface{}{params.UserID, params.StartPeriod, params.EndPeriod}

	if params.ServiceName != nil && *params.ServiceName != "" {
		args = append(args, *params.ServiceName)
		query += fmt.Sprintf(` AND service_name = $%d`, len(args))
	}

	start := time.Now()
	var total int
	err := s.pool.QueryRow(ctx, query, args...).Scan(&total)
	if err != nil {
		s.log.Error("calculate total cost failed", slog.Any("error", err))
		return 0, err
	}

	s.log.Info("calculate total cost",
		slog.String("user_id", params.UserID.String()),
		slog.Int("total", total),
		slog.Duration("latency", time.Since(start)))
	return total, nil
}
