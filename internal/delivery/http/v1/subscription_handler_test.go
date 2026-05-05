package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"subscriptionServices/internal/domain"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// mockService — ручной мок domain.SubscriptionService для тестирования хендлеров.
type mockService struct {
	createFn        func(ctx context.Context, p domain.SubscriptionCreateParams) (*domain.Subscription, error)
	getFn           func(ctx context.Context, id uuid.UUID) (*domain.Subscription, error)
	updateFn        func(ctx context.Context, id uuid.UUID, p domain.SubscriptionUpdateParams) (*domain.Subscription, error)
	deleteFn        func(ctx context.Context, id uuid.UUID) error
	listFn          func(ctx context.Context, limit, offset int) ([]*domain.Subscription, error)
	calculateCostFn func(ctx context.Context, p domain.CalculateCostParams) (int, error)
}

func (m *mockService) CreateSubscription(ctx context.Context, p domain.SubscriptionCreateParams) (*domain.Subscription, error) {
	return m.createFn(ctx, p)
}
func (m *mockService) GetSubscription(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	return m.getFn(ctx, id)
}
func (m *mockService) UpdateSubscription(ctx context.Context, id uuid.UUID, p domain.SubscriptionUpdateParams) (*domain.Subscription, error) {
	return m.updateFn(ctx, id, p)
}
func (m *mockService) DeleteSubscription(ctx context.Context, id uuid.UUID) error {
	return m.deleteFn(ctx, id)
}
func (m *mockService) ListSubscriptions(ctx context.Context, limit, offset int) ([]*domain.Subscription, error) {
	return m.listFn(ctx, limit, offset)
}
func (m *mockService) CalculateTotalCost(ctx context.Context, p domain.CalculateCostParams) (int, error) {
	return m.calculateCostFn(ctx, p)
}

// setup создает тестовый gin-роутер с зарегистрированным хендлером.
func setup(svc domain.SubscriptionService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSubscriptionHandler(svc)
	subs := r.Group("/api/v1/subscriptions")
	{
		subs.POST("", h.Create)
		subs.GET("", h.List)
		subs.GET("/cost", h.CalculateCost)
		subs.GET("/:id", h.GetByID)
		subs.PATCH("/:id", h.Update)
		subs.DELETE("/:id", h.Delete)
	}
	return r
}

func fakeSub() *domain.Subscription {
	return domain.NewSubscription()
}

// --- Create ---

