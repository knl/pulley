package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type contextDescriptor struct {
	repo    *regexp.Regexp
	context *regexp.Regexp
}

type Config struct { // nolint
	Host           string              // APP_HOST
	Port           string              // APP_PORT
	WebhookPath    string              // WEBHOOK_PATH
	WebhookToken   []byte              // WEBHOOK_TOKEN
	GitHubContexts []contextDescriptor // GITHUB_CONTEXT_<int> = repo_regex <RS> regex
	MetricsPath    string              // METRICS_PATH
}

type ContextChecker func(repo, context string) bool

func (config *Config) DefaultContextChecker() ContextChecker {
	return func(repo, context string) bool {
		for _, entry := range config.GitHubContexts {
			if entry.repo.MatchString(repo) {
				return entry.context.MatchString(context)
			}
		}

		return false
	}
}

func processGithubContexts() ([]contextDescriptor, error) {
	// Process all GITHUB_CONTEXT_<int> fields
	githubContexts := make(map[uint64]contextDescriptor)

	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if strings.HasPrefix(pair[0], "GITHUB_CONTEXT_") {
			entryID, err := strconv.ParseUint(strings.TrimPrefix(pair[0], "GITHUB_CONTEXT_"), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("environment variable '%s' is not properly formatted, doesn't end with a positive integer, err=%v", pair[0], err)
			}

			descriptor := strings.SplitN(pair[1], "\x1F", 2)
			if len(descriptor) != 2 {
				return nil, fmt.Errorf("environment variable '%s' doesn't have two regexes separated by <US>", e)
			}

			repoRegexp, err := regexp.Compile(descriptor[0])
			if err != nil {
				return nil, fmt.Errorf("could not compile the repository name regex '%s' passed via %s, err=%v", repoRegexp, pair[0], err)
			}

			contextRegexp, err := regexp.Compile(descriptor[1])
			if err != nil {
				return nil, fmt.Errorf("could not compile the status check name regex '%s' passed via %s, err=%v", contextRegexp, pair[0], err)
			}

			githubContexts[entryID] = contextDescriptor{
				repo:    repoRegexp,
				context: contextRegexp,
			}
		}
	}

	// Sort them by priority
	var keys []uint64
	for k := range githubContexts {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	var descriptors []contextDescriptor
	for _, k := range keys {
		descriptors = append(descriptors, githubContexts[k])
	}

	return descriptors, nil
}

func DefaultConfig() *Config {
	var descriptors []contextDescriptor
	descriptors = append(descriptors, contextDescriptor{
		repo:    regexp.MustCompile(".*"),
		context: regexp.MustCompile(":all-jobs$"),
	})

	return &Config{
		Host:           "localhost",
		Port:           "1701",
		WebhookPath:    "",
		WebhookToken:   make([]byte, 0),
		GitHubContexts: descriptors,
		MetricsPath:    "metricz",
	}
}

// Setup configurations with environment variables
func Setup() (*Config, error) {
	config := DefaultConfig()

	host, ok := os.LookupEnv("APP_HOST")
	if ok {
		config.Host = host
	}

	port, ok := os.LookupEnv("APP_PORT")
	if ok {
		config.Port = port
	}

	webhookPath, ok := os.LookupEnv("WEBHOOK_PATH")
	if ok {
		config.WebhookPath = webhookPath
	}

	webhookToken, err := base64.StdEncoding.DecodeString(os.Getenv("WEBHOOK_TOKEN"))
	if err != nil {
		return nil, fmt.Errorf("could not decode the webhook secret token from WEBHOOK_TOKEN, %v", err)
	}

	config.WebhookToken = webhookToken

	metricsPath, ok := os.LookupEnv("METRICS_PATH")
	if ok {
		config.MetricsPath = metricsPath
	}

	githubContexts, err := processGithubContexts()
	if err != nil {
		return nil, err
	}

	if len(githubContexts) != 0 {
		config.GitHubContexts = githubContexts
	}

	return config, nil
}
