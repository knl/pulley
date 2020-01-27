/*
HookHandler - listen for github webhooks, sending updates on channel.
*/
package main

import (
	"log"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/google/go-github/v29/github"
	//"github.com/prometheus/client_golang/prometheus/promhttp"

	"prosli/internal/config"
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
func HookHandler(token []byte, prUp chan<- PullUpdate, cUp chan<- CommitUpdate, brUp chan<- BranchUpdate) http.HandlerFunc {
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
			prUp <- PullUpdate{
				Number:    *e.Number,
				SHA:       *e.PullRequest.Head.SHA,
				Action:    *e.Action,
				Timestamp: *e.PullRequest.UpdatedAt,
				Merged:    *e.PullRequest.Merged,
				Repo:      *e.Repo.FullName,
			}
		case *github.PushEvent:
			brUp <- BranchUpdate{
				SHA:       *e.After,
				OldSHA:    *e.Before,
				Created:   *e.Created,
				Deleted:   *e.Deleted,
				Timestamp: e.Repo.PushedAt.Time,
				Repo:      *e.Repo.FullName,
			}
		case *github.StatusEvent:
			cUp <- CommitUpdate{
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

type contextChecker func(repo, context string) bool

func makeContextChecker(cfg map[*regexp.Regexp]*regexp.Regexp) contextChecker {
	return func(repo, context string) bool {
		for repoRegexp, contextRegexp := range cfg {
			if repoRegexp.MatchString(repo) && contextRegexp.MatchString(context) {
				return true
			}
		}
		return false
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
func MetricsProcessor(contextOk contextChecker) (chan<- PullUpdate, chan<- CommitUpdate, chan<- BranchUpdate) {
	prUp := make(chan PullUpdate)
	cUp := make(chan CommitUpdate)
	brUp := make(chan BranchUpdate)

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
		for {
			select {
			case p := <-prUp:
				// When a PR is opened, its tracking starts.
				log.Printf("updated pr: %d to commit: %s, action=%s\n", p.Number, p.SHA, p.Action)
				if !actionsOfInterest[p.Action] {
					log.Printf("Skipping action %s", p.Action)
					break
				}

				if p.Action == "opened" || p.Action == "reopened" || p.Action == "ready_for_review" {
					liveSHAs[p.SHA] = p.Timestamp
				} else if p.Action == "closed" {
					delete(liveSHAs, p.SHA)
				}
			case br := <-brUp:
				log.Printf("updated a branch to commit: %s (from %s)", br.SHA, br.OldSHA)
				if br.Created {
					// we are not interested in created, as it should be handled by the PR creation
					continue
				}
				if br.Deleted {
					// br.SHA would be all 0s, we need OldSHA here
					log.Printf("Branch is deleted, removing live SHA %s", br.OldSHA)
					delete(liveSHAs, br.SHA)
				} else {
					// This means the branch was updated
					log.Printf("Branch is updated, replacing live SHA %s with %s", br.OldSHA, br.SHA)
					delete(liveSHAs, br.OldSHA)
					liveSHAs[br.SHA] = br.Timestamp
				}
			case c := <-cUp:
				// track good, bad, overall
				// Find which PRs are the ones with the status as the HEAD
				// and use that
				log.Printf("updated commit: %s context: %s status: %s", c.SHA, c.Context, c.Status)
				// We only match certain contexts
				if !contextOk(c.Repo, c.Context) {
					log.Printf("skipping context %s", c.Context)
					continue
				}
				if _, ok := liveSHAs[c.SHA]; !ok {
					log.Printf("Could not find the start time for SHA %s, skipping", c.SHA)
					continue
				}
				switch c.Status {
				case "pending":
					log.Printf("CI Start time for SHA %s is %s", c.SHA, c.Timestamp.Sub(liveSHAs[c.SHA]))
				case "success", "failure", "error":
					log.Printf("Validation time for SHA %s is %s with status %s", c.SHA, c.Timestamp.Sub(liveSHAs[c.SHA]), c.Status)
				default:
					log.Printf("Unknown status type %s", c.Status)
				}
			}
		}
	}()
	return prUp, cUp, brUp
}

// https://godoc.org/github.com/prometheus/client_golang/prometheus
// var (
// 	cpuTemp = prometheus.NewGauge(prometheus.GaugeOpts{
// 		Name: "cpu_temperature_celsius",
// 		Help: "Current temperature of the CPU.",
// 	})
// 	hdFailures = prometheus.NewCounterVec(
// 		prometheus.CounterOpts{
// 			Name: "hd_errors_total",
// 			Help: "Number of hard-disk errors.",
// 		},
// 		[]string{"device"},
// 	)
// )

// func init() {
// 	// Metrics have to be registered to be exposed:
// 	prometheus.MustRegister(cpuTemp)
// 	prometheus.MustRegister(hdFailures)
// }

func main() {
	log.Println("server started")

	prUp, cUp, brUp := MetricsProcessor(makeContextChecker(config.Config.GitHubContexts))
	http.HandleFunc("/"+config.Config.WebhookPath, HookHandler(config.Config.WebhookToken, prUp, cUp, brUp))
	// http.Handle("/metrics", promhttp.Handler())

	// Listen & Serve
	addr := net.JoinHostPort(config.Config.Host, config.Config.Port)
	log.Printf("[service] listening on %s", addr)

	log.Fatal(http.ListenAndServe(addr, nil))
}