func TestCreate_Success(t *testing.T) {
	svc := &mockService{
		createFn: func(_ context.Context, _ domain.SubscriptionCreateParams) (*domain.Subscription, error) {
			return fakeSub(), nil
		},
	}
	r := setup(svc)

	body, _ := json.Marshal(map[string]any{
		"service_name": "Netflix",
		"price":        799,
		"user_id":      uuid.New().String(),
		"start_date":   "01-2025",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreate_MissingRequiredField(t *testing.T) {
	r := setup(&mockService{})

	// price отсутствует
	body, _ := json.Marshal(map[string]any{
		"service_name": "Netflix",
		"user_id":      uuid.New().String(),
		"start_date":   "01-2025",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCreate_NegativePrice(t *testing.T) {
	r := setup(&mockService{})

	body, _ := json.Marshal(map[string]any{
		"service_name": "Netflix",
		"price":        -100,
		"user_id":      uuid.New().String(),
		"start_date":   "01-2025",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCreate_InvalidDateFormat(t *testing.T) {
	svc := &mockService{
		createFn: func(_ context.Context, _ domain.SubscriptionCreateParams) (*domain.Subscription, error) {
			return nil, errors.New("invalid start_date format, expected MM-YYYY")
		},
	}
	r := setup(svc)

	body, _ := json.Marshal(map[string]any{
		"service_name": "Netflix",
		"price":        799,
		"user_id":      uuid.New().String(),
		"start_date":   "2025-01", // неверный формат
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCreate_InvalidUserID(t *testing.T) {
	r := setup(&mockService{})

	body, _ := json.Marshal(map[string]any{
		"service_name": "Netflix",
		"price":        799,
		"user_id":      "not-a-uuid",
		"start_date":   "01-2025",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// --- GetByID ---

func TestGetByID_Success(t *testing.T) {
	id := uuid.New()
	svc := &mockService{
		getFn: func(_ context.Context, _ uuid.UUID) (*domain.Subscription, error) {
			return fakeSub(), nil
		},
	}
	r := setup(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/"+id.String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	id := uuid.New()
	svc := &mockService{
		getFn: func(_ context.Context, _ uuid.UUID) (*domain.Subscription, error) {
			return nil, errors.New("subscription not found")
		},
	}
	r := setup(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/"+id.String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestGetByID_InternalError(t *testing.T) {
	id := uuid.New()
	svc := &mockService{
		getFn: func(_ context.Context, _ uuid.UUID) (*domain.Subscription, error) {
			return nil, errors.New("db connection lost")
		},
	}
	r := setup(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/"+id.String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestGetByID_InvalidUUID(t *testing.T) {
	r := setup(&mockService{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/not-a-uuid", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// --- Delete ---

func TestDelete_Success(t *testing.T) {
	id := uuid.New()
	svc := &mockService{
		deleteFn: func(_ context.Context, _ uuid.UUID) error { return nil },
	}
	r := setup(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/subscriptions/"+id.String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", w.Code)
	}
}

func TestDelete_NotFound(t *testing.T) {
	id := uuid.New()
	svc := &mockService{
		deleteFn: func(_ context.Context, _ uuid.UUID) error {
			return errors.New("subscription not found")
		},
	}
	r := setup(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/subscriptions/"+id.String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestDelete_InvalidUUID(t *testing.T) {
	r := setup(&mockService{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/subscriptions/bad-id", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// --- Update ---

func TestUpdate_Success(t *testing.T) {
	id := uuid.New()
	svc := &mockService{
		updateFn: func(_ context.Context, _ uuid.UUID, _ domain.SubscriptionUpdateParams) (*domain.Subscription, error) {
			return fakeSub(), nil
		},
	}
	r := setup(svc)

	body, _ := json.Marshal(map[string]any{"price": 599})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subscriptions/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	id := uuid.New()
	svc := &mockService{
		updateFn: func(_ context.Context, _ uuid.UUID, _ domain.SubscriptionUpdateParams) (*domain.Subscription, error) {
			return nil, errors.New("subscription not found")
		},
	}
	r := setup(svc)

	body, _ := json.Marshal(map[string]any{"price": 599})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subscriptions/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestUpdate_InvalidDateOrder(t *testing.T) {
	id := uuid.New()
	svc := &mockService{
		updateFn: func(_ context.Context, _ uuid.UUID, _ domain.SubscriptionUpdateParams) (*domain.Subscription, error) {
			return nil, errors.New("start date must not be later than end date")
		},
	}
	r := setup(svc)

	body, _ := json.Marshal(map[string]any{"end_date": "01-2020"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subscriptions/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// --- List ---

func TestList_Success(t *testing.T) {
	svc := &mockService{
		listFn: func(_ context.Context, _, _ int) ([]*domain.Subscription, error) {
			return []*domain.Subscription{fakeSub()}, nil
		},
	}
	r := setup(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions?limit=5&offset=0", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestList_InvalidLimit(t *testing.T) {
	r := setup(&mockService{})

	cases := []string{"-1", "0", "101", "abc"}
	for _, lim := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/subscriptions?limit=%s", lim), nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("limit=%s: want 400, got %d", lim, w.Code)
		}
	}
}

func TestList_EmptyResult(t *testing.T) {
	svc := &mockService{
		listFn: func(_ context.Context, _, _ int) ([]*domain.Subscription, error) {
			return nil, nil
		},
	}
	r := setup(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	// Пустой список должен быть [], а не null
	if w.Body.String() != "[]" {
		t.Errorf("want [], got %s", w.Body.String())
	}
}

// --- CalculateCost ---

func TestCalculateCost_Success(t *testing.T) {
	svc := &mockService{
		calculateCostFn: func(_ context.Context, _ domain.CalculateCostParams) (int, error) {
			return 4800, nil
		},
	}
	r := setup(svc)

	w := httptest.NewRecorder()
	url := fmt.Sprintf("/api/v1/subscriptions/cost?user_id=%s&start_period=01-2025&end_period=12-2025", uuid.New())
	req := httptest.NewRequest(http.MethodGet, url, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCalculateCost_MissingUserID(t *testing.T) {
	r := setup(&mockService{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/cost?start_period=01-2025&end_period=12-2025", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCalculateCost_InvalidPeriodFormat(t *testing.T) {
	svc := &mockService{
		calculateCostFn: func(_ context.Context, _ domain.CalculateCostParams) (int, error) {
			return 0, errors.New("invalid start_period format, expected MM-YYYY")
		},
	}
	r := setup(svc)

	w := httptest.NewRecorder()
	url := fmt.Sprintf("/api/v1/subscriptions/cost?user_id=%s&start_period=2025-01&end_period=12-2025", uuid.New())
	req := httptest.NewRequest(http.MethodGet, url, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCalculateCost_StartAfterEnd(t *testing.T) {
	svc := &mockService{
		calculateCostFn: func(_ context.Context, _ domain.CalculateCostParams) (int, error) {
			return 0, errors.New("start date must not be later than end date")
		},
	}
	r := setup(svc)

	w := httptest.NewRecorder()
	url := fmt.Sprintf("/api/v1/subscriptions/cost?user_id=%s&start_period=12-2025&end_period=01-2025", uuid.New())
	req := httptest.NewRequest(http.MethodGet, url, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}
