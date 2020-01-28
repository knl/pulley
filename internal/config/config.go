package config

import (
	"encoding/base64"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Config object for the application
var (
	Config *config
)

func init() {
	Setup()
}

type contextDescriptor struct {
	repo    *regexp.Regexp
	context *regexp.Regexp
}

type config struct { // nolint
	Host           string              // APP_HOST
	Port           string              // APP_PORT
	WebhookPath    string              // WEBHOOK_PATH
	WebhookToken   []byte              // WEBHOOK_TOKEN
	GitHubContexts []contextDescriptor // GITHUB_CONTEXT_<int> = repo_regex <RS> regex
	MetricsPath    string              // METRICS_PATH
}

type ContextChecker func(repo, context string) bool

func DefaultContextChecker() ContextChecker {
	return func(repo, context string) bool {
		for _, entry := range Config.GitHubContexts {
			if entry.repo.MatchString(repo) {
			  return entry.context.MatchString(context)
			}
		}

		return false
	}
}

func processGithubContexts() []contextDescriptor {
	// Process all GITHUB_CONTEXT_<int> fields
	githubContexts := make(map[uint64]contextDescriptor)

	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if strings.HasPrefix(pair[0], "GITHUB_CONTEXT_") {
			entryID, err := strconv.ParseUint(strings.TrimPrefix(pair[0], "GITHUB_CONTEXT_"), 10, 64)
			if err != nil {
				log.Fatalf("Entry '%s' is not properly formatted, doesn't end with a positive integer, err=%s", pair[0], err)
			}

			descriptor := strings.SplitN(pair[1], "\x1F", 2)
			if len(descriptor) != 2 {
				log.Fatalf("Entry '%s' doesn't have two regexes separated by <US>", e)
			}

			repoRegexp, err := regexp.Compile(descriptor[0])
			if err != nil {
				log.Fatalf("Could not compile the repository name regex '%s' passed via %s, err=%s, exiting.", repoRegexp, pair[0], err)
			}

			contextRegexp, err := regexp.Compile(descriptor[1])
			if err != nil {
				log.Fatalf("Could not compile the status check name regex '%s' passed via %s, err=%s, exiting.", contextRegexp, pair[0], err)
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

	return descriptors
}

func DefaultConfig() *config {
	var descriptors []contextDescriptor
	descriptors = append(descriptors, contextDescriptor{
		repo:    regexp.MustCompile(".*"),
		context: regexp.MustCompile(":all-jobs$"),
	})

	return &config{
		Host: "localhost",
		Port: "1701",
		WebhookPath: "",
		WebhookToken: make([]byte, 0),
		GitHubContexts: descriptors,
		MetricsPath: "metricz",
	}
}

// Setup configurations with environment variables
func Setup() {
	Config = DefaultConfig()

	host, ok := os.LookupEnv("APP_HOST")
	if ok {
		Config.Host = host
	}

	port, ok := os.LookupEnv("APP_PORT")
	if ok {
		Config.Port = port
	}

	webhookPath, ok := os.LookupEnv("WEBHOOK_PATH")
	if ok {
		Config.WebhookPath = webhookPath
	}

	webhookToken, err := base64.StdEncoding.DecodeString(os.Getenv("WEBHOOK_TOKEN"))
	if err != nil {
		log.Fatal("Could not decode the webhook secret token from WEBHOOK_TOKEN", err)
	}

	Config.WebhookToken = webhookToken

	metricsPath, ok := os.LookupEnv("METRICS_PATH")
	if ok {
		Config.MetricsPath = metricsPath
	}

	githubContexts := processGithubContexts()
	if len(githubContexts) != 0 {
		Config.GitHubContexts = githubContexts
	}
}
