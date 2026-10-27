package audit

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeObserver struct {
	events   []Event
	closeErr error
}

func (o *fakeObserver) Update(e Event) error {
	o.events = append(o.events, e)
	return nil
}

func (o *fakeObserver) Close() error { return o.closeErr }

func TestFileObserverAppendsJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")

	observer, err := NewFileObserver(path)
	require.NoError(t, err)

	require.NoError(t, observer.Update(Event{Timestamp: 1, Metrics: []string{"Alloc"}, IPAddress: "192.168.0.42"}))
	require.NoError(t, observer.Update(Event{Timestamp: 2, Metrics: []string{"Frees"}, IPAddress: "10.0.0.1"}))
	require.NoError(t, observer.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(string(data), "\n"))

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 2)

	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &raw))
	assert.Contains(t, raw, "ts")
	assert.Contains(t, raw, "metrics")
	assert.Contains(t, raw, "ip_address")

	var second Event
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &second))
	assert.Equal(t, int64(2), second.Timestamp)
	assert.Equal(t, []string{"Frees"}, second.Metrics)
	assert.Equal(t, "10.0.0.1", second.IPAddress)
}

func TestFileObserverAppendsToExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	require.NoError(t, os.WriteFile(path, []byte("existing\n"), 0o644))

	observer, err := NewFileObserver(path)
	require.NoError(t, err)
	require.NoError(t, observer.Update(Event{Timestamp: 1, Metrics: []string{"Alloc"}, IPAddress: "10.0.0.1"}))
	require.NoError(t, observer.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, "existing", lines[0])
}

func TestURLObserverPostsEvent(t *testing.T) {
	var gotMethod, gotContentType string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	observer := NewURLObserver(srv.URL)
	require.NoError(t, observer.Update(Event{Timestamp: 42, Metrics: []string{"Alloc", "Frees"}, IPAddress: "192.168.0.42"}))
	require.NoError(t, observer.Close())

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "application/json", gotContentType)

	var event Event
	require.NoError(t, json.Unmarshal(gotBody, &event))
	assert.Equal(t, int64(42), event.Timestamp)
	assert.Equal(t, []string{"Alloc", "Frees"}, event.Metrics)
	assert.Equal(t, "192.168.0.42", event.IPAddress)
}

func TestURLObserverErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	observer := NewURLObserver(srv.URL)
	assert.Error(t, observer.Update(Event{}))
	assert.NoError(t, observer.Close())
}

func TestNotifierDispatchesToAllObservers(t *testing.T) {
	first := &fakeObserver{}
	second := &fakeObserver{}

	notifier := NewNotifier(nil, first, second)
	notifier.Notify([]string{"Alloc", "Frees"}, "192.168.0.42")

	require.Len(t, first.events, 1)
	require.Len(t, second.events, 1)
	assert.Equal(t, first.events[0], second.events[0])
	assert.Equal(t, []string{"Alloc", "Frees"}, first.events[0].Metrics)
	assert.Equal(t, "192.168.0.42", first.events[0].IPAddress)
	assert.Positive(t, first.events[0].Timestamp)
}

func TestNotifierSubscribe(t *testing.T) {
	notifier := NewNotifier(nil)
	observer := &fakeObserver{}

	notifier.Subscribe(observer)
	notifier.Notify([]string{"Alloc"}, "10.0.0.1")

	require.Len(t, observer.events, 1)
	assert.Equal(t, []string{"Alloc"}, observer.events[0].Metrics)
}

func TestNilNotifier(t *testing.T) {
	var notifier *Notifier

	assert.NotPanics(t, func() {
		notifier.Notify([]string{"Alloc"}, "127.0.0.1")
	})
	assert.NoError(t, notifier.Close())
}

func TestNotifierCloseAggregatesErrors(t *testing.T) {
	notifier := NewNotifier(nil, &fakeObserver{}, &fakeObserver{closeErr: errors.New("close failed")})
	assert.Error(t, notifier.Close())
}
