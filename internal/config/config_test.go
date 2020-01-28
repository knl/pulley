package config

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigDefaults(t *testing.T) {
	expected := DefaultConfig()
	actual := Setup()
	assert.Equal(t, expected, actual)
}

func TestChangeDefaults(t *testing.T) {
	zero := []byte{0}

	os.Setenv("APP_HOST", "::")
	os.Setenv("APP_PORT", "1337")
	os.Setenv("WEBHOOK_PATH", "webhooks")
	os.Setenv("METRICS_PATH", "metrics")
	os.Setenv("WEBHOOK_TOKEN", base64.StdEncoding.EncodeToString(zero))

	actual := Setup()

	expected := DefaultConfig()
	expected.Host = "::"
	expected.Port = "1337"
	expected.WebhookPath = "webhooks"
	expected.MetricsPath = "metrics"
	expected.WebhookToken = zero

	assert.Equal(t, expected, actual)
}
