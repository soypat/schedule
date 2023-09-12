package schedule_test

import (
	"fmt"
	"time"

	"github.com/soypat/schedule"
)

func ExampleGroup() {
	type addAction = schedule.Action[int]
	actions := []addAction{
		{Duration: time.Second / 2, Value: 20},
		{Duration: time.Second / 2, Value: 30},
		{Duration: time.Second / 2, Value: 50},
	}

	g, err := schedule.NewGroup(actions, schedule.GroupConfig{})
	if err != nil {
		panic(err)
	}

	fmt.Println("total runtime:", g.Runtime())

	const resolution = time.Second / 4
	var sum int
	g.Begin()
	for range time.NewTicker(resolution).C {
		v, ok, next, err := g.Update()
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
