package domain

import (
	"time"
)

type Payment struct {
	CorrelationID string `json:"correlationId" validate:"required,uuid"`
	Amount int64 `json:"amount" validate:"required"` 
	Timestamp     time.Time `json:"timestamp,omitempty"`
}

type ProcessorPayment struct {
	CorrelationID string  `json:"correlationId"`
	Amount        int64 `json:"amount"`
	RequestedAt   string  `json:"requestedAt"`
}

func NewPayment(correlationID string, amount int64) *Payment {
	return &Payment{
		CorrelationID: correlationID,
		Amount:        amount,
		Timestamp:   time.Now().UTC(),
	}
}

func (p *Payment) Validate() error {
    if len(p.CorrelationID) != 36 {
        return ErrInvalidCorrelationID
    }
    
    if p.Amount <= 0 {
        return ErrInvalidAmount
    }

    return nil
}

func (p *Payment) ToProcessorPayload() ProcessorPayment {
	return ProcessorPayment{
		CorrelationID: p.CorrelationID,
		Amount:        p.Amount,
		RequestedAt:   p.Timestamp.Format(time.RFC3339),
	}
}


var (
	ErrInvalidCorrelationID = NewPaymentError("invalid correlation ID")
	ErrInvalidAmount       = NewPaymentError("invalid amount")
)

type PaymentError struct {
	Message string
}

func NewPaymentError(message string) *PaymentError {
	return &PaymentError{Message: message}
}

func (e *PaymentError) Error() string {
	return e.Message
}