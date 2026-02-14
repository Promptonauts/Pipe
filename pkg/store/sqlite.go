package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Promptonauts/pipe/pkg/models"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStore struct {
	db       *sql.DB
	mu       sync.RWMutex
	watchers map[models.ResourceKind][]chan ResourceEvent
	watchMu  sync.RWMutex
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &SQLiteStore{
		db:       db,
		watchers: make(map[models.ResourceKind][]chan ResourceEvent),
	}, nil
}

func (s *SQLiteStore) Migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS resources (
		kind TEXT NOT NULL,
		namespace TEXT NOT NULL,
		name TEXT NOT NULL,
		uid TEXT NOT NULL,
		data TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (kind, namespace, name)
	);

	CREATE TABLE IF NOT EXISTS executions (
		id TEXT PRIMARY KEY,
		namespace TEXT NOT NULL,
		agent_name TEXT NOT NULL,
		pipeline_name TEXT DEFAULT '',
		state TEXT NOT NULL,
		data TEXT NOT NULL,
		checkpoint BLOB,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS execution_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		execution_id TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		level TEXT NOT NULL,
		message TEXT NOT NULL,
		step INTEGER DEFAULT 0,
		FOREIGN KEY (execution_id) REFERENCES executions(id)
	);

	CREATE INDEX IF NOT EXISTS idx_executions_namespace ON executions(namespace);
	CREATE INDEX IF NOT EXISTS idx_executions_state ON executions(state);
	CREATE INDEX IF NOT EXISTS idx_execution_logs_exec_id ON execution_logs(execution_id);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) Put(resource *models.GenericResource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	isNew := false

	if resource.Metadata.UID == "" {
		resource.Metadata.UID = uuid.New().String()
		resource.Metadata.CreatedAt = now
		isNew = true
	}
	resource.Metadata.UpdatedAt = now

	if resource.Status.State == "" {
		resource.Status.State = "Registered"
		resource.Status.Health = "Unknown"
	}
	resource.Status.LastUpdated = now

	data, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("marshal resource: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO resources (kind, namespace, name, uid, data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(kind, namespace, name) DO UPDATE SET
			data = excluded.data,
			updated_at = excluded.updated_at
	`, string(resource.Kind), resource.Metadata.Namespace, resource.Metadata.Name,
		resource.Metadata.UID, string(data), now, now)
	if err != nil {
		return fmt.Errorf("upsert resource: %w", err)
	}

	evtType := EventUpdated
	if isNew {
		evtType = EventCreated
	}
	s.emit(resource.Kind, ResourceEvent{Type: evtType, Resource: resource})
	return nil
}

func (s *SQLiteStore) Get(kind models.ResourceKind, namespace, name string) (*models.GenericResource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data string
	err := s.db.QueryRow(
		"SELECT data FROM resources WHERE kind = ? AND namespace = ? AND name = ?",
		string(kind), namespace, name,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("resource %s/%s/%s not found", kind, namespace, name)
	}
	if err != nil {
		return nil, fmt.Errorf("query resource: %w", err)
	}

	var res models.GenericResource
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		return nil, fmt.Errorf("unmarshal resource: %w", err)
	}
	return &res, nil
}

func (s *SQLiteStore) List(kind models.ResourceKind, namespace string) ([]*models.GenericResource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := "SELECT data FROM resources WHERE kind = ?"
	args := []interface{}{string(kind)}
	if namespace != "" {
		query += " AND namespace = ?"
		args = append(args, namespace)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	defer rows.Close()

	var results []*models.GenericResource
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var res models.GenericResource
		if err := json.Unmarshal([]byte(data), &res); err != nil {
			return nil, err
		}
		results = append(results, &res)
	}
	return results, nil
}

func (s *SQLiteStore) Delete(kind models.ResourceKind, namespace, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.getUnlocked(kind, namespace, name)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		"DELETE FROM resources WHERE kind = ? AND namespace = ? AND name = ?",
		string(kind), namespace, name,
	)
	if err != nil {
		return fmt.Errorf("delete resource: %w", err)
	}

	s.emit(kind, ResourceEvent{Type: EventDeleted, Resource: res})
	return nil
}

func (s *SQLiteStore) UpdateStatus(kind models.ResourceKind, namespace, name string, status models.ResourceStatus) error {
	res, err := s.Get(kind, namespace, name)
	if err != nil {
		return err
	}
	res.Status = status
	res.Status.LastUpdated = time.Now().UTC()
	return s.Put(res)
}

func (s *SQLiteStore) getUnlocked(kind models.ResourceKind, namespace, name string) (*models.GenericResource, error) {
	var data string
	err := s.db.QueryRow(
		"SELECT data FROM resources WHERE kind = ? AND namespace = ? AND name = ?",
		string(kind), namespace, name,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("not found")
	}
	if err != nil {
		return nil, err
	}
	var res models.GenericResource
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *SQLiteStore) CreateExecution(exec *models.ExecutionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if exec.ID == "" {
		exec.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	exec.CreatedAt = now
	exec.UpdatedAt = now

	data, err := json.Marshal(exec)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO executions (id, namespace, agent_name, pipeline_name, state, data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, exec.ID, exec.Namespace, exec.AgentName, exec.PipelineName, string(exec.State), string(data), now, now)
	return err
}

func (s *SQLiteStore) GetExecution(id string) (*models.ExecutionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data string
	err := s.db.QueryRow("SELECT data FROM executions WHERE id = ?", id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("execution %s not found", id)
	}
	if err != nil {
		return nil, err
	}

	var exec models.ExecutionRecord
	if err := json.Unmarshal([]byte(data), &exec); err != nil {
		return nil, err
	}
	return &exec, nil
}

func (s *SQLiteStore) UpdateExecution(exec *models.ExecutionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	exec.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(exec)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		UPDATE executions SET state = ?, data = ?, updated_at = ? WHERE id = ?
	`, string(exec.State), string(data), exec.UpdatedAt, exec.ID)
	return err
}

