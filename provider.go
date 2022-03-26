package video_archiver

import (
	"errors"
	"math"
	"sort"
)

var (
	ErrDuplicateProvider = errors.New("duplicate provider name")
	ErrInvalidProvider   = errors.New("invalid provider")
	// TODO: ErrNoMatch should wrap an inner error, so we can show why a match failed
	ErrNoMatch         = errors.New("no provider matched the input")
	ErrUnknownProvider = errors.New("unknown provider")
)

var (
	PriorityHighest int16 = math.MinInt16
	PriorityDefault int16 = 0
	PriorityLowest  int16 = math.MaxInt16
)

type Provider struct {
	Name  string
	Match func(string) (Source, error)
	// Priority of the matcher, lower (including negative) means matching earlier.
	Priority int16
}

type Match struct {
	ProviderName string
	Source       Source
}

type ProviderRegistry struct {
	providers   []*Provider
	providerMap map[string]*Provider
}

// Add registers a Provider with the ProviderRegistry. Provider.Name and Provider.Match must be set, and
// Provider.Name must be unique within the ProviderRegistry.
func (r *ProviderRegistry) Add(p Provider) error {
	if r.providerMap == nil {
		r.providerMap = make(map[string]*Provider)
	}
	if p.Name == "" || p.Match == nil {
		return ErrInvalidProvider
	}
	if _, ok := r.providerMap[p.Name]; ok {
		return ErrDuplicateProvider
	}
	r.providerMap[p.Name] = &p
	r.providers = append(r.providers, r.providerMap[p.Name])
	r.sortByPriority()
	return nil
}

// Create is a shortcut for Add(Provider{Name: ..., Match: ...}).
func (r *ProviderRegistry) Create(name string, f func(string) (Source, error)) error {
	return r.Add(Provider{
		Name:  name,
		Match: f,
	})
}

// CreatePriority is a shortcut for Add(Provider{Name: ..., Match: ..., Priority: ...}).
func (r *ProviderRegistry) CreatePriority(name string, f func(string) (Source, error), priority int16) error {
	return r.Add(Provider{
		Name:     name,
		Match:    f,
		Priority: priority,
	})
}

// SetPriority adjust the priority of a named Provider.
func (r *ProviderRegistry) SetPriority(name string, priority int16) error {
	if p, ok := r.providerMap[name]; ok {
		p.Priority = priority
		r.sortByPriority()
		return nil
	} else {
		return ErrUnknownProvider
	}
}

// List returns the names of registered providers in priority order.
func (r *ProviderRegistry) List() []string {
	names := make([]string, 0, len(r.providers))
	for _, p := range r.providers {
		names = append(names, p.Name)
	}
	return names
}

// GetPriority gets the priority of the named Provider. If ErrUnknownProvider is returned, the returned priority is the
// default priority.
func (r *ProviderRegistry) GetPriority(name string) (int16, error) {
	if p, ok := r.providerMap[name]; ok {
		return p.Priority, nil
	} else {
		return 0, ErrUnknownProvider
	}
}

// Match a string against each Provider in priority order, or return ErrNoMatch.
func (r *ProviderRegistry) Match(s string) (*Match, error) {
	for _, p := range r.providers {
		if source, err := p.Match(s); source != nil && err == nil {
			match := &Match{
				ProviderName: p.Name,
				Source:       source,
			}
			return match, nil
		}
	}
	return nil, ErrNoMatch
}

// MatchWith will attempt to match a string against a specific provider.
func (r *ProviderRegistry) MatchWith(name string, s string) (*Match, error) {
	if p, ok := r.providerMap[name]; ok {
		if source, err := p.Match(s); source != nil && err == nil {
			match := &Match{
				ProviderName: p.Name,
				Source:       source,
			}
			return match, nil
		} else {
			return nil, ErrNoMatch
		}
	} else {
		return nil, ErrUnknownProvider
	}
}

func (r *ProviderRegistry) sortByPriority() {
	sort.Slice(r.providers, func(i, j int) bool {
		return r.providers[i].Priority < r.providers[j].Priority
	})
}
