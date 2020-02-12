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
	Host            string              // APP_HOST
	Port            string              // APP_PORT
	WebhookPath     string              // WEBHOOK_PATH
	WebhookToken    []byte              // WEBHOOK_TOKEN
	GitHubContexts  []contextDescriptor // GITHUB_STATUS_<int>_REPO = repo_regex && GITHUB_STATUS_<int>_CONTEXT = regex
	MetricsPath     string              // METRICS_PATH
	TrackBuildTimes bool                // TRACK_BUILD_TIMES
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

const (
	statusPrefix  = "GITHUB_STATUS_"
	repoSuffix    = "_REPO"
	contextSuffix = "_CONTEXT"
)

func processGithubContexts() ([]contextDescriptor, error) {
	// Process all GITHUB_STATUS_<int> fields
	githubContexts := make(map[uint64]contextDescriptor)

	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if strings.HasPrefix(pair[0], statusPrefix) && strings.HasSuffix(pair[0], repoSuffix) {
			number := strings.TrimSuffix(strings.TrimPrefix(pair[0], statusPrefix), repoSuffix)

			entryID, err := strconv.ParseUint(number, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("environment variable '%s' is not properly formatted, doesn't contain a positive integer, err=%v", pair[0], err)
			}

			contextEnvName := fmt.Sprintf("%s%d%s", statusPrefix, entryID, contextSuffix)

			contextEnv := os.Getenv(contextEnvName)
			if contextEnv == "" {
				return nil, fmt.Errorf("variable '%s' empty or unset", contextEnvName)
			}

			repoRegexp, err := regexp.Compile(pair[1])
			if err != nil {
				return nil, fmt.Errorf("could not compile the repository name regex '%s' passed via %s, err=%v", repoRegexp, pair[0], err)
			}

			contextRegexp, err := regexp.Compile(contextEnv)
			if err != nil {
				return nil, fmt.Errorf("could not compile the status check name regex '%s' passed via %s, err=%v", contextRegexp, contextEnv, err)
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
		Host:            "localhost",
		Port:            "1701",
		WebhookPath:     "",
		WebhookToken:    make([]byte, 0),
		GitHubContexts:  descriptors,
		MetricsPath:     "metrics",
		TrackBuildTimes: false,
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

	if b, err := strconv.ParseBool(os.Getenv("TRACK_BUILD_TIMES")); err == nil {
		config.TrackBuildTimes = b
	}

	return config, nil
}
