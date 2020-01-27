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
	MetricsPath    string                            // METRICS_PATH
}

type ContextChecker func(repo, context string) bool

func DefaultContextChecker() ContextChecker {
	return func(repo, context string) bool {
		for repoRegexp, contextRegexp := range Config.GitHubContexts {
			if repoRegexp.MatchString(repo) && contextRegexp.MatchString(context) {
				return true
			}
		}

		return false
	}
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

	webhookToken, err := base64.StdEncoding.DecodeString(os.Getenv("WEBHOOK_TOKEN"))
	if err != nil {
		log.Fatal("Could not decode the webhook secret token from WEBHOOK_TOKEN", err)
	}

	metricsPath, ok := os.LookupEnv("METRICS_PATH")
	if !ok {
		metricsPath = "metricz"
	}
	// Process all GITHUB_CONTEXT fields
	githubContexts := make(map[*regexp.Regexp]*regexp.Regexp)

	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if strings.HasPrefix(pair[0], "GITHUB_CONTEXT_") {
			contextRegexp, err := regexp.Compile(pair[1])
			if err != nil {
				log.Fatalf("Could not compile the regex passed via %s, err=%s, exiting.", pair[0], err)
			}

			regex := strings.TrimPrefix(pair[0], "GITHUB_CONTEXT_")

			repoRegexp, err := regexp.Compile(regex)
			if err != nil {
				log.Fatalf("Could not compile the regex '%s' passed via %s, err=%s, exiting.", regex, pair[0], err)
			}

			githubContexts[repoRegexp] = contextRegexp
		}
	}

	if len(githubContexts) == 0 {
		githubContexts[regexp.MustCompile(".*")] = regexp.MustCompile(":all-jobs$")
	}

	Config = &config{
		Port:           port,
		Host:           host,
		WebhookPath:    os.Getenv("WEBHOOK_PATH"),
		WebhookToken:   webhookToken,
		GitHubContexts: githubContexts,
		MetricsPath:    metricsPath,
	}
}
