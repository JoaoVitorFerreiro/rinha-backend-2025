package main

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/JoaoVitorFerreiro/rinha-backend-2025/internal/application"
	"github.com/JoaoVitorFerreiro/rinha-backend-2025/internal/infra/circuit"
	"github.com/JoaoVitorFerreiro/rinha-backend-2025/internal/infra/client"
	httpHandler "github.com/JoaoVitorFerreiro/rinha-backend-2025/internal/infra/http"
	"github.com/JoaoVitorFerreiro/rinha-backend-2025/internal/infra/memory"
)

func main() {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.Server.ReadTimeout = 5 * time.Second
	e.Server.WriteTimeout = 5 * time.Second
	e.Server.IdleTimeout = 60 * time.Second

	metricsStore := memory.NewMetricsStore()
	circuitBreaker := circuit.NewCircuitBreaker()
	processorClient := client.NewProcessorClient()

	paymentService := application.NewPaymentService(
		processorClient,
		metricsStore,
		circuitBreaker,
	)

	paymentHandler := httpHandler.NewPaymentHandler(paymentService)

	e.POST("/payments", paymentHandler.ProcessPayment)
	e.GET("/payments-summary", paymentHandler.GetSummary)

	e.GET("/health", paymentHandler.HealthCheck)

	port := "8080"

	slog.Info("Starting Rinha Backend 2025 server", "port", port)

	if err := e.Start(":" + port); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("failed to start server", "error", err)
	}
}