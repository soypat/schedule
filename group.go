package schedule

import (
	"errors"
	"time"
)

var (
	errMissedAction  = errors.New("missed action. This happens if event loop Update is not called at enough high frequency to prevent missing an action between calls")
	errGroupFailed   = errors.New("group failed")
	ErrSmallDuration = errors.New("small duration. This may cause missed action errors")
)

type GroupConfig struct {
	// Restart specifies if after the last action has been run the group should
	// continue with the first action, effectively running forever.
	Restart bool
}

// NewGroup returns a newly initialized group.
func NewGroup[T any](actions []Action[T], cfg GroupConfig) (*Group[T], error) {
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

// Begin starts or restarts the group timer. Update should be called soon after Begin
// to acquire first action.
func (g *Group[T]) Begin() {
	g.start = time.Now()
	g.lastIdx = -1
	g.failed = false
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
//
// If ok is false and next is zero the group is done.
func (g *Group[T]) Update() (v T, ok bool, next time.Duration, err error) {
	if g.start.IsZero() {
		panic("Update called before Begin")
	}
	if g.failed {
		return v, false, next, errGroupFailed
	}
	elapsed := time.Since(g.start)
	runtime := g.Runtime()
	if g.restartOnEnd {
		elapsed = elapsed % runtime
	} else if elapsed > runtime && g.lastIdx != len(g.actions)-1 {
		// Easy case of missed last action.
		g.failed = true
		return v, false, next, errMissedAction
	} else if elapsed > runtime {
		// Is done.
		return v, false, next, nil
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
	ok = true
	return g.actions[nextIdx].Value, ok, next, nil

}
