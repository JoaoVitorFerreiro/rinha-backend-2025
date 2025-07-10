package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/JoaoVitorFerreiro/rinha-backend-2025/internal/application"

	"github.com/redis/go-redis/v9"
)

type MetricsStore struct {
	defaultRequests  int64
	defaultAmount    int64
	fallbackRequests int64
	fallbackAmount   int64

	redisClient *redis.Client
	payments    []PaymentRecord
	mu          sync.RWMutex
}

type PaymentRecord struct {
	ProcessorType application.ProcessorType `json:"processorType"`
	Amount        int64                     `json:"amount"`
	ProcessedAt   time.Time                 `json:"processedAt"`
}

func NewMetricsStore() *MetricsStore {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:            redisAddr,
		Password:        "",
		DB:              0,
		DialTimeout:     2 * time.Second,
		ReadTimeout:     500 * time.Millisecond,
		WriteTimeout:    500 * time.Millisecond,
		PoolSize:        10,
		MinIdleConns:    2,
		MaxRetries:      2,
		PoolTimeout:     3 * time.Second,
		ConnMaxIdleTime: 10 * time.Minute, 
	})

	store := &MetricsStore{
		redisClient: rdb,
		payments:    make([]PaymentRecord, 0, 100000),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	if err := rdb.Ping(ctx).Err(); err != nil {
		fmt.Printf("Redis connection failed, using memory-only mode: %v\n", err)
	} else {
		fmt.Println("Redis connected successfully")
	}

	go store.loadFromRedis()
	return store
}

func (m *MetricsStore) IncrementDefault(amount int64) {
	atomic.AddInt64(&m.defaultRequests, 1)
	atomic.AddInt64(&m.defaultAmount, amount)

	record := PaymentRecord{
		ProcessorType: application.ProcessorDefault,
		Amount:        amount,
		ProcessedAt:   time.Now().UTC(),
	}

	m.addPaymentRecord(record)
}

func (m *MetricsStore) IncrementFallback(amount int64) {
	atomic.AddInt64(&m.fallbackRequests, 1)
	atomic.AddInt64(&m.fallbackAmount, amount)

	record := PaymentRecord{
		ProcessorType: application.ProcessorFallback,
		Amount:        amount,
		ProcessedAt:   time.Now().UTC(),
	}

	m.addPaymentRecord(record)
}

func (m *MetricsStore) GetSummary(from, to *time.Time) application.PaymentSummary {
	if from == nil && to == nil {
		return application.PaymentSummary{
			Default: application.ProcessorSummary{
				TotalRequests: atomic.LoadInt64(&m.defaultRequests),
				TotalAmount:   atomic.LoadInt64(&m.defaultAmount),
			},
			Fallback: application.ProcessorSummary{
				TotalRequests: atomic.LoadInt64(&m.fallbackRequests),
				TotalAmount:   atomic.LoadInt64(&m.fallbackAmount),
			},
		}
	}

	return m.getSummaryWithFilters(from, to)
}

func (m *MetricsStore) addPaymentRecord(record PaymentRecord) {
	m.mu.Lock()
	m.payments = append(m.payments, record)
	m.mu.Unlock()

	go m.persistToRedis(record)
}

func (m *MetricsStore) persistToRedis(record PaymentRecord) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	data, err := json.Marshal(record)
	if err != nil {
		return
	}

	key := fmt.Sprintf("payments:%s", time.Now().Format("2006-01-02"))
	
	pipe := m.redisClient.Pipeline()
	pipe.LPush(ctx, key, data)
	pipe.Expire(ctx, key, 7*24*time.Hour)
	
	_, err = pipe.Exec(ctx)
	if err != nil {
		return
	}
}

func (m *MetricsStore) getSummaryWithFilters(from, to *time.Time) application.PaymentSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var defaultRequests, defaultAmount int64
	var fallbackRequests, fallbackAmount int64

	for _, payment := range m.payments {
		if from != nil && payment.ProcessedAt.Before(*from) {
			continue
		}
		if to != nil && payment.ProcessedAt.After(*to) {
			continue
		}

		if payment.ProcessorType == application.ProcessorDefault {
			defaultRequests++
			defaultAmount += payment.Amount
		} else {
			fallbackRequests++
			fallbackAmount += payment.Amount
		}
	}

	return application.PaymentSummary{
		Default: application.ProcessorSummary{
			TotalRequests: defaultRequests,
			TotalAmount:   defaultAmount,
		},
		Fallback: application.ProcessorSummary{
			TotalRequests: fallbackRequests,
			TotalAmount:   fallbackAmount,
		},
	}
}

func (m *MetricsStore) loadFromRedis() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < 7; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		key := fmt.Sprintf("payments:%s", date)

		results, err := m.redisClient.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			continue
		}

		for _, result := range results {
			var record PaymentRecord
			if err := json.Unmarshal([]byte(result), &record); err == nil {
				m.mu.Lock()
				m.payments = append(m.payments, record)
				
				if record.ProcessorType == application.ProcessorDefault {
					atomic.AddInt64(&m.defaultRequests, 1)
					atomic.AddInt64(&m.defaultAmount, record.Amount)
				} else {
					atomic.AddInt64(&m.fallbackRequests, 1)
					atomic.AddInt64(&m.fallbackAmount, record.Amount)
				}
				m.mu.Unlock()
			}
		}
	}
}