package models

import "time"

type ExecutionState string

const (
	ExecPending   ExecutionState = "Pending"
	ExecRunning   ExecutionState = "Running"
	ExecPaused    ExecutionState = "Paused"
	ExecFailed    ExecutionState = "Failed"
	ExecCompleted ExecutionState = "Completed"
	ExecRetrying  ExecutionState = "Retrying"
)

type ExecutionSpec struct {
	AgentName    string            `yaml:"agentName" json:"agentName"`
	PipelineName string            `yaml:"pipelineName,omitempty" json:"pipelineName,omitempty"`
	Input        map[string]string `yaml:"input" json:"input"`
	Priority     int               `yaml:"priority" json:"priority"`
	Namespace    string            `yaml:"namespace" json:"namespace"`
}

type ExecutionRecord struct {
	ID           string                 `json:"id"`
	AgentName    string                 `json:"agentName"`
	PipelineName string                 `json:"pipelineName,omitempty"`
	Namespace    string                 `json:"namespace"`
	State        ExecutionState         `json:"state"`
	Input        map[string]string      `json:"input"`
	Output       map[string]interface{} `json:"output,omitempty"`
	CurrentStep  int                    `json:"currentStep"`
	TotalSteps   int                    `json:"totalSteps"`
	Checkpoint   []byte                 `json:"checkpoint,omitempty"`
	RetryCount   int                    `json:"retryCount"`
	MaxRetries   int                    `json:"maxRetries"`
	Priority     int                    `json:"priority"`
	Error        string                 `json:"error,omitempty"`
	Logs         []ExecutionLog         `json:"logs,omitempty"`
	TokensUsed   int64                  `json:"tokensUsed"`
	LatencyMs    int64                  `json:"latencyMs"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
	StartedAt    *time.Time             `json:"startedAt,omitempty"`
	CompletedAt  *time.Time             `json:"completedAt,omitempty"`
}

type ExecutionLog struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Step      int       `json:"step"`
}
