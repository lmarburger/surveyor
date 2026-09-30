package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestParseConfig(t *testing.T) {
	t.Run("uses defaults with only a password", func(t *testing.T) {
		cfg, err := parseConfig(nil, env(map[string]string{"SURVEYOR_MODEM_PASSWORD": "hunter2"}))
		require.NoError(t, err)

		assert.Equal(t, "https://192.168.100.1/HNAP1/", cfg.modemURL)
		assert.Equal(t, "admin", cfg.username)
		assert.Equal(t, "hunter2", cfg.password)
		assert.Equal(t, 30*time.Second, cfg.interval)
	})

	t.Run("reads the modem address from the environment", func(t *testing.T) {
		cfg, err := parseConfig(nil, env(map[string]string{
			"SURVEYOR_MODEM_PASSWORD": "hunter2",
			"SURVEYOR_MODEM_URL":      "https://10.0.0.1/HNAP1/",
			"SURVEYOR_MODEM_USERNAME": "root",
		}))
		require.NoError(t, err)

		assert.Equal(t, "https://10.0.0.1/HNAP1/", cfg.modemURL)
		assert.Equal(t, "root", cfg.username)
	})

	t.Run("requires a password", func(t *testing.T) {
		_, err := parseConfig(nil, env(nil))
		assert.ErrorContains(t, err, "SURVEYOR_MODEM_PASSWORD")
	})

	t.Run("rejects a max backoff shorter than the interval", func(t *testing.T) {
		_, err := parseConfig([]string{"-interval", "1m", "-max-backoff", "30s"},
			env(map[string]string{"SURVEYOR_MODEM_PASSWORD": "hunter2"}))
		assert.ErrorContains(t, err, "max-backoff")
	})
}
