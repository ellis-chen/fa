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
	// TransferJobCtx ...
	TransferJobCtx struct {
		Prefix string
		// +checklocks:mu
		innerMapCtx map[string]*transferJob
		mu          *sync.RWMutex
		wg          *sync.WaitGroup

		timeout time.Duration

		tjDoneC chan string
		doneC   chan struct{}

		// +checkatomic
		closed   int32
		stopOnce *sync.Once
	}

	transferJob struct {
		ctx        context.Context
		cancelFunc context.CancelFunc
		timeout    time.Duration

		appName string
		jf      JobFunc
		atjc    *TransferJobCtx
		start   time.Time
		end     time.Time

		// +checklocks:mu
		state JobState

		mu sync.Mutex
	}

	// JobFunc stands for a app transfer process
	JobFunc func(ctx context.Context, data interface{})

	// JobState status of a Job
	JobState string
)

var (
	// +checklocks:atjcMutex
	atjc *TransferJobCtx

	atjcOnce sync.Once

	atjcMutex sync.RWMutex
)

const (
	// PENDING a job is to be run
	PENDING JobState = "pending"
	// RUNNING a job is processing
	RUNNING JobState = "running"
	// SUCCESS job done
	SUCCESS JobState = "success"
	// FAILED job done
	FAILED JobState = "failed"
)

// NewAppTransferJobCtx give a new TransferJobCtx
func NewAppTransferJobCtx(timeout time.Duration) *TransferJobCtx {
	atjcOnce.Do(func() {
		ReloadAppAppTransferJobCtx(timeout)
	})

	atjcMutex.RLock()
	defer atjcMutex.RUnlock()

	return atjc
}

func ReloadAppAppTransferJobCtx(timeout time.Duration) *TransferJobCtx {
	atjcMutex.Lock()
	defer atjcMutex.Unlock()
	ctx := newAppTransferJobCtx(timeout)
	atjc = &ctx

	return &ctx
}

func newAppTransferJobCtx(timeout time.Duration) TransferJobCtx {
	atjc := TransferJobCtx{
		Prefix:      "/opt/ellis-chen/softwares/",
		innerMapCtx: make(map[string]*transferJob),
		mu:          &sync.RWMutex{},
		wg:          &sync.WaitGroup{},
		timeout:     timeout,

		tjDoneC: make(chan string),
		doneC:   make(chan struct{}),

		stopOnce: &sync.Once{},
	}

	go atjc.scavenge()

	return atjc
}

// RegisterJob register a new job to be future processed (async)
func (atjc *TransferJobCtx) RegisterJob(appName string, jf JobFunc) (ctx context.Context, cancelFunc context.CancelFunc, err error) {
	return atjc.RegisterJobWithTimeout(appName, atjc.timeout, jf)
}

// RegisterJobWithTimeout register a job that should be done at given timeout, if not done, then cancel
func (atjc *TransferJobCtx) RegisterJobWithTimeout(appName string, timeout time.Duration, jf JobFunc) (ctx context.Context, cancelFunc context.CancelFunc, err error) {
	ctx, cancelFunc = atjc.GetJobCtx(appName)
	if ctx != nil {
		return
	}

	ctx, cancelFunc = context.WithTimeout(context.TODO(), timeout)

	tj := &transferJob{
		atjc:    atjc,
		appName: appName,
		mu:      sync.Mutex{},

		jf: jf,

		ctx:        ctx,
		cancelFunc: cancelFunc,
		timeout:    timeout,
		state:      PENDING,

		start: time.Now(),
	}

	atjc.mu.Lock()
	atjc.innerMapCtx[appName] = tj
	atjc.mu.Unlock()

	go func() {
		err := tj.block()
		if err != nil {
			logrus.Warnf("job exit with error: [ %s ]", err.Error())
		}
	}()

	return ctx, cancelFunc, nil
}

// GetJobCtx get the jobCtx if exists, otherwise return nil
func (atjc *TransferJobCtx) GetJobCtx(appName string) (ctx context.Context, cancelFunc context.CancelFunc) {
	atjc.mu.Lock()
	tj := atjc.innerMapCtx[appName]
	atjc.mu.Unlock()

	if tj == nil {
		return nil, nil
	}
	tj.mu.Lock()
	ctx = tj.ctx
	cancelFunc = tj.cancelFunc
	tj.mu.Unlock()

	return
}

// DeRegister will de-register the running job or pending job, and cancel their context
func (atjc *TransferJobCtx) DeRegister(appName string) bool {
	atjc.mu.RLock()
	tj := atjc.innerMapCtx[appName]
	if tj == nil {
		atjc.mu.RUnlock()

		return false
	}
	atjc.mu.RUnlock()

	tj.Close()

	atjc.mu.Lock()
	delete(atjc.innerMapCtx, appName)
	atjc.mu.Unlock()

	return true
}

// scavenge scan the already registered job, and if find any job done, delete it from the ctx map
func (atjc *TransferJobCtx) scavenge() {
	for {
		select {
		case app, ok := <-atjc.tjDoneC:
			if ok {
				atjc.mu.Lock()
				delete(atjc.innerMapCtx, app)
				atjc.mu.Unlock()

				continue
			}

			return
		case <-atjc.doneC:
			return
		}
	}
}

// Stop will stop all the possible running job, interupt it if possible
func (atjc *TransferJobCtx) Stop() {
	if atomic.LoadInt32(&atjc.closed) == 1 {
		return
	}

	atjc.stopOnce.Do(func() {
		atjc.mu.Lock()
		for _, v := range atjc.innerMapCtx {
			v.Close()
		}
		atjc.mu.Unlock()

		atjc.wg.Wait()
		close(atjc.tjDoneC)
		close(atjc.doneC)
	})

	atomic.StoreInt32(&atjc.closed, 1)

	logrus.Warnf("Successfully close TransferJobCtx  .")
}

func (tj *transferJob) block() (err error) {
	tj.atjc.wg.Add(1)
	go func() {
		defer tj.cancelFunc()
		tj.jf(tj.ctx, nil)
	}()

	defer func() {
		tj.mu.Lock()
		tj.state = SUCCESS
		tj.mu.Unlock()

		tj.end = time.Now()

		tj.atjc.tjDoneC <- tj.appName
		tj.atjc.wg.Done()
	}()

	<-tj.Done()
	if errors.Is(tj.ctx.Err(), context.Canceled) {
		return nil
	}

	return tj.ctx.Err()
}

// Done wrap a context.Done() channel to be blocked
func (tj *transferJob) Done() <-chan struct{} {
	return tj.ctx.Done()
}

// Deadline return a deadline of a job
func (tj *transferJob) Deadline() (deadline time.Time, ok bool) {
	return tj.ctx.Deadline()
}

// Error return the possible errorz of a job
func (tj *transferJob) Error() error {
	return tj.ctx.Err()
}

// Close cancel the running job
func (tj *transferJob) Close() {
	tj.cancelFunc()
}
