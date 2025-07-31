package healthchecker

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type ServiceStatus struct {
	available       int32
	minResponseTime int
	failureCount    int32
	lastFailure     time.Time
	mutex           sync.RWMutex
}

type HealthChecker struct {
	defaultService  *ServiceStatus
	fallbackService *ServiceStatus
	client          *http.Client
	quit            chan bool
}

type HealthCheckResponse struct {
	Failing         bool `json:"failing"`
	MinResponseTime int  `json:"minResponseTime"`
}

func New() *HealthChecker {
	return &HealthChecker{
		defaultService:  &ServiceStatus{},
		fallbackService: &ServiceStatus{},
		client: &http.Client{
			Timeout: 2 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        5,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     30 * time.Second,
				DisableKeepAlives:   false,
			},
		},
		quit: make(chan bool),
	}
}

func (hm *HealthChecker) Start() {
	log.Println("Starting health monitoring...")
	hm.setServiceStatus(hm.defaultService, false, 0)
	hm.setServiceStatus(hm.fallbackService, false, 0)
	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			go hm.checkService("default", "http://payment-processor-default:8080", hm.defaultService)
			go hm.checkService("fallback", "http://payment-processor-fallback:8080", hm.fallbackService)

		case <-hm.quit:
			log.Println("Stopping health monitoring...")
			return
		}
	}
}

func (hm *HealthChecker) Stop() {
	close(hm.quit)
}

func (hm *HealthChecker) IsDefaultServiceAvailable() bool {
	return hm.isServiceAvailable(hm.defaultService)
}

func (hm *HealthChecker) IsFallbackServiceAvailable() bool {
	return hm.isServiceAvailable(hm.fallbackService)
}

func (hm *HealthChecker) GetDefaultMinResponseTime() int {
	return hm.getMinResponseTime(hm.defaultService)
}

func (hm *HealthChecker) GetFallbackMinResponseTime() int {
	return hm.getMinResponseTime(hm.fallbackService)
}

func (hm *HealthChecker) RegisterServiceFailure(serviceName string) {
	var service *ServiceStatus
	if serviceName == "default" {
		service = hm.defaultService
	} else {
		service = hm.fallbackService
	}

	atomic.AddInt32(&service.failureCount, 1)
	service.mutex.Lock()
	service.lastFailure = time.Now()
	service.mutex.Unlock()
}

func (hm *HealthChecker) RegisterServiceSuccess(serviceName string) {
	var service *ServiceStatus
	if serviceName == "default" {
		service = hm.defaultService
	} else {
		service = hm.fallbackService
	}

	atomic.StoreInt32(&service.failureCount, 0)
}

func (hm *HealthChecker) isServiceAvailable(service *ServiceStatus) bool {
	if atomic.LoadInt32(&service.failureCount) >= 10 {
		service.mutex.RLock()
		lastFailure := service.lastFailure
		service.mutex.RUnlock()
		if time.Since(lastFailure) < 60*time.Second {
			return false
		}
		atomic.StoreInt32(&service.failureCount, 0)
	}
	return atomic.LoadInt32(&service.available) == 1
}

func (hm *HealthChecker) getMinResponseTime(service *ServiceStatus) int {
	service.mutex.RLock()
	defer service.mutex.RUnlock()
	return service.minResponseTime
}

func (hm *HealthChecker) setServiceStatus(service *ServiceStatus, available bool, minResponseTime int) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	if available {
		atomic.StoreInt32(&service.available, 1)
	} else {
		atomic.StoreInt32(&service.available, 0)
	}
	service.minResponseTime = minResponseTime
}

func (hm *HealthChecker) checkService(serviceName, baseURL string, service *ServiceStatus) {
	url := fmt.Sprintf("%s/payments/service-health", baseURL)
	resp, err := hm.client.Get(url)
	if err != nil {
		log.Printf("Health check failed for %s: %v", serviceName, err)
		hm.setServiceStatus(service, false, 0)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		log.Printf("Rate limited on health check for %s - keeping current status", serviceName)
		return
	}
	if resp.StatusCode != 200 {
		log.Printf("Health check for %s returned status %d", serviceName, resp.StatusCode)
		hm.setServiceStatus(service, false, 0)
		return
	}
	var health HealthCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		log.Printf("Failed to decode health response for %s: %v", serviceName, err)
		hm.setServiceStatus(service, false, 0)
		return
	}
	available := !health.Failing
	hm.setServiceStatus(service, available, health.MinResponseTime)
	log.Printf("Health status updated for %s: available=%t, minResponseTime=%dms",
		serviceName, available, health.MinResponseTime)
}
