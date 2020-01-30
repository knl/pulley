package service

import (
	"log"
	"time"

	"github.com/knl/pulley/internal/config"
	"github.com/knl/pulley/internal/events"
	"github.com/knl/pulley/internal/metrics"
)

type shaState struct {
	Time      time.Time
	CheckSeen bool // Set to true if a status check has been received
}

type liveSHAMap = map[string]shaState

func processPullUpdate(up events.PullUpdate, liveSHAs *liveSHAMap, publisher metrics.Publisher) {
	// Possible values for PR actions are:
	// "assigned", "unassigned", "review_requested", "review_request_removed", "labeled", "unlabeled",
	// "opened", "edited", "closed", "ready_for_review", "locked", "unlocked", or "reopened".
	switch up.Action {
	case events.Opened, events.Reopened, events.ReadyForReview:
		(*liveSHAs)[up.SHA] = shaState{
			Time:      up.Timestamp,
			CheckSeen: false,
		}
	case events.Closed:
		if _, ok := (*liveSHAs)[up.SHA]; !ok {
			log.Printf("%s is not in live SHAs, skipping.", up.SHA)
			break
		}

		if up.Merged {
			mergeTime := up.Timestamp.Sub((*liveSHAs)[up.SHA].Time).Seconds()
			publisher.RegisterMerge(up.Repo, mergeTime)
		}

		delete(*liveSHAs, up.SHA)

	default:
		log.Printf("Skipping action %s", up.Action)
		return
	}

	publisher.RegisterPREvent(up.Repo, up.Action)
}

func processBranchUpdate(up events.BranchUpdate, liveSHAs *liveSHAMap, publisher metrics.Publisher) {
	switch up.Action {
	case events.Deleted:
		// up.SHA would be all 0s, we need OldSHA here
		log.Printf("Branch is deleted, removing live SHA %s", up.OldSHA)

		delete(*liveSHAs, up.SHA)
	case events.Rebased:
		// This means the branch was updated
		log.Printf("Branch is updated, replacing live SHA %s with %s", up.OldSHA, up.SHA)

		delete(*liveSHAs, up.OldSHA)
		(*liveSHAs)[up.SHA] = shaState{
			Time:      up.Timestamp,
			CheckSeen: false,
		}
	}

	publisher.RegisterBranchEvent(up.Repo, up.Action)
}

func processCommitUpdate(up events.CommitUpdate, liveSHAs *liveSHAMap, publisher metrics.Publisher, contextOk config.ContextChecker) {
	publisher.RegisterStatusCheck(up.Repo, up.Status)

	state, ok := (*liveSHAs)[up.SHA]
	if !ok {
		log.Printf("Could not find the start time for SHA %s, skipping", up.SHA)
		return
	}

	switch up.Status {
	case events.Pending:
		if state.CheckSeen {
			log.Printf("Received a 'pending' check already for SHA %s, skipping", up.SHA)
			break
		}

		state.CheckSeen = true
		(*liveSHAs)[up.SHA] = state

		startTime := up.Timestamp.Sub(state.Time)
		log.Printf("CI Start time for SHA %s is %s", up.SHA, startTime)
		publisher.RegisterStart(up.Repo, startTime.Seconds())

	case events.Success, events.Failure, events.Error:
		// We only match certain contexts here
		// Not done for pending, as there we want to observe the very first check
		if !contextOk(up.Repo, up.Context) {
			log.Printf("skipping context %s", up.Context)
			break
		}

		validationTime := up.Timestamp.Sub((*liveSHAs)[up.SHA].Time)
		log.Printf("Validation time for SHA %s is %s with status %s", up.SHA, validationTime, up.Status)
		publisher.RegisterValidation(up.Repo, up.Status, validationTime.Seconds())

	default:
		log.Printf("Unknown status type %s", up.Status)
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
func MetricsProcessor(contextOk config.ContextChecker, publisher metrics.Publisher) chan<- interface{} {
	updates := make(chan interface{}, 100)

	// Keep track of live SHAs -- we don't need separation per repository, as SHAs are pretty unique
	// map[commitSHA]shaState
	liveSHAs := make(liveSHAMap)

	go func() {
		for update := range updates {
			switch up := update.(type) {
			case events.PullUpdate:
				// When a PR is opened, its tracking starts.
				log.Printf("updated pr: %d to commit: %s, action=%s\n", up.Number, up.SHA, up.Action)

				processPullUpdate(up, &liveSHAs, publisher)

			case events.BranchUpdate:
				log.Printf("updated a branch to commit: %s (from %s)", up.SHA, up.OldSHA)

				processBranchUpdate(up, &liveSHAs, publisher)

			case events.CommitUpdate:
				// track good, bad, overall
				// Find which PRs are the ones with the status as the HEAD
				// and use that
				log.Printf("updated commit: %s context: %s status: %s", up.SHA, up.Context, up.Status)

				processCommitUpdate(up, &liveSHAs, publisher, contextOk)
			}
		}
	}()

	return updates
}