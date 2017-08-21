package utils_test

import (
	"context"
	"sync"
	"testing"
	"time"

	. "github.com/ShevaXu/web-utils"
	"github.com/ShevaXu/web-utils/assert"
)

func TestNewSemaphore(t *testing.T) {
	assert := assert.NewAssert(t)
	const n = 5

	sema := NewSemaphore(n)
	assert.Equal(0, sema.Count(), "No one queued")
	assert.Equal(n, sema.Capacity(), "Full cap")

	assert.True(!sema.Closed(), "It is not closed")
	sema.Close()
	assert.True(sema.Closed(), "It is closed now")
}

func TestSemaphore_ObtainRelease(t *testing.T) {
	assert := assert.NewAssert(t)
	const n = 2 // easier to test

	sema := NewSemaphore(n)

	assert.True(!sema.Release(), "Release on full returns false immediately")

	// obtain
	bc := context.Background()
	assert.True(sema.Obtain(bc), "Obtained immediately")
	assert.Equal(1, sema.Count(), "Now has one queued")

	sema.Obtain(bc) // obtain one more
	ctx, _ := context.WithTimeout(bc, 10*time.Millisecond)
	assert.True(!sema.Obtain(ctx), "Should fail")

	assert.Equal(sema.Capacity(), sema.Count(), "Now it is full")

	// release
	assert.True(sema.Release(), "Release succeed")
	assert.Equal(1, sema.Count(), "Now has one queued again")
	assert.True(sema.Obtain(bc), "Can obtain again")

	sema.Close()
	assert.True(!sema.Obtain(bc), "Should fail immediately when closed")
}

func TestSemaphore_Sync(t *testing.T) {
	// TODO: better cases?
	assert := assert.NewAssert(t)
	const n = 2 // easier to test
	const m = 5 // workers
	wg := sync.WaitGroup{}
	ctx := context.Background()

	sema := NewSemaphore(n)
	for i := 0; i < m; i++ {
		go func() {
			wg.Add(1)
			sema.Obtain(ctx)
			wg.Done()
		}()
	}

	time.Sleep(10 * time.Millisecond)
	assert.Equal(n, sema.Count(), "Full and overflowed")

	for i := 0; i < m-n; i++ {
		sema.Release()
	}

	wg.Wait()
	assert.Equal(n, sema.Count(), "Still full but buffered")
}
