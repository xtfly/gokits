package gtime

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

var (
	expfunc = func() {
		fmt.Println("-----------------> run")
	}
)

func TestScheculer(t *testing.T) {
	wheel := NewTimerWheel()

	wheel.AfterFunc(4*time.Second, 1, expfunc)
	if len(wheel.wheel[40].items) == 0 {
		t.Fail()
	}
}

func TestScheculer1(t *testing.T) {
	wheel := NewTimerWheel()
	wheel.AfterFunc(1*time.Minute, 1, expfunc)
	if len(wheel.wheel[88].items) == 0 {
		t.Fail()
	}

}

func TestNotifyTask4s(t *testing.T) {
	wheel := NewTimerWheel()
	wheel.Start()
	wheel.AfterFunc(1*time.Second, 1, expfunc)
	wheel.AfterFunc(2*time.Second, 1, expfunc)
	wheel.AfterFunc(3*time.Second, 1, expfunc)
	wheel.AfterFunc(500*time.Millisecond+3*time.Second, 1, expfunc)
	wheel.AfterFunc(4*time.Minute, 1, expfunc)
	time.Sleep(6 * time.Second)
}

func TestNotifyTask3times(t *testing.T) {
	wheel := NewTimerWheel()
	wheel.Start()
	w := &sync.WaitGroup{}
	w.Add(3)
	wheel.AfterFunc(1*time.Second, 3, func() {
		fmt.Println("-----------------> run")
		w.Done()
	})
	w.Wait()
}

func TestRemove(t *testing.T) {
	wheel := NewTimerWheel()
	wheel.Start()
	tid, _ := wheel.AfterFunc(3*time.Second, 1, expfunc)
	time.Sleep(1 * time.Millisecond)
	wheel.Cancel(tid)
	time.Sleep(3 * time.Millisecond)
}
