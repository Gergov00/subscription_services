package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Subscription struct {
	ID          uuid.UUID
	ServiceName string
	Price       int
	UserID      uuid.UUID
	StartDate   string
	EndDate     *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewSubscription() *Subscription {
	return &Subscription{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

type SubscriptionCreateParams struct {
	ServiceName string
	Price       int
	UserID      uuid.UUID
	StartDate   string
	EndDate     *string
}

type SubscriptionUpdateParams struct {
	ServiceName *string
	Price       *int
	StartDate   *string
	EndDate     *string
}

type CalculateCostParams struct {
	UserID      uuid.UUID
	ServiceName *string
	StartPeriod string
	EndPeriod   string
}

type SubscriptionService interface {
	CreateSubscription(ctx context.Context, params SubscriptionCreateParams) (*Subscription, error)
	GetSubscription(ctx context.Context, id uuid.UUID) (*Subscription, error)
	UpdateSubscription(ctx context.Context, id uuid.UUID, params SubscriptionUpdateParams) (*Subscription, error)
	DeleteSubscription(ctx context.Context, id uuid.UUID) error
	ListSubscriptions(ctx context.Context, limit, offset int) ([]*Subscription, error)
	CalculateTotalCost(ctx context.Context, params CalculateCostParams) (int, error)
}

type SubscriptionRepository interface {
	Save(ctx context.Context, sub *Subscription) error
	GetByID(ctx context.Context, id uuid.UUID) (*Subscription, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]*Subscription, error)
	CalculateTotalCost(ctx context.Context, params CalculateCostParams) (int, error)
}
