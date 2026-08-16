package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlushInterval(t *testing.T) {
	t.Parallel()

	cfg := valid()

	require.Equal(t, "500ms", cfg.Ingest.FlushInterval().String())
}
