package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	strictSignature = flag.Bool("strict-signature", false, "reject requests without a valid signature")
)

// shutdownTimeout — сколько ждём завершения активных запросов после сигнала.
const shutdownTimeout = 5 * time.Second

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
	requireSignature := shared.ValueOr(cfg.SignatureRequired, *strictSignature)

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
		fileStore, err := models.NewFileStorage(models.NewMemStorage(), storagePath, needRestore)
		if err != nil {
			sg.Errorw("failed to init file storage", "error", err)
			os.Exit(1)
		}

		fileStore.StartSync(saveInterval, func(err error) {
			sg.Errorw("failed to save metrics", "error", err)
		})

		storage = fileStore

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

	var root http.Handler = r
	root = middlware.RequestHashMiddlware(root, key, requireSignature)
	root = middlware.GunZipMiddlware(root)
	root = middlware.ResponseHashMiddlware(root, key)

	srv := &http.Server{Addr: addr, Handler: root}

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.ListenAndServe()
	}()

	sg.Infow("server started", "address", addr)

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			sg.Errorw("server failed", "error", err)
			os.Exit(1)
		}
	case <-signalCtx.Done():
		sg.Infow("shutdown signal received")
	}

	// возвращаем сигналам штатную обработку: повторный SIGTERM завершает процесс сразу
	stopSignals()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		sg.Errorw("graceful shutdown failed", "error", err)
	}

	// финальный сброс: для файлового хранилища это последние метрики за интервал
	if err := storage.Close(); err != nil {
		sg.Errorw("failed to close storage", "error", err)
		os.Exit(1)
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
