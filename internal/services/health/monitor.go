package health

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/diogomassis/rinha-2025/internal/services/processor"
)

type RinhaMonitor struct {
	processors []processor.PaymentProcessor
	cache      map[string]processor.HealthStatus
	mutex      sync.RWMutex
	stopChan   chan struct{}
}

func NewMonitor(processors ...processor.PaymentProcessor) *RinhaMonitor {
	return &RinhaMonitor{
		processors: processors,
		cache:      make(map[string]processor.HealthStatus),
		stopChan:   make(chan struct{}),
	}
}

func (m *RinhaMonitor) GetStatus(processorName string) (processor.HealthStatus, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	status, found := m.cache[processorName]
	return status, found
}

func (m *RinhaMonitor) Start() {
	log.Println("[health] Starting health monitor...")
	ticker := time.NewTicker(6 * time.Second)
	go func() {
		m.checkAllProcessors()
		for {
			select {
			case <-ticker.C:
				m.checkAllProcessors()
			case <-m.stopChan:
				ticker.Stop()
				log.Println("[health] Health monitor stopped.")
				return
			}
		}
	}()
}

func (m *RinhaMonitor) checkAllProcessors() {
	log.Println("[health] Running periodic health checks...")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	for _, p := range m.processors {
		status, err := p.CheckHealth(ctx)
		if err != nil {
			log.Printf("[health] ERROR checking health for %s: %v. Marking as failing.", p.GetName(), err)
			m.updateStatus(p.GetName(), processor.HealthStatus{Failing: true})
			continue
		}
		m.updateStatus(p.GetName(), *status)
	}
}

func (m *RinhaMonitor) updateStatus(name string, status processor.HealthStatus) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.cache[name] = status
	log.Printf("[health] Updated status for %s: Failing=%t, MinResponseTime=%dms", name, status.Failing, status.MinResponseTime)
}
