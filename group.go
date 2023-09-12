package schedule

import (
	"errors"
	"math"
	"time"
)

var (
	errMissedAction  = errors.New("missed action. This happens if event loop Update is not called at enough high frequency to prevent missing an action between calls")
	errGroupFailed   = errors.New("group failed")
	ErrSmallDuration = errors.New("small duration. This may cause missed action errors")
)

// NewGroup returns a newly initialized group.
func NewGroup[T any](actions []Action[T]) (*Group[T], error) {
	if len(actions) == 0 {
		return nil, errors.New("empty actions")
	}
	g := &Group[T]{
		actions: actions,
	}
	var hasSmallDuration bool
	for _, v := range actions {
		switch {
		case v.Duration == 0:
			return nil, errors.New("zero action duration")
		case v.Duration < 0:
			return nil, errors.New("negative action duration")
		case v.Duration < time.Millisecond:
			hasSmallDuration = true
		}
		g.runtime += v.Duration
	}
	if hasSmallDuration {
		// This may be intentional and the user may have access to really
		// tight-timed hardware so we still return g.
		return g, ErrSmallDuration
	}

	return g, nil
}

// Group specifies a group of actions that should be executed one after another
// while prioritizing the time between actions and the periodicity of the group.
// This is to say that if the group ran for a long one could calculate how
// many times the group was executed knowing
type Group[T any] struct {
	start        time.Time
	runtime      time.Duration
	lastIdx      int
	actions      []Action[T]
	restartOnEnd bool
	failed       bool
}

type Action[T any] struct {
	Duration time.Duration
	Value    T
}

// Begin starts or restarts the group action and returns first action
// which should be executed immediately.
func (g *Group[T]) Begin() T {
	g.start = time.Now()
	g.lastIdx = math.MinInt
	g.failed = false
	return g.actions[0].Value
}

// Start time returns the time the group was Started at. If not started returns zero value.
func (g *Group[T]) StartTime() time.Time {
	return g.start
}

// Runtime returns the time it takes to fully execute all actions in group.
func (g *Group[T]) Runtime() (sum time.Duration) {
	return g.runtime
}

// Update checks current time against time Group started and returns
// the next executable action when `ok` is true and `next` duration until next
// ready action.
func (g *Group[T]) Update() (v T, ok bool, next time.Duration, err error) {
	if g.failed {
		return v, ok, next, errGroupFailed
	}
	elapsed := time.Since(g.start)
	if g.restartOnEnd {
		elapsed = elapsed % g.Runtime()
	}
	var endOfAction time.Duration = 0
	var nextIdx int
	for i, action := range g.actions {
		endOfAction += action.Duration
		if elapsed < endOfAction {
			nextIdx = i
			break
		}
	}
	next = endOfAction - elapsed
	if nextIdx == g.lastIdx {
		return v, false, next, nil // Still need to execute current action.
	}

	if nextIdx == -1 {
		if g.lastIdx != len(g.actions)-1 {
			g.failed = true
			return v, false, 0, errMissedAction // Too late to execute actions.
		}
		return v, false, 0, nil // No more actions to execute.
	}

	if (!g.restartOnEnd && nextIdx != g.lastIdx+1) ||
		(g.restartOnEnd && nextIdx != (g.lastIdx+1)%len(g.actions)) {
		g.failed = true
		return v, false, 0, errMissedAction // Missed an action
	}

	g.lastIdx = nextIdx
	return g.actions[nextIdx].Value, true, next, nil

}
