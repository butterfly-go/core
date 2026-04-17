package testsuite

import (
	"context"
	"log/slog"
	"sync"
)

// LogEntry is a captured slog record for assertions in tests.
type LogEntry struct {
	Level   slog.Level
	Message string
	Attrs   map[string]any
}

// MockLog captures slog output in memory for tests.
type MockLog struct {
	mu      sync.Mutex
	entries []LogEntry
	logger  *slog.Logger
}

// NewMockLog creates an in-memory slog logger and its capture helper.
func NewMockLog() (*slog.Logger, *MockLog) {
	m := &MockLog{}
	h := &mockLogHandler{mock: m}
	m.logger = slog.New(h)
	return m.logger, m
}

// SetAsDefault replaces slog.Default() with the mock logger and returns a restore func.
func (m *MockLog) SetAsDefault() func() {
	old := slog.Default()
	slog.SetDefault(m.logger)
	return func() {
		slog.SetDefault(old)
	}
}

// Entries returns a copy of all captured log entries.
func (m *MockLog) Entries() []LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	res := make([]LogEntry, 0, len(m.entries))
	for _, e := range m.entries {
		attrs := make(map[string]any, len(e.Attrs))
		for k, v := range e.Attrs {
			attrs[k] = v
		}
		res = append(res, LogEntry{
			Level:   e.Level,
			Message: e.Message,
			Attrs:   attrs,
		})
	}
	return res
}

// Messages returns the message text of all captured entries.
func (m *MockLog) Messages() []string {
	entries := m.Entries()
	messages := make([]string, 0, len(entries))
	for _, e := range entries {
		messages = append(messages, e.Message)
	}
	return messages
}

// ContainsMessage reports whether any captured entry has the exact message.
func (m *MockLog) ContainsMessage(message string) bool {
	entries := m.Entries()
	for _, e := range entries {
		if e.Message == message {
			return true
		}
	}
	return false
}

// CountLevel returns the number of entries at the given level.
func (m *MockLog) CountLevel(level slog.Level) int {
	entries := m.Entries()
	count := 0
	for _, e := range entries {
		if e.Level == level {
			count++
		}
	}
	return count
}

// Reset clears all captured entries.
func (m *MockLog) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = nil
}

func (m *MockLog) add(entry LogEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entry)
}

type mockLogHandler struct {
	mock   *MockLog
	attrs  []resolvedAttr
	groups []string
}

type resolvedAttr struct {
	key   string
	value any
}

func (h *mockLogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *mockLogHandler) Handle(_ context.Context, r slog.Record) error {
	entry := LogEntry{
		Level:   r.Level,
		Message: r.Message,
		Attrs:   make(map[string]any),
	}

	for _, attr := range h.attrs {
		entry.Attrs[attr.key] = attr.value
	}

	r.Attrs(func(attr slog.Attr) bool {
		h.addAttr(entry.Attrs, attr)
		return true
	})

	h.mock.add(entry)
	return nil
}

func (h *mockLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cp := &mockLogHandler{
		mock:   h.mock,
		groups: append([]string(nil), h.groups...),
		attrs:  append([]resolvedAttr(nil), h.attrs...),
	}
	for _, attr := range attrs {
		key := attr.Key
		if len(cp.groups) != 0 {
			key = joinGroup(cp.groups, key)
		}
		cp.attrs = append(cp.attrs, resolvedAttr{
			key:   key,
			value: valueToAny(attr.Value),
		})
	}
	return cp
}

func (h *mockLogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	cp := &mockLogHandler{
		mock:   h.mock,
		attrs:  append([]resolvedAttr(nil), h.attrs...),
		groups: append([]string(nil), h.groups...),
	}
	cp.groups = append(cp.groups, name)
	return cp
}

func (h *mockLogHandler) addAttr(dst map[string]any, attr slog.Attr) {
	key := attr.Key
	if len(h.groups) != 0 {
		key = joinGroup(h.groups, key)
	}
	dst[key] = valueToAny(attr.Value)
}

func joinGroup(groups []string, key string) string {
	totalLen := len(key)
	for _, g := range groups {
		totalLen += len(g) + 1
	}
	buf := make([]byte, 0, totalLen)
	for i, g := range groups {
		if i > 0 {
			buf = append(buf, '.')
		}
		buf = append(buf, g...)
	}
	if len(buf) > 0 {
		buf = append(buf, '.')
	}
	buf = append(buf, key...)
	return string(buf)
}

func valueToAny(v slog.Value) any {
	v = v.Resolve()
	switch v.Kind() {
	case slog.KindAny:
		return v.Any()
	case slog.KindBool:
		return v.Bool()
	case slog.KindDuration:
		return v.Duration()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindString:
		return v.String()
	case slog.KindTime:
		return v.Time()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindGroup:
		attrs := make(map[string]any)
		for _, attr := range v.Group() {
			attrs[attr.Key] = valueToAny(attr.Value)
		}
		return attrs
	default:
		return v.Any()
	}
}
