package service

import (
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"

	"github.com/knl/pulley/internal/events"
	"github.com/knl/pulley/internal/test"

	"github.com/stretchr/testify/assert"
)

type Key struct {
	MetricName, Event, Repository string
}

type fakeMetrics struct {
	database map[Key]float64
}

func (m *fakeMetrics) RegisterMerge(repository string, durationSeconds float64) {
}

func (m *fakeMetrics) RegisterStart(repository string, durationSeconds float64) {
}

func (m *fakeMetrics) RegisterValidation(repository string, status events.Status, durationSeconds float64) {
	key := Key{"ci_validation", status.String(), repository}
	val := m.database[key]
	m.database[key] = val + durationSeconds
}

func (m *fakeMetrics) RegisterBuildDone(repository string, build string, status events.Status, durationSeconds float64) {
}

func (m *fakeMetrics) RegisterPREvent(repository string, event events.PREvent) {
	key := Key{"pr_event", event.String(), repository}
	val := m.database[key]
	m.database[key] = val + 1
}

func (m *fakeMetrics) RegisterBranchEvent(repository string, event events.BranchEvent) {
	key := Key{"branch_event", event.String(), repository}
	val := m.database[key]
	m.database[key] = val + 1
}

func (m *fakeMetrics) RegisterStatusCheck(repository string, state events.Status) {
	key := Key{"status_check", state.String(), repository}
	val := m.database[key]
	m.database[key] = val + 1
}

func (m *fakeMetrics) RegisterMissedPending(repository string) {
	key := Key{"pending", "missed", repository}
	val := m.database[key]
	m.database[key] = val + 1
}

func matchAllContexts(repo, context string) bool {
	return true
}

func collectKeys(database map[Key]float64, metric string) []Key {
	keys := make([]Key, 0, len(database))

	for k := range database {
		if k.MetricName == metric {
			keys = append(keys, k)
		}
	}

	return keys
}

// SETUP
// Importantly you need to call Run() once you've done what you need.
func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

// If only PullEvents occur, there should be no other events detected.
func TestPullEventsRecognized(t *testing.T) {
	m := fakeMetrics{
		database: make(map[Key]float64),
	}

	pulley := Pulley{
		Updates: make(chan interface{}),
		Metrics: &m,
		Token:   nil,
	}

	pulley.MetricsProcessor(matchAllContexts, false)

	// Not the best approach, but we don't test MetricsProcessor directly,
	// in order to avoid data races
	for i := 0; i < 10; i++ {
		pulley.Updates <- test.MakePullUpdate()
	}

	close(pulley.Updates)
	pulley.WG.Wait()

	assert := assert.New(t)

	assert.NotEmpty(collectKeys(m.database, "pr_event"))
	assert.Empty(collectKeys(m.database, "branch_event"))
	assert.Empty(collectKeys(m.database, "commit_event"))
}

// If only BranchEvents occur, there should be no other events detected.
func TestBranchEventsRecognized(t *testing.T) {
	m := fakeMetrics{
		database: make(map[Key]float64),
	}

	pulley := Pulley{
		Updates: make(chan interface{}),
		Metrics: &m,
		Token:   nil,
	}

	pulley.MetricsProcessor(matchAllContexts, false)

	// Not the best approach, but we don't test MetricsProcessor directly,
	// in order to avoid data races
	for i := 0; i < 10; i++ {
		pulley.Updates <- test.MakeBranchUpdate()
	}

	close(pulley.Updates)
	pulley.WG.Wait()

	assert := assert.New(t)

	assert.NotEmpty(collectKeys(m.database, "branch_event"))
	assert.Empty(collectKeys(m.database, "pr_event"))
	assert.Empty(collectKeys(m.database, "commit_event"))
}

// Correctly count CI validation times
// Send a new PR event, CI pending, CI success.
func TestCIValidationWithPending(t *testing.T) {
	m := fakeMetrics{
		database: make(map[Key]float64),
	}
	pulley := Pulley{
		Updates: make(chan interface{}),
		Metrics: &m,
		Token:   nil,
	}

	pulley.MetricsProcessor(matchAllContexts, false)

	iterations := 10
	pendingTimeSeconds := 13
	buildTimeSeconds := 60

	// Not the best approach, but we don't test MetricsProcessor directly,
	// in order to avoid data races
	for i := 0; i < iterations; i++ {
		pu := test.MakePullUpdate()

		pulley.Updates <- pu

		cu := events.CommitUpdate{
			Repo:      pu.Repo,
			Status:    events.Pending,
			Context:   "some",
			SHA:       pu.SHA,
			Timestamp: pu.Timestamp.Add(time.Second * time.Duration(pendingTimeSeconds)),
		}

		pulley.Updates <- cu

		cu.Status = events.Success
		cu.Timestamp = cu.Timestamp.Add(time.Second * time.Duration(buildTimeSeconds))

		pulley.Updates <- cu
	}

	close(pulley.Updates)
	pulley.WG.Wait()

	assert := assert.New(t)
	duration := m.database[Key{"ci_validation", events.Success.String(), test.DefaultRepository}]
	expected := float64(iterations * (pendingTimeSeconds + buildTimeSeconds))

	assert.GreaterOrEqual(duration, 0.99*expected)
	assert.LessOrEqual(duration, 1.01*expected)
}

// Correctly count CI validation times
// Send a new PR event, straight to CI success.
func TestCIValidationWithoutPending(t *testing.T) {
	m := fakeMetrics{
		database: make(map[Key]float64),
	}
	pulley := Pulley{
		Updates: make(chan interface{}),
		Metrics: &m,
		Token:   nil,
	}

	pulley.MetricsProcessor(matchAllContexts, false)

	iterations := 10
	buildTimeSeconds := 60

	for i := 0; i < iterations; i++ {
		pu := test.MakePullUpdate()

		pulley.Updates <- pu

		cu := events.CommitUpdate{
			Repo:      pu.Repo,
			Status:    events.Success,
			Context:   "some",
			SHA:       pu.SHA,
			Timestamp: pu.Timestamp.Add(time.Second * time.Duration(buildTimeSeconds)),
		}

		pulley.Updates <- cu
	}

	close(pulley.Updates)
	pulley.WG.Wait()

	assert := assert.New(t)
	duration := m.database[Key{"ci_validation", events.Success.String(), test.DefaultRepository}]
	expected := float64(iterations * buildTimeSeconds)

	assert.GreaterOrEqual(duration, 0.99*expected)
	assert.LessOrEqual(duration, 1.01*expected)
}
