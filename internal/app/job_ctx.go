package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

type (
	JobCtx struct {
		// +checklocks:mu
		innerMapCtx map[string]*Job
		mu          *sync.RWMutex
		wg          *sync.WaitGroup

		timeout time.Duration

		tjDoneC chan string
		doneC   chan struct{}

		// +checkatomic
		closed   int32
		stopOnce *sync.Once

		ctx context.Context
	}

	Job struct {
		ctx        context.Context
		cancelFunc context.CancelFunc
		timeout    time.Duration

		jobID string
		cb    JobCb
		jc    *JobCtx
		start time.Time
		end   time.Time

		// +checklocks:mu
		state JobState

		mu sync.Mutex

		sc     chan struct{}
		scOnce sync.Once

		ec     chan error
		ecOnce sync.Once

		// +checkatomic
		closed int32
	}

	// JobCb should follow the context pattern and respond to the cancellation signal
	JobCb func(ctx context.Context) error
)

var (
	jc *JobCtx

	jcOnce sync.Once

	jcMutex sync.RWMutex
)

// NewJobCtx give a new JobCtx
func NewJobCtx(timeout time.Duration) *JobCtx {
	jcOnce.Do(func() {
		ReloadJobCtx(timeout)
	})

	return jc
}

func ReloadJobCtx(timeout time.Duration) *JobCtx {
	jcMutex.Lock()
	defer jcMutex.Unlock()
	jc = &JobCtx{
		innerMapCtx: make(map[string]*Job),
		mu:          &sync.RWMutex{},
		wg:          &sync.WaitGroup{},
		timeout:     timeout,

		tjDoneC: make(chan string),
		doneC:   make(chan struct{}),

		stopOnce: &sync.Once{},
		ctx:      context.TODO(),

		closed: 0,
	}

	go jc.scavenge()

	return jc
}

// RegisterJob register a new job to be future processed (async)
func (jc *JobCtx) RegisterJob(ctx0 context.Context, jobID string, jf JobCb) (ctx context.Context, cancelFunc context.CancelFunc, successChan chan struct{}, errorC chan error) {
	return jc.RegisterJobWithTimeout(ctx0, jobID, jc.timeout, jf)
}

// RegisterJobWithTimeout register a job that should be done at given timeout, if not done, then cancel
func (jc *JobCtx) RegisterJobWithTimeout(ctx0 context.Context, jobID string, timeout time.Duration, cb JobCb) (ctx context.Context, cancelFunc context.CancelFunc, successC chan struct{}, errorC chan error) {
	ctx, cancelFunc, successC = jc.GetJobCtx(jobID)
	if ctx != nil {
		return
	}

	ctx, cancelFunc0 := context.WithTimeout(ctx0, timeout)
	successC = make(chan struct{})
	errorC = make(chan error)
	j := &Job{
		jc:    jc,
		jobID: jobID,
		mu:    sync.Mutex{},

		cb: cb,

		ctx:        ctx,
		cancelFunc: cancelFunc0,
		timeout:    timeout,
		state:      PENDING,

		start:  time.Now(),
		sc:     successC,
		ec:     errorC,
		closed: 0,
	}

	cancelFunc = j.close

	jc.mu.Lock()
	jc.innerMapCtx[jobID] = j
	jc.mu.Unlock()

	jc.wg.Add(1)
	go func() {
		defer func() {
			j.jc.tjDoneC <- j.jobID
			j.close()

			j.jc.wg.Done()
		}()

		err := j.block()
		if atomic.LoadInt32(&j.closed) != 1 {
			if err != nil {
				logrus.Errorf("job [ %s ] exit with error: [ %s ]", j.jobID, err.Error())
				j.ec <- err
			} else {
				j.sc <- struct{}{}
			}
		}
	}()

	return ctx, cancelFunc, successC, errorC
}

// GetJobCtx get the jobCtx if exists, otherwise return nil
func (jc *JobCtx) GetJobCtx(jobID string) (ctx context.Context, cancelFunc context.CancelFunc, successC chan struct{}) {
	jc.mu.Lock()
	tj := jc.innerMapCtx[jobID]
	jc.mu.Unlock()

	if tj == nil {
		return nil, nil, nil
	}
	tj.mu.Lock()
	ctx = tj.ctx
	cancelFunc = tj.cancelFunc
	successC = tj.sc
	tj.mu.Unlock()

	return
}

// DeRegister will de-register the running job or pending job, and cancel their context
func (jc *JobCtx) DeRegister(jobID string) bool {
	jc.mu.RLock()
	tj := jc.innerMapCtx[jobID]
	if tj == nil {
		jc.mu.RUnlock()

		return false
	}
	jc.mu.RUnlock()

	logrus.Infof("de-register job: %s", jobID)
	tj.close()

	jc.mu.Lock()
	delete(jc.innerMapCtx, jobID)
	jc.mu.Unlock()

	return true
}

// scavenge scan the already registered job, and if find any job done, delete it from the ctx map
func (jc *JobCtx) scavenge() {
	for {
		select {
		case app, ok := <-jc.tjDoneC:
			if ok {
				jc.mu.Lock()
				delete(jc.innerMapCtx, app)
				jc.mu.Unlock()

				continue
			}

			return
		case <-jc.doneC:

			return
		}
	}
}

// Stop will stop all the possible running job, interupt it if possible
func (jc *JobCtx) Stop() {
	logrus.Infof("Closing JobCtx")
	if !atomic.CompareAndSwapInt32(&jc.closed, 0, 1) {
		return
	}

	jc.stopOnce.Do(func() {
		jc.mu.Lock()
		for _, v := range jc.innerMapCtx {
			v.close()
		}
		jc.mu.Unlock()

		jc.wg.Wait()
		close(jc.tjDoneC)
		close(jc.doneC)
	})

	logrus.Warnf("Successfully close JobCtx ....")
}

func (jc *JobCtx) Drain() {
	jc.wg.Wait()
}

func (jc *JobCtx) IsClosed() bool {
	return atomic.LoadInt32(&jc.closed) == 1
}

func (j *Job) block() (err error) {
	defer func() {
		j.mu.Lock()
		if err == nil {
			j.state = SUCCESS
		} else {
			j.state = FAILED
		}
		j.mu.Unlock()

		j.end = time.Now()
	}()

	err = j.cb(j.ctx)

	return
}

// Done wrap a context.Done() channel to be blocked
func (j *Job) Done() <-chan struct{} {
	return j.ctx.Done()
}

// Deadline return a deadline of a job
func (j *Job) Deadline() (deadline time.Time, ok bool) {
	return j.ctx.Deadline()
}

// Error return the possible errorz of a job
func (j *Job) Error() error {
	if errors.Is(j.ctx.Err(), context.Canceled) {
		return nil
	}

	return j.ctx.Err()
}

// close cancel the running job
func (j *Job) close() {
	defer func() {
		j.scOnce.Do(func() {
			close(j.sc)
		})
		j.ecOnce.Do(func() {
			close(j.ec)
		})

		atomic.StoreInt32(&j.closed, 1)
	}()

	select {
	case <-j.ctx.Done(): // already canceled
		return
	default:
	}
	j.cancelFunc()
}
