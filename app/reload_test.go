package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"butterfly.orx.me/core/internal/config"
)

type reloadCfg struct {
	Name  string `yaml:"name"`
	Limit int    `yaml:"limit"`
}

func (c *reloadCfg) Print() {}

func (c *reloadCfg) Validate() error {
	if c.Limit < 0 {
		return errors.New("limit must not be negative")
	}
	return nil
}

type plainCfg struct {
	Name string `yaml:"name"`
}

func (c *plainCfg) Print() {}

func TestParseAppConfigValidates(t *testing.T) {
	cfg := &reloadCfg{}
	if err := parseAppConfig([]byte("name: a\nlimit: 3\n"), cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "a" || cfg.Limit != 3 {
		t.Fatalf("cfg = %+v", cfg)
	}

	err := parseAppConfig([]byte("limit: -1\n"), &reloadCfg{})
	if err == nil || !strings.Contains(err.Error(), "limit must not be negative") {
		t.Fatalf("error = %v, want the Validate error", err)
	}
	if err := parseAppConfig([]byte("limit: [1\n"), &reloadCfg{}); err == nil {
		t.Fatal("malformed YAML should be an error")
	}
	// A type without Validate is accepted as is.
	if err := parseAppConfig([]byte("name: x\n"), &plainCfg{}); err != nil {
		t.Fatal(err)
	}
}

func TestNewConfigLike(t *testing.T) {
	orig := &reloadCfg{Name: "keep", Limit: 9}
	fresh, err := newConfigLike(orig)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := fresh.(*reloadCfg)
	if !ok || got == orig || *got != (reloadCfg{}) {
		t.Fatalf("fresh = %#v, want a distinct zero *reloadCfg", fresh)
	}
	if _, err := newConfigLike((*reloadCfg)(nil)); err == nil {
		t.Fatal("nil pointer should be an error")
	}
}

func newReloadApp(cfg config.AppConfig, onChange func(config.AppConfig)) *App {
	return New(&Config{Service: "svc", Config: cfg, OnConfigChange: onChange})
}

func TestReloadHandsOverFreshValidatedConfig(t *testing.T) {
	orig := &reloadCfg{Name: "orig", Limit: 1}
	var got []config.AppConfig
	a := newReloadApp(orig, func(c config.AppConfig) { got = append(got, c) })

	if err := a.reload([]byte("name: new\nlimit: 5\n")); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("callbacks = %d, want 1", len(got))
	}
	if n := got[0].(*reloadCfg); n.Name != "new" || n.Limit != 5 {
		t.Fatalf("new config = %+v", n)
	}
	if *orig != (reloadCfg{Name: "orig", Limit: 1}) {
		t.Fatalf("the original config must never be modified, got %+v", orig)
	}
}

func TestReloadRejectsBadDocuments(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{"app validation", "limit: -5\n", "limit must not be negative"},
		{"malformed yaml", "name: [\n", "unmarshal"},
		{"core validation", "store:\n  redis:\n    c:\n      mode: ring\n      addr: \"h:1\"\n", "store.redis.c"},
		{"core log", "log:\n  level: loud\n", "log: level"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			a := newReloadApp(&reloadCfg{}, func(config.AppConfig) { called = true })
			err := a.reload([]byte(tt.doc))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
			if called {
				t.Fatal("OnConfigChange must not run for a rejected document")
			}
		})
	}
}

func TestCheckReloadConfig(t *testing.T) {
	noop := func(config.AppConfig) {}
	if err := newReloadApp(nil, nil).checkReloadConfig(); err != nil {
		t.Fatalf("no callback needs nothing: %v", err)
	}
	if err := newReloadApp(nil, noop).checkReloadConfig(); err == nil {
		t.Fatal("OnConfigChange without Config should be rejected")
	}
	if err := newReloadApp(&reloadCfg{}, noop).checkReloadConfig(); err != nil {
		t.Fatal(err)
	}
	if err := newReloadApp((*reloadCfg)(nil), noop).checkReloadConfig(); err == nil {
		t.Fatal("a nil Config pointer should be rejected")
	}
}

// fakeBackend is a config backend that can push changes.
type fakeBackend struct {
	mu      sync.Mutex
	watcher func([]byte)
	ready   chan struct{}
	initial []byte
}

func (f *fakeBackend) Get(context.Context, string) ([]byte, error) { return nil, nil }

func (f *fakeBackend) Watch(ctx context.Context, _ string, initial []byte, onChange func([]byte)) error {
	f.mu.Lock()
	f.watcher, f.initial = onChange, initial
	f.mu.Unlock()
	close(f.ready)
	<-ctx.Done()
	return nil
}

func (f *fakeBackend) push(doc string) {
	f.mu.Lock()
	fn := f.watcher
	f.mu.Unlock()
	fn([]byte(doc))
}

// staticBackend cannot watch.
type staticBackend struct{}

func (staticBackend) Get(context.Context, string) ([]byte, error) { return nil, nil }

func TestStartConfigWatchDeliversReloads(t *testing.T) {
	got := make(chan *reloadCfg, 4)
	a := newReloadApp(&reloadCfg{}, func(c config.AppConfig) { got <- c.(*reloadCfg) })
	backend := &fakeBackend{ready: make(chan struct{})}

	w := a.startConfigWatch(context.Background(), backend, []byte("name: boot\n"))
	if w == nil {
		t.Fatal("expected a running watch")
	}
	<-backend.ready
	if string(backend.initial) != "name: boot\n" {
		t.Fatalf("baseline = %q", backend.initial)
	}

	backend.push("name: one\nlimit: 1\n")
	if c := <-got; c.Name != "one" {
		t.Fatalf("got %+v", c)
	}
	backend.push("limit: -1\n") // rejected, the watch carries on
	backend.push("name: two\n")
	select {
	case c := <-got:
		if c.Name != "two" {
			t.Fatalf("got %+v, want the next valid document", c)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}

	stopped := make(chan struct{})
	go func() { w.Stop(context.Background()); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return")
	}
	w.Stop(context.Background()) // idempotent
}

func TestStartConfigWatchSkipsWhenNothingToWatch(t *testing.T) {
	if w := newReloadApp(&reloadCfg{}, nil).startConfigWatch(context.Background(), &fakeBackend{}, nil); w != nil {
		t.Fatal("no OnConfigChange: no watch")
	}
	a := newReloadApp(&reloadCfg{}, func(config.AppConfig) {})
	if w := a.startConfigWatch(context.Background(), staticBackend{}, nil); w != nil {
		t.Fatal("a backend that cannot watch should be skipped, not fail")
	}
}

func TestWatchStopIsBoundedByContext(t *testing.T) {
	entered := make(chan struct{})
	block := make(chan struct{})
	defer close(block)
	a := newReloadApp(&reloadCfg{}, func(config.AppConfig) {
		close(entered)
		<-block // never returns on its own
	})
	backend := &fakeBackend{ready: make(chan struct{})}
	w := a.startConfigWatch(context.Background(), backend, nil)
	<-backend.ready
	go backend.push("name: x\n")
	<-entered // the callback is now stuck

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	w.Stop(ctx)
	if time.Since(start) > time.Second {
		t.Fatal("Stop should give up when its context ends")
	}
}
