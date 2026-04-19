package repository

// Store defines persistence dependencies used by the service layer.
type Store interface{}

// NoopStore is a placeholder repository implementation for scaffolding.
type NoopStore struct{}

func NewNoopStore() *NoopStore {
	return &NoopStore{}
}
