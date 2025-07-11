package http

import (
	"net/http"
	"time"

	"github.com/JoaoVitorFerreiro/rinha-backend-2025/internal/application"
	"github.com/JoaoVitorFerreiro/rinha-backend-2025/internal/domain"

	"github.com/labstack/echo/v4"
)

type PaymentHandler struct {
    paymentService *application.PaymentService 
}

func NewPaymentHandler(service *application.PaymentService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: service,
	}
}

func (h *PaymentHandler) ProcessPayment(c echo.Context) error {
    var req struct {
        CorrelationID string  `json:"correlationId"`
        Amount        float64 `json:"amount"`
    }

    if err := c.Bind(&req); err != nil {
        return c.JSON(400, map[string]string{"error": "Invalid JSON"})
    }

    if len(req.CorrelationID) != 36 || req.Amount <= 0 {
        return c.JSON(400, map[string]string{"error": "Invalid input"})
    }

    payment := domain.Payment{
        CorrelationID: req.CorrelationID,
        Amount:        int64(req.Amount * 100),
        Timestamp:     time.Now().UTC(),
    }

    result, err := h.paymentService.ProcessPayment(c.Request().Context(), &payment)
    if err != nil {
        return c.JSON(500, map[string]string{"error": err.Error()})
    }
    
    return c.JSON(200, result)
}

func (h *PaymentHandler) GetSummary(c echo.Context) error {
	// Parse dos parâmetros de data
	fromStr := c.QueryParam("from")
	toStr := c.QueryParam("to")
	
	var from, to *time.Time
	
	if fromStr != "" {
		if parsed, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = &parsed
		}
	}
	
	if toStr != "" {
		if parsed, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = &parsed
		}
	}

	summary := h.paymentService.GetSummary(c.Request().Context(), from, to)
	
	response := map[string]interface{}{
		"default": map[string]interface{}{
			"totalRequests": summary.Default.TotalRequests,
			"totalAmount":   float64(summary.Default.TotalAmount) / 100,
		},
		"fallback": map[string]interface{}{
			"totalRequests": summary.Fallback.TotalRequests,
			"totalAmount":   float64(summary.Fallback.TotalAmount) / 100,
		},
	}
	
	return c.JSON(http.StatusOK, response)
}

func (h *PaymentHandler) HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}