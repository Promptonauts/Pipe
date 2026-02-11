package models

type PipelineSpec struct {
	Description string         `yaml:"description" json:"description"`
	Steps       []PipelineStep `yaml:"steps" json:"steps"`
	OnFailure   string         `yaml:"onFailure" json:"onFailure"`
	MaxRetries  int            `yaml:"maxRetries" json:"maxRetries"`
}

type PipelineStep struct {
	Name       string            `yaml:"name" json:"name"`
	Agent      string            `yaml:"agent" json:"agent"`
	Input      map[string]string `yaml:"input,omitempty" json:"input,omitemtpy"`
	DependsOn  []string          `yaml:"dependsOn,omiempty" json:"dependsOn,omiempty"`
	Timeout    string            `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	RetryCount int               `yaml:"retryCount,omitempty" json:"retryCount,omitempty"`
}
