package config

import (
	"encoding/base64"
	"os"
	"strings"
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

	os.Setenv("PULLEY_HOST", "::")
	os.Setenv("PULLEY_PORT", "1337")
	os.Setenv("PULLEY_WEBHOOK_PATH", "webhooks")
	os.Setenv("PULLEY_METRICS_PATH", "metrics")
	os.Setenv("PULLEY_WEBHOOK_TOKEN", base64.StdEncoding.EncodeToString(zero))
	os.Setenv("PULLEY_TRACK_BUILD_TIMES", "true")

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

	os.Setenv("PULLEY_WEBHOOK_TOKEN", "123")

	_, err := Setup()
	assert.Error(t, err)
}

var contextErrorDetectingTests = []struct {
	name    string
	envVars []string
	isError bool
}{
	{"MissingSuffix", []string{"PULLEY_GITHUB_STATUS_=123"}, false}, // not an error, since doesn't have _REPO
	{"MissingNumber", []string{"PULLEY_GITHUB_STATUS__REPO=123", "PULLEY_GITHUB_STATUS__CONTEXT=123"}, true},
	{"MissingNumberOneUnderscore", []string{"PULLEY_GITHUB_STATUS_REPO=123", "PULLEY_GITHUB_STATUS_CONTEXT=123"}, true},
	{"MissingRepo", []string{"PULLEY_GITHUB_STATUS_0_CONTEXT=123"}, false}, // not an error as well, as we always look for _REPO first
	{"MissingContext", []string{"PULLEY_GITHUB_STATUS_0_REPO=123"}, true},
	{"BothPresent", []string{"PULLEY_GITHUB_STATUS_0_REPO=123", "PULLEY_GITHUB_STATUS_0_CONTEXT=123"}, false},
	{"NumberMismatch", []string{"PULLEY_GITHUB_STATUS_0_REPO=123", "PULLEY_GITHUB_STATUS_1_CONTEXT=123"}, true},
	{"BothMissing", []string{}, false},
	{"BrokenRepoRegex", []string{"PULLEY_GITHUB_STATUS_123_REPO=*", "PULLEY_GITHUB_STATUS_123_CONTEXT=123"}, true},
	{"BrokenRepoRegex", []string{"PULLEY_GITHUB_STATUS_123_REPO=123", "PULLEY_GITHUB_STATUS_123_CONTEXT=*"}, true},
}

func TestGithubContextSimpleParser(t *testing.T) {
	for _, tt := range contextErrorDetectingTests {
		tt := tt // see: https://github.com/kyoh86/scopelint/issues/4
		t.Run(tt.name, func(t *testing.T) {
			// Needed to ensure the test is correct
			os.Clearenv()

			for _, e := range tt.envVars {
				pair := strings.SplitN(e, "=", 2)
				os.Setenv(pair[0], pair[1])
			}

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
