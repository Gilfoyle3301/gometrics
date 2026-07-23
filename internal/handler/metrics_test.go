package handler

import (
	"bytes"
	"compress/gzip"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	models "github.com/Gilfoyle3301/gometrics/internal/model"
)

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
				val, ok := s.GetGauge("Alloc")
				assert.True(t, ok)
				assert.Equal(t, 123.45, val)
			},
		},
		{
			name:        "valid counter",
			vars:        map[string]string{"type": models.Counter, "name": "PollCount", "value": "10"},
			contentType: "text/plain",
			wantStatus:  http.StatusOK,
			check: func(t *testing.T, s models.Storage) {
				val, ok := s.GetCounter("PollCount")
				assert.True(t, ok)
				assert.Equal(t, int64(10), val)
			},
		},
		{
			name:        "counter accumulates value",
			vars:        map[string]string{"type": models.Counter, "name": "PollCount", "value": "5"},
			contentType: "text/plain",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "invalid content type",
			vars:        map[string]string{"type": models.Gauge, "name": "Alloc", "value": "1"},
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
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
			h := New(strg)
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
				val, ok := s.GetGauge("Alloc")
				assert.True(t, ok)
				assert.Equal(t, 123.45, val)
			},
		},
		{
			name:        "valid counter",
			body:        `{"id":"PollCount","type":"counter","delta":10}`,
			contentType: "application/json",
			wantStatus:  http.StatusOK,
			check: func(t *testing.T, s models.Storage) {
				val, ok := s.GetCounter("PollCount")
				assert.True(t, ok)
				assert.Equal(t, int64(10), val)
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
			h := New(models.NewMemStorage())
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

	h := New(models.NewMemStorage())
	h.storage.SetGauge("Alloc", 42.5)
	req := httptest.NewRequest(http.MethodPost, "/value", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()

	h.GetMetric(w, req)

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
	h := New(models.NewMemStorage())
	h.storage.SetGauge("Alloc", 42.5)
	req := httptest.NewRequest(http.MethodPost, "/value", bytes.NewBufferString(`{"id":"Alloc","type":"gauge"}`))
	req.Header.Set("Content-Type", "application/json")
	w := &failWriter{header: make(http.Header)}

	assert.NotPanics(t, func() {
		h.GetMetric(w, req)
	})
}

func TestGetMetrics(t *testing.T) {
	h := New(models.NewMemStorage())
	h.storage.SetGauge("Alloc", 42.5)
	h.storage.AddCounter("PollCount", 7)

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
			name:       "missing gauge - current (buggy) behaviour",
			vars:       map[string]string{"type": models.Gauge, "name": "DoesNotExist"},
			wantStatus: http.StatusNotFound,
			wantBody:   "Gauge not found\n",
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
		h := New(models.NewMemStorage())
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		h.MainPage(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	})

	t.Run("renders metrics sorted by name", func(t *testing.T) {
		h := New(models.NewMemStorage())
		h.storage.SetGauge("Zeta", 1.1)
		h.storage.SetGauge("Alpha", 2.2)
		h.storage.AddCounter("PollCount", 3)

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
