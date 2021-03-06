package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

type contextDescriptor struct {
	Repo    *regexp.Regexp
	Context *regexp.Regexp
}

type TimingStrategy int

const (
	_ TimingStrategy = iota
	AggregateStrategy
)

var strategyToString = map[TimingStrategy]string{
	AggregateStrategy: "aggregate",
}

func (ts TimingStrategy) String() string {
	return strategyToString[ts]
}

func parseStrategy(in string) (TimingStrategy, error) {
	for s, ss := range strategyToString {
		if in == ss {
			return s, nil
		}
	}

	allowed := make([]string, 0, len(strategyToString))
	for _, s := range strategyToString {
		allowed = append(allowed, s)
	}

	return 0, fmt.Errorf("could not translate '%s' into an appropriate strategy (allowed values: %v)", in, allowed)
}

type Config struct {
	Host            string         // PULLEY_HOST
	Port            string         // PULLEY_PORT
	WebhookPath     string         // PULLEY_WEBHOOK_PATH
	WebhookToken    []byte         // PULLEY_WEBHOOK_TOKEN
	Strategy        TimingStrategy // PULLEY_PR_TIMING_STRATEGY
	MetricsPath     string         // PULLEY_METRICS_PATH
	TrackBuildTimes bool           // PULLEY_TRACK_BUILD_TIMES
	// Used iff the strategy is 'aggregate'
	AggregateStrategyContexts []contextDescriptor // PULLEY_STRATEGY_AGGREGATE_REPO_REGEX_<int> = repo_regex && PULLEY_STRATEGY_AGGREGATE_CONTEXT_REGEX_<int> = regex
}

func DefaultConfig() *Config {
	var descriptors []contextDescriptor
	descriptors = append(descriptors, contextDescriptor{
		Repo:    regexp.MustCompile(".*"),
		Context: regexp.MustCompile(":all-jobs$"),
	})

	return &Config{
		Host:                      "localhost",
		Port:                      "1701",
		WebhookPath:               "",
		WebhookToken:              make([]byte, 0),
		Strategy:                  AggregateStrategy,
		AggregateStrategyContexts: descriptors,
		MetricsPath:               "metrics",
		TrackBuildTimes:           false,
	}
}

type ContextChecker func(repo, context string) bool

func (config *Config) DefaultContextChecker() ContextChecker {
	return func(repo, context string) bool {
		for _, entry := range config.AggregateStrategyContexts {
			if entry.Repo.MatchString(repo) {
				return entry.Context.MatchString(context)
			}
		}

		return false
	}
}

const (
	repoPrefix    = "PULLEY_STRATEGY_AGGREGATE_REPO_REGEX_"
	contextPrefix = "PULLEY_STRATEGY_AGGREGATE_CONTEXT_REGEX_"
)

func processAggregateStrategyContexts() ([]contextDescriptor, error) {
	// Process all PULLEY_REGEX_TIMING_<int> fields
	aggregateStrategyContexts := make(map[uint64]contextDescriptor)

	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if strings.HasPrefix(pair[0], repoPrefix) {
			entryID, err := strconv.ParseUint(strings.TrimPrefix(pair[0], repoPrefix), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("environment variable '%s' is not properly formatted, doesn't end with a positive integer, err=%v", pair[0], err)
			}

			contextEnvName := fmt.Sprintf("%s%d", contextPrefix, entryID)

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

			aggregateStrategyContexts[entryID] = contextDescriptor{
				Repo:    repoRegexp,
				Context: contextRegexp,
			}
		}
	}

	// Sort them by priority
	var keys []uint64
	for k := range aggregateStrategyContexts {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	var descriptors []contextDescriptor
	for _, k := range keys {
		descriptors = append(descriptors, aggregateStrategyContexts[k])
	}

	return descriptors, nil
}

func configStrategies(config *Config) (*Config, error) {
	strategyString, ok := os.LookupEnv("PULLEY_PR_TIMING_STRATEGY")
	if ok {
		s, err := parseStrategy(strategyString)
		if err != nil {
			return nil, err
		}

		config.Strategy = s
	} else {
		config.Strategy = AggregateStrategy
	}

	switch config.Strategy {
	case AggregateStrategy:
		aggregateStrategyContexts, err := processAggregateStrategyContexts()
		if err != nil {
			return nil, err
		}

		if len(aggregateStrategyContexts) != 0 {
			config.AggregateStrategyContexts = aggregateStrategyContexts
		}
	default:
		return nil, fmt.Errorf("broken configuration, unrecognized strategy '%s'", config.Strategy.String())
	}

	return config, nil
}

// Setup configurations with environment variables.
func Setup() (*Config, error) {
	config := DefaultConfig()

	host, ok := os.LookupEnv("PULLEY_HOST")
	if ok {
		config.Host = host
	}

	port, ok := os.LookupEnv("PULLEY_PORT")
	if ok {
		config.Port = port
	}

	webhookPath, ok := os.LookupEnv("PULLEY_WEBHOOK_PATH")
	if ok {
		config.WebhookPath = webhookPath
	}

	webhookToken, err := base64.StdEncoding.DecodeString(os.Getenv("PULLEY_WEBHOOK_TOKEN"))
	if err != nil {
		return nil, fmt.Errorf("could not decode the webhook secret token from PULLEY_WEBHOOK_TOKEN, %v", err)
	}

	config.WebhookToken = webhookToken

	metricsPath, ok := os.LookupEnv("PULLEY_METRICS_PATH")
	if ok {
		config.MetricsPath = metricsPath
	}

	if b, err := strconv.ParseBool(os.Getenv("PULLEY_TRACK_BUILD_TIMES")); err == nil {
		config.TrackBuildTimes = b
	}

	return configStrategies(config)
}

var configOutputTmpl = `
pulley is starting with the following configuration:
  Host:            {{.Host}}
  Port:            {{.Port}}
  MetricsPath:     /{{.MetricsPath}}
  WebhookPath:     /{{.WebhookPath}}
  WebhookToken:    {{with .WebhookToken}}{{printf "%+.4q" .| dequote}}...{{else}}<empty>{{end}}
  TrackBuildTimes: {{.TrackBuildTimes}}
  Strategy:        {{.Strategy}}
  {{template "aggregate" .AggregateStrategyContexts}}
`

var aggregateOutputTmpl = `
{{define "aggregate"}}Aggregate Strategy configuration:{{range .}}
   - repo:    {{.Repo}}
     context: {{.Context}}
  {{end}}
{{end}}
`

// Returns a string containing the configuration, useful for logging.
func (config *Config) Print() (string, error) {
	t := template.Must(template.New("config").Funcs(template.FuncMap{
		"dequote": func(s string) string {
			return strings.Trim(s, `"`)
		},
	}).Parse(configOutputTmpl))

	_, err := t.Parse(aggregateOutputTmpl)
	if err != nil {
		return "", fmt.Errorf("problem parsing aggregate configuration: %s", err)
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "config", config); err != nil {
		return "", fmt.Errorf("could not output configuration, %s", err)
	}

	return strings.TrimSpace(buf.String()), nil
}
