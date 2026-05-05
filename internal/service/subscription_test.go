package service

import (
	"context"
	"errors"
	"subscriptionServices/internal/domain"
	"testing"

	"github.com/google/uuid"
)

// mockRepo — ручной мок репозитория для тестов.
type mockRepo struct {
	saveFn                func(ctx context.Context, sub *domain.Subscription) error
	getByIDFn             func(ctx context.Context, id uuid.UUID) (*domain.Subscription, error)
	deleteFn              func(ctx context.Context, id uuid.UUID) error
	listFn                func(ctx context.Context, limit, offset int) ([]*domain.Subscription, error)
	calculateTotalCostFn  func(ctx context.Context, params domain.CalculateCostParams) (int, error)
}

func (m *mockRepo) Save(ctx context.Context, sub *domain.Subscription) error {
	return m.saveFn(ctx, sub)
}
func (m *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.deleteFn(ctx, id)
}
func (m *mockRepo) List(ctx context.Context, limit, offset int) ([]*domain.Subscription, error) {
	return m.listFn(ctx, limit, offset)
}
func (m *mockRepo) CalculateTotalCost(ctx context.Context, params domain.CalculateCostParams) (int, error) {
	return m.calculateTotalCostFn(ctx, params)
}

// --- Тесты validateMonthYear ---

func TestValidateMonthYear_Valid(t *testing.T) {
	cases := []string{"01-2024", "12-2025", "07-2000"}
	for _, tc := range cases {
		if err := validateMonthYear(tc); err != nil {
			t.Errorf("validateMonthYear(%q) = error, want nil", tc)
		}
	}
}

func TestValidateMonthYear_Invalid(t *testing.T) {
	cases := []string{"2024-01", "13-2025", "00-2024", "january-2025", ""}
	for _, tc := range cases {
		if err := validateMonthYear(tc); err == nil {
			t.Errorf("validateMonthYear(%q) = nil, want error", tc)
		}
	}
}

// --- Тесты validateDateOrder ---

func TestValidateDateOrder_Valid(t *testing.T) {
	cases := [][2]string{
		{"01-2025", "12-2025"},
		{"06-2024", "06-2024"}, // одинаковые — ок
		{"01-2020", "01-2030"},
	}
	for _, tc := range cases {
		if err := validateDateOrder(tc[0], tc[1]); err != nil {
			t.Errorf("validateDateOrder(%q, %q) = error, want nil", tc[0], tc[1])
		}
	}
}

func TestValidateDateOrder_Invalid(t *testing.T) {
	cases := [][2]string{
		{"12-2025", "01-2025"},
		{"07-2025", "06-2025"},
	}
	for _, tc := range cases {
		if err := validateDateOrder(tc[0], tc[1]); err == nil {
			t.Errorf("validateDateOrder(%q, %q) = nil, want error", tc[0], tc[1])
		}
	}
}

// --- Тесты CreateSubscription ---

