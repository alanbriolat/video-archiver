package gui

import (
	"fmt"
	"net/url"

	"github.com/alanbriolat/video-archiver/generic"
)

type ValidationResult struct {
	errors map[string][]string
}

func (r *ValidationResult) IsOk() bool {
	return len(r.errors) == 0
}

func (r *ValidationResult) AddError(key, format string, args ...interface{}) {
	if r.errors == nil {
		r.errors = make(map[string][]string)
	}
	errors, _ := r.errors[key]
	r.errors[key] = append(errors, fmt.Sprintf(format, args...))
}

func (r *ValidationResult) HasErrors(key string) bool {
	errors, _ := r.errors[key]
	return len(errors) > 0
}

func (r *ValidationResult) GetErrors(key string) []string {
	errors, _ := r.errors[key]
	return errors
}

func (r *ValidationResult) GetAllErrors() []string {
	var errors []string
	for _, e := range r.errors {
		errors = append(errors, e...)
	}
	return errors
}

var validURLSchemes = generic.NewSet("http", "https")

func ValidateURL(s string) error {
	if parsed, err := url.Parse(s); err != nil {
		return err
	} else if !validURLSchemes.Contains(parsed.Scheme) {
		return fmt.Errorf("unrecognized scheme: %v", parsed.Scheme)
	} else if parsed.Hostname() == "" {
		return fmt.Errorf("missing host")
	} else {
		return nil
	}
}
