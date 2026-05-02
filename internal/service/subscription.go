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

	subs, err := s.repo.GetSubscriptionsForCalculation(ctx, params.UserID, params.ServiceName, params.StartPeriod, params.EndPeriod)
	if err != nil {
		return 0, err
	}

	totalCost := 0

	for _, sub := range subs {
		months, err := overlapMonths(params.StartPeriod, params.EndPeriod, sub.StartDate, sub.EndDate)
		if err != nil {
			return 0, err
		}
		totalCost += months * sub.Price
	}

	return totalCost, nil
}


func validateMonthYear(my string) error {
	_, err := time.Parse("01-2006", my)
	return err
}

func parseMonthYear(my string) (int, error) {
	t, err := time.Parse("01-2006", my)
	if err != nil {
		return 0, err
	}
	return t.Year()*12 + int(t.Month()), nil
}

func overlapMonths(reqStartStr, reqEndStr, subStartStr string, subEndStr *string) (int, error) {
	reqStart, err := parseMonthYear(reqStartStr)
	if err != nil {
		return 0, err
	}
	reqEnd, err := parseMonthYear(reqEndStr)
	if err != nil {
		return 0, err
	}

	subStart, err := parseMonthYear(subStartStr)
	if err != nil {
		return 0, err
	}

	subEnd := reqEnd 
	if subEndStr != nil && *subEndStr != "" {
		se, err := parseMonthYear(*subEndStr)
		if err != nil {
			return 0, err
		}
		subEnd = se
	}

	start := max(reqStart, subStart)
	end := min(reqEnd, subEnd)

	overlap := end - start + 1
	if overlap <= 0 {
		return 0, nil
	}

	return overlap, nil
}
