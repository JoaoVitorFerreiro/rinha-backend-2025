package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/JoaoVitorFerreiro/rinha-backend-2025/internal/application"
	"github.com/JoaoVitorFerreiro/rinha-backend-2025/internal/domain"
)

type ProcessorClient struct {
	httpClient   *http.Client
	defaultURL   string
	fallbackURL  string
	healthCache  map[application.ProcessorType]*application.HealthStatus
	healthMu     sync.RWMutex
	lastHealthCheck time.Time
}

func NewProcessorClient() *ProcessorClient {
	defaultURL := os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT")
	if defaultURL == "" {
		defaultURL = "http://payment-processor-default:8080"
	}

	fallbackURL := os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK")
	if fallbackURL == "" {
		fallbackURL = "http://payment-processor-fallback:8080"
	}

	client := &ProcessorClient{
        httpClient: &http.Client{
            Timeout: 5 * time.Second, 
            Transport: &http.Transport{
                MaxIdleConns:        200,  
                MaxIdleConnsPerHost: 50,   
                IdleConnTimeout:     30 * time.Second, 
                DisableKeepAlives:   false,
                MaxConnsPerHost:     100,  
                WriteBufferSize:     32 * 1024,
                ReadBufferSize:      32 * 1024,
            },
        },
	}
	// Inicia health checker
	go client.healthChecker()

	return client
}

func (c *ProcessorClient) SendPayment(ctx context.Context, payment domain.ProcessorPayment, processorType application.ProcessorType) error {
	url := c.getURL(processorType)
	
	payloadBytes, err := json.Marshal(payment)
	if err != nil {
		return fmt.Errorf("failed to marshal payment: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url+"/payments", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("processor returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *ProcessorClient) GetHealth(ctx context.Context, processorType application.ProcessorType) (*application.HealthStatus, error) {
	c.healthMu.RLock()
	health, exists := c.healthCache[processorType]
	c.healthMu.RUnlock()

	if exists {
		return health, nil
	}

	// Se não tem cache, retorna estado conservador
	return &application.HealthStatus{
		Failing:         false,
		MinResponseTime: 100,
	}, nil
}

func (c *ProcessorClient) healthChecker() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.checkHealth(application.ProcessorDefault)
		c.checkHealth(application.ProcessorFallback)
	}
}

func (c *ProcessorClient) checkHealth(processorType application.ProcessorType) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	url := c.getURL(processorType)
	req, err := http.NewRequestWithContext(ctx, "GET", url+"/payments/service-health", nil)
	if err != nil {
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var health application.HealthStatus
		if err := json.NewDecoder(resp.Body).Decode(&health); err == nil {
			c.healthMu.Lock()
			c.healthCache[processorType] = &health
			c.healthMu.Unlock()
		}
	}
}

func (c *ProcessorClient) getURL(processorType application.ProcessorType) string {
	if processorType == application.ProcessorDefault {
		return c.defaultURL
	}
	return c.fallbackURL
}