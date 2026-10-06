// Package registry is the M19 model registry with promotion gates: a model
// reaches production only by moving dev -> stage -> prod in order, and only when
// it is signed and clears the eval gate. This keeps "who can push a model to
// prod" a deterministic control, not an honor system.
package registry

import (
	"errors"
	"fmt"
	"sync"
)

// Stage is a lifecycle stage; promotion must move up one step at a time.
type Stage int

const (
	Dev Stage = iota
	Staging
	Prod
)

func (s Stage) String() string {
	switch s {
	case Staging:
		return "staging"
	case Prod:
		return "prod"
	default:
		return "dev"
	}
}

// Model is a registered artifact.
type Model struct {
	Name    string
	Version string
	Stage   Stage
	Signed  bool
	EvalF1  float64
}

var (
	ErrUnknown    = errors.New("registry: no such model")
	ErrSkipStage  = errors.New("registry: promotion must advance one stage at a time")
	ErrUnsigned   = errors.New("registry: model must be signed to advance")
	ErrEvalGate   = errors.New("registry: model fails the eval gate")
	ErrAtProd     = errors.New("registry: already at prod")
)

// Registry holds models keyed by name@version.
type Registry struct {
	mu        sync.Mutex
	models    map[string]*Model
	minProdF1 float64
}

// New returns a registry whose prod eval gate requires EvalF1 >= minProdF1.
func New(minProdF1 float64) *Registry {
	return &Registry{models: map[string]*Model{}, minProdF1: minProdF1}
}

func key(name, version string) string { return name + "@" + version }

// Register adds a model (at Dev unless set otherwise).
func (r *Registry) Register(m Model) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models[key(m.Name, m.Version)] = &m
}

// Promote advances a model one stage. Reaching prod requires a signed model that
// clears the eval gate; stages cannot be skipped.
func (r *Registry) Promote(name, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.models[key(name, version)]
	if !ok {
		return ErrUnknown
	}
	switch m.Stage {
	case Prod:
		return ErrAtProd
	case Staging: // -> Prod: the gated transition
		if !m.Signed {
			return ErrUnsigned
		}
		if m.EvalF1 < r.minProdF1 {
			return fmt.Errorf("%w (F1 %.2f < %.2f)", ErrEvalGate, m.EvalF1, r.minProdF1)
		}
		m.Stage = Prod
	default: // Dev -> Staging
		if !m.Signed {
			return ErrUnsigned
		}
		m.Stage = Staging
	}
	return nil
}

// SetEval records a model's eval score (e.g. from the persisted inventory)
// before a gated promotion reads it.
func (r *Registry) SetEval(name, version string, f1 float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.models[key(name, version)]
	if !ok {
		return ErrUnknown
	}
	m.EvalF1 = f1
	return nil
}

// Stage reports a model's current stage.
func (r *Registry) Stage(name, version string) (Stage, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.models[key(name, version)]
	if !ok {
		return Dev, false
	}
	return m.Stage, true
}
