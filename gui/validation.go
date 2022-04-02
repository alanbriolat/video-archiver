package gui

type ValidationResult struct {
	errors map[string][]string
}

func (r *ValidationResult) IsOk() bool {
	return len(r.errors) == 0
}

func (r *ValidationResult) AddError(key, msg string) {
	if r.errors == nil {
		r.errors = make(map[string][]string)
	}
	errors, _ := r.errors[key]
	r.errors[key] = append(errors, msg)
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
