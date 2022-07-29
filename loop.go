package quickjs

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
