# go-module-template
[![go.dev reference](https://pkg.go.dev/badge/github.com/soypat/schedule)](https://pkg.go.dev/github.com/soypat/schedule)
[![Go Report Card](https://goreportcard.com/badge/github.com/soypat/schedule)](https://goreportcard.com/report/github.com/soypat/schedule)
[![codecov](https://codecov.io/gh/soypat/schedule/branch/main/graph/badge.svg)](https://codecov.io/gh/soypat/schedule)
[![Go](https://github.com/soypat/schedule/actions/workflows/go.yml/badge.svg)](https://github.com/soypat/schedule/actions/workflows/go.yml)
[![sourcegraph](https://sourcegraph.com/github.com/soypat/schedule/-/badge.svg)](https://sourcegraph.com/github.com/soypat/schedule?badge)
[![stability-experimental](https://img.shields.io/badge/stability-experimental-orange.svg)](https://github.com/emersion/stability-badges#experimental)



Action scheduling using event loops.

The basic building unit of schedules is the `Group` interface.


```go
type Group interface {
    // Begin sets the Group's starting time.
	Begin(time.Time)
    // ScheduleNext returns the next action when `ok` is true 
    // and returns the action value v. 
    // When ok=false and next=0 the Group is done.
	ScheduleNext(time.Time) (v any, ok bool, next time.Duration, err error)
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
    g.Begin(start)
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
	fmt.Println("done!", time.Since(start))
```

Outputs:
```
added 20 to sum 20
added 30 to sum 50
added 50 to sum 100
done! 1.5s
```