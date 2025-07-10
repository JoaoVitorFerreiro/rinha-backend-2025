package application

import (
	"context"
	"time"

	"github.com/JoaoVitorFerreiro/rinha-backend-2025/internal/domain"
)

type PaymentService struct {
	processorClient ProcessorClient
	metricsStore    MetricsStore
	circuitBreaker  CircuitBreaker
	paymentQueue    chan *domain.Payment
}

// Interfaces que a infra implementa
type ProcessorClient interface {
	SendPayment(ctx context.Context, payment domain.ProcessorPayment, processorType ProcessorType) error
	GetHealth(ctx context.Context, processorType ProcessorType) (*HealthStatus, error)
}

type MetricsStore interface {
	IncrementDefault(amount int64)
	IncrementFallback(amount int64)
	GetSummary(from, to *time.Time) PaymentSummary
}

type CircuitBreaker interface {
	IsOpen() bool
	RecordSuccess()
	RecordFailure()
}

type ProcessorType int

const (
	ProcessorDefault ProcessorType = iota
	ProcessorFallback
)

type HealthStatus struct {
	Failing         bool `json:"failing"`
	MinResponseTime int  `json:"minResponseTime"`
}

type PaymentSummary struct {
	Default  ProcessorSummary `json:"default"`
	Fallback ProcessorSummary `json:"fallback"`
}

type ProcessorSummary struct {
	TotalRequests int64 `json:"totalRequests"`
	TotalAmount   int64 `json:"totalAmount"`
}

func NewPaymentService(client ProcessorClient, metrics MetricsStore, breaker CircuitBreaker) *PaymentService {
	service := &PaymentService{
		processorClient: client,
		metricsStore:    metrics,
		circuitBreaker:  breaker,
		paymentQueue:    make(chan *domain.Payment, 10000),
	}

	// Inicia workers
	for i := 0; i < 10; i++ {
		go service.paymentWorker()
	}

	return service
}

func (s *PaymentService) ProcessPayment(ctx context.Context, payment *domain.Payment) error {
	if err := payment.Validate(); err != nil {
		return err
	}

	select {
	case s.paymentQueue <- payment:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return domain.NewPaymentError("payment queue is full")
	}
}

func (s *PaymentService) GetSummary(ctx context.Context, from, to *time.Time) PaymentSummary {
	return s.metricsStore.GetSummary(from, to)
}

func (s *PaymentService) paymentWorker() {
	for payment := range s.paymentQueue {
		s.processPaymentSync(payment)
	}
}

func (s *PaymentService) processPaymentSync(payment *domain.Payment) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	processorType := s.chooseProcessor(ctx)
	processorPayment := payment.ToProcessorPayload()

	err := s.processorClient.SendPayment(ctx, processorPayment, processorType)

	if err != nil {
		if processorType == ProcessorDefault {
			s.circuitBreaker.RecordFailure()
			fallbackErr := s.processorClient.SendPayment(ctx, processorPayment, ProcessorFallback)
			if fallbackErr == nil {
				s.metricsStore.IncrementFallback(payment.Amount)
			}
		}
		return
	}

	if processorType == ProcessorDefault {
		s.circuitBreaker.RecordSuccess()
		s.metricsStore.IncrementDefault(payment.Amount)
	} else {
		s.metricsStore.IncrementFallback(payment.Amount)
	}
}

func (s *PaymentService) chooseProcessor(ctx context.Context) ProcessorType {
	if s.circuitBreaker.IsOpen() {
		return ProcessorFallback
	}

	health, err := s.processorClient.GetHealth(ctx, ProcessorDefault)
	if err != nil || health.Failing {
		return ProcessorFallback
	}

	return ProcessorDefault
}