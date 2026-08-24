package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	require.NotNil(t, cfg)
	assert.Nil(t, cfg.Address)
	assert.Nil(t, cfg.ReportInterval)
	assert.Nil(t, cfg.PollInterval)
}
