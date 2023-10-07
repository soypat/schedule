# schedule
[![go.dev reference](https://pkg.go.dev/badge/github.com/soypat/schedule)](https://pkg.go.dev/github.com/soypat/schedule)
[![Go Report Card](https://goreportcard.com/badge/github.com/soypat/schedule)](https://goreportcard.com/report/github.com/soypat/schedule)
[![codecov](https://codecov.io/gh/soypat/schedule/branch/main/graph/badge.svg)](https://codecov.io/gh/soypat/schedule)
[![Go](https://github.com/soypat/schedule/actions/workflows/go.yml/badge.svg)](https://github.com/soypat/schedule/actions/workflows/go.yml)
[![sourcegraph](https://sourcegraph.com/github.com/soypat/schedule/-/badge.svg)](https://sourcegraph.com/github.com/soypat/schedule?badge)


Action scheduling using event loops.

The basic building unit of schedules is the `Group` interface.


```go
type Group interface {
	// Begins sets the start time of the group. It must be called before ScheduleNext.
	// It should reset internal state of the Group so that Group can be reused.
	Begins(time.Time)
	// ScheduleNext returns the next action when `ok` is true 
	// and returns the action value v. 
	// When ok=false and next=0 the Group is done.
	ScheduleNext(time.Time) (v any, ok bool, next time.Duration, err error)
}
```

A more complete interface could be:
```go
type GroupFull interface {
	Group
	// Start time returns the time the group was started at.
	StartTime() time.Time
	// Durations returns how long a single iteration lasts.
	Duration() time.Duration
	// Amount of times group will run. -1 for infinite iterations.
	Iterations() int
}
```

## Example
The example below demonstrates a group scheduled to add values to
sum over the course of 1.5 seconds.

```go
	// g is a Group with an integer value type.
	var sum int
	const resolution = time.Second/6
	start := time.Now()
	g.Begins(start)
	for {
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
		time.Sleep(resolution)
	}
	fmt.Println("done!", time.Since(start))
```

Outputs:
```
added 20 to sum 20
added 30 to sum 50
added 50 to sum 100
done! 1.5s
```