package health

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/diogomassis/rinha-2025/internal/services/processor"
)

type RinhaHealthCheckerMonitor struct {
	processors []processor.PaymentProcessor
	cache      map[string]processor.HealthStatus
	mutex      sync.RWMutex
	stopChan   chan struct{}
}

func NewRinhaHealthCheckerMonitor(processors ...processor.PaymentProcessor) *RinhaHealthCheckerMonitor {
	return &RinhaHealthCheckerMonitor{
		processors: processors,
		cache:      make(map[string]processor.HealthStatus),
		stopChan:   make(chan struct{}),
	}
}

func (m *RinhaHealthCheckerMonitor) GetStatus(processorName string) (processor.HealthStatus, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	status, found := m.cache[processorName]
	return status, found
}

func (m *RinhaHealthCheckerMonitor) Stop() {
	close(m.stopChan)
}

func (m *RinhaHealthCheckerMonitor) Start() {
	log.Info().Msg("Starting health monitor...")
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		m.checkAllProcessors()
		for {
			select {
			case <-ticker.C:
				m.checkAllProcessors()
			case <-m.stopChan:
				ticker.Stop()
				log.Info().Msg("Health monitor stopped.")
				return
			}
		}
	}()
}

func (m *RinhaHealthCheckerMonitor) checkAllProcessors() {
	for _, p := range m.processors {
		status, err := p.CheckHealth()
		if err != nil {
			m.updateStatus(p.GetName(), processor.HealthStatus{Failing: true})
			continue
		}
		m.updateStatus(p.GetName(), *status)
	}
}

func (m *RinhaHealthCheckerMonitor) updateStatus(name string, status processor.HealthStatus) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.cache[name] = status
}
