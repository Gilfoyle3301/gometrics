package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type stubPinger struct {
	err error
}

func (s stubPinger) Ping(context.Context) error {
	return s.err
}

func TestPingHandler(t *testing.T) {
	tests := []struct {
		name       string
		pinger     stubPinger
		wantStatus int
		wantBody   string
	}{
		{
			name:       "database available",
			pinger:     stubPinger{},
			wantStatus: http.StatusOK,
			wantBody:   "OK",
		},
		{
			name:       "database unavailable",
			pinger:     stubPinger{err: errors.New("connection refused")},
			wantStatus: http.StatusInternalServerError,
			wantBody:   "database is unavailable\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewDBHandler(tt.pinger, nil)

			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			w := httptest.NewRecorder()

			assert.NotPanics(t, func() {
				h.Ping(w, req)
			})
			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantBody, w.Body.String())
		})
	}
}
