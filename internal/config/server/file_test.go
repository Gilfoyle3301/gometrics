package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFileConfigAllFields(t *testing.T) {
	data := `{
		"address": "localhost:9090",
		"restore": true,
		"store_interval": "1s",
		"store_file": "/tmp/metrics.db",
		"database_dsn": "postgres://localhost/db",
		"key": "secret",
		"crypto_key": "/tmp/private.pem",
		"signature_required": true,
		"audit_file": "/tmp/audit.log",
		"audit_url": "http://localhost:9999/audit"
	}`
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))

	cfg, err := LoadFileConfig(path)
	require.NoError(t, err)

	require.NotNil(t, cfg.Address)
	assert.Equal(t, "localhost:9090", *cfg.Address)
	require.NotNil(t, cfg.Restore)
	assert.True(t, *cfg.Restore)
	require.NotNil(t, cfg.StoreInterval)
	assert.Equal(t, "1s", *cfg.StoreInterval)
	require.NotNil(t, cfg.StoreFile)
	assert.Equal(t, "/tmp/metrics.db", *cfg.StoreFile)
	require.NotNil(t, cfg.DatabaseDSN)
	assert.Equal(t, "postgres://localhost/db", *cfg.DatabaseDSN)
	require.NotNil(t, cfg.Key)
	assert.Equal(t, "secret", *cfg.Key)
	require.NotNil(t, cfg.CryptoKey)
	assert.Equal(t, "/tmp/private.pem", *cfg.CryptoKey)
	require.NotNil(t, cfg.SignatureRequired)
	assert.True(t, *cfg.SignatureRequired)
	require.NotNil(t, cfg.AuditFile)
	assert.Equal(t, "/tmp/audit.log", *cfg.AuditFile)
	require.NotNil(t, cfg.AuditURL)
	assert.Equal(t, "http://localhost:9999/audit", *cfg.AuditURL)
}

func TestLoadFileConfigEmptyPath(t *testing.T) {
	cfg, err := LoadFileConfig("")
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Nil(t, cfg.Address)
	assert.Nil(t, cfg.Restore)
	assert.Nil(t, cfg.StoreInterval)
}

func TestLoadFileConfigPartial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"address":"localhost:1"}`), 0o644))

	cfg, err := LoadFileConfig(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Address)
	assert.Equal(t, "localhost:1", *cfg.Address)
	assert.Nil(t, cfg.StoreInterval)
	assert.Nil(t, cfg.DatabaseDSN)
}

func TestLoadFileConfigErrors(t *testing.T) {
	_, err := LoadFileConfig(filepath.Join(t.TempDir(), "missing.json"))
	assert.Error(t, err)

	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{not a json"), 0o644))

	_, err = LoadFileConfig(path)
	assert.Error(t, err)
}
