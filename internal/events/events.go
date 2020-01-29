package events

import (
	"fmt"
	"time"
)

type PREvent int

const (
	_ PREvent = iota
	Assigned
	Unassigned
	ReviewRequested
	ReviewRequestRemoved
	Labeled
	Unlabeled
	Opened
	Edited
	Closed
	ReadyForReview
	Locked
	Unlocked
	Reopened
)

var prToString = map[PREvent]string{
	Assigned:             "assigned",
	Unassigned:           "unassigned",
	ReviewRequested:      "review_requested",
	ReviewRequestRemoved: "review_request_removed",
	Labeled:              "labeled",
	Unlabeled:            "unlabeled",
	Opened:               "opened",
	Edited:               "edited",
	Closed:               "closed",
	ReadyForReview:       "ready_for_review",
	Locked:               "locked",
	Unlocked:             "unlocked",
	Reopened:             "reopened",
}

func (pre PREvent) String() string {
	return prToString[pre]
}

func ParsePREvent(s string) (PREvent, error) {
	for pre, ss := range prToString {
		if s == ss {
			return pre, nil
		}
	}

	return 0, fmt.Errorf("could not translate '%s' into a PREvent", s)
}

type BranchEvent int

const (
	Created BranchEvent = iota
	Deleted
	Rebased
)

var beToString = map[BranchEvent]string{
	Created: "created",
	Deleted: "deleted",
	Rebased: "rebased",
}

func (be BranchEvent) String() string {
	return beToString[be]
}

type Status int

const (
	_ Status = iota
	Pending
	Success
	Failure
	Error
)

var statusToString = map[Status]string{
	Pending: "pending",
	Success: "success",
	Failure: "failure",
	Error:   "error",
}

func (s Status) String() string {
	return statusToString[s]
}

func ParseStatus(in string) (Status, error) {
	for s, ss := range statusToString {
		if in == ss {
			return s, nil
		}
	}

	return 0, fmt.Errorf("could not translate '%s' into a Status", in)
}

// When there is an update to a Pull Request, such as creation, closing, re-opening
type PullUpdate struct {
	Repo      string
	Action    PREvent
	SHA       string
	Number    int
	Merged    bool
	Timestamp time.Time
}

// When the branch has been updated, either due to a push or force-push
// Not interested in closing, that PullUpdate handles
type BranchUpdate struct {
	Repo      string
	Action    BranchEvent
	SHA       string
	OldSHA    string
	Timestamp time.Time
}

// When we get a status notification from CI
type CommitUpdate struct {
	Repo      string
	Status    Status
	Context   string
	SHA       string
	Timestamp time.Time
}
