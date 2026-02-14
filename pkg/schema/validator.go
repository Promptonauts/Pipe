package schema

import (
	"fmt"
	"strings"

	"github.com/Promptonauts/pipe/pkg/models"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("field '%s':%s", e.Field, e.Message)
}

type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

func ValidationResource(r *models.GenericResource) ValidationResult {
	var errs []ValidationError

	if r.APIVersion == "" {
		errs = append(errs, ValidationError{Field: "kind", Message: "required"})
	}

	if r.Kind == "" {
		errs = append(errs, ValidationError{Field: "kind", Message: "required"})
	} else {
		if _, err := models.ParseResourceKind(string(r.Kind)); err != nil {
			errs = append(errs, ValidationError{Field: "kind", Message: err.Error()})
		}
	}

	if r.Metadata.Name == "" {
		errs = append(errs, ValidationError{Field: "metadata.name", Message: "required"})
	}

	if r.Metadata.Namespace == "" {
		errs = append(errs, ValidationError{Field: "metadata.namespace", Message: "required"})
	}

	if r.Metadata.Version == "" {
		errs = append(errs, ValidationError{Field: "metadata.version", Message: "required"})
	}

	if !isValidName(r.Metadata.Name) {
		errs = append(errs, ValidationError{
			Field:   "metadata.name",
			Message: "must be lowercase alphanumeric with hyphens , max 63 chars",
		})
	}

	if !isValidName(r.Metadata.Namespace) {
		errs = append(errs, ValidationError{
			Field:   "metadata.namespace",
			Message: "must be lowercase alphanumeric with hyphens, max 63 chars",
		})
	}

	if r.Spec == nil {
		errs = append(errs, ValidationError{Field: "spec", Message: "required"})
	}

	switch r.Kind {
	case models.KindAgent:
		errs = append(errs, validateAgentSpec(r.Spec)...)
	case models.KindTool:
		errs = append(errs, validateToolSpec(r.Spec)...)
	case models.KindGuardrail:
		errs = append(errs, validateGuardrailSpec(r.Spec)...)
	case models.KindPipeline:
		errs = append(errs, validatePipelineSpec(r.Spec)...)
	case models.KindExecution:
		errs = append(errs, validateExecutionSpec(r.Spec)...)
	}

	return ValidationResult{
		Valid:  len(errs) == 0,
		Errors: errs,
	}

}

func isValidName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}

	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return !strings.HasPrefix(name, "-") && !strings.HasSuffix(name, "-")
}

func requireStringField(spec map[string]interface{}, field string) *ValidationError {
	v, ok := spec[field]
	if !ok {
		return &ValidationError{Field: "spec." + field, Message: "required"}
	}

	if s, ok := v.(string); !ok || s == "" {
		return &ValidationError{Field: "spec." + field, Message: "must be a non-empty string"}
	}

	return nil
}

func validateAgentSpec(spec map[string]interface{}) []ValidationError {
	var errs []ValidationError
	for _, f := range []string{"runtime"} {
		if e := requireStringField(spec, f); e != nil {
			errs = append(errs, *e)
		}
	}

	if _, ok := spec["model"]; !ok {
		errs = append(errs, ValidationError{Field: "spec.model", Message: "required"})
	}
	return errs
}

func validateToolSpec(spec map[string]interface{}) []ValidationError {
	var errs []ValidationError
	for _, f := range []string{"type"} {
		if e := requireStringField(spec, f); e != nil {
			errs = append(errs, *e)
		}
	}
	return errs
}

func validateGuardrailSpec(spec map[string]interface{}) []ValidationError {
	var errs []ValidationError
	for _, f := range []string{"type", "phase", "action"} {
		if e := requireStringField(spec, f); e != nil {
			errs = append(errs, *e)
		}
	}
	phase, _ := spec["phase"].(string)
	if phase != "" && phase != "pre" && phase != "post" && phase != "both" {
		errs = append(errs, ValidationError{Field: "spec.phase", Message: "must be 'pre' , 'post' , or ,both'"})
	}

	action, _ := spec["action"].(string)
	if action != "" && action != "block" && action != "warn" && action != "log" {
		errs = append(errs, ValidationError{Field: "spec.action", Message: "must be 'block', 'warn', or 'log'"})
	}
	return errs
}

func validatePipelineSpec(spec map[string]interface{}) []ValidationError {
	var errs []ValidationError
	if _, ok := spec["steps"]; !ok {
		errs = append(errs, ValidationError{Field: "spec.steps", Message: "required"})
	}
	return errs
}

func validateExecutionSpec(spec map[string]interface{}) []ValidationError {
	var errs []ValidationError
	if e := requireStringField(spec, "agentName"); e != nil {
		errs = append(errs, *e)
	}
	return errs
}
