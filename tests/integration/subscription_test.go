//go:build integration

package integration

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"testing"

	"subscriptionServices/internal/domain"
	postgresrepo "subscriptionServices/internal/repository/postgres"
	"subscriptionServices/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var (
	pool *pgxpool.Pool
	svc  domain.SubscriptionService
)

func TestMain(m *testing.M) {
	// Загружаем .env из корня проекта (тесты запускаются из tests/integration/)
	_ = godotenv.Load("../../.env")

	user := getEnv("DB_USER", "user")
	password := getEnv("DB_PASSWORD", "password")
	host := getEnv("DB_HOST", "localhost")

	port := getEnv("TEST_DB_PORT", "5433")

	dbName := getEnv("DB_NAME", "subscriptions")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbName)

	var err error
	pool, err = pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("failed to connect to DB: %v", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("failed to ping DB: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := postgresrepo.NewSubscriptionStorage(pool, logger)
	svc = service.NewSubscriptionService(repo)

	os.Exit(m.Run())
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// truncate очищает таблицу после каждого теста.
func truncate(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), "TRUNCATE TABLE subscriptions")
		if err != nil {
			t.Logf("truncate failed: %v", err)
		}
	})
}

// helpers

func ptr(s string) *string { return &s }

func mustCreate(t *testing.T, p domain.SubscriptionCreateParams) *domain.Subscription {
	t.Helper()
	sub, err := svc.CreateSubscription(context.Background(), p)
	if err != nil {
		t.Fatalf("mustCreate: %v", err)
	}
	return sub
}

func calcCost(t *testing.T, userID uuid.UUID, start, end string, serviceName *string) (int, error) {
	t.Helper()
	return svc.CalculateTotalCost(context.Background(), domain.CalculateCostParams{
		UserID:      userID,
		StartPeriod: start,
		EndPeriod:   end,
		ServiceName: serviceName,
	})
}

// ===========================================================================
// Тесты CRUD
// ===========================================================================

func TestCreateAndGet(t *testing.T) {
	truncate(t)
	userID := uuid.New()

	sub := mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Netflix",
		Price:       799,
		UserID:      userID,
		StartDate:   "01-2025",
	})

	got, err := svc.GetSubscription(context.Background(), sub.ID)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if got.ServiceName != "Netflix" || got.Price != 799 {
		t.Errorf("got %+v, want Netflix/799", got)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	truncate(t)
	_, err := svc.GetSubscription(context.Background(), uuid.New())
	if err == nil || err.Error() != "subscription not found" {
		t.Fatalf("expected 'subscription not found', got: %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	truncate(t)
	err := svc.DeleteSubscription(context.Background(), uuid.New())
	if err == nil || err.Error() != "subscription not found" {
		t.Fatalf("expected 'subscription not found', got: %v", err)
	}
}

func TestDelete_Existing(t *testing.T) {
	truncate(t)
	sub := mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Spotify",
		Price:       299,
		UserID:      uuid.New(),
		StartDate:   "01-2025",
	})

	if err := svc.DeleteSubscription(context.Background(), sub.ID); err != nil {
		t.Fatalf("DeleteSubscription: %v", err)
	}

	_, err := svc.GetSubscription(context.Background(), sub.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestUpdate_NotFound(t *testing.T) {
	truncate(t)
	_, err := svc.UpdateSubscription(context.Background(), uuid.New(), domain.SubscriptionUpdateParams{
		Price: ptrInt(500),
	})
	if err == nil || err.Error() != "subscription not found" {
		t.Fatalf("expected 'subscription not found', got: %v", err)
	}
}

func TestUpdate_Fields(t *testing.T) {
	truncate(t)
	sub := mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Yandex Plus",
		Price:       400,
		UserID:      uuid.New(),
		StartDate:   "01-2025",
	})

	updated, err := svc.UpdateSubscription(context.Background(), sub.ID, domain.SubscriptionUpdateParams{
		Price:   ptrInt(599),
		EndDate: ptr("12-2025"),
	})
	if err != nil {
		t.Fatalf("UpdateSubscription: %v", err)
	}
	if updated.Price != 599 {
		t.Errorf("want price=599, got %d", updated.Price)
	}
	if updated.EndDate == nil || *updated.EndDate != "12-2025" {
		t.Errorf("want end_date=12-2025, got %v", updated.EndDate)
	}
}

func TestUpdate_ClearEndDate(t *testing.T) {
	truncate(t)
	sub := mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Netflix",
		Price:       799,
		UserID:      uuid.New(),
		StartDate:   "01-2025",
		EndDate:     ptr("12-2025"),
	})

	updated, err := svc.UpdateSubscription(context.Background(), sub.ID, domain.SubscriptionUpdateParams{
		EndDate: ptr(""),
	})
	if err != nil {
		t.Fatalf("UpdateSubscription: %v", err)
	}
	if updated.EndDate != nil {
		t.Errorf("expected end_date to be cleared, got %v", *updated.EndDate)
	}
}

