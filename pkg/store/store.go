package store

import (
	"github.com/Promptonauts/pipe/pkg/models"
)

type Store interface {
	Put(resource *models.GenericResource) error
	Get(kind models.ResourceKind, namespace, name string) (*models.GenericResource, error)
	List(kind models.ResourceKind, namespace string) ([]*models.GenericResource, error)
	Delete(kind models.ResourceKind, namespace, name string, status models.ResourceStatus) error

	CreateExecution(exec *models.ExecutionRecord) error
	GetExecution(id string) (*models.ExecutionRecord, error)
	UpdateExecution(Exec *models.ExecutionRecord) error
	ListExecutions(namespace string, limit int) ([]*models.ExecutionRecord, error)
	GetExecutionLogs(id string) ([]models.ExecutionLog, error)
	AppendExecutionLog(id string, log models.ExecutionLog) error
	SaveCheckpoint(executionID string, data []byte) error
	LoadCheckpoint(executionID string) ([]byte, error)

	Watch(kind models.ResourceKind) <-chan ResourceEvent

	Migrate() error
	Close() error
}

type EventType string

const (
	EventCreated EventType = "CREATED"
	EventUpdated EventType = "UPDATED"
	EventDeleted EventType = "DELETED"
)

type ResourceEvent struct {
	Type     EventType
	Resource *models.GenericResource
}
