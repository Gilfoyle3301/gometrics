package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Gilfoyle3301/gometrics/internal/config/server"
	"github.com/Gilfoyle3301/gometrics/internal/handler"
	"github.com/Gilfoyle3301/gometrics/internal/middleware"
	models "github.com/Gilfoyle3301/gometrics/internal/model"
	"github.com/caarlos0/env/v11"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

var (
	storeInterval   *time.Duration
	fileStoragePath *string
	restore         *bool
	address         *string
)

func init() {
	storeInterval = flag.Duration("i", time.Second*300, "time interval save to file")
	fileStoragePath = flag.String("s", "/opt/metrics_server/metrics", "path save metrics")
	restore = flag.Bool("r", false, "restore metrics from file")
	address = flag.String("a", "localhost:8080", "server address")

}

func main() {
	flag.Parse()

	cfg := server.NewConfig()

	if err := env.Parse(cfg); err != nil {
		slog.Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	addr := valueOr(cfg.Address, *address)
	saveInterval := valueOr(cfg.StoreInterval, *storeInterval)
	storagePath := valueOr(cfg.FileStoragePath, *fileStoragePath)
	needRestore := valueOr(cfg.Restore, *restore)

	if saveInterval < 0 {
		slog.Error("store interval must not be negative")
		os.Exit(1)
	}

	storageFile, err := os.OpenFile(storagePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		slog.Error("failed to open file", "error", err)
		os.Exit(1)
	}
	defer storageFile.Close()

	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	sg := logger.Sugar()
	r := mux.NewRouter()
	store := models.NewMemStorage()

	handle := handler.New(store)

	if needRestore {
		data, err := io.ReadAll(storageFile)
		if err != nil {
			slog.Error("failed to read file", "error", err)
		}
		if len(data) > 0 {
			if err := store.Restore(data); err != nil {
				slog.Error("failed to restore metrics", "error", err)
			}
		}
	}

	if saveInterval > 0 {
		saveTicker := time.NewTicker(saveInterval)
		defer saveTicker.Stop()

		go func() {
			for range saveTicker.C {
				if err := saveMetrics(storageFile, store); err != nil {
					sg.Error("failed to save metrics", "error", err)
				}
			}
		}()
	}

	r.Handle("/update/{type}/{name}/{value}", middleware.LoggerMiddlware(http.HandlerFunc(handle.UpdateMetrics), sg)).Methods("POST")
	r.Handle("/update", middleware.LoggerMiddlware(http.HandlerFunc(handle.UpdateMetric), sg)).Methods("POST")
	r.Handle("/value/{type}/{name}", middleware.LoggerMiddlware(http.HandlerFunc(handle.GetMetrics), sg)).Methods("GET")
	r.Handle("/value", middleware.LoggerMiddlware(http.HandlerFunc(handle.GetMetric), sg)).Methods("POST")
	r.Handle("/", middleware.LoggerMiddlware(http.HandlerFunc(handle.MainPage), sg)).Methods("GET")
	if err := http.ListenAndServe(addr, middleware.GunZipMiddleware(r)); err != nil {
		panic(err)
	}
}

func saveMetrics(file *os.File, store *models.MemStorage) error {
	data, err := json.Marshal(toMetrics(store.GetAllMetrics()))
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}

	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("truncate storage file: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek storage file: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write storage file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync storage file: %w", err)
	}

	return nil
}

func toMetrics(rows []models.MetricRow) []models.Metrics {
	result := make([]models.Metrics, 0, len(rows))
	for _, row := range rows {
		metric := models.Metrics{
			ID:    row.Name,
			MType: row.Type,
		}

		switch row.Type {
		case models.Gauge:
			value, ok := row.Value.(float64)
			if !ok {
				continue
			}
			metric.Value = &value
		case models.Counter:
			delta, ok := row.Value.(int64)
			if !ok {
				continue
			}
			metric.Delta = &delta
		default:
			continue
		}

		result = append(result, metric)
	}

	return result
}

func valueOr[T any](ptr *T, defaultValue T) T {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}