func TestUpdate_InvalidDateOrder(t *testing.T) {
	truncate(t)
	sub := mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Netflix",
		Price:       799,
		UserID:      uuid.New(),
		StartDate:   "06-2025",
	})

	_, err := svc.UpdateSubscription(context.Background(), sub.ID, domain.SubscriptionUpdateParams{
		EndDate: ptr("01-2025"), // End before start
	})
	if err == nil {
		t.Fatal("expected error when setting end_date before start_date")
	}
}

func TestList_Pagination(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	for i := range 5 {
		mustCreate(t, domain.SubscriptionCreateParams{
			ServiceName: fmt.Sprintf("Service%d", i),
			Price:       100,
			UserID:      userID,
			StartDate:   "01-2025",
		})
	}

	list, err := svc.ListSubscriptions(context.Background(), 3, 0)
	if err != nil {
		t.Fatalf("ListSubscriptions: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("want 3 results, got %d", len(list))
	}

	list2, err := svc.ListSubscriptions(context.Background(), 3, 3)
	if err != nil {
		t.Fatalf("ListSubscriptions offset: %v", err)
	}
	if len(list2) != 2 {
		t.Errorf("want 2 results (offset=3 of 5), got %d", len(list2))
	}
}

// ===========================================================================
// Тесты CalculateTotalCost — краевые случаи
// ===========================================================================

// Случай 1: Бессрочная подписка — считается до конца запрошенного периода.
func TestCost_OpenEndedSubscription(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	// Подписка с марта 2025, без end_date.
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Yandex Plus",
		Price:       400,
		UserID:      userID,
		StartDate:   "03-2025",
	})

	// Период: март–декабрь = 10 месяцев.
	total, err := calcCost(t, userID, "01-2025", "12-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 10*400 {
		t.Errorf("want %d, got %d", 10*400, total)
	}
}

// Случай 2: Подписка ровно совпадает с запрошенным периодом.
func TestCost_ExactPeriodMatch(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Netflix",
		Price:       799,
		UserID:      userID,
		StartDate:   "03-2025",
		EndDate:     ptr("05-2025"),
	})

	// 3 месяца (март, апрель, май).
	total, err := calcCost(t, userID, "03-2025", "05-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3*799 {
		t.Errorf("want %d, got %d", 3*799, total)
	}
}

// Случай 3: Подписка начинается ДО периода, заканчивается ВНУТРИ.
func TestCost_SubStartsBeforePeriod(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Spotify",
		Price:       299,
		UserID:      userID,
		StartDate:   "01-2025",
		EndDate:     ptr("06-2025"),
	})

	// Период апрель–декабрь. Подписка активна апрель–июнь = 3 месяца.
	total, err := calcCost(t, userID, "04-2025", "12-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3*299 {
		t.Errorf("want %d, got %d", 3*299, total)
	}
}

// Случай 4: Подписка начинается ВНУТРИ периода, заканчивается ПОСЛЕ.
func TestCost_SubEndsAfterPeriod(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Apple TV",
		Price:       500,
		UserID:      userID,
		StartDate:   "10-2025",
		EndDate:     ptr("03-2026"),
	})

	// Период июль–декабрь. Подписка активна октябрь–декабрь = 3 месяца.
	total, err := calcCost(t, userID, "07-2025", "12-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3*500 {
		t.Errorf("want %d, got %d", 3*500, total)
	}
}

// Случай 5: Подписка ПОЛНОСТЬЮ покрывает период (начало до, конец после).
func TestCost_SubFullyCoversPeriod(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "VK Music",
		Price:       200,
		UserID:      userID,
		StartDate:   "01-2025",
		EndDate:     ptr("12-2025"),
	})

	// Период апрель–сентябрь = 6 месяцев.
	total, err := calcCost(t, userID, "04-2025", "09-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 6*200 {
		t.Errorf("want %d, got %d", 6*200, total)
	}
}

// Случай 6: Подписка полностью ВНЕ периода (до него) — должна быть 0.
func TestCost_SubBeforePeriod(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Netflix",
		Price:       799,
		UserID:      userID,
		StartDate:   "01-2024",
		EndDate:     ptr("06-2024"),
	})

	total, err := calcCost(t, userID, "01-2025", "12-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("want 0, got %d", total)
	}
}

// Случай 7: Подписка полностью ВНЕ периода (после него) — должна быть 0.
func TestCost_SubAfterPeriod(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Netflix",
		Price:       799,
		UserID:      userID,
		StartDate:   "01-2026",
	})

	total, err := calcCost(t, userID, "01-2025", "12-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("want 0, got %d", total)
	}
}

