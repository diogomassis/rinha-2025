package healthchecker

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diogomassis/rinha-2025/internal/env"
)

type ServiceStatus struct {
	available       bool
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
	hm.setServiceStatus(hm.defaultService, false, 0)
	hm.setServiceStatus(hm.fallbackService, false, 0)
	ticker := time.NewTicker(5 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				go hm.checkService("d", env.Env.ProcessorDefaultUrl, hm.defaultService)
				go hm.checkService("f", env.Env.ProcessorFallbackUrl, hm.fallbackService)

			case <-hm.quit:
				ticker.Stop()
				return
			}
		}
	}()
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
	service := hm.getServiceByName(serviceName)
	if service == nil {
		return
	}

	atomic.AddInt32(&service.failureCount, 1)
	service.mutex.Lock()
	service.lastFailure = time.Now()
	service.mutex.Unlock()
}

func (hm *HealthChecker) RegisterServiceSuccess(serviceName string) {
	service := hm.getServiceByName(serviceName)
	if service == nil {
		return
	}
	atomic.StoreInt32(&service.failureCount, 0)
}

func (sc *HealthChecker) ChooseNextService() (string, error) {
	if sc.IsDefaultServiceAvailable() {
		return "d", nil
	}
	if sc.IsFallbackServiceAvailable() {
		return "f", nil
	}
	return "", errors.New("no payment service is available at the moment")
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
	service.mutex.RLock()
	isAvailable := service.available
	service.mutex.RUnlock()
	return isAvailable
}

func (hm *HealthChecker) getMinResponseTime(service *ServiceStatus) int {
	service.mutex.RLock()
	defer service.mutex.RUnlock()
	return service.minResponseTime
}

func (hm *HealthChecker) setServiceStatus(service *ServiceStatus, available bool, minResponseTime int) {
	service.mutex.Lock()
	defer service.mutex.Unlock()

	service.available = available
	service.minResponseTime = minResponseTime
}

func (hm *HealthChecker) getServiceByName(name string) *ServiceStatus {
	if name == "d" {
		return hm.defaultService
	}
	if name == "f" {
		return hm.fallbackService
	}
	return nil
}

func (hm *HealthChecker) checkService(serviceName, baseURL string, service *ServiceStatus) {
	url := fmt.Sprintf("%s/payments/service-health", baseURL)
	resp, err := hm.client.Get(url)
	if err != nil {
		hm.setServiceStatus(service, false, 0)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		hm.setServiceStatus(service, false, 0)
		return
	}

	var health HealthCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		hm.setServiceStatus(service, false, 0)
		return
	}

	available := !health.Failing
	hm.setServiceStatus(service, available, health.MinResponseTime)
}
