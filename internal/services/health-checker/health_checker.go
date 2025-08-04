package healthchecker

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diogomassis/rinha-2025/internal/env"
)

const (
	failureRateThreshold  = 0.5
	minRequestsForCircuit = 20
	openStateDuration     = 30 * time.Second
	healthCheckInterval   = 5 * time.Second
	statsResetInterval    = 2 * time.Minute
)

type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

type ServiceStatus struct {
	name            string
	url             string
	mutex           sync.RWMutex
	state           CircuitState
	lastStateChange time.Time

	successCount    atomic.Int64
	requestCount    atomic.Int64
	avgResponseTime atomic.Int64
}

type HealthChecker struct {
	services map[string]*ServiceStatus
	client   *http.Client
	quit     chan bool
}

type HealthCheckResponse struct {
	Failing         bool `json:"failing"`
	MinResponseTime int  `json:"minResponseTime"`
}

func New() *HealthChecker {
	hc := &HealthChecker{
		services: make(map[string]*ServiceStatus),
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 10,
				MaxIdleConns:        20,
			},
		},
		quit: make(chan bool),
	}
	hc.services["d"] = &ServiceStatus{name: "d", url: env.Env.ProcessorDefaultUrl}
	hc.services["f"] = &ServiceStatus{name: "f", url: env.Env.ProcessorFallbackUrl}
	return hc
}

func (hc *HealthChecker) Start() {
	go hc.runHealthChecks()
	go hc.resetStatsPeriodically()
}

func (hc *HealthChecker) Stop() {
	close(hc.quit)
}

func (hc *HealthChecker) RegisterServiceFailure(serviceName string) {
	s, ok := hc.services[serviceName]
	if !ok {
		return
	}

	totalRequests := s.requestCount.Add(1)
	successes := s.successCount.Load()

	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.state == StateClosed && totalRequests >= minRequestsForCircuit {
		failureRate := float64(totalRequests-successes) / float64(totalRequests)
		if failureRate > failureRateThreshold {
			log.Printf("WARN: Circuit Breaker for '%s' OPEN. Failure rate: %.2f%%", s.name, failureRate*100)
			s.state = StateOpen
			s.lastStateChange = time.Now()
		}
	}
}

func (hc *HealthChecker) RegisterServiceSuccess(serviceName string, responseTime time.Duration) {
	s, ok := hc.services[serviceName]
	if !ok {
		return
	}
	s.requestCount.Add(1)
	s.successCount.Add(1)

	currentAvg := s.avgResponseTime.Load()
	newAvg := (currentAvg + responseTime.Milliseconds()) / 2
	s.avgResponseTime.Store(newAvg)

	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.state == StateHalfOpen {
		log.Printf("INFO: Service '%s' recovered. Circuit Breaker CLOSED.", s.name)
		s.state = StateClosed
		s.resetStats()
	}
}

func (hc *HealthChecker) ChooseNextService() (string, error) {
	var bestService *ServiceStatus
	maxScore := -1.0

	for _, s := range hc.services {
		s.mutex.RLock()
		state := s.state
		s.mutex.RUnlock()

		if state == StateOpen {
			continue
		}

		score := hc.calculateHealthScore(s)
		if score > maxScore {
			maxScore = score
			bestService = s
		}
	}

	if bestService == nil {
		return "", errors.New("nenhum serviço de pagamento está disponível no momento")
	}
	return bestService.name, nil
}

func (hc *HealthChecker) calculateHealthScore(s *ServiceStatus) float64 {
	requests := s.requestCount.Load()
	if requests < 5 {
		return 1.0
	}
	successes := s.successCount.Load()

	successRate := float64(successes) / float64(requests)

	latency := float64(s.avgResponseTime.Load())
	latencyScore := math.Exp(-0.005 * latency)

	return (0.7 * successRate) + (0.3 * latencyScore)
}

func (s *ServiceStatus) resetStats() {
	s.requestCount.Store(0)
	s.successCount.Store(0)
	s.avgResponseTime.Store(0)
}

func (hc *HealthChecker) runHealthChecks() {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, s := range hc.services {
				go hc.checkService(s)
			}
		case <-hc.quit:
			return
		}
	}
}

func (hc *HealthChecker) resetStatsPeriodically() {
	ticker := time.NewTicker(statsResetInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			log.Println("INFO: Resetting health statistics of services.")
			for _, s := range hc.services {
				s.mutex.Lock()
				s.resetStats()
				s.mutex.Unlock()
			}
		case <-hc.quit:
			return
		}
	}
}

func (hc *HealthChecker) checkService(s *ServiceStatus) {
	s.mutex.Lock()
	if s.state == StateOpen && time.Since(s.lastStateChange) > openStateDuration {
		log.Printf("INFO: Circuit Breaker for '%s' in HALF-OPEN mode.", s.name)
		s.state = StateHalfOpen
	}
	s.mutex.Unlock()

	s.mutex.RLock()
	state := s.state
	s.mutex.RUnlock()
	if state == StateOpen {
		return
	}

	url := fmt.Sprintf("%s/payments/service-health", s.url)
	resp, err := hc.client.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		s.mutex.Lock()
		if s.state == StateHalfOpen {
			log.Printf("WARN: Health check failed. Circuit Breaker for '%s' OPEN again.", s.name)
			s.state = StateOpen
			s.lastStateChange = time.Now()
		}
		s.mutex.Unlock()
		return
	}
	defer resp.Body.Close()

	var health HealthCheckResponse
	if json.NewDecoder(resp.Body).Decode(&health) == nil && !health.Failing {
		s.mutex.Lock()
		if s.state == StateHalfOpen {
			log.Printf("INFO: Health check succeeded. Circuit Breaker for '%s' CLOSED.", s.name)
			s.state = StateClosed
			s.resetStats()
		}
		s.mutex.Unlock()
	}
}