// Случай 8: Несколько подписок — суммируются все.
func TestCost_MultipleSubscriptions(t *testing.T) {
	truncate(t)
	userID := uuid.New()

	// Netflix: январь–июнь = 6 мес × 799 = 4794
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Netflix",
		Price:       799,
		UserID:      userID,
		StartDate:   "01-2025",
		EndDate:     ptr("06-2025"),
	})
	// Yandex Plus: март–декабрь = 10 мес × 400 = 4000
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Yandex Plus",
		Price:       400,
		UserID:      userID,
		StartDate:   "03-2025",
	})

	total, err := calcCost(t, userID, "01-2025", "12-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := 6*799 + 10*400
	if total != want {
		t.Errorf("want %d, got %d", want, total)
	}
}

// Случай 9: Фильтрация по service_name.
func TestCost_FilterByServiceName(t *testing.T) {
	truncate(t)
	userID := uuid.New()

	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Netflix",
		Price:       799,
		UserID:      userID,
		StartDate:   "01-2025",
		EndDate:     ptr("06-2025"),
	})
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Spotify",
		Price:       299,
		UserID:      userID,
		StartDate:   "01-2025",
		EndDate:     ptr("12-2025"),
	})

	// Только Netflix: 6 мес × 799
	total, err := calcCost(t, userID, "01-2025", "12-2025", ptr("Netflix"))
	if err != nil {
		t.Fatal(err)
	}
	if total != 6*799 {
		t.Errorf("want %d, got %d", 6*799, total)
	}
}

// Случай 10: Подписки другого пользователя не влияют на результат.
func TestCost_OtherUserNotAffected(t *testing.T) {
	truncate(t)
	user1 := uuid.New()
	user2 := uuid.New()

	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Netflix",
		Price:       799,
		UserID:      user1,
		StartDate:   "01-2025",
	})
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Netflix",
		Price:       799,
		UserID:      user2,
		StartDate:   "01-2025",
	})

	total, err := calcCost(t, user1, "01-2025", "12-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Только user1: 12 мес × 799
	if total != 12*799 {
		t.Errorf("want %d, got %d", 12*799, total)
	}
}

// Случай 11: Нет подписок — результат 0.
func TestCost_NoSubscriptions(t *testing.T) {
	truncate(t)
	total, err := calcCost(t, uuid.New(), "01-2025", "12-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("want 0, got %d", total)
	}
}

// Случай 12: Период из одного месяца.
func TestCost_SingleMonthPeriod(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Yandex Plus",
		Price:       400,
		UserID:      userID,
		StartDate:   "06-2025",
	})

	total, err := calcCost(t, userID, "06-2025", "06-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 400 {
		t.Errorf("want 400, got %d", total)
	}
}

// Случай 13: Подписка активна ровно 1 месяц на границе периода.
func TestCost_SubOnPeriodBoundary(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	// Подписка только в декабре 2025.
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Apple TV",
		Price:       500,
		UserID:      userID,
		StartDate:   "12-2025",
		EndDate:     ptr("12-2025"),
	})

	total, err := calcCost(t, userID, "01-2025", "12-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 500 {
		t.Errorf("want 500, got %d", total)
	}
}

// Случай 14: Переход через границу года (считаем в разные годы).
func TestCost_CrossYearBoundary(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Spotify",
		Price:       299,
		UserID:      userID,
		StartDate:   "10-2024",
		EndDate:     ptr("03-2025"),
	})

	// Период ноябрь 2024 - февраль 2025 (4 месяца)
	total, err := calcCost(t, userID, "11-2024", "02-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4*299 {
		t.Errorf("want %d, got %d", 4*299, total)
	}
}

// Случай 15: Подписка с нулевой стоимостью.
func TestCost_ZeroPrice(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Free Service",
		Price:       0,
		UserID:      userID,
		StartDate:   "01-2025",
	})

	total, err := calcCost(t, userID, "01-2025", "12-2025", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("want 0, got %d", total)
	}
}

// Случай 16: Граница года и один месяц.
func TestCost_ExactOneMonthCrossYear(t *testing.T) {
	truncate(t)
	userID := uuid.New()
	mustCreate(t, domain.SubscriptionCreateParams{
		ServiceName: "Spotify",
		Price:       299,
		UserID:      userID,
		StartDate:   "12-2024",
	})

	total, err := calcCost(t, userID, "12-2024", "12-2024", nil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 299 {
		t.Errorf("want 299, got %d", total)
	}
}

// ===========================================================================
// helpers
// ===========================================================================

func ptrInt(i int) *int { return &i }
