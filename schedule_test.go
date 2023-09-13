package schedule_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/soypat/schedule"
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/rand"
	"golang.org/x/exp/slices"
)

type actionInt = schedule.Action[int]

type GroupInt interface {
	Begin(time.Time)
	// Expect v to be zero only
	ScheduleNext(time.Time) (v int, ok bool, next time.Duration, err error)
	Duration() time.Duration
	StartTime() time.Time
}

func ExampleGroup() {
	type addAction = schedule.Action[int]
	actions := []addAction{
		{Duration: time.Second / 2, Value: 20},
		{Duration: time.Second / 2, Value: 30},
		{Duration: time.Second / 2, Value: 50},
	}

	g, err := schedule.NewGroupSync(actions, schedule.GroupSyncConfig{})
	if err != nil {
		panic(err)
	}

	fmt.Println("total runtime:", g.Duration())

	const resolution = time.Second / 4
	var sum int
	g.Begin(time.Now())
	for range time.NewTicker(resolution).C {
		v, ok, next, err := g.ScheduleNext(time.Now())
		if err != nil {
			panic(err)
		}

		done := !ok && next == 0
		if done {
			break
		} else if !ok {
			continue
		}
		sum += v
		fmt.Println("added", v, "to sum", sum)
	}
	fmt.Println("done!")
	//Output:
	// total runtime: 1.5s
	// added 20 to sum 20
	// added 30 to sum 50
	// added 50 to sum 100
	// done!
}

// TestGroupCommon tests functionality common across all Group* types.
func TestGroupCommon(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	restart := false
	const maxN = 100
	actionsCp := make([]actionInt, maxN)
	for n := 1; n < maxN; n++ {
		for maxD := time.Duration(2); maxD < 4; maxD++ {
			for minD := time.Duration(1); minD <= maxD; minD++ {
				actions, _ := randomIntActions(rng, minD, maxD, n)
				copy(actionsCp, actions)
				gs, err := schedule.NewGroupSync(actions, schedule.GroupSyncConfig{Restart: restart})
				if err != nil && !errors.Is(err, schedule.ErrSmallDuration) {
					t.Fatal(err)
				}
				testGroupCommon(t, gs, actions, false)
				if !slices.Equal(actions, actionsCp[:n]) {
					t.Error("unexpected modification to actions slice from GroupSync implementation", actions, actionsCp[:n])
				}

				gl, err := schedule.NewGroupLoose(actions, schedule.GroupLooseConfig{Restart: restart})
				if err != nil && !errors.Is(err, schedule.ErrSmallDuration) {
					t.Fatal(err)
				}
				testGroupCommon(t, gl, actions, false)
				if !slices.Equal(actions, actionsCp[:n]) {
					t.Error("unexpected modification to actions slice from GroupLoose implementation", actions, actionsCp[:n])
				}

				if t.Failed() {
					t.FailNow()
				}
			}
		}
	}
}

func testGroupCommon(t *testing.T, g GroupInt, actions []actionInt, restart bool) {
	n := len(actions)
	if n == 0 {
		panic("nil or 0 length group")
	}
	if restart {
		panic("unsupported as of yet")
	}
	var groupDuration time.Duration
	for _, action := range actions {
		dur := action.Duration
		if dur < 0 {
			panic("negative duration")
		}
		groupDuration += dur
	}
	if groupDuration != g.Duration() {
		t.Errorf("bad duration calc got %d, wanted %d", g.Duration(), groupDuration)
	}
	var start time.Time
	start = start.Add(1)
	g.Begin(start) // Setup group.
	if got := g.StartTime(); !got.Equal(start) {
		t.Error("bad StartTime result", got, "expected", start)
	}

	// Handle first object entry as a special case before main loop.
	v, ok, next, err := g.ScheduleNext(start)
	if err != nil {
		t.Fatal("error on first schedule:", err)
	}
	if v != actions[0].Value {
		t.Error("first action value mismatch")
	}
	if !ok {
		t.Error("expected first action to yield ok=true")
	}
	if next != actions[0].Duration {
		t.Error("expected the time until second action to be first actions duration", next, actions[0].Duration, actions[0].Duration)
	}

	// Main loop.
	now := start
	var elapsed time.Duration
	currentActionIdx := 0
	elapsedToNext := actions[0].Duration
	for ; elapsed <= groupDuration; elapsed++ {
		now = start.Add(elapsed)
		v, ok, next, err := g.ScheduleNext(now)
		if err != nil {
			t.Fatal("got error during scheduling:", err)
		}
		if got := g.StartTime(); !got.Equal(start) {
			t.Error("bad StartTime result", got, "want", start)
		}
		done := !ok && next == 0
		wantDone := elapsed == groupDuration
		if done != wantDone {
			t.Error("unexpected value of done", done, "wanted", wantDone)
		}
		if !ok && v != 0 {
			t.Error("!ok action returned non-zero Value", v, "of max", n)
		}
		if done {
			break
		}
		if !ok {
			wantNext := elapsedToNext - elapsed
			if next != wantNext {
				t.Error("unexpected `next` for !ok", next, "wanted", wantNext)
			}
			continue
		}
		// Gotten to this point we scheduled an action.
		wantValue := currentActionIdx + 2
		if v != wantValue {
			t.Error("unexpected value", v, "wanted", wantValue)
		}
		currentActionIdx++
		elapsedToNext += actions[currentActionIdx].Duration
		wantNext := elapsedToNext - elapsed
		if next != wantNext {
			t.Error("unexpected `next`", next, "wanted", wantNext)
		}
	}

	// By now the group is done.
	// We can test that future calls to ScheduleNext still return "done"
	for i := 0; i < 10; i++ {
		v, ok, next, err := g.ScheduleNext(now)
		if err != nil {
			t.Fatal(i, "should not error after end:", err)
		}
		if v != 0 {
			t.Error(i, "got non-zero value after end", v, "of max", n)
		}
		if ok {
			t.Error(i, "wanted ok=false after end")
		}
		if next != 0 {
			t.Error(i, "wanted next=0 after end", next)
		}
	}
}

// returns actions with ordered values 1..n and random durations from minD to maxD.
// The second parameter returned is the total duration of the actions.
func randomIntActions(rng *rand.Rand, minD, maxD time.Duration, n int) ([]schedule.Action[int], time.Duration) {
	switch {
	case n <= 0:
		panic("bad length")
	case minD > maxD || minD < 0:
		panic("bad duration range")
	}

	v := make([]schedule.Action[int], n)
	var sum time.Duration
	rangeD := int(maxD-minD) + 1
	for i := range v {
		dur := time.Duration(rng.Intn(rangeD)) + minD
		v[i].Duration = max(dur, minD)
		v[i].Value = i + 1
		sum += dur
	}
	return v, sum
}

func max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func min[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}
