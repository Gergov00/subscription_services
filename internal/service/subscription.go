package service

import (
	"context"
	"errors"
	"subscriptionServices/internal/domain"
	"time"

	"github.com/google/uuid"
)

type subscriptionService struct {
	repo domain.SubscriptionRepository
}

func NewSubscriptionService(repo domain.SubscriptionRepository) domain.SubscriptionService {
	return &subscriptionService{repo: repo}
}

func (s *subscriptionService) CreateSubscription(ctx context.Context, params domain.SubscriptionCreateParams) (*domain.Subscription, error) {
	if err := validateMonthYear(params.StartDate); err != nil {
		return nil, errors.New("invalid start_date format, expected MM-YYYY")
	}
	if params.EndDate != nil && *params.EndDate != "" {
		if err := validateMonthYear(*params.EndDate); err != nil {
			return nil, errors.New("invalid end_date format, expected MM-YYYY")
		}
		if err := validateDateOrder(params.StartDate, *params.EndDate); err != nil {
			return nil, err
		}
	}

	sub := domain.NewSubscription()
	sub.ServiceName = params.ServiceName
	sub.Price = params.Price
	sub.UserID = params.UserID
	sub.StartDate = params.StartDate
	sub.EndDate = params.EndDate

	if err := s.repo.Save(ctx, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *subscriptionService) GetSubscription(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *subscriptionService) UpdateSubscription(ctx context.Context, id uuid.UUID, params domain.SubscriptionUpdateParams) (*domain.Subscription, error) {
	sub, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if params.ServiceName != nil {
		sub.ServiceName = *params.ServiceName
	}
	if params.Price != nil {
		sub.Price = *params.Price
	}
	if params.StartDate != nil {
		if err := validateMonthYear(*params.StartDate); err != nil {
			return nil, errors.New("invalid start_date format, expected MM-YYYY")
		}
		sub.StartDate = *params.StartDate
	}
	if params.EndDate != nil {
		if *params.EndDate != "" {
			if err := validateMonthYear(*params.EndDate); err != nil {
				return nil, errors.New("invalid end_date format, expected MM-YYYY")
			}
			if err := validateDateOrder(sub.StartDate, *params.EndDate); err != nil {
				return nil, err
			}
			sub.EndDate = params.EndDate
		} else {
			sub.EndDate = nil
		}
	}

	sub.UpdatedAt = time.Now()

	if err := s.repo.Save(ctx, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *subscriptionService) DeleteSubscription(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *subscriptionService) ListSubscriptions(ctx context.Context, limit, offset int) ([]*domain.Subscription, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *subscriptionService) CalculateTotalCost(ctx context.Context, params domain.CalculateCostParams) (int, error) {
	if err := validateMonthYear(params.StartPeriod); err != nil {
		return 0, errors.New("invalid start_period format, expected MM-YYYY")
	}
	if err := validateMonthYear(params.EndPeriod); err != nil {
		return 0, errors.New("invalid end_period format, expected MM-YYYY")
	}
	if err := validateDateOrder(params.StartPeriod, params.EndPeriod); err != nil {
		return 0, err
	}

	return s.repo.CalculateTotalCost(ctx, params)
}

func validateMonthYear(my string) error {
	_, err := time.Parse("01-2006", my)
	return err
}

func validateDateOrder(startStr, endStr string) error {
	start, err := time.Parse("01-2006", startStr)
	if err != nil {
		return err
	}
	end, err := time.Parse("01-2006", endStr)
	if err != nil {
		return err
	}
	if start.After(end) {
		return errors.New("start date must not be later than end date")
	}
	return nil
}