func TestCreateSubscription_Success(t *testing.T) {
	repo := &mockRepo{
		saveFn: func(ctx context.Context, sub *domain.Subscription) error { return nil },
	}
	svc := NewSubscriptionService(repo)

	params := domain.SubscriptionCreateParams{
		ServiceName: "Yandex Plus",
		Price:       400,
		UserID:      uuid.New(),
		StartDate:   "01-2025",
	}
	sub, err := svc.CreateSubscription(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.ServiceName != params.ServiceName {
		t.Errorf("got ServiceName=%q, want %q", sub.ServiceName, params.ServiceName)
	}
}

func TestCreateSubscription_InvalidStartDate(t *testing.T) {
	svc := NewSubscriptionService(&mockRepo{})

	params := domain.SubscriptionCreateParams{
		ServiceName: "Test",
		Price:       100,
		UserID:      uuid.New(),
		StartDate:   "2025-01", // неверный формат
	}
	_, err := svc.CreateSubscription(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for invalid start_date format")
	}
}

func TestCreateSubscription_EndBeforeStart(t *testing.T) {
	svc := NewSubscriptionService(&mockRepo{})

	endDate := "01-2025"
	params := domain.SubscriptionCreateParams{
		ServiceName: "Test",
		Price:       100,
		UserID:      uuid.New(),
		StartDate:   "12-2025",
		EndDate:     &endDate,
	}
	_, err := svc.CreateSubscription(context.Background(), params)
	if err == nil {
		t.Fatal("expected error when start_date is after end_date")
	}
}

// --- Тесты UpdateSubscription ---

func TestUpdateSubscription_Success(t *testing.T) {
	repo := &mockRepo{
		getByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
			return &domain.Subscription{ID: id, ServiceName: "Old", Price: 100, StartDate: "01-2025"}, nil
		},
		saveFn: func(ctx context.Context, sub *domain.Subscription) error { return nil },
	}
	svc := NewSubscriptionService(repo)

	id := uuid.New()
	end := "12-2025"
	price := 200
	updated, err := svc.UpdateSubscription(context.Background(), id, domain.SubscriptionUpdateParams{
		Price:   &price,
		EndDate: &end,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Price != 200 {
		t.Errorf("got Price=%d, want 200", updated.Price)
	}
	if updated.EndDate == nil || *updated.EndDate != "12-2025" {
		t.Errorf("got EndDate=%v, want 12-2025", updated.EndDate)
	}
}

func TestUpdateSubscription_EndBeforeStart(t *testing.T) {
	repo := &mockRepo{
		getByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
			return &domain.Subscription{ID: id, ServiceName: "Test", Price: 100, StartDate: "05-2025"}, nil
		},
	}
	svc := NewSubscriptionService(repo)

	id := uuid.New()
	end := "01-2025" // before existing StartDate
	_, err := svc.UpdateSubscription(context.Background(), id, domain.SubscriptionUpdateParams{
		EndDate: &end,
	})
	if err == nil {
		t.Fatal("expected error when new end_date is before existing start_date")
	}
}

func TestUpdateSubscription_ClearEndDate(t *testing.T) {
	endStr := "12-2025"
	repo := &mockRepo{
		getByIDFn: func(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
			return &domain.Subscription{ID: id, ServiceName: "Test", Price: 100, StartDate: "01-2025", EndDate: &endStr}, nil
		},
		saveFn: func(ctx context.Context, sub *domain.Subscription) error { return nil },
	}
	svc := NewSubscriptionService(repo)

	id := uuid.New()
	empty := ""
	updated, err := svc.UpdateSubscription(context.Background(), id, domain.SubscriptionUpdateParams{
		EndDate: &empty, // should clear it
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.EndDate != nil {
		t.Errorf("expected EndDate to be nil, got %v", *updated.EndDate)
	}
}

// --- Тесты CalculateTotalCost ---

func TestCalculateTotalCost_Success(t *testing.T) {
	repo := &mockRepo{
		calculateTotalCostFn: func(ctx context.Context, params domain.CalculateCostParams) (int, error) {
			return 4800, nil
		},
	}
	svc := NewSubscriptionService(repo)

	params := domain.CalculateCostParams{
		UserID:      uuid.New(),
		StartPeriod: "01-2025",
		EndPeriod:   "12-2025",
	}
	total, err := svc.CalculateTotalCost(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 4800 {
		t.Errorf("got total=%d, want 4800", total)
	}
}

func TestCalculateTotalCost_InvalidPeriod(t *testing.T) {
	svc := NewSubscriptionService(&mockRepo{})

	params := domain.CalculateCostParams{
		UserID:      uuid.New(),
		StartPeriod: "12-2025",
		EndPeriod:   "01-2025", // конец раньше начала
	}
	_, err := svc.CalculateTotalCost(context.Background(), params)
	if err == nil {
		t.Fatal("expected error when start_period is after end_period")
	}
}

func TestCalculateTotalCost_RepoError(t *testing.T) {
	repo := &mockRepo{
		calculateTotalCostFn: func(ctx context.Context, params domain.CalculateCostParams) (int, error) {
			return 0, errors.New("db connection error")
		},
	}
	svc := NewSubscriptionService(repo)

	params := domain.CalculateCostParams{
		UserID:      uuid.New(),
		StartPeriod: "01-2025",
		EndPeriod:   "12-2025",
	}
	_, err := svc.CalculateTotalCost(context.Background(), params)
	if err == nil {
		t.Fatal("expected error from repository")
	}
}

// --- Тесты DeleteSubscription ---

func TestDeleteSubscription_Success(t *testing.T) {
	repo := &mockRepo{
		deleteFn: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}
	svc := NewSubscriptionService(repo)

	err := svc.DeleteSubscription(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDeleteSubscription_NotFound(t *testing.T) {
	repo := &mockRepo{
		deleteFn: func(ctx context.Context, id uuid.UUID) error {
			return errors.New("subscription not found")
		},
	}
	svc := NewSubscriptionService(repo)

	err := svc.DeleteSubscription(context.Background(), uuid.New())
	if err == nil || err.Error() != "subscription not found" {
		t.Fatalf("expected 'subscription not found' error, got: %v", err)
	}
}
