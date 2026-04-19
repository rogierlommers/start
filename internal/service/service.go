package service

import "start/internal/repository"

// Service contains application use-cases.
type Service struct {
	store repository.Store
}

func New(store repository.Store) *Service {
	return &Service{store: store}
}

func (s *Service) HealthStatus() map[string]string {
	return map[string]string{"status": "ok"}
}

func (s *Service) ServiceStatus() map[string]string {
	return map[string]string{"service": "start", "status": "running"}
}
