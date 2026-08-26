package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Gilfoyle3301/gometrics/internal/config/server"
	"github.com/Gilfoyle3301/gometrics/internal/handler"
	"github.com/Gilfoyle3301/gometrics/internal/middlware"
	models "github.com/Gilfoyle3301/gometrics/internal/model"
	"github.com/Gilfoyle3301/gometrics/internal/shared"
	"github.com/Gilfoyle3301/gometrics/migrations"
	"github.com/caarlos0/env/v11"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

var (
	storeInterval   = flag.Int("i", 300, "time interval save to file in seconds")
	fileStoragePath = flag.String("s", "", "path save metrics")
	restore         = flag.Bool("r", false, "restore metrics from file")
	address         = flag.String("a", "localhost:8080", "server address")
	dataBaseDSN     = flag.String("d", "", "database DSN")
	keyFlag         = flag.String("k", "", "secret key for data signing")
)

func main() {
	flag.Parse()

	cfg := server.NewConfig()

	if err := env.Parse(cfg); err != nil {
		slog.Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	addr := shared.ValueOr(cfg.Address, *address)
	saveIntervalSec := shared.ValueOr(cfg.StoreInterval, *storeInterval)
	storagePath := shared.ValueOr(cfg.FileStoragePath, *fileStoragePath)
	needRestore := shared.ValueOr(cfg.Restore, *restore)
	dataBaseDSN := shared.ValueOr(cfg.DatabaseDSN, *dataBaseDSN)
	key := shared.ValueOr(cfg.Key, *keyFlag)

	ctx := context.Background()

	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	sg := logger.Sugar()

	if saveIntervalSec < 0 {
		slog.Error("store interval must not be negative")
		os.Exit(1)
	}

	saveInterval := time.Duration(saveIntervalSec) * time.Second

	r := mux.NewRouter()
	var storage models.Storage
	switch {
	case dataBaseDSN != "":
		dbpool, err := pgxpool.New(ctx, dataBaseDSN)
		if err != nil {
			sg.Error("failed to connect to database", zap.Error(err))
			return
		}
		defer dbpool.Close()

		if err := applyMigrations(dataBaseDSN); err != nil {
			sg.Error("failed to apply migrations", zap.Error(err))
			return
		}

		storage = models.NewDBStorage(dbpool)

		dbh := handler.NewDBHandler(dbpool, sg)
		r.Handle("/ping", middlware.LoggerMiddlware(http.HandlerFunc(dbh.Ping), sg)).Methods("GET")

	case storagePath != "":
		if err := os.MkdirAll(filepath.Dir(storagePath), 0755); err != nil {
			slog.Error("failed to create storage directory", "error", err)
			os.Exit(1)
		}

		storageFile, err := os.OpenFile(storagePath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			slog.Error("failed to open file", "error", err)
			os.Exit(1)
		}
		defer storageFile.Close()

		memStore := models.NewMemStorage()

		if needRestore {
			data, err := io.ReadAll(storageFile)
			if err != nil {
				slog.Error("failed to read file", "error", err)
			} else if len(data) > 0 {
				if ms, ok := memStore.(*models.MemStorage); ok {
					if err := ms.Restore(data); err != nil {
						slog.Error("failed to restore metrics", "error", err)
					}
				}
			}
		}

		storage = memStore

		if saveInterval > 0 {
			saveTicker := time.NewTicker(saveInterval)
			defer saveTicker.Stop()

			go func() {
				for range saveTicker.C {
					if err := saveMetrics(ctx, storageFile, memStore); err != nil {
						sg.Error("failed to save metrics", "error", err)
					}
				}
			}()
		}

	default:
		storage = models.NewMemStorage()
	}

	handle := handler.New(storage, sg)

	updateParam := middlware.LoggerMiddlware(middlware.Decompress(http.HandlerFunc(handle.UpdateMetrics)), sg)
	updateJSON := middlware.LoggerMiddlware(middlware.Decompress(http.HandlerFunc(handle.UpdateMetric)), sg)
	updateBatch := middlware.LoggerMiddlware(middlware.Decompress(http.HandlerFunc(handle.UpdateMetricsBatch)), sg)
	getParam := middlware.LoggerMiddlware(middlware.Decompress(http.HandlerFunc(handle.GetMetrics)), sg)
	getJSON := middlware.LoggerMiddlware(middlware.Decompress(http.HandlerFunc(handle.GetMetric)), sg)

	r.Handle("/update/{type}/{name}/{value}", updateParam).Methods("POST")
	r.Handle("/update", updateJSON).Methods("POST")
	r.Handle("/update/", updateJSON).Methods("POST")
	r.Handle("/updates/", updateBatch).Methods("POST")
	r.Handle("/updates", updateBatch).Methods("POST")
	r.Handle("/value/{type}/{name}", getParam).Methods("GET")
	r.Handle("/value", getJSON).Methods("POST")
	r.Handle("/value/", getJSON).Methods("POST")

	r.Handle("/", middlware.LoggerMiddlware(http.HandlerFunc(handle.MainPage), sg)).Methods("GET")

	// Порядок обёрток: проверка подписи запроса видит исходные байты тела
	// до декомпрессии, а подпись ответа считается по финальным байтам
	// (уже сжатым, если клиент принимает gzip).
	var root http.Handler = r
	root = middlware.RequestHashMiddlware(root, key)
	root = middlware.GunZipMiddlware(root)
	root = middlware.ResponseHashMiddlware(root, key)

	if err := http.ListenAndServe(addr, root); err != nil {
		panic(err)
	}
}

func applyMigrations(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database for migrations: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	return nil
}

// func toMetrics(rows []models.Metrics) []models.Metrics {
// 	result := make([]models.Metrics, 0, len(rows))
// 	for _, row := range rows {
// 		metric := models.Metrics{
// 			ID:    row.Name,
// 			MType: row.Type,
// 		}

// 		switch row.Type {
// 		case models.Gauge:
// 			value := row.Value
// 			if value == nil {
// 				continue
// 			}
// 			metric.Value = value
// 		case models.Counter:
// 			delta := row.Delta
// 			if delta == nil {
// 				continue
// 			}
// 			metric.Delta = delta
// 		default:
// 			continue
// 		}

// 		result = append(result, metric)
// 	}

// 	return result
// }

func saveMetrics(ctx context.Context, file *os.File, store models.Storage) error {
	if store == nil {
		return fmt.Errorf("store is nil")
	}
	mstore, err := store.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("get all metrics: %w", err)
	}
	data, err := json.Marshal(mstore)
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
