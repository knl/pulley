package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/knl/pulley/internal/events"
	"github.com/knl/pulley/internal/version"
)

// https://godoc.org/github.com/prometheus/client_golang/prometheus
type GithubMetrics struct {
	PREvents            *prometheus.CounterVec   // The number of times a particular PR event occurred
	BranchEvents        *prometheus.CounterVec   // The number of branch creations, deletions, and rebases
	StatusChecks        *prometheus.CounterVec   // The number of status checks received
	MissedPendings      *prometheus.CounterVec   // The number of missed pending events
	CINoticedDuration   *prometheus.HistogramVec // The distribution of the durations between PR creation and the first ping by the CI
	PRValidatedDuration *prometheus.HistogramVec // The distribution of the durations between PR creation and the status check that makes the PR mergeable (required status check)
	PRMergedDuration    *prometheus.HistogramVec // The distribution of the duration between PR creation and the time it was merged
	BuildDuration       *prometheus.HistogramVec // The distribution of the build durations
}

func NewGithubMetrics() *GithubMetrics {
	return &GithubMetrics{
		PREvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "github_pull_request_events_total",
			Help: "The number of Pull Request events",
		},
			[]string{"repository", "event"},
		),
		BranchEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "github_branch_events_total",
			Help: "The number branch creations, rebases, and deletions",
		},
			[]string{"repository", "event"},
		),
		StatusChecks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "github_status_checks_total",
			Help: "The number of status checks",
		},
			[]string{"repository", "state"},
		),
		MissedPendings: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "github_ci_missed_pending",
			Help: "The number of times there was a success/failure/error without corresponding pending status",
		},
			[]string{"repository"},
		),
		CINoticedDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "github_ci_noticed_duration_seconds",
				Help: "The time it takes for a CI to send the first 'pending' status check, measured from opening the PR",
				// Start from 1 second, move up to 8*1024 seconds (~80min)
				Buckets: prometheus.ExponentialBuckets(1, 2, 14),
			},
			[]string{"repository"},
		),
		PRValidatedDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "github_pull_request_validated_duration_seconds",
				Help: "The time it takes for a CI to build a PR, measured from opening the PR until the required status check is finished, per status",
				// Start from 1 second, move up to 8*1024 seconds (~2h30)
				Buckets: prometheus.ExponentialBuckets(1, 2, 14),
			},
			[]string{"repository", "status"},
		),
		PRMergedDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "github_pull_request_merged_duration_seconds",
				Help: "The time it takes for a PR to be merged, measured from opening the PR",
				// Start from 1 minute, move up to 8*1024 seconds (~6 days)
				Buckets: prometheus.ExponentialBuckets(60, 2, 14),
			},
			[]string{"repository"},
		),
		BuildDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "github_ci_build_duration_seconds",
				Help: "The time it takes for a build",
				// Start from 1 second, move up to 512s (~9min)
				Buckets: prometheus.ExponentialBuckets(1, 2, 10),
			},
			[]string{"repository", "build", "status"},
		),
	}
}

type Publisher interface {
	Setup()
	RegisterMerge(repository string, durationSeconds float64)
	RegisterStart(repository string, durationSeconds float64)
	RegisterValidation(repository string, status events.Status, durationSeconds float64)
	RegisterBuildDone(repository string, build string, state events.Status, durationSeconds float64)
	RegisterPREvent(repository string, event events.PREvent)
	RegisterBranchEvent(repository string, event events.BranchEvent)
	RegisterStatusCheck(repository string, state events.Status)
	RegisterMissedPending(repository string)
}

func (m *GithubMetrics) Setup() {
	prometheus.MustRegister(m.PREvents)
	prometheus.MustRegister(m.BranchEvents)
	prometheus.MustRegister(m.StatusChecks)
	prometheus.MustRegister(m.MissedPendings)
	prometheus.MustRegister(m.CINoticedDuration)
	prometheus.MustRegister(m.PRValidatedDuration)
	prometheus.MustRegister(m.PRMergedDuration)
	prometheus.MustRegister(m.BuildDuration)
	prometheus.MustRegister(version.NewCollector())
}

func (m *GithubMetrics) RegisterMerge(repository string, durationSeconds float64) {
	m.PRMergedDuration.With(prometheus.Labels{"repository": repository}).Observe(durationSeconds)
}

func (m *GithubMetrics) RegisterStart(repository string, durationSeconds float64) {
	m.CINoticedDuration.With(prometheus.Labels{"repository": repository}).Observe(durationSeconds)
}

func (m *GithubMetrics) RegisterValidation(repository string, status events.Status, durationSeconds float64) {
	m.PRValidatedDuration.With(prometheus.Labels{"repository": repository, "status": status.String()}).Observe(durationSeconds)
}

func (m *GithubMetrics) RegisterBuildDone(repository string, build string, status events.Status, durationSeconds float64) {
	m.BuildDuration.With(prometheus.Labels{"repository": repository, "build": build, "status": status.String()}).Observe(durationSeconds)
}

func (m *GithubMetrics) RegisterPREvent(repository string, event events.PREvent) {
	m.PREvents.With(prometheus.Labels{"repository": repository, "event": event.String()}).Inc()
}

func (m *GithubMetrics) RegisterBranchEvent(repository string, event events.BranchEvent) {
	m.BranchEvents.With(prometheus.Labels{"repository": repository, "event": event.String()}).Inc()
}

func (m *GithubMetrics) RegisterStatusCheck(repository string, state events.Status) {
	m.StatusChecks.With(prometheus.Labels{"repository": repository, "state": state.String()}).Inc()
}

func (m *GithubMetrics) RegisterMissedPending(repository string) {
	m.MissedPendings.With(prometheus.Labels{"repository": repository}).Inc()
}
