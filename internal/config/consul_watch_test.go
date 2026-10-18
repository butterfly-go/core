package config

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	capi "github.com/hashicorp/consul/api"
)

// fakeKV is a one-key Consul KV endpoint that implements blocking queries:
// a request carrying the current ?index= waits for a change.
type fakeKV struct {
	mu      sync.Mutex
	value   []byte
	exists  bool
	index   uint64
	changed chan struct{}
	failing int // number of upcoming requests that answer 500
	token   string
}

func newFakeKV(value string) *fakeKV {
	return &fakeKV{value: []byte(value), exists: true, index: 10, changed: make(chan struct{})}
}

func (f *fakeKV) bump() {
	f.index++
	close(f.changed)
	f.changed = make(chan struct{})
}

func (f *fakeKV) set(value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.value, f.exists = []byte(value), true
	f.bump()
}

func (f *fakeKV) touch() { // index moves, value does not
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bump()
}

func (f *fakeKV) del() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.value, f.exists = nil, false
	f.bump()
}

// reset simulates a Consul snapshot restore: the index goes backwards.
func (f *fakeKV) reset(index uint64, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.value, f.exists = []byte(value), true
	f.index = index
	close(f.changed)
	f.changed = make(chan struct{})
}

func (f *fakeKV) failNext(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failing = n
}

func (f *fakeKV) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.token = r.Header.Get("X-Consul-Token")
	if f.failing > 0 {
		f.failing--
		f.mu.Unlock()
		http.Error(w, "boom", http.StatusInternalServerError)
		return
	}
	if idx, _ := strconv.ParseUint(r.URL.Query().Get("index"), 10, 64); idx > 0 && idx == f.index {
		ch := f.changed
		f.mu.Unlock()
		select {
		case <-ch:
		case <-time.After(150 * time.Millisecond): // the blocking query's wait time
		case <-r.Context().Done():
			return
		}
		f.mu.Lock()
	}
	index, value, exists := f.index, f.value, f.exists
	f.mu.Unlock()

	w.Header().Set("X-Consul-Index", strconv.FormatUint(index, 10))
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode([]map[string]any{{
		"Key": "svc", "Value": base64.StdEncoding.EncodeToString(value), "ModifyIndex": index,
	}})
}

func newWatchConsul(t *testing.T, kv *fakeKV) *ConsulConfig {
	t.Helper()
	srv := httptest.NewServer(kv)
	t.Cleanup(srv.Close)
	client, err := capi.NewClient(&capi.Config{Address: srv.Listener.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	return &ConsulConfig{
		client:     client,
		kv:         client.KV(),
		watchWait:  time.Second,
		watchGap:   5 * time.Millisecond,
		minBackoff: 5 * time.Millisecond,
		maxBackoff: 20 * time.Millisecond,
	}
}

// startWatch runs Watch in the background and returns the channel of values
// it reports. The watch is stopped and awaited when the test ends.
func startWatch(t *testing.T, c *ConsulConfig, initial string, onChange func([]byte)) (<-chan string, func()) {
	t.Helper()
	got := make(chan string, 16)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- c.Watch(ctx, "svc", []byte(initial), func(v []byte) {
			if onChange != nil {
				onChange(v)
			}
			got <- string(v)
		})
	}()
	stop := func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Watch returned %v on cancel", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("Watch did not return after its context was cancelled")
		}
	}
	t.Cleanup(cancel)
	return got, stop
}

