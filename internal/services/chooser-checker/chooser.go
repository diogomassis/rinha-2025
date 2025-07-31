package chooserchecker

import (
	"errors"

	healthchecker "github.com/diogomassis/rinha-2025/internal/services/health-checker"
)

var ErrNoServiceAvailable = errors.New("no payment service is available at the moment")

type ChooserService struct {
	monitor *healthchecker.HealthChecker
}

func New(monitor *healthchecker.HealthChecker) *ChooserService {
	return &ChooserService{
		monitor: monitor,
	}
}

func (sc *ChooserService) ChooseNextService() (string, error) {
	if sc.monitor.IsDefaultServiceAvailable() {
		return "d", nil
	}
	if sc.monitor.IsFallbackServiceAvailable() {
		return "f", nil
	}
	return "", ErrNoServiceAvailable
}
