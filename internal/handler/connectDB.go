package handler

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

type DBPinger interface {
	Ping(ctx context.Context) error
}

type DBHandler struct {
	db     DBPinger
	logger *zap.SugaredLogger
}

func NewDBHandler(db DBPinger, logger *zap.SugaredLogger) *DBHandler {
	return &DBHandler{db: db, logger: logger}
}

func (h *DBHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Ping(r.Context()); err != nil {
		if h.logger != nil {
			h.logger.Error("database is unavailable", zap.Error(err))
		}
		http.Error(w, "database is unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
