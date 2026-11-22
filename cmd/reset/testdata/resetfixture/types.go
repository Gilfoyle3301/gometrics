package resetfixture

import "time"

type Child struct {
	N int
}

func (c *Child) Reset() {
	c.N = 0
}

// generate:reset
type Sample struct {
	i      int
	str    string
	b      bool
	f      float64
	strP   *string
	intP   *int
	s      []int
	strs   []string
	m      map[string]string
	child  *Child
	kid    Child
	dur    time.Duration
	ch     chan int
	fn     func()
	iface  any
	nested struct{ X int }
	arr    [3]int
	_      int
}

// generate:reset
type Node struct {
	Value int
	Next  *Node
}
