package quickjs

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// https://github.com/robertkrimen/natto/blob/5296a7476556988e191b1f0f739bd2e001273d56/natto.go
// https://github.com/dop251/goja_nodejs/blob/master/eventloop/eventloop_test.go

type Job func()

type Loop struct {
	jobChan chan Job
}

func NewLoop() *Loop {
	return &Loop{
		jobChan: make(chan Job),
	}
}

// AddJob adds a job to the loop.
func (l *Loop) ScheduleJob(j Job) error {
	l.jobChan <- j
	return nil
}

// AddJob adds a job to the loop.
func (l *Loop) IsLoopPending() bool {
	return len(l.jobChan) > 0
}

var loopLock sync.Mutex

// run executes all pending jobs.
func (l *Loop) Run() error {
	for {
		select {
		case job, ok := <-l.jobChan:
			if !ok {
				break
			}
			job()
		default:
			// Escape valve!
			// If this isn't here, we deadlock...
		}

		if len(l.jobChan) == 0 {
			break
		}
	}
	return nil
}

// stop stops the loop.
func (l *Loop) Stop() error {
	close(l.jobChan)
	return nil
}

func Goid() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}
