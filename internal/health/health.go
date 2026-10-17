// Package health implements liveness and readiness reporting.
//
// Liveness answers "is the process alive?" and never looks at dependencies.
// Readiness answers "should this instance receive traffic?". It depends on the
// lifecycle state (starting, ready, draining) and, once ready, on a set of
// dependency checks.
package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"butterfly.orx.me/core/internal/log"
)

// Default tuning, overridable with options.
const (
	// DefaultCheckTimeout bounds each individual check.
	DefaultCheckTimeout = 2 * time.Second
	// DefaultCacheTTL is how long a readiness result is reused, so a burst of
	// probes (several load balancers, kubelet, humans with curl) triggers one
	// round of dependency pings rather than one each.
	DefaultCacheTTL = time.Second
)

// State is the lifecycle state of the instance.
type State int32

const (
	// Starting: initialization has not finished. Not ready.
	Starting State = iota
	// Ready: serving. Readiness then depends on the dependency checks.
	Ready
	// Draining: shutdown has begun. Not ready, so load balancers stop routing
	// to this instance while in-flight requests finish.
	Draining
)

func (s State) String() string {
	switch s {
	case Starting:
		return "starting"
	case Ready:
		return "ready"
	case Draining:
		return "draining"
	default:
		return "unknown"
	}
}

// Status values used in reports.
const (
	StatusOK   = "ok"
	StatusFail = "fail"
)

// Check is a named dependency check. Check must return nil when the dependency
// is healthy and should honor ctx cancellation.
type Check struct {
	Name  string
	Check func(ctx context.Context) error
}

// CheckResult is the outcome of one check.
type CheckResult struct {
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
}

// Report is the readiness response body. Error details are deliberately not
// included: they can contain hostnames or credentials and are logged instead.
type Report struct {
	Status string                 `json:"status"`
	State  string                 `json:"state,omitempty"`
	Checks map[string]CheckResult `json:"checks,omitempty"`
}

// OK reports whether the instance is ready for traffic.
func (r Report) OK() bool { return r.Status == StatusOK }

// Option configures a Registry.
type Option func(*Registry)

// WithCheckTimeout sets the per-check timeout.
func WithCheckTimeout(d time.Duration) Option {
	return func(r *Registry) { r.timeout = d }
}

// WithCacheTTL sets how long a readiness result is cached. Zero disables caching.
func WithCacheTTL(d time.Duration) Option {
	return func(r *Registry) { r.ttl = d }
}

// Registry holds checks and lifecycle state. It is safe for concurrent use.
type Registry struct {
	timeout time.Duration
	ttl     time.Duration

	// state is atomic so SetState never waits behind a round of checks: the
	// flip to Draining must be immediate.
	state atomic.Int32

	mu     sync.Mutex // guards checks and names
	checks []Check
	names  map[string]struct{}

	// runMu serializes rounds of checks and guards the fields below.
	runMu      sync.Mutex
	cached     Report
	cachedAt   time.Time
	hasCache   bool
	lastStatus map[string]string // to log only status changes
}

