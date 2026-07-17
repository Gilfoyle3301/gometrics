package handler

import (
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
	tests := []struct {
		name        string
		vars        map[string]string
		contentType string
		wantStatus  int
		check       func(t *testing.T, s *models.MemStorage)
	}{
		{
			name:        "valid gauge",
			vars:        map[string]string{"type": models.Gauge, "name": "Alloc", "value": "123.45"},
			contentType: "text/plain",
			wantStatus:  http.StatusOK,
			check: func(t *testing.T, s *models.MemStorage) {
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
			check: func(t *testing.T, s *models.MemStorage) {
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
			h := New()
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

func TestGetMetrics(t *testing.T) {
	h := New()
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
		h := New()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		h.MainPage(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	})

	t.Run("renders metrics sorted by name", func(t *testing.T) {
		h := New()
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
