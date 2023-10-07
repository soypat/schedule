package schedule

import (
	"errors"
	"time"
)

type GroupLooseConfig struct {
	// Iterations specifies how many times to run the group. Must be greater than zero
	// or -1 to indicate infinite iterations.
	Iterations int
}

// NewGroupLoose returns a newly initialized loose timing group.
func NewGroupLoose[T any](actions []Action[T], cfg GroupLooseConfig) (*GroupLoose[T], error) {
	duration, err := actionsDuration(actions, true)
	switch {
	case err != nil && !errors.Is(err, ErrSmallDuration):
		return nil, err
	case len(actions) == 0:
		return nil, errors.New("empty actions")
	case cfg.Iterations <= 0 && cfg.Iterations != -1:
		return nil, errBadIterations
	}

	g := &GroupLoose[T]{
		actions:    actions,
		duration:   duration,
		iterations: cfg.Iterations,
	}
	return g, nil // ignore ErrSmallDuration for loose groups.
}

// GroupLoose specifies a group of actions that should be executed one after another.
// Use GroupLoose when synchonizing between groups is not a priority and when action
// durations may be very small. Some observations on GroupLoose's usage:
//
//   - Each action is guaranteed to run for at least it's duration.
//   - There is no penalty for triggering an action late. GroupLoose will not fail.
type GroupLoose[T any] struct {
	start           time.Time
	lastActionStart time.Time
	duration        time.Duration
	lastIdx         int
	actions         []Action[T]
	iterations      int
}

// Begins sets the start time of the group. It must be called before ScheduleNext.
// It effectively resets internal state of the group.
func (g *GroupLoose[T]) Begins(start time.Time) {
	g.start = start
	g.lastActionStart = time.Time{}
	g.lastIdx = -1
}

// StartTime time returns the time the group was Started at. If not started returns zero value.
func (g *GroupLoose[T]) StartTime() time.Time {
	return g.start
}

// Iterations returns the number of iterations the group will run for.
// It may be -1 for infinite iterations.
func (g *GroupLoose[T]) Iterations() int {
	return g.iterations
}

// Duration returns the time it takes to fully execute all actions in group.
// For GroupLoose it may be zero.
func (g *GroupLoose[T]) Duration() time.Duration {
	return g.duration
}

// ScheduleNext checks `now` against time GroupLoose started and returns
// the next executable action when `ok` is true and `next` duration until next
// ready action.
//
// If ok is false and next is zero the group is done.
func (g *GroupLoose[T]) ScheduleNext(now time.Time) (v T, ok bool, next time.Duration, err error) {
	if g.start.IsZero() {
		return v, false, 0, errBeginNotCalled
	}
	elapsed := now.Sub(g.start)
	if elapsed < 0 {
		return v, false, -elapsed, nil // Still waiting for start time.
	}

	if g.lastIdx == -1 {
		// Special case for first action.
		g.lastActionStart = now
		g.lastIdx = 0
		return g.actions[0].Value, true, g.actions[0].Duration, nil
	}
	actionElapsed := now.Sub(g.lastActionStart)
	safeIdx := g.lastIdx % len(g.actions)
	currAction := g.actions[safeIdx]

	if actionElapsed < currAction.Duration {
		return v, false, currAction.Duration - actionElapsed, nil // Still waiting for next action.
	}
	nextIdx := g.lastIdx + 1
	nextActionEnabled := g.iterations == -1 || nextIdx < len(g.actions)*g.iterations
	if !nextActionEnabled {
		return v, false, 0, nil // Done.
	}
	g.lastIdx++
	g.lastActionStart = now
	safeIdx = g.lastIdx % len(g.actions)
	// We return the full time of the action duration when we start it since we
	// guarantee each action will take at least it's duration to complete.
	// This is the same guarantee that time.Sleep provides with regards to the sleep duration.
	return g.actions[safeIdx].Value, true, g.actions[safeIdx].Duration, nil
}
