package config

import (
	"encoding/base64"
	"log"
	"os"
	"regexp"
)

// Config object for the application
var (
	Config *config
)

func init() {
	Setup()
}

type config struct { // nolint
	Host          string         // APP_HOST
	Port          string         // APP_PORT
	WebhookPath   string         // WEBHOOK_PATH
	WebhookToken  []byte         // WEBHOOK_TOKEN
	GitHubContext *regexp.Regexp // GITHUB_CONTEXT
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
	githubContext, ok := os.LookupEnv("GITHUB_CONTEXT")
	if !ok {
		githubContext = ".*all-jobs$"
	}
	Config = &config{
		Port:          port,
		Host:          host,
		WebhookPath:   webhookPath,
		WebhookToken:  webhookToken,
		GitHubContext: regexp.MustCompile(githubContext),
	}
}
