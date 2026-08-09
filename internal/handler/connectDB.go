package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type DBHandler struct {
	dbpool *pgxpool.Pool
	logger *zap.SugaredLogger
}

func NewDBHandler(dbpool *pgxpool.Pool) *DBHandler {
	return &DBHandler{dbpool: dbpool}
}

func (h *DBHandler) Ping(w http.ResponseWriter, r *http.Request) {
	// Может стоит перейти на timeout, но пока оставим так
	if err := h.dbpool.Ping(r.Context()); err != nil {
		h.logger.Error("database is unavailable", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "database is unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))

}
