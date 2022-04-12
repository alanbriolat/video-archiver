package video_archiver

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/hashicorp/go-multierror"

	"github.com/alanbriolat/video-archiver/generic"
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

type MatchFunc = func(string) (Source, error)

// A Provider matches any URL it knows how to handle, giving a Source that can be used to download the video.
type Provider struct {
	Name  string
	Match MatchFunc
	// Priority of the matcher, lower (including negative) means matching earlier.
	Priority int16
}

func (p Provider) WithName(name string) Provider {
	p.Name = name
	return p
}

func (p Provider) WithPriority(priority int16) Provider {
	p.Priority = priority
	return p
}

// A Match is the result of a Provider successfully matching a URL.
type Match struct {
	ProviderName string
	Source       Source
}

// A ProviderRegistry is a collection of Provider instances which can be used to try to match URLs.
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
func (r *ProviderRegistry) Create(name string, f MatchFunc) error {
	return r.Add(Provider{
		Name:  name,
		Match: f,
	})
}

// CreatePriority is a shortcut for Add(Provider{Name: ..., Match: ..., Priority: ...}).
func (r *ProviderRegistry) CreatePriority(name string, f MatchFunc, priority int16) error {
	return r.Add(Provider{
		Name:     name,
		Match:    f,
		Priority: priority,
	})
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

// List returns the names of registered providers in priority order.
func (r *ProviderRegistry) List() []string {
	names := make([]string, 0, len(r.providers))
	for _, p := range r.providers {
		names = append(names, p.Name)
	}
	return names
}

// Match a string against each Provider in priority order, or return ErrNoMatch.
func (r *ProviderRegistry) Match(s string) (*Match, error) {
	var result error
	for _, p := range r.providers {
		if source, err := p.Match(s); source != nil && err == nil {
			match := &Match{
				ProviderName: p.Name,
				Source:       source,
			}
			return match, nil
		} else {
			result = multierror.Append(result, multierror.Prefix(err, fmt.Sprintf("[%v]", p.Name)))
		}
	}
	return nil, result
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

// MustAdd wraps Add but panics if there is an error.
func (r *ProviderRegistry) MustAdd(p Provider) {
	generic.Unwrap_(r.Add(p))
}

// MustCreate wraps Create but panics if there is an error.
func (r *ProviderRegistry) MustCreate(name string, f MatchFunc) {
	generic.Unwrap_(r.Create(name, f))
}

// MustCreatePriority wraps CreatePriority but panics if there is an error.
func (r *ProviderRegistry) MustCreatePriority(name string, f MatchFunc, priority int16) {
	generic.Unwrap_(r.CreatePriority(name, f, priority))
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

func (r *ProviderRegistry) sortByPriority() {
	sort.Slice(r.providers, func(i, j int) bool {
		return r.providers[i].Priority < r.providers[j].Priority
	})
}

var DefaultProviderRegistry ProviderRegistry
