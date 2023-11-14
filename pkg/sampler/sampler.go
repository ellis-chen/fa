package sampler

import (
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	samplerMu sync.RWMutex
)

// SampleFunc ...
type SampleFunc func(int64) interface{}

// Sampler ...
type Sampler struct {
	// +checklocks:mu
	ctx                   map[int64]*SampleJob // replace with sync.Map
	mu                    sync.RWMutex
	newProducerRegistered chan int64
	jobConsumerRegistered chan int64
	done                  chan struct{}
	once                  sync.Once
}

// SampleJob ...
type SampleJob struct {
	fn     SampleFunc
	c      chan interface{}
	close  chan struct{}
	done   chan struct{}
	mu     sync.Mutex
	id     int64
	ticker *time.Ticker
}

// Close ...
func (sj *SampleJob) Close() {
	logrus.Warnf("Stopping %v job ...", sj.id)
	sj.close <- struct{}{}
	<-sj.done
	close(sj.c)
}

// Sample ...
func (sj *SampleJob) Sample() {
	defer func() {
		logrus.Warnf("Exiting %v job sample", sj.id)
		sj.done <- struct{}{}
	}()
	for {
		select {
		case <-sj.ticker.C:
			sj.c <- sj.fn(sj.id) //may block forever if no consumer registered, use context.Deadline() to return
		case <-sj.close:
			logrus.Infof("%v job closing .", sj.id)
			sj.ticker.Stop()
			logrus.Infof("%v job closed .", sj.id)

			return
		}
	}
}

// NewSampler ...
func NewSampler() *Sampler {
	samplerMu.Lock()
	defer samplerMu.Unlock()

	sampler := &Sampler{
		ctx:                   make(map[int64]*SampleJob, 0),
		newProducerRegistered: make(chan int64),
		jobConsumerRegistered: make(chan int64),
		done:                  make(chan struct{}),
	}

	return sampler
}

// Register a user
func (s *Sampler) Register(user int64, fn SampleFunc, ticker *time.Ticker) error {
	s.mu.Lock()
	_, found := s.ctx[user]
	if found {
		s.mu.Unlock()

		return errors.New("sampler job registered already! Try unregister first")
	}
	job := &SampleJob{fn: fn, mu: sync.Mutex{}, id: user, ticker: ticker, c: make(chan interface{}), close: make(chan struct{}), done: make(chan struct{})}
	s.ctx[user] = job
	s.mu.Unlock()
	logrus.Infof("Register a job sampler for [%v]", user)
	s.newProducerRegistered <- user

	return nil
}

// SampleHandler ...
type SampleHandler func(<-chan interface{})

// Handle ...
func (s *Sampler) Handle(sampleHandler SampleHandler) {
	go func() {
		for {
			select {
			case user, ok := <-s.jobConsumerRegistered:
				if ok {
					logrus.Infof("Handling the sample response for [%v]", user)
					ch := s.Channel(user)
					go sampleHandler(ch)
				}
			case <-s.done:
				return
			}
		}
	}()
}

// Channel ...
func (s *Sampler) Channel(user int64) <-chan interface{} {
	s.mu.RLock()
	sampleHandle := s.ctx[user]
	if sampleHandle == nil {
		s.mu.RUnlock()

		return nil
	}
	s.mu.RUnlock()

	return sampleHandle.c
}

// DeRegister a user
func (s *Sampler) DeRegister(user int64) {
	s.Stop(user)
	s.mu.Lock()
	delete(s.ctx, user)
	s.mu.Unlock()
}

// Stop ...
func (s *Sampler) Stop(user int64) {
	s.mu.RLock()
	sj := s.ctx[user]
	s.mu.RUnlock()
	if sj == nil {
		return
	}
	sj.Close()
}

// StopAll ...
func (s *Sampler) StopAll() {
	select {
	case <-s.done:
		return
	default:
	}
	s.once.Do(func() {
		s.mu.Lock()
		for user, sj := range s.ctx {
			sj.Close()
			delete(s.ctx, user)
		}
		s.mu.Unlock()

		close(s.done)
	})
}

// Start ...
func (s *Sampler) Start() {
	go func() {
		for {
			select {
			case user := <-s.newProducerRegistered:
				s.mu.Lock()
				sampleJob := s.ctx[user]
				s.mu.Unlock()
				logrus.Infof("Kickstart the job sampler for [%v]", user)
				go sampleJob.Sample()
				go func() {
					s.jobConsumerRegistered <- user
				}()
			case <-s.done:
				logrus.Warnf("Exit the job sample monitor")

				return
			}
		}
	}()
}
