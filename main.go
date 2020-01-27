/*
HookHandler - listen for github webhooks, sending updates on channel.
*/
package main

import (
	"log"
	"net"
	"net/http"
	"time"

	"github.com/google/go-github/v29/github"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/knl/pulley/internal/config"
)

// https://godoc.org/github.com/prometheus/client_golang/prometheus
var (
	prEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "github_pull_request_events_total",
		Help: "The number of various Pull Request events",
	},
		[]string{"repository", "event"},
	)
	branchRebases = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "github_branch_rebases_total",
		Help: "The number branch rebases",
	},
		[]string{"repository"},
	)
	branchEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "github_branch_events_total",
		Help: "The number branch creations and deletions",
	},
		[]string{"repository", "event"},
	)
	prStartTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "github_ci_start_time_seconds",
			Help: "The time it takes for a CI to build a PR, measured from opening the PR until the required status check is finished, per status",
			// Start from 1 second, move up to 8*1024 seconds (~80min)
			Buckets: prometheus.ExponentialBuckets(1, 2, 14),
		},
		[]string{"repository"},
	)
	prValidationTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "github_ci_validation_time_seconds",
			Help: "The time it takes for a CI to build a PR, measured from opening the PR until the required status check is finished, per status",
			// Start from 1 second, move up to 8*1024 seconds (~80min)
			Buckets: prometheus.ExponentialBuckets(1, 2, 14),
		},
		[]string{"repository", "status"},
	)
	prMergeTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "github_merge_time_seconds",
			Help: "The time it takes for a PR, measured from opening the PR",
			// Start from 1 second, move up to 8*1024 seconds (~80min)
			Buckets: prometheus.ExponentialBuckets(1, 2, 14),
		},
		[]string{"repository"},
	)
)

// When there is an update to a Pull Request, such as creation, closing, re-opening
type PullUpdate struct {
	Repo      string
	Action    string
	SHA       string
	Number    int
	Merged    bool
	Timestamp time.Time
}

// When the branch has been updated, either due to a push or force-push
// Not interested in closing, that PullUpdate handles
type BranchUpdate struct {
	Repo      string
	SHA       string
	OldSHA    string
	Created   bool
	Deleted   bool
	Timestamp time.Time
}

// When we get a status notification from CI
type CommitUpdate struct {
	Repo      string
	Status    string
	Context   string
	SHA       string
	Timestamp time.Time
}

// HookHandler parses GitHub webhooks and sends an update to MetricsProcessor.
func HookHandler(token []byte, updates chan<- interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405) // Return 405 Method Not Allowed.
			return
		}

		payload, err := github.ValidatePayload(r, token)
		if err != nil {
			log.Printf("error reading request body: err=%s\n", err)
			w.WriteHeader(400) // Return 400 Bad Request.

			return
		}
		defer r.Body.Close()

		event, err := github.ParseWebHook(github.WebHookType(r), payload)
		if err != nil {
			log.Printf("could not parse webhook: err=%s\n", err)
			w.WriteHeader(400) // Return 400 Bad Request.

			return
		}

		// send PR or Branch updates to the MetricsProcessor
		// send commit status (from CircleCI) to the MetricsProcessor
		switch e := event.(type) {
		case *github.PullRequestEvent:
			updates <- PullUpdate{
				Number:    *e.Number,
				SHA:       *e.PullRequest.Head.SHA,
				Action:    *e.Action,
				Timestamp: *e.PullRequest.UpdatedAt,
				Merged:    *e.PullRequest.Merged,
				Repo:      *e.Repo.FullName,
			}
		case *github.PushEvent:
			updates <- BranchUpdate{
				SHA:       *e.After,
				OldSHA:    *e.Before,
				Created:   *e.Created,
				Deleted:   *e.Deleted,
				Timestamp: e.Repo.PushedAt.Time,
				Repo:      *e.Repo.FullName,
			}
		case *github.StatusEvent:
			updates <- CommitUpdate{
				// State is the new state. Possible values are: "pending", "success", "failure", "error".
				Status:    *e.State,
				Context:   *e.Context,
				SHA:       *e.SHA,
				Timestamp: e.UpdatedAt.Time,
				Repo:      *e.Repo.FullName,
			}
		default:
			log.Printf("unknown WebHookType: %s, webhook-id: %s skipping\n", github.WebHookType(r), r.Header.Get("X-GitHub-Delivery"))
		}
	}
}

