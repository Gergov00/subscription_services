package v1

import (
	"fmt"
	"net/http"
	"subscriptionServices/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SubscriptionHandler struct {
	service domain.SubscriptionService
}

func NewSubscriptionHandler(service domain.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{service: service}
}

// DTOs for HTTP Layer

type CreateSubscriptionRequest struct {
	ServiceName string  `json:"service_name" binding:"required"`
	Price       int     `json:"price" binding:"required,gt=0"`
	UserID      string  `json:"user_id" binding:"required,uuid"`
	StartDate   string  `json:"start_date" binding:"required"`
	EndDate     *string `json:"end_date,omitempty"`
}

type UpdateSubscriptionRequest struct {
	ServiceName *string `json:"service_name,omitempty"`
	Price       *int    `json:"price,omitempty" binding:"omitempty,gt=0"`
	StartDate   *string `json:"start_date,omitempty"`
	EndDate     *string `json:"end_date,omitempty"`
}

type CalculateCostRequest struct {
	UserID      string  `form:"user_id" binding:"required,uuid"`
	ServiceName *string `form:"service_name,omitempty"`
	StartPeriod string  `form:"start_period" binding:"required"`
	EndPeriod   string  `form:"end_period" binding:"required"`
}

type SubscriptionResponse struct {
	ID          string  `json:"id"`
	ServiceName string  `json:"service_name"`
	Price       int     `json:"price"`
	UserID      string  `json:"user_id"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func mapToResponse(sub *domain.Subscription) SubscriptionResponse {
	return SubscriptionResponse{
		ID:          sub.ID.String(),
		ServiceName: sub.ServiceName,
		Price:       sub.Price,
		UserID:      sub.UserID.String(),
		StartDate:   sub.StartDate,
		EndDate:     sub.EndDate,
		CreatedAt:   sub.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   sub.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// Handlers

// Create godoc
// @Summary Создать новую подписку
// @Description Создает новую запись о подписке для пользователя
// @Tags subscriptions
// @Accept json
// @Produce json
// @Param request body CreateSubscriptionRequest true "Данные подписки"
// @Success 201 {object} SubscriptionResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /subscriptions [post]
func (h *SubscriptionHandler) Create(c *gin.Context) {
	var req CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id format"})
		return
	}

	params := domain.SubscriptionCreateParams{
		ServiceName: req.ServiceName,
		Price:       req.Price,
		UserID:      userID,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
	}

	sub, err := h.service.CreateSubscription(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, mapToResponse(sub))
}

// GetByID godoc
// @Summary Получить подписку по ID
// @Description Возвращает данные подписки по ее уникальному идентификатору
// @Tags subscriptions
// @Produce json
// @Param id path string true "ID подписки"
// @Success 200 {object} SubscriptionResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /subscriptions/{id} [get]
func (h *SubscriptionHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription ID"})
		return
	}

	sub, err := h.service.GetSubscription(c.Request.Context(), id)
	if err != nil {
		if err.Error() == "subscription not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, mapToResponse(sub))
}

// Update godoc
// @Summary Обновить подписку
// @Description Частичное обновление данных подписки
// @Tags subscriptions
// @Accept json
// @Produce json
// @Param id path string true "ID подписки"
// @Param request body UpdateSubscriptionRequest true "Поля для обновления"
// @Success 200 {object} SubscriptionResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /subscriptions/{id} [patch]
func (h *SubscriptionHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription ID"})
		return
	}

	var req UpdateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	params := domain.SubscriptionUpdateParams{
		ServiceName: req.ServiceName,
		Price:       req.Price,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
	}

	sub, err := h.service.UpdateSubscription(c.Request.Context(), id, params)
	if err != nil {
		switch err.Error() {
		case "subscription not found":
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case "start date must not be later than end date",
			"invalid start_date format, expected MM-YYYY",
			"invalid end_date format, expected MM-YYYY":
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, mapToResponse(sub))
}

// Delete godoc
// @Summary Удалить подписку
// @Description Удаляет запись о подписке по ее ID
// @Tags subscriptions
// @Param id path string true "ID подписки"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /subscriptions/{id} [delete]
func (h *SubscriptionHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription ID"})
		return
	}

	if err := h.service.DeleteSubscription(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// List godoc
// @Summary Список всех подписок
// @Description Возвращает список всех подписок с поддержкой пагинации
// @Tags subscriptions
// @Produce json
// @Param limit query int false "Лимит" default(10)
// @Param offset query int false "Смещение" default(0)
// @Success 200 {array} SubscriptionResponse
// @Failure 500 {object} map[string]string
// @Router /subscriptions [get]
func (h *SubscriptionHandler) List(c *gin.Context) {
	limit, err := parseInt(c.DefaultQuery("limit", "10"))
	if err != nil || limit <= 0 || limit > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer no greater than 100"})
		return
	}

	offset, err := parseInt(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "offset must be a non-negative integer"})
		return
	}

	subs, err := h.service.ListSubscriptions(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var resp []SubscriptionResponse
	for _, sub := range subs {
		resp = append(resp, mapToResponse(sub))
	}

	if resp == nil {
		resp = []SubscriptionResponse{}
	}

	c.JSON(http.StatusOK, resp)
}

// CalculateCost godoc
// @Summary Подсчет стоимости подписок
// @Description Считает суммарную стоимость всех подписок за выбранный период с фильтрацией по ID пользователя и опционально названию сервиса
// @Tags subscriptions
// @Produce json
// @Param user_id query string true "ID Пользователя"
// @Param service_name query string false "Название сервиса"
// @Param start_period query string true "Начало периода (MM-YYYY)"
// @Param end_period query string true "Конец периода (MM-YYYY)"
// @Success 200 {object} map[string]int
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /subscriptions/cost [get]
func (h *SubscriptionHandler) CalculateCost(c *gin.Context) {
	var req CalculateCostRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id format"})
		return
	}

	params := domain.CalculateCostParams{
		UserID:      userID,
		ServiceName: req.ServiceName,
		StartPeriod: req.StartPeriod,
		EndPeriod:   req.EndPeriod,
	}

	totalCost, err := h.service.CalculateTotalCost(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"total_cost": totalCost})
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