func (s *SQLiteStore) ListExecutions(namespace string, limit int) ([]*models.ExecutionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := "SELECT data FROM executions"
	args := []interface{}{}
	if namespace != "" {
		query += " WHERE namespace = ?"
		args = append(args, namespace)
	}
	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.ExecutionRecord
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var exec models.ExecutionRecord
		if err := json.Unmarshal([]byte(data), &exec); err != nil {
			return nil, err
		}
		results = append(results, &exec)
	}
	return results, nil
}

func (s *SQLiteStore) AppendExecutionLog(id string, logEntry models.ExecutionLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO execution_logs (execution_id, timestamp, level, message, step)
		VALUES (?, ?, ?, ?, ?)
	`, id, logEntry.Timestamp, logEntry.Level, logEntry.Message, logEntry.Step)
	return err
}

func (s *SQLiteStore) GetExecutionLogs(id string) ([]models.ExecutionLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		"SELECT timestamp, level, message, step FROM execution_logs WHERE execution_id = ? ORDER BY timestamp ASC",
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.ExecutionLog
	for rows.Next() {
		var l models.ExecutionLog
		if err := rows.Scan(&l.Timestamp, &l.Level, &l.Message, &l.Step); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (s *SQLiteStore) SaveCheckpoint(executionID string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("UPDATE executions SET checkpoint = ?, updated_at = ? WHERE id = ?",
		data, time.Now().UTC(), executionID)
	return err
}

func (s *SQLiteStore) LoadCheckpoint(executionID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data []byte
	err := s.db.QueryRow("SELECT checkpoint FROM executions WHERE id = ?", executionID).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return data, err
}

// Watch support

func (s *SQLiteStore) Watch(kind models.ResourceKind) <-chan ResourceEvent {
	s.watchMu.Lock()
	defer s.watchMu.Unlock()

	ch := make(chan ResourceEvent, 100)
	s.watchers[kind] = append(s.watchers[kind], ch)
	return ch
}

func (s *SQLiteStore) emit(kind models.ResourceKind, event ResourceEvent) {
	s.watchMu.RLock()
	defer s.watchMu.RUnlock()

	for _, ch := range s.watchers[kind] {
		select {
		case ch <- event:
		default:
			// Drop event if channel is full — non-blocking
		}
	}
}
