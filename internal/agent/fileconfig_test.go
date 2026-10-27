package agent

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
		"report_interval": "1s",
		"poll_interval": "500ms",
		"key": "secret",
		"crypto_key": "/tmp/public.pem",
		"rate_limit": 5
	}`
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))

	cfg, err := LoadFileConfig(path)
	require.NoError(t, err)

	require.NotNil(t, cfg.Address)
	assert.Equal(t, "localhost:9090", *cfg.Address)
	require.NotNil(t, cfg.ReportInterval)
	assert.Equal(t, "1s", *cfg.ReportInterval)
	require.NotNil(t, cfg.PollInterval)
	assert.Equal(t, "500ms", *cfg.PollInterval)
	require.NotNil(t, cfg.Key)
	assert.Equal(t, "secret", *cfg.Key)
	require.NotNil(t, cfg.CryptoKey)
	assert.Equal(t, "/tmp/public.pem", *cfg.CryptoKey)
	require.NotNil(t, cfg.RateLimit)
	assert.Equal(t, 5, *cfg.RateLimit)
}

func TestLoadFileConfigEmptyPath(t *testing.T) {
	cfg, err := LoadFileConfig("")
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Nil(t, cfg.Address)
	assert.Nil(t, cfg.RateLimit)
}

func TestLoadFileConfigErrors(t *testing.T) {
	_, err := LoadFileConfig(filepath.Join(t.TempDir(), "missing.json"))
	assert.Error(t, err)

	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{not a json"), 0o644))

	_, err = LoadFileConfig(path)
	assert.Error(t, err)
}