// New returns a Registry in the Starting state.
func New(opts ...Option) *Registry {
	r := &Registry{
		timeout:    DefaultCheckTimeout,
		ttl:        DefaultCacheTTL,
		names:      make(map[string]struct{}),
		lastStatus: make(map[string]string),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Add registers checks. Names must be non-empty and unique, and Check non-nil.
// Nothing is registered if any check is invalid.
func (r *Registry) Add(checks ...Check) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	seen := make(map[string]struct{}, len(checks))
	for _, c := range checks {
		switch {
		case c.Name == "":
			return errors.New("health check name must not be empty")
		case c.Check == nil:
			return fmt.Errorf("health check %q has no check function", c.Name)
		}
		if _, dup := r.names[c.Name]; dup {
			return fmt.Errorf("duplicate health check %q", c.Name)
		}
		if _, dup := seen[c.Name]; dup {
			return fmt.Errorf("duplicate health check %q", c.Name)
		}
		seen[c.Name] = struct{}{}
	}
	for _, c := range checks {
		r.names[c.Name] = struct{}{}
		r.checks = append(r.checks, c)
	}
	return nil
}

// SetState moves the registry to s.
func (r *Registry) SetState(s State) { r.state.Store(int32(s)) }

// State returns the current lifecycle state.
func (r *Registry) State() State { return State(r.state.Load()) }

// Readiness evaluates readiness. The checks only run in the Ready state;
// Starting and Draining short-circuit to a failing report. Concurrent callers
// share one round of checks.
func (r *Registry) Readiness(ctx context.Context) Report {
	if st := r.State(); st != Ready {
		return Report{Status: StatusFail, State: st.String()}
	}

	r.runMu.Lock()
	defer r.runMu.Unlock()
	if r.hasCache && r.ttl > 0 && time.Since(r.cachedAt) < r.ttl {
		return r.cached
	}

	r.mu.Lock()
	checks := append([]Check(nil), r.checks...)
	r.mu.Unlock()

	// The result is shared with other callers, so one prober hanging up must
	// not fail the round for everyone.
	rep := r.runChecks(context.WithoutCancel(ctx), checks)
	rep.State = Ready.String()
	r.cached, r.cachedAt, r.hasCache = rep, time.Now(), true
	return rep
}

// runChecks runs all checks concurrently. Callers hold runMu.
func (r *Registry) runChecks(ctx context.Context, checks []Check) Report {
	type outcome struct {
		name string
		res  CheckResult
		err  error
	}
	out := make(chan outcome, len(checks))
	for _, c := range checks {
		go func() {
			start := time.Now()
			err := r.runOne(ctx, c)
			status := StatusOK
			if err != nil {
				status = StatusFail
			}
			out <- outcome{c.Name, CheckResult{Status: status, DurationMS: time.Since(start).Milliseconds()}, err}
		}()
	}

	rep := Report{Status: StatusOK, Checks: make(map[string]CheckResult, len(checks))}
	for range checks {
		o := <-out
		rep.Checks[o.name] = o.res
		if o.err != nil {
			rep.Status = StatusFail
		}
		r.logTransition(o.name, o.res.Status, o.err)
	}
	return rep
}

// logTransition logs when a check starts failing or recovers, rather than on
// every round, so a broken dependency does not flood the logs.
func (r *Registry) logTransition(name, status string, err error) {
	prev, seen := r.lastStatus[name]
	r.lastStatus[name] = status
	if prev == status || (!seen && status == StatusOK) {
		return
	}
	logger := log.CoreLogger("health")
	if err != nil {
		logger.Warn("health check failing", "check", name, "error", err.Error())
		return
	}
	logger.Info("health check recovered", "check", name)
}

// runOne runs a single check with a timeout. It returns as soon as the timeout
// fires even if the check ignores its context (that goroutine is then
// abandoned until the check returns). A panicking check counts as a failure.
func (r *Registry) runOne(ctx context.Context, c Check) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				done <- fmt.Errorf("panic: %v", p)
			}
		}()
		done <- c.Check(ctx)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("check timed out or was cancelled: %w", ctx.Err())
	}
}

// Names returns the registered check names, sorted.
func (r *Registry) Names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.checks))
	for _, c := range r.checks {
		names = append(names, c.Name)
	}
	sort.Strings(names)
	return names
}

// LiveHandler serves liveness: always 200 while the process can answer.
func (r *Registry) LiveHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, Report{Status: StatusOK})
	})
}

// ReadyHandler serves readiness: 200 when ready, 503 otherwise.
func (r *Registry) ReadyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rep := r.Readiness(req.Context())
		code := http.StatusOK
		if !rep.OK() {
			code = http.StatusServiceUnavailable
		}
		writeJSON(w, code, rep)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// Handler is the subset of *http.ServeMux that Mount needs.
type Handler interface {
	Handle(pattern string, handler http.Handler)
}

// Mount registers /healthz and /readyz on mux.
func (r *Registry) Mount(mux Handler) {
	mux.Handle("/healthz", r.LiveHandler())
	mux.Handle("/readyz", r.ReadyHandler())
}
