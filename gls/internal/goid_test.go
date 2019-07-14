package internal

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"
)

type Int64Slice []int64

func (s Int64Slice) Len() int {
	return len(s)
}

func (s Int64Slice) Less(i, j int) bool {
	return s[i] < s[j]
}

func (s Int64Slice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func TestGoroutineIdConsistency(t *testing.T) {
	cnt := 100
	exit := make(chan error)
	goids := make(Int64Slice, cnt)

	for i := 0; i < cnt; i++ {
		go func(n int) {
			id1 := int64(uintptr(Getg()))
			time.Sleep(time.Duration(rand.Int63n(100)) * time.Millisecond)
			id2 := int64(uintptr(Getg()))

			if id1 != id2 {
				exit <- fmt.Errorf("Inconsistent goroutine id. [old:%v] [new:%v]", id1, id2)
				return
			}

			goids[n] = id1
			exit <- nil
		}(i)
	}

	failed := false

	for i := 0; i < cnt; i++ {
		err := <-exit

		if err != nil {
			t.Logf("Found error. [err:%v]", err)
			failed = true
		}
	}

	if failed {
		t.Fatalf("Test failed.")
	}

	// Goid must be unique.
	t.Logf("Goid list: %v", goids)
	sort.Sort(goids)

	for i := 1; i < len(goids); i++ {
		if goids[i-1] == goids[i] {
			t.Fatalf("Found duplicated goid. [goid:%v]", goids[i])
		}
	}
}
