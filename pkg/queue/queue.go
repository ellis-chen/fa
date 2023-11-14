package queue

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

const (
	// MaxWorker ...
	MaxWorker = 10
	// MaxQueue ...
	MaxQueue = 200
)

var (
	// jobQueue ...
	jobQueue chan Job

	limiter      *rate.Limiter = rate.NewLimiter(3, 6)
	executorOnce               = sync.Once{}

	// dispatcher ...
	// +checklocks:dispatcherMutex
	dispatcher *Dispatcher

	dispatcherMutex sync.RWMutex
)

// JobFunc ...
type JobFunc func(context.Context, interface{})

// Job ...
type Job struct {
	PayLoad interface{}
	Action  JobFunc
}

// Worker ...
type Worker struct {
	WorkerPool chan chan Job
	JobChannel chan Job
	quit       chan bool
	wg         *sync.WaitGroup
}

// NewWorker ...
func NewWorker(workerPool chan chan Job, wg *sync.WaitGroup) Worker {
	return Worker{
		WorkerPool: workerPool,
		JobChannel: make(chan Job),
		quit:       make(chan bool),
		wg:         wg,
	}
}

// Start ...
func (w Worker) Start() {
	w.wg.Add(1)
	var start time.Time
	var ctx context.Context
	var cancel context.CancelFunc
	go func() {
		for {
			ctx, cancel = context.WithCancel(context.TODO())
			// every worker have a channel to recv job, add the channel to pool so that the dispatcher can pick this worker
			w.WorkerPool <- w.JobChannel
			select {
			case job := <-w.JobChannel:
				start = time.Now()
				go func(ctx context.Context) {
					defer cancel()
					job.Action(ctx, job.PayLoad)
					logrus.Debugf("It cost %.2f sec to finish the job", float64(time.Since(start).Milliseconds())/1e3)
				}(ctx)
				select {
				case <-ctx.Done():
				case <-w.quit: // if on processing, we recv a quit signal, we cancel the context immediately
					cancel()
					w.wg.Done()

					return
				}
			case <-w.quit:
				cancel()
				w.wg.Done()

				return
			}
		}
	}()
}

func (w Worker) stop() {
	go func() {
		w.quit <- true
	}()
}

// Dispatcher ...
type Dispatcher struct {
	mu      sync.Mutex
	wg      sync.WaitGroup
	workers []Worker
	pool    chan chan Job
	queue   chan Job
	doneC   chan struct{}
}

// NewDispatcher ...
func NewDispatcher(maxWorkers int) *Dispatcher {
	executorOnce.Do(func() {
		dispatcher = ReloadDispatcher(maxWorkers)
		limiter = rate.NewLimiter(rate.Limit(maxWorkers), maxWorkers*2) // maxWorkers requests and then three more requests per second
	})

	dispatcherMutex.RLock()
	defer dispatcherMutex.RUnlock()

	return dispatcher
}

func ReloadDispatcher(maxWorkers int) *Dispatcher {
	dispatcherMutex.Lock()
	defer dispatcherMutex.Unlock()

	return createDispatcher(maxWorkers)
}

func createDispatcher(maxWorkers int) *Dispatcher {
	jobQueue = make(chan Job)

	if maxWorkers < 0 {
		maxWorkers = MaxWorker
	}
	pool := make(chan chan Job, maxWorkers)
	dispatcher := &Dispatcher{
		workers: make([]Worker, maxWorkers),
		pool:    pool,
		queue:   make(chan Job),
		doneC:   make(chan struct{}),
	}

	return dispatcher
}

// Start ...
func (d *Dispatcher) Start() {
	for i := 0; i < len(d.workers); i++ {
		worker := NewWorker(d.pool, &d.wg)
		d.workers[i] = worker
		worker.Start()
	}
	go d.dispatch()
}

func (d *Dispatcher) dispatch() {
	for {
		select {
		case job, ok := <-jobQueue:
			if ok {
				go func(job Job) {
					// pop out the channel, and send job to it
					jobChannel := <-d.pool
					jobChannel <- job
				}(job)
			}
		case <-d.doneC:
			logrus.Warn("recv done sig, exit dispatch method .")

			return
		}
	}
}

// Close stop all associated workers, and wait for done of all pending jobs
func (d *Dispatcher) Close() {
	logrus.Warn("Closing queue dispatcher ...")
	close(d.doneC)
	for i := 0; i < len(d.workers); i++ {
		d.workers[i].stop()
	}
	close(jobQueue) // stop from recving new jobs
	d.wg.Wait()
}

// Enqueue ...
func (d *Dispatcher) Enqueue(job Job) {
	d.queue <- job
}

func SubmitJob(job Job) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := limiter.Wait(ctx); err != nil {
		logrus.Warn("job submit too frequent, or too many jobs")
	}

	jobQueue <- job
}
