package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_readsSavedConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { config = nil })

	saved := &Config{
		Server: "example.com:7586",
		Certificates: Certificates{
			CaCertPath: "/certs/ca.crt",
			CertPath:   "/certs/client.crt",
			KeyPath:    "/certs/client.key",
		},
	}
	config = saved
	require.NoError(t, SaveConfig())

	config = nil
	require.NoError(t, LoadConfig())
	assert.Equal(t, saved, config)
}