func expect(t *testing.T, got <-chan string, want string) {
	t.Helper()
	select {
	case v := <-got:
		if v != want {
			t.Fatalf("got value %q, want %q", v, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for value %q", want)
	}
}

func expectNothing(t *testing.T, got <-chan string) {
	t.Helper()
	select {
	case v := <-got:
		t.Fatalf("unexpected callback with %q", v)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestWatchReportsEachChange(t *testing.T) {
	kv := newFakeKV("v: 1")
	got, stop := startWatch(t, newWatchConsul(t, kv), "v: 1", nil)
	defer stop()

	expectNothing(t, got) // the baseline is not a change
	kv.set("v: 2")
	expect(t, got, "v: 2")
	kv.set("v: 3")
	expect(t, got, "v: 3")
}

func TestWatchIgnoresIndexBumpsWithoutValueChange(t *testing.T) {
	kv := newFakeKV("v: 1")
	got, stop := startWatch(t, newWatchConsul(t, kv), "v: 1", nil)
	defer stop()

	kv.touch()
	kv.set("v: 1")
	expectNothing(t, got)
	kv.set("v: 2")
	expect(t, got, "v: 2")
}

func TestWatchReportsChangeThatLandedBeforeItStarted(t *testing.T) {
	kv := newFakeKV("v: new")
	got, stop := startWatch(t, newWatchConsul(t, kv), "v: old", nil)
	defer stop()
	expect(t, got, "v: new")
}

func TestWatchKeepsCurrentConfigWhenKeyIsDeleted(t *testing.T) {
	kv := newFakeKV("v: 1")
	got, stop := startWatch(t, newWatchConsul(t, kv), "v: 1", nil)
	defer stop()

	kv.del()
	expectNothing(t, got)
	kv.set("v: 1") // restored with the same value: still nothing to report
	expectNothing(t, got)
	kv.set("v: 2")
	expect(t, got, "v: 2")
}

func TestWatchRetriesAfterErrors(t *testing.T) {
	kv := newFakeKV("v: 1")
	kv.failNext(3)
	got, stop := startWatch(t, newWatchConsul(t, kv), "v: 1", nil)
	defer stop()

	time.Sleep(100 * time.Millisecond) // let it burn through the failures
	kv.set("v: 2")
	expect(t, got, "v: 2")
}

func TestWatchHandlesIndexGoingBackwards(t *testing.T) {
	kv := newFakeKV("v: 1")
	got, stop := startWatch(t, newWatchConsul(t, kv), "v: 1", nil)
	defer stop()

	kv.set("v: 2")
	expect(t, got, "v: 2")
	kv.reset(3, "v: restored")
	expect(t, got, "v: restored")
}

func TestWatchSurvivesPanickingCallback(t *testing.T) {
	kv := newFakeKV("v: 1")
	first := true
	got, stop := startWatch(t, newWatchConsul(t, kv), "v: 1", func([]byte) {
		if first {
			first = false
			panic("callback bug")
		}
	})
	defer stop()

	kv.set("v: 2")
	// The panicking call never reaches the channel send, the next one does.
	time.Sleep(100 * time.Millisecond)
	kv.set("v: 3")
	expect(t, got, "v: 3")
}

func TestWatchCallbackOwnsItsSlice(t *testing.T) {
	kv := newFakeKV("v: 1")
	c := newWatchConsul(t, kv)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vals := make(chan []byte, 2)
	go func() {
		_ = c.Watch(ctx, "svc", []byte("v: 1"), func(v []byte) { vals <- v })
	}()
	kv.set("v: 2")
	v := <-vals
	for i := range v {
		v[i] = 'x' // scribbling over it must not corrupt the watcher's baseline
	}
	kv.touch()
	select {
	case v := <-vals:
		t.Fatalf("a mutated slice caused a spurious change: %q", v)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestWatchReturnsNilOnCancelWhileBackingOff(t *testing.T) {
	kv := newFakeKV("v: 1")
	kv.failNext(1 << 30)
	c := newWatchConsul(t, kv)
	c.minBackoff, c.maxBackoff = time.Hour, time.Hour
	_, stop := startWatch(t, c, "v: 1", nil)
	time.Sleep(100 * time.Millisecond)
	stop()
}

func TestWatchFuncUnsupportedBackend(t *testing.T) {
	old := config
	t.Cleanup(func() { config = old })
	config = &FileConfig{path: "unused"}
	if err := Watch(context.Background(), "k", func([]byte) {}); err != ErrWatchUnsupported {
		t.Fatalf("Watch on the file backend = %v, want ErrWatchUnsupported", err)
	}
}

func TestWatchFuncReadsBaselineThenWatches(t *testing.T) {
	kv := newFakeKV("v: 1")
	old := config
	t.Cleanup(func() { config = old })
	config = newWatchConsul(t, kv)

	got := make(chan string, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = Watch(ctx, "svc", func(v []byte) { got <- string(v) }) }()

	expectNothing(t, got)
	kv.set("v: 2")
	expect(t, got, "v: 2")
}
