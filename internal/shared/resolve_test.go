package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePriority(t *testing.T) {
	env := "env"
	file := "file"

	assert.Equal(t, "flag", Resolve(true, "flag", &env, &file),
		"explicitly set flag wins over env and file")
	assert.Equal(t, "env", Resolve(false, "default", &env, &file),
		"env wins over file when the flag is not set explicitly")
	assert.Equal(t, "file", Resolve(false, "default", nil, &file),
		"file is used when env is not set")
	assert.Equal(t, "default", Resolve(false, "default", nil, nil),
		"flag default is the last resort")
}

func TestParseIntervalSeconds(t *testing.T) {
	tests := []struct {
		value   string
		want    int
		wantErr bool
	}{
		{value: "1s", want: 1},
		{value: "2m", want: 120},
		{value: "500ms", want: 0},
		{value: "30", want: 30},
		{value: "abc", wantErr: true},
		{value: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := ParseIntervalSeconds(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIntervalPtrSeconds(t *testing.T) {
	result, err := IntervalPtrSeconds(nil)
	require.NoError(t, err)
	assert.Nil(t, result)

	value := "5s"
	result, err = IntervalPtrSeconds(&value)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 5, *result)

	bad := "not-an-interval"
	_, err = IntervalPtrSeconds(&bad)
	assert.Error(t, err)
}
