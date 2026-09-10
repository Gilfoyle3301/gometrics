package models

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestFileStorage(t *testing.T, path string, restore bool) *FileStorage {
	t.Helper()

	fs, err := NewFileStorage(NewMemStorage(), path, restore)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fs.Close() })

	return fs
}

func TestFileStorageSaveAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")

	fs := newTestFileStorage(t, path, false)
	require.NoError(t, fs.Update(context.Background(), &Metrics{
		ID:    "Alloc",
		MType: Gauge,
		Value: float64Ptr(1.5),
	}))
	require.NoError(t, fs.Update(context.Background(), &Metrics{
		ID:    "PollCount",
		MType: Counter,
		Delta: int64Ptr(3),
	}))
	require.NoError(t, fs.Close())

	restored := newTestFileStorage(t, path, true)

	gauge, ok := restored.GetGauge("Alloc")
	require.True(t, ok)
	assert.Equal(t, 1.5, gauge)

	counter, ok := restored.GetCounter("PollCount")
	require.True(t, ok)
	assert.Equal(t, int64(3), counter)
}

func TestFileStorageRestoreOverwritesCounters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")

	source := newTestFileStorage(t, path, false)
	require.NoError(t, source.Update(context.Background(), &Metrics{
		ID:    "PollCount",
		MType: Counter,
		Delta: int64Ptr(3),
	}))
	require.NoError(t, source.Close())

	mem := NewMemStorage()
	mem.AddCounter("PollCount", 7)

	fs, err := NewFileStorage(mem, path, true)
	require.NoError(t, err)
	defer func() { _ = fs.Close() }()

	counter, ok := fs.GetCounter("PollCount")
	require.True(t, ok)
	assert.Equal(t, int64(3), counter, "restore must assign the stored value, not add to the current one")
}

func TestFileStorageCloseIsIdempotent(t *testing.T) {
	fs := newTestFileStorage(t, filepath.Join(t.TempDir(), "metrics.json"), false)
	fs.StartSync(time.Millisecond, func(error) {})

	require.NoError(t, fs.Close())
	require.NoError(t, fs.Close(), "second Close must not touch the closed file or panic")
}

func TestFileStorageStartSyncSavesPeriodically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")

	fs := newTestFileStorage(t, path, false)
	fs.StartSync(5*time.Millisecond, func(err error) {
		t.Errorf("periodic save failed: %v", err)
	})

	require.NoError(t, fs.Update(context.Background(), &Metrics{
		ID:    "Alloc",
		MType: Gauge,
		Value: float64Ptr(1.5),
	}))

	require.Eventually(t, func() bool {
		data, err := os.ReadFile(path)
		return err == nil && len(data) > 2
	}, time.Second, 2*time.Millisecond, "metrics must reach the file without an explicit Close")

	require.NoError(t, fs.Close())

	restored := newTestFileStorage(t, path, true)
	gauge, ok := restored.GetGauge("Alloc")
	require.True(t, ok)
	assert.Equal(t, 1.5, gauge)
}

// Регрессия на панику: раньше ошибки фонового сохранения логировались
// неинициализированным *zap.SugaredLogger, и горутина роняла весь процесс.
func TestFileStorageStartSyncReportsSaveErrors(t *testing.T) {
	fs := newTestFileStorage(t, filepath.Join(t.TempDir(), "metrics.json"), false)

	require.NoError(t, fs.file.Close())

	errCh := make(chan error, 1)
	fs.StartSync(time.Millisecond, func(err error) {
		select {
		case errCh <- err:
		default:
		}
	})

	select {
	case err := <-errCh:
		assert.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("save error was not reported to the callback")
	}

	assert.Error(t, fs.Close())
}

func TestNewFileStorageReturnsErrorOnUnusablePath(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocker, nil, 0644))

	fs, err := NewFileStorage(NewMemStorage(), filepath.Join(blocker, "metrics.json"), false)

	require.Error(t, err, "constructor must return an error instead of terminating the process")
	assert.Nil(t, fs)
}

func TestNewFileStorageRestoreRejectsBrokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0644))

	fs, err := NewFileStorage(NewMemStorage(), path, true)

	require.Error(t, err)
	assert.Nil(t, fs)

	_, statErr := os.Stat(path)
	assert.NoError(t, statErr, "storage file must survive a failed restore")
}

func TestFileStorageStartSyncIgnoresNonPositiveInterval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")

	fs := newTestFileStorage(t, path, false)
	fs.StartSync(0, func(err error) {
		t.Errorf("sync goroutine must not start for interval <= 0: %v", err)
	})

	time.Sleep(20 * time.Millisecond)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Empty(t, data, "nothing must be written before Close")

	require.NoError(t, fs.Close())

	data, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(data), "Close must flush the empty snapshot")
}

func int64Ptr(v int64) *int64 {
	p := new(int64)
	*p = v
	return p
}
