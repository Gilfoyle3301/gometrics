package pool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type buffer struct {
	data []byte
	n    int
}

func (b *buffer) Reset() {
	b.data = b.data[:0]
	b.n = 0
}

func TestGetCreatesObjectViaFactory(t *testing.T) {
	created := 0
	p := New(func() *buffer {
		created++
		return &buffer{data: make([]byte, 0, 16)}
	})

	b := p.Get()

	require.NotNil(t, b)
	assert.Equal(t, 1, created)
	assert.Equal(t, 16, cap(b.data))
	assert.Zero(t, len(b.data))
}

func TestPutResetsStateAndObjectIsReused(t *testing.T) {
	created := 0
	p := New(func() *buffer {
		created++
		return &buffer{data: make([]byte, 0, 16)}
	})

	b := p.Get()
	b.data = append(b.data, "hello"...)
	b.n = 42

	p.Put(b)

	reused := p.Get()

	assert.Equal(t, 1, created, "object must be reused, not recreated")
	assert.Same(t, b, reused)
	assert.Zero(t, len(reused.data), "Put must reset the state before storing")
	assert.Zero(t, reused.n)
	assert.Equal(t, 16, cap(reused.data), "capacity must be preserved")
}

func TestConcurrentGetPut(t *testing.T) {
	p := New(func() *buffer {
		return &buffer{data: make([]byte, 0, 8)}
	})

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			b := p.Get()
			b.data = append(b.data, 'x')
			b.n++
			p.Put(b)
		}()
	}
	wg.Wait()
}
