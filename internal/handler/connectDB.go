package handler

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

// DBPinger — минимальный интерфейс проверки доступности базы данных.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// DBHandler — HTTP-хендлер проверки соединения с базой данных.
type DBHandler struct {
	db     DBPinger
	logger *zap.SugaredLogger
}

// NewDBHandler создаёт хендлер /ping. logger может быть nil.
func NewDBHandler(db DBPinger, logger *zap.SugaredLogger) *DBHandler {
	return &DBHandler{db: db, logger: logger}
}

// Ping обрабатывает GET /ping: отвечает 200, если база доступна, иначе 500.
func (h *DBHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Ping(r.Context()); err != nil {
		if h.logger != nil {
			h.logger.Errorw("database is unavailable", "error", err)
		}
		http.Error(w, "database is unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
