package config

import (
	"encoding/base64"
	"log"
	"os"
	"regexp"
	"strings"
)

// Config object for the application
var (
	Config *config
)

func init() {
	Setup()
}

type config struct { // nolint
	Host           string                            // APP_HOST
	Port           string                            // APP_PORT
	WebhookPath    string                            // WEBHOOK_PATH
	WebhookToken   []byte                            // WEBHOOK_TOKEN
	GitHubContexts map[*regexp.Regexp]*regexp.Regexp // GITHUB_CONTEXT_repo_regex = regex
}

// Setup configurations with environment variables
func Setup() {
	host, ok := os.LookupEnv("APP_HOST")
	if !ok {
		host = "localhost"
	}
	port, ok := os.LookupEnv("APP_PORT")
	if !ok {
		port = "1701"
	}
	webhookPath, ok := os.LookupEnv("WEBHOOK_PATH")
	if !ok {
		webhookPath = "webhook"
	}
	webhookToken, err := base64.StdEncoding.DecodeString(os.Getenv("WEBHOOK_TOKEN"))
	if err != nil {
		log.Fatal("Could not decode the webhook secret token from WEBHOOK_TOKEN", err)
	}
	// Process all GITHUB_CONTEXT fields
	githubContexts := make(map[*regexp.Regexp]*regexp.Regexp)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if strings.HasPrefix(pair[0], "GITHUB_CONTEXT_") {
			contextRegexp := regexp.MustCompile(pair[1])
			repoRegexp := regexp.MustCompile(strings.TrimPrefix(pair[0], "GITHUB_CONTEXT_"))
			githubContexts[repoRegexp] = contextRegexp
		}
	}
	if len(githubContexts) == 0 {
		githubContexts[regexp.MustCompile(".*")] = regexp.MustCompile(":all-jobs$")
	}
	Config = &config{
		Port:           port,
		Host:           host,
		WebhookPath:    webhookPath,
		WebhookToken:   webhookToken,
		GitHubContexts: githubContexts,
	}
}
