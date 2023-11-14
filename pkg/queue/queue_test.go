package queue

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestQueue(t *testing.T) {
	t.SkipNow()
	d := NewDispatcher(MaxWorker)
	d.Start()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		work := Job{PayLoad: "hello", Action: func(ctx context.Context, args interface{}) {
			t.Logf("args are: %v", args)
			time.Sleep(time.Duration(rand.Intn(30)) * time.Second)
			t.Log("job done: ", args)
			wg.Done()
		}}
		jobQueue <- work
	}
	wg.Wait()
	time.Sleep(time.Hour)
}

func TestContextCancel(t *testing.T) {
	t.SkipNow()
	var start time.Time
	var ch chan struct{}
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(context.TODO(), time.Second*5)

	ch = make(chan struct{})
	go func(c chan struct{}) {
		defer cancel()
		start = time.Now()
		time.Sleep(10 * time.Second)
		logrus.Infof("It cost %.2f sec to finish the job", float64(time.Since(start).Milliseconds())/1e3)
		c <- struct{}{}
	}(ch)
	select {
	case <-ch:
	case <-ctx.Done():
	}
	logrus.Infof("It cost %.2f sec to reach the end", float64(time.Since(start).Milliseconds())/1e3)
	go func(c chan struct{}) {
		defer cancel()
		start = time.Now()
		time.Sleep(5 * time.Second)
		logrus.Infof("It cost %.2f sec to finish the job", float64(time.Since(start).Milliseconds())/1e3)
		c <- struct{}{}
	}(ch)
	select {
	case <-ch:
	case <-ctx.Done():
	}
	<-time.After(time.Minute)
}

func TestSubmitJob(t *testing.T) {
	dis := NewDispatcher(10)
	dis.Start()
	defer dis.Close()

	var wg sync.WaitGroup
	wg.Add(10)

	for i := 0; i < 10; i++ {
		go func() {
			SubmitJob(Job{PayLoad: "hello", Action: func(ctx context.Context, args interface{}) {
				t.Logf("args are: %v", args)
			}})
			wg.Done()
		}()
	}

	wg.Wait()
}
