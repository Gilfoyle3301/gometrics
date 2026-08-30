package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Gilfoyle3301/gometrics/internal/middlware"
	models "github.com/Gilfoyle3301/gometrics/internal/model"
)

func float64Ptr(v float64) *float64 { p := new(float64); *p = v; return p }
func int64Ptr(v int64) *int64       { p := new(int64); *p = v; return p }

func setGauge(t *testing.T, s models.Storage, name string, val float64) {
	t.Helper()
	err := s.Update(context.Background(), &models.Metrics{
		ID:    name,
		MType: models.Gauge,
		Value: float64Ptr(val),
	})
	require.NoError(t, err)
}

func addCounter(t *testing.T, s models.Storage, name string, val int64) {
	t.Helper()
	err := s.Update(context.Background(), &models.Metrics{
		ID:    name,
		MType: models.Counter,
		Delta: int64Ptr(val),
	})
	require.NoError(t, err)
}

func newRequest(method, target string, vars map[string]string, contentType string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return mux.SetURLVars(req, vars)
}

func TestUpdateMetrics(t *testing.T) {
	strg := models.NewMemStorage()
	tests := []struct {
		name        string
		vars        map[string]string
		contentType string
		wantStatus  int
		check       func(t *testing.T, s models.Storage)
	}{
		{
			name:        "valid gauge",
			vars:        map[string]string{"type": models.Gauge, "name": "Alloc", "value": "123.45"},
			contentType: "text/plain",
			wantStatus:  http.StatusOK,
			check: func(t *testing.T, s models.Storage) {
				val, err := s.Get(context.Background(), "Alloc", models.Gauge)
				require.NoError(t, err)
				require.NotNil(t, val)
				require.NotNil(t, val.Value)
				assert.Equal(t, 123.45, *val.Value)
			},
		},
		{
			name:        "valid counter",
			vars:        map[string]string{"type": models.Counter, "name": "PollCount", "value": "10"},
			contentType: "text/plain",
			wantStatus:  http.StatusOK,
			check: func(t *testing.T, s models.Storage) {
				val, err := s.Get(context.Background(), "PollCount", models.Counter)
				require.NoError(t, err)
				require.NotNil(t, val)
				require.NotNil(t, val.Delta)
				assert.Equal(t, int64(10), *val.Delta)
			},
		},
		{
			name:        "counter accumulates value",
			vars:        map[string]string{"type": models.Counter, "name": "PollCount", "value": "5"},
			contentType: "text/plain",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "any content type accepted for param update",
			vars:        map[string]string{"type": models.Gauge, "name": "Alloc", "value": "1"},
			contentType: "application/json",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "unknown metric type",
			vars:        map[string]string{"type": "unknown", "name": "Alloc", "value": "1"},
			contentType: "text/plain",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "invalid gauge value",
			vars:        map[string]string{"type": models.Gauge, "name": "Alloc", "value": "not-a-number"},
			contentType: "text/plain",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "invalid counter value",
			vars:        map[string]string{"type": models.Counter, "name": "PollCount", "value": "not-a-number"},
			contentType: "text/plain",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "missing name var -> less than expectedParts",
			vars:        map[string]string{"type": models.Gauge, "value": "1"},
			contentType: "text/plain",
			wantStatus:  http.StatusNotFound,
		},
		{
			name:        "empty name value",
			vars:        map[string]string{"type": models.Gauge, "name": "", "value": "1"},
			contentType: "text/plain",
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(strg, nil)
			req := newRequest(http.MethodPost, "/update", tt.vars, tt.contentType)
			w := httptest.NewRecorder()

			h.UpdateMetrics(w, req)

			require.Equal(t, tt.wantStatus, w.Code)
			if tt.check != nil {
				tt.check(t, h.storage)
			}
		})
	}
}

func TestUpdateMetric(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		wantStatus  int
		check       func(t *testing.T, s models.Storage)
	}{
		{
			name:        "valid gauge",
			body:        `{"id":"Alloc","type":"gauge","value":123.45}`,
			contentType: "application/json",
			wantStatus:  http.StatusOK,
			check: func(t *testing.T, s models.Storage) {
				val, err := s.Get(context.Background(), "Alloc", models.Gauge)
				require.NoError(t, err)
				require.NotNil(t, val)
				require.NotNil(t, val.Value)
				assert.Equal(t, 123.45, *val.Value)
			},
		},
		{
			name:        "valid counter",
			body:        `{"id":"PollCount","type":"counter","delta":10}`,
			contentType: "application/json",
			wantStatus:  http.StatusOK,
			check: func(t *testing.T, s models.Storage) {
				val, err := s.Get(context.Background(), "PollCount", models.Counter)
				require.NoError(t, err)
				require.NotNil(t, val)
				require.NotNil(t, val.Delta)
				assert.Equal(t, int64(10), *val.Delta)
			},
		},
		{
			name:        "missing gauge value",
			body:        `{"id":"Alloc","type":"gauge"}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "missing counter delta",
			body:        `{"id":"PollCount","type":"counter"}`,
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "invalid content type",
			body:        `{"id":"Alloc","type":"gauge","value":123.45}`,
			contentType: "text/plain",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(models.NewMemStorage(), nil)
			req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()

			h.UpdateMetric(w, req)

			require.Equal(t, tt.wantStatus, w.Code)
			if tt.check != nil {
				tt.check(t, h.storage)
			}
		})
	}
}

func TestGetMetricGzip(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write([]byte(`{"id":"Alloc","type":"gauge"}`))
	require.NoError(t, err)
	require.NoError(t, gz.Close())

	h := New(models.NewMemStorage(), nil)
	setGauge(t, h.storage, "Alloc", 42.5)

	req := httptest.NewRequest(http.MethodPost, "/value", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()

	middlware.Decompress(http.HandlerFunc(h.GetMetric)).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"type":"gauge"`)
	assert.Contains(t, w.Body.String(), `"value":42.5`)
}

