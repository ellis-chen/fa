package sampler

import (
	"context"
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/ellis-chen/fa/pkg/queue"

	"github.com/sirupsen/logrus"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestNewHandle(t *testing.T) {
	t.SkipNow()

	executor := queue.NewDispatcher(5)
	executor.Start()
	defer executor.Close()

	sampler := NewSampler()
	sampler.Start()
	defer sampler.StopAll()

	rpcBilling := func(_ context.Context, v interface{}) {
		time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)))
		logrus.Infof("Success invoke billing with val %v", v)
	}

	logrus.Info("sampler register user 1 ...")

	err := sampler.Register(1, func(user int64) interface{} {
		logrus.Infof("Do the sampling for user %v", user)

		return "hello"
	}, time.NewTicker(time.Microsecond*50))
	if err != nil {
		log.Fatal(err)
	}

	logrus.Info("sampler register user 2 ...")
	err = sampler.Register(2, func(user int64) interface{} {
		logrus.Infof("Do the sampling for user %v", user)

		return "again"
	}, time.NewTicker(time.Microsecond*50))
	if err != nil {
		log.Fatal(err)
	}

	logrus.Info("sampler register handling logic ...")

	sampler.Handle(func(ch <-chan interface{}) {
		count := 0
		for {
			v, ok := <-ch
			if ok {
				count++
				if count%10 == 0 {
					queue.SubmitJob(queue.Job{PayLoad: v, Action: rpcBilling})
				}
			} else {
				logrus.Infof("Exiting the %v job iteration", 1)

				return
			}
		}
	})
}
