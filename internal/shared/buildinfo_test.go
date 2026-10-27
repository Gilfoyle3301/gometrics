package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatBuildInfoAllEmpty(t *testing.T) {
	expected := "Build version: N/A\nBuild date: N/A\nBuild commit: N/A\n"
	assert.Equal(t, expected, FormatBuildInfo("", "", ""))
}

func TestFormatBuildInfoAllFilled(t *testing.T) {
	expected := "Build version: v1.2.3\nBuild date: 2026-10-27\nBuild commit: abc1234\n"
	assert.Equal(t, expected, FormatBuildInfo("v1.2.3", "2026-10-27", "abc1234"))
}

func TestFormatBuildInfoPartiallyFilled(t *testing.T) {
	expected := "Build version: v1.0.0\nBuild date: N/A\nBuild commit: def5678\n"
	assert.Equal(t, expected, FormatBuildInfo("v1.0.0", "", "def5678"))
}