type failWriter struct {
	header http.Header
}

func (f *failWriter) Header() http.Header {
	return f.header
}

func (f *failWriter) WriteHeader(status int) {}

func (f *failWriter) Write(b []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestGetMetricEncodeErrorWithNilLoggerDoesNotPanic(t *testing.T) {
	h := New(models.NewMemStorage(), nil)
	setGauge(t, h.storage, "Alloc", 42.5)

	req := httptest.NewRequest(http.MethodPost, "/value", bytes.NewBufferString(`{"id":"Alloc","type":"gauge"}`))
	req.Header.Set("Content-Type", "application/json")
	w := &failWriter{header: make(http.Header)}

	assert.NotPanics(t, func() {
		h.GetMetric(w, req)
	})
}

func TestGetMetrics(t *testing.T) {
	h := New(models.NewMemStorage(), nil)
	setGauge(t, h.storage, "Alloc", 42.5)
	addCounter(t, h.storage, "PollCount", 7)

	tests := []struct {
		name       string
		vars       map[string]string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "existing gauge",
			vars:       map[string]string{"type": models.Gauge, "name": "Alloc"},
			wantStatus: http.StatusOK,
			wantBody:   "42.5",
		},
		{
			name:       "existing counter",
			vars:       map[string]string{"type": models.Counter, "name": "PollCount"},
			wantStatus: http.StatusOK,
			wantBody:   "7",
		},
		{
			name:       "unknown type",
			vars:       map[string]string{"type": "unknown", "name": "Alloc"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing gauge",
			vars:       map[string]string{"type": models.Gauge, "name": "DoesNotExist"},
			wantStatus: http.StatusNotFound,
			wantBody:   "Metric not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newRequest(http.MethodGet, "/value", tt.vars, "")
			w := httptest.NewRecorder()

			h.GetMetrics(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantBody != "" {
				assert.Equal(t, tt.wantBody, w.Body.String())
			}
		})
	}
}

func TestMainPage(t *testing.T) {
	t.Run("renders empty state", func(t *testing.T) {
		h := New(models.NewMemStorage(), nil)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		h.MainPage(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	})

	t.Run("renders metrics sorted by name", func(t *testing.T) {
		h := New(models.NewMemStorage(), nil)
		setGauge(t, h.storage, "Zeta", 1.1)
		setGauge(t, h.storage, "Alpha", 2.2)
		addCounter(t, h.storage, "PollCount", 3)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		h.MainPage(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()

		assert.Contains(t, body, "Alpha")
		assert.Contains(t, body, "Zeta")
		assert.Contains(t, body, "PollCount")

		assert.Less(t, strings.Index(body, "Alpha"), strings.Index(body, "Zeta"))
	})
}

func TestUpdateMetricsBatch(t *testing.T) {
	t.Run("applies batch in order", func(t *testing.T) {
		strg := models.NewMemStorage()
		h := New(strg, nil)
		addCounter(t, strg, "Counter", 100)

		body := `[
			{"id":"Counter","type":"counter","delta":1},
			{"id":"Gauge","type":"gauge","value":1.5},
			{"id":"Counter","type":"counter","delta":2},
			{"id":"Gauge","type":"gauge","value":2.5}
		]`

		req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.UpdateMetricsBatch(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		counter, err := strg.Get(context.Background(), "Counter", models.Counter)
		require.NoError(t, err)
		require.NotNil(t, counter.Delta)
		assert.Equal(t, int64(103), *counter.Delta, "counter must accumulate both deltas")

		gauge, err := strg.Get(context.Background(), "Gauge", models.Gauge)
		require.NoError(t, err)
		require.NotNil(t, gauge.Value)
		assert.Equal(t, 2.5, *gauge.Value, "gauge must keep the last value from the batch")
	})

	t.Run("empty batch is a no-op", func(t *testing.T) {
		h := New(nil, nil)

		req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader("[]"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			h.UpdateMetricsBatch(w, req)
		})
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid metric rejects the whole batch", func(t *testing.T) {
		strg := models.NewMemStorage()
		h := New(strg, nil)

		body := `[
			{"id":"Valid","type":"counter","delta":1},
			{"id":"Broken","type":"gauge"}
		]`

		req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.UpdateMetricsBatch(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		_, err := strg.Get(context.Background(), "Valid", models.Counter)
		assert.Error(t, err, "valid metric from a rejected batch must not be stored")
	})

	t.Run("invalid content type", func(t *testing.T) {
		h := New(models.NewMemStorage(), nil)

		req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader("[]"))
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()

		h.UpdateMetricsBatch(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		h := New(models.NewMemStorage(), nil)

		req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader("{not-json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.UpdateMetricsBatch(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("accepts gzip request body", func(t *testing.T) {
		strg := models.NewMemStorage()
		h := New(strg, nil)

		raw := `[{"id":"Gauge","type":"gauge","value":4.2}]`
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err := gz.Write([]byte(raw))
		require.NoError(t, err)
		require.NoError(t, gz.Close())

		req := httptest.NewRequest(http.MethodPost, "/updates/", &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		w := httptest.NewRecorder()

		middlware.Decompress(http.HandlerFunc(h.UpdateMetricsBatch)).ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		gauge, err := strg.Get(context.Background(), "Gauge", models.Gauge)
		require.NoError(t, err)
		require.NotNil(t, gauge.Value)
		assert.Equal(t, 4.2, *gauge.Value)
	})
}
