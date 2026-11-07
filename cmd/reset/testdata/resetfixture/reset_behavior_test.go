package resetfixture

import (
	"testing"
	"time"
)

func TestSampleReset(t *testing.T) {
	str := "value"
	num := 42

	s := &Sample{
		i:      7,
		str:    "hello",
		b:      true,
		f:      1.5,
		strP:   &str,
		intP:   &num,
		s:      []int{1, 2, 3},
		strs:   []string{"a", "b"},
		m:      map[string]string{"k": "v"},
		child:  &Child{N: 9},
		kid:    Child{N: 3},
		dur:    5 * time.Second,
		ch:     make(chan int),
		fn:     func() {},
		iface:  "x",
		nested: struct{ X int }{X: 1},
		arr:    [3]int{1, 2, 3},
	}

	s.Reset()

	if s.i != 0 || s.f != 0 {
		t.Errorf("numeric fields not zeroed: i=%v f=%v", s.i, s.f)
	}
	if s.str != "" || s.b {
		t.Errorf("string/bool not zeroed: str=%q b=%v", s.str, s.b)
	}
	if *s.strP != "" || *s.intP != 0 {
		t.Errorf("pointer targets not zeroed: strP=%q intP=%v", *s.strP, *s.intP)
	}
	if len(s.s) != 0 || cap(s.s) != 3 {
		t.Errorf("slice must be truncated to zero length keeping capacity: len=%d cap=%d", len(s.s), cap(s.s))
	}
	if len(s.strs) != 0 {
		t.Errorf("string slice not truncated: %v", s.strs)
	}
	if len(s.m) != 0 {
		t.Errorf("map not cleared: %v", s.m)
	}
	if s.child.N != 0 || s.kid.N != 0 {
		t.Errorf("children not reset: child=%+v kid=%+v", s.child, s.kid)
	}
	if s.dur != 0 || s.ch != nil || s.fn != nil || s.iface != nil {
		t.Errorf("duration/chan/func/any not zeroed: dur=%v ch=%v fnIsNil=%v iface=%v", s.dur, s.ch, s.fn == nil, s.iface)
	}
	if s.nested.X != 0 || s.arr != [3]int{} {
		t.Errorf("nested struct and array not zeroed: nested=%+v arr=%v", s.nested, s.arr)
	}
}

func TestNodeReset(t *testing.T) {
	n := &Node{Value: 1, Next: &Node{Value: 2, Next: &Node{Value: 3}}}

	n.Reset()

	if n.Value != 0 || n.Next.Value != 0 || n.Next.Next.Value != 0 {
		t.Errorf("recursive reset failed: %d %d %d", n.Value, n.Next.Value, n.Next.Next.Value)
	}
}

func TestNilReceiverSafe(t *testing.T) {
	var s *Sample
	s.Reset()

	var n *Node
	n.Reset()
}
