package config

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigDefaults(t *testing.T) {
	// Needed to ensure the test is correct
	os.Clearenv()

	expected := DefaultConfig()
	actual, err := Setup()
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestChangeDefaults(t *testing.T) {
	// Needed to ensure the test is correct
	os.Clearenv()

	zero := []byte{0}

	os.Setenv("APP_HOST", "::")
	os.Setenv("APP_PORT", "1337")
	os.Setenv("WEBHOOK_PATH", "webhooks")
	os.Setenv("METRICS_PATH", "metrics")
	os.Setenv("WEBHOOK_TOKEN", base64.StdEncoding.EncodeToString(zero))
	os.Setenv("TRACK_BUILD_TIMES", "true")

	actual, err := Setup()
	assert.NoError(t, err)

	expected := DefaultConfig()
	expected.Host = "::"
	expected.Port = "1337"
	expected.WebhookPath = "webhooks"
	expected.MetricsPath = "metrics"
	expected.WebhookToken = zero
	expected.TrackBuildTimes = true

	assert.Equal(t, expected, actual)
}

func TestBadToken(t *testing.T) {
	// Needed to ensure the test is correct
	os.Clearenv()

	os.Setenv("WEBHOOK_TOKEN", "123")

	_, err := Setup()
	assert.Error(t, err)
}

var contextErrorDetectingTests = []struct {
	name     string
	envName  string
	envValue string
	isError  bool
}{
	{"MissingNumber", "GITHUB_CONTEXT_", "123", true},
	{"MissingUS", "GITHUB_CONTEXT_0", "123", true},
	{"WithUS", "GITHUB_CONTEXT_0", "123\x1F123", false},
	{"MissingRepoRegex", "GITHUB_CONTEXT_0", "\x1F123", true},
	{"MissingContextRegex", "GITHUB_CONTEXT_0", "123\x1F", true},
	{"MissingBothRegexes", "GITHUB_CONTEXT_0", "\x1F", true},
	{"BrokenRepoRegex", "GITHUB_CONTEXT_0", "*\x1F123", true},
	{"BrokenContextRegex", "GITHUB_CONTEXT_0", "123\x1F*", true},
}

func TestGithubContextSimpleParser(t *testing.T) {
	for _, tt := range contextErrorDetectingTests {
		tt := tt // see: https://github.com/kyoh86/scopelint/issues/4
		t.Run(tt.name, func(t *testing.T) {
			// Needed to ensure the test is correct
			os.Clearenv()

			os.Setenv(tt.envName, tt.envValue)

			_, err := Setup()
			switch tt.isError {
			case true:
				assert.Error(t, err)
			case false:
				assert.NoError(t, err)
			}
		})
	}
}
