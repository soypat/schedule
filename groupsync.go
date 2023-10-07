package schedule

import (
	"errors"
	"fmt"
	"time"
)

var (
	errBeginNotCalled = errors.New("ScheduleNext called before Begin")
	errMissedAction   = errors.New("missed action. This happens if event loop Update is not called at enough high frequency to prevent missing an action between calls")
	errGroupFailed    = errors.New("group failed")
	ErrSmallDuration  = errors.New("small duration. This may cause missed action errors")
	errZeroDuration   = errors.New("zero duration in GroupSync. Use GroupLoose for when actions can have zero duration")
	errBadIterations  = errors.New("zero or negative iterations")
)

type GroupSyncConfig struct {
	// Iterations specifies how many times to run the group. Must be greater than zero
	// or -1 to indicate infinite iterations.
	Iterations int
}

// NewGroupSync returns a newly initialized group. Action duration must be greater than zero.
func NewGroupSync[T any](actions []Action[T], cfg GroupSyncConfig) (*GroupSync[T], error) {
	duration, err := actionsDuration(actions, false)
	switch {
	case err != nil && !errors.Is(err, ErrSmallDuration):
		return nil, err
	case len(actions) == 0:
		return nil, errors.New("empty actions")
	case cfg.Iterations <= 0 && cfg.Iterations != -1:
		return nil, errBadIterations
	}

	g := &GroupSync[T]{
		actions:    actions,
		duration:   duration,
		iterations: cfg.Iterations,
	}
	return g, err // return ErrSmallDuration as a warning to users.
}

// GroupSync specifies a group of actions that should be executed one after another
// while prioritizing the time between actions and the periodicity of the group.
// This is to say that if the group ran for a long one could calculate how
// many times the group was executed knowing.
//
// Some observations on when to use a GroupSync:
//
//   - Actions take much longer than the period of the event loop.
//   - Keeping synchonization with other groups over time is more important
//     than matching the action duration.
//
// Important things to note when using GroupSync:
//
//   - Actions that are not triggered exactly on schedule will have their duration
//     shortened to not delay the scheduling of the next action.
//   - If an action is not scheduled during its allotted time the group will fail
//     and errors will be returned then onwards until Begin is called again.
type GroupSync[T any] struct {
	start time.Time
	// elapsedToRestart necessary to prevent a bug where a whole schedule is missed.
	// Add this to start to get time of last restart.
	elapsedToRestart time.Duration
	duration         time.Duration
	lastIdx          int
	actions          []Action[T]
	iterations       int
	failed           bool
}

type Action[T any] struct {
	Duration time.Duration
	Value    T
}

// Begins sets the start time of the group. It must be called before ScheduleNext.
// It effectively resets internal state of the group.
func (g *GroupSync[T]) Begins(start time.Time) {
	g.start = start
	g.elapsedToRestart = 0
	g.lastIdx = -1
	g.failed = false
}

// StartTime time returns the time the group was Started at. If not started returns zero value.
func (g *GroupSync[T]) StartTime() time.Time {
	return g.start
}

// Duration returns the time it takes to fully execute all actions in group.
func (g *GroupSync[T]) Duration() time.Duration {
	return g.duration
}

// Iterations returns the number of iterations the group will run for.
// It may be -1 for infinite iterations.
func (g *GroupSync[T]) Iterations() int {
	return g.iterations
}

// ScheduleNext checks `now` against time GroupSync started and returns
// the next executable action when `ok` is true and `next` duration until next
// ready action.
//
// If ok is false and next is zero the group is done.
func (g *GroupSync[T]) ScheduleNext(now time.Time) (v T, ok bool, next time.Duration, err error) {
	if g.start.IsZero() {
		return v, false, 0, errBeginNotCalled
	}
	if g.failed {
		return v, false, next, errGroupFailed
	}
	return g.scheduleNext(now)
}

func (g *GroupSync[T]) scheduleNext(now time.Time) (v T, ok bool, next time.Duration, err error) {
	elapsed := now.Sub(g.start)
	if elapsed < 0 {
		return v, false, -elapsed, nil // Still waiting for start time.
	}
	runtime := g.Duration()

	restartActive := g.iterations == -1 || g.iterations > 1 && elapsed < time.Duration(g.iterations)*runtime
	if restartActive {
		elapsed = elapsed % runtime
	}

	// Find index of next action.
	nextIdx, next := currentIdx(g.actions, elapsed)
	if nextIdx == g.lastIdx {
		return v, false, next, nil // Still need to execute current action.
	}
	// We check the worst case scenario where we missed an action.
	if nextIdx != -1 && !restartActive && nextIdx != g.lastIdx+1 ||
		(nextIdx != -1 && restartActive && nextIdx != (g.lastIdx+1)%(len(g.actions))) {
		g.failed = true
		return v, false, 0, errMissedAction // Missed action.
	} else if nextIdx == -1 {
		// We are done, time exceeded.
		return v, false, 0, nil
	}

	if nextIdx == g.lastIdx+1 || (restartActive && nextIdx == 0 && g.lastIdx == len(g.actions)-1) {
		// It is time for the next action.
		g.lastIdx = nextIdx
		return g.actions[nextIdx].Value, true, next, nil
	}
	return v, false, next, fmt.Errorf("unexpected nextIdx: %d, lastIdx: %d", nextIdx, g.lastIdx)
}

func actionsDuration[T any](actions []Action[T], canZero bool) (duration time.Duration, err error) {
	var hasSmallDuration bool
	for _, v := range actions {
		switch {
		case !canZero && v.Duration == 0:
			return 0, errZeroDuration
		case v.Duration < 0:
			return 0, errors.New("negative action duration")
		case v.Duration < time.Millisecond:
			hasSmallDuration = true
		}
		duration += v.Duration
	}
	if hasSmallDuration {
		err = ErrSmallDuration
	}
	return duration, err
}

func currentIdx[T any](actions []Action[T], elapsed time.Duration) (int, time.Duration) {
	var endOfAction time.Duration = 0
	for i, action := range actions {
		endOfAction += action.Duration
		if elapsed < endOfAction {
			return i, endOfAction - elapsed
		}
	}
	return -1, 0
}