// MetricsProcessor receives updates when
// - a pull request is opened/updated/closed
// - a branch receives a new push (merge to master is a push event)
// - a status has been received for a commit
// The pullUpdate and branchUpdate channels will update a branch or PR SHA
// to the current one.
//
// The MetricsProcessor works by keeping a track of "live" SHAs. They get
// created when a PR is opened/reopened. They get updated when a branch is being
// pushed to (while the old value gets removed). For each of these live SHAs, it
// keeps the creation time (when PR/branch has been created).
//
// The assumption is that the CI builds everything (branches and PRs). If there are
// branches that linger around, it's not a problem, because there aren't so many of them.
func MetricsProcessor(contextOk config.ContextChecker) chan<- interface{} {
	updates := make(chan interface{}, 100)

	// Keep track of live SHAs -- we don't need separation per repository, as SHAs are pretty unique
	// map[commitSHA]time
	liveSHAs := make(map[string]time.Time)
	// Track when the first notification arrived from the CI
	// prFirstStatusTimes := make(map[int]time.Time)
	// Track when the PR validation was completed (either success or failure)
	// prValidationTimes := make(map[int]time.Time)
	// Track when the PR was merged
	// prMergeTimes := make(map[int]time.Time)

	// Possible values for PR actions are:
	// "assigned", "unassigned", "review_requested", "review_request_removed", "labeled", "unlabeled",
	// "opened", "edited", "closed", "ready_for_review", "locked", "unlocked", or "reopened".
	// These are the ones to act on, as they are meaningful for processing
	actionsOfInterest := map[string]bool{
		"opened":           true,
		"closed":           true,
		"reopened":         true,
		"ready_for_review": true,
	}

	go func() {
		for update := range updates {
			switch up := update.(type) {
			case PullUpdate:
				// When a PR is opened, its tracking starts.
				log.Printf("updated pr: %d to commit: %s, action=%s\n", up.Number, up.SHA, up.Action)

				if !actionsOfInterest[up.Action] {
					log.Printf("Skipping action %s", up.Action)
					break
				}

				switch up.Action {
				case "opened", "reopened", "ready_for_review":
					liveSHAs[up.SHA] = up.Timestamp
				case "closed":
					delete(liveSHAs, up.SHA)

					if up.Merged {
						mergeTime := up.Timestamp.Sub(liveSHAs[up.SHA]).Seconds()
						prMergeTime.With(prometheus.Labels{"repository": up.Repo}).Observe(mergeTime)
					}
				}

				prEvents.With(prometheus.Labels{"repository": up.Repo, "event": up.Action}).Inc()
			case BranchUpdate:
				log.Printf("updated a branch to commit: %s (from %s)", up.SHA, up.OldSHA)

				switch {
				case up.Created:
					// we are not interested in created, as it should be handled by the PR creation
					branchEvents.With(prometheus.Labels{"repository": up.Repo, "event": "created"}).Inc()
				case up.Deleted:
					// up.SHA would be all 0s, we need OldSHA here
					log.Printf("Branch is deleted, removing live SHA %s", up.OldSHA)

					delete(liveSHAs, up.SHA)

					branchEvents.With(prometheus.Labels{"repository": up.Repo, "event": "deleted"}).Inc()
				default:
					// This means the branch was updated
					log.Printf("Branch is updated, replacing live SHA %s with %s", up.OldSHA, up.SHA)

					delete(liveSHAs, up.OldSHA)
					liveSHAs[up.SHA] = up.Timestamp

					branchRebases.With(prometheus.Labels{"repository": up.Repo}).Inc()
				}
			case CommitUpdate:
				// track good, bad, overall
				// Find which PRs are the ones with the status as the HEAD
				// and use that
				log.Printf("updated commit: %s context: %s status: %s", up.SHA, up.Context, up.Status)

				if _, ok := liveSHAs[up.SHA]; !ok {
					log.Printf("Could not find the start time for SHA %s, skipping", up.SHA)
					break
				}

				switch up.Status {
				case "pending":
					log.Printf("CI Start time for SHA %s is %s", up.SHA, up.Timestamp.Sub(liveSHAs[up.SHA]))
					startTime := up.Timestamp.Sub(liveSHAs[up.SHA]).Seconds()
					prStartTime.With(prometheus.Labels{"repository": up.Repo}).Observe(startTime)
				case "success", "failure", "error":
					// We only match certain contexts here
					// Not done for pending, as there we want to observe the very first check
					if !contextOk(up.Repo, up.Context) {
						log.Printf("skipping context %s", up.Context)
						break
					}

					log.Printf("Validation time for SHA %s is %s with status %s", up.SHA, up.Timestamp.Sub(liveSHAs[up.SHA]), up.Status)
					validationTime := up.Timestamp.Sub(liveSHAs[up.SHA]).Seconds()
					prValidationTime.With(prometheus.Labels{"repository": up.Repo, "status": up.Status}).Observe(validationTime)

				default:
					log.Printf("Unknown status type %s", up.Status)
				}
			}
		}
	}()

	return updates
}

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(prEvents)
	prometheus.MustRegister(branchRebases)
	prometheus.MustRegister(branchEvents)
	prometheus.MustRegister(prValidationTime)
	prometheus.MustRegister(prStartTime)
	prometheus.MustRegister(prMergeTime)
}

func main() {
	log.Println("server started")

	updates := MetricsProcessor(config.DefaultContextChecker())
	http.HandleFunc("/"+config.Config.WebhookPath, HookHandler(config.Config.WebhookToken, updates))
	http.Handle("/"+config.Config.MetricsPath, promhttp.Handler())

	// Listen & Serve
	addr := net.JoinHostPort(config.Config.Host, config.Config.Port)
	log.Printf("[service] listening on %s", addr)

	log.Fatal(http.ListenAndServe(addr, nil))
}
