package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Gilfoyle3301/gometrics/internal/audit"
	models "github.com/Gilfoyle3301/gometrics/internal/model"
)

type recordingObserver struct {
	events []audit.Event
}

func (o *recordingObserver) Update(e audit.Event) error {
	o.events = append(o.events, e)
	return nil
}

func (o *recordingObserver) Close() error { return nil }

func newAuditHandler(t *testing.T) (*Handler, *recordingObserver) {
	t.Helper()

	observer := &recordingObserver{}
	notifier := audit.NewNotifier(nil, observer)
	t.Cleanup(func() { require.NoError(t, notifier.Close()) })

	return New(models.NewMemStorage(), nil).WithAudit(notifier), observer
}

func TestUpdateMetricsAuditEvent(t *testing.T) {
	h, observer := newAuditHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/123.45", nil)
	req = mux.SetURLVars(req, map[string]string{"type": models.Gauge, "name": "Alloc", "value": "123.45"})
	w := httptest.NewRecorder()

	h.UpdateMetrics(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, observer.events, 1)
	assert.Equal(t, []string{"Alloc"}, observer.events[0].Metrics)
	assert.Equal(t, "192.0.2.1", observer.events[0].IPAddress)
	assert.Positive(t, observer.events[0].Timestamp)
}

func TestUpdateMetricAuditEvent(t *testing.T) {
	h, observer := newAuditHandler(t)

	body := `{"id":"Alloc","type":"gauge","value":123.45}`
	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateMetric(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, observer.events, 1)
	assert.Equal(t, []string{"Alloc"}, observer.events[0].Metrics)
	assert.Equal(t, "192.0.2.1", observer.events[0].IPAddress)
}

func TestUpdateMetricsBatchAuditEvent(t *testing.T) {
	h, observer := newAuditHandler(t)

	body := `[{"id":"Alloc","type":"gauge","value":1},{"id":"PollCount","type":"counter","delta":2}]`
	req := httptest.NewRequest(http.MethodPost, "/updates", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateMetricsBatch(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, observer.events, 1)
	assert.Equal(t, []string{"Alloc", "PollCount"}, observer.events[0].Metrics)
}

func TestNoAuditEventOnFailure(t *testing.T) {
	h, observer := newAuditHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.UpdateMetric(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, observer.events)
}

func TestNilAuditDoesNotPanic(t *testing.T) {
	h := New(models.NewMemStorage(), nil)

	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader(`{"id":"Alloc","type":"gauge","value":1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() { h.UpdateMetric(w, req) })
	require.Equal(t, http.StatusOK, w.Code)
}
