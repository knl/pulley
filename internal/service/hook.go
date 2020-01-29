package service

import (
	"log"
	"net/http"

	"github.com/google/go-github/v29/github"

	"github.com/knl/pulley/internal/events"
)

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

		// send PR, Branch, and Commit updates to the MetricsProcessor
	LOOP:
		switch e := event.(type) {
		case *github.PullRequestEvent:
			action, err := events.ParsePREvent(*e.Action)
			if err != nil {
				log.Printf("Skipping Pull Request Event, due to %v", err)
				break
			}
			updates <- events.PullUpdate{
				Number:    *e.Number,
				SHA:       *e.PullRequest.Head.SHA,
				Action:    action,
				Timestamp: *e.PullRequest.UpdatedAt,
				Merged:    *e.PullRequest.Merged,
				Repo:      *e.Repo.FullName,
			}
		case *github.PushEvent:
			var action events.BranchEvent
			switch {
			case !*e.Created && !*e.Deleted:
				action = events.Rebased
			case *e.Created && !*e.Deleted:
				action = events.Created
			case !*e.Created && *e.Deleted:
				action = events.Deleted
			default:
				log.Printf("Weird state where branch is both created and deleted, skipping.")
				break LOOP
			}
			updates <- events.BranchUpdate{
				SHA:       *e.After,
				OldSHA:    *e.Before,
				Action:    action,
				Timestamp: e.Repo.PushedAt.Time,
				Repo:      *e.Repo.FullName,
			}
		case *github.StatusEvent:
			status, err := events.ParseStatus(*e.State)
			if err != nil {
				log.Printf("Skipping a status event, due to: %v", err)
				break
			}
			updates <- events.CommitUpdate{
				Status:    status,
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
