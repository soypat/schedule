package schedule

import (
	"errors"
	"time"
)

type GroupLooseConfig struct {
	// Restart specifies if after the last action has been run the group should
	// continue with the first action, effectively running forever.
	Restart bool
}

// NewGroupLoose returns a newly initialized loose timing group.
func NewGroupLoose[T any](actions []Action[T], cfg GroupLooseConfig) (*GroupLoose[T], error) {
	if len(actions) == 0 {
		return nil, errors.New("empty actions")
	}
	duration, err := actionsDuration(actions, true)
	if err != nil && !errors.Is(err, ErrSmallDuration) {
		return nil, err
	}
	g := &GroupLoose[T]{
		actions:      actions,
		duration:     duration,
		restartOnEnd: cfg.Restart,
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
	restartOnEnd    bool
}

// Begin starts or restarts the group timer. For GroupLoose its call before
// CheckNext is not required but is useful for synchronizing the StartTime
// between other groups.
func (g *GroupLoose[T]) Begin(start time.Time) {
	g.start = start
	g.lastActionStart = time.Time{}
	g.lastIdx = -1
}

// StartTime time returns the time the group was Started at. If not started returns zero value.
func (g *GroupLoose[T]) StartTime() time.Time {
	return g.start
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
		panic("CheckNext called before Begin")
	}
	if g.lastIdx == -1 {
		// Special case for first action.
		g.lastActionStart = now
		g.lastIdx = 0
		return g.actions[0].Value, true, g.actions[0].Duration, nil
	}
	actionElapsed := now.Sub(g.lastActionStart)
	currAction := g.actions[g.lastIdx]
	nextIdx := g.lastIdx + 1
	if !g.restartOnEnd && nextIdx == len(g.actions) {
		// We're at the last action.
		remaining := currAction.Duration - actionElapsed
		if remaining > 0 {
			return v, false, remaining, nil
		}
		return v, false, 0, nil // Done.
	} else if g.restartOnEnd && nextIdx == len(g.actions) {
		nextIdx = 0 // Restart actions from first.
	}

	if actionElapsed < currAction.Duration {
		next = currAction.Duration - actionElapsed
		return v, false, next, nil // Still waiting for next action
	}
	g.lastIdx = nextIdx
	g.lastActionStart = now

	// We return the full time of the action duration when we start it since we
	// guarantee each action will take at least it's duration to complete.
	// This is the same guarantee that time.Sleep provides with regards to the sleep duration.
	return g.actions[nextIdx].Value, true, g.actions[nextIdx].Duration, nil
}
