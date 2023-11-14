package download

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
func TestDl(t *testing.T) {
	mgr := NewDlCacheMgr(DlOption{DlLimit: 100, Timeout: 1}, func(_ context.Context, i interface{}) {
		log.Printf("recv an transfer: [ %v ]", i)
	})

	var testWg sync.WaitGroup

	mgr.Record(Dl{
		PathKey:   "hello/1",
		Path:      "hello/1",
		Size:      50,
		From:      "efs-1",
		To:        "10.0.0.23",
		TenantID:  10010,
		UserID:    10,
		ClientIP:  "10.0.0.2",
		StartTime: time.Now(),
	})
	mgr.Record(Dl{
		PathKey:   "hello/1",
		Path:      "hello/1",
		Size:      50,
		From:      "efs-1",
		To:        "10.0.0.23",
		TenantID:  10010,
		UserID:    10,
		ClientIP:  "10.0.0.2",
		StartTime: time.Now(),
	})
	testWg.Add(1)
	go func() {
		mgr.Record(Dl{
			PathKey:   "hello/2",
			Path:      "hello/2",
			Size:      110,
			From:      "efs-1",
			To:        "10.0.0.23",
			TenantID:  10010,
			UserID:    10,
			ClientIP:  "10.0.0.2",
			StartTime: time.Now(),
		})
		testWg.Done()
	}()
	mgr.Record(Dl{
		PathKey:   "hello/3",
		Path:      "hello/3",
		Size:      33,
		From:      "efs-1",
		To:        "10.0.0.23",
		TenantID:  10010,
		UserID:    10,
		ClientIP:  "10.0.0.2",
		StartTime: time.Now(),
	})
	testWg.Add(1)
	go func() {
		mgr.Record(Dl{
			PathKey:   "hello/4",
			Path:      "hello/4",
			Size:      44,
			From:      "efs-1",
			To:        "10.0.0.23",
			TenantID:  10010,
			UserID:    10,
			ClientIP:  "10.0.0.2",
			StartTime: time.Now(),
		})
		testWg.Done()
	}()
	mgr.Record(Dl{
		PathKey:   "hello/5",
		Path:      "hello/5",
		Size:      55,
		From:      "efs-1",
		To:        "10.0.0.23",
		TenantID:  10010,
		UserID:    10,
		ClientIP:  "10.0.0.2",
		StartTime: time.Now(),
	})
	testWg.Add(1)
	go func() {
		mgr.Record(Dl{
			PathKey:   "hello/5",
			Path:      "hello/5",
			Size:      55,
			From:      "efs-1",
			To:        "10.0.0.23",
			TenantID:  10010,
			UserID:    10,
			ClientIP:  "10.0.0.2",
			StartTime: time.Now(),
		})
		testWg.Done()
	}()
	testWg.Add(1)
	go func() {
		mgr.Record(Dl{
			PathKey:   "hello/6",
			Path:      "hello/6",
			Size:      99,
			From:      "efs-1",
			To:        "10.0.0.23",
			TenantID:  10010,
			UserID:    10,
			ClientIP:  "10.0.0.2",
			StartTime: time.Now(),
		})
		testWg.Done()
	}()

	testWg.Add(1)
	go func() {
		mgr.Record(Dl{
			PathKey:   "hello/3",
			Path:      "hello/3",
			Size:      55,
			From:      "efs-1",
			To:        "10.0.0.23",
			TenantID:  10010,
			UserID:    10,
			ClientIP:  "10.0.0.2",
			StartTime: time.Now(),
		})
		testWg.Done()
	}()

	testWg.Add(1)
	go func() {
		mgr.Record(Dl{
			PathKey:   "hello/3",
			Path:      "hello/3",
			Size:      13,
			From:      "efs-1",
			To:        "10.0.0.23",
			TenantID:  10010,
			UserID:    10,
			ClientIP:  "10.0.0.2",
			StartTime: time.Now(),
		})
		testWg.Done()
	}()

	testWg.Wait()
	mgr.Close()

	require.Equal(t, 100, mgr.stat.get("hello/1"))
	require.Equal(t, 110, mgr.stat.get("hello/2"))
	require.Equal(t, 101, mgr.stat.get("hello/3"))
	require.Equal(t, 44, mgr.stat.get("hello/4"))
	require.Equal(t, 110, mgr.stat.get("hello/5"))
	require.Equal(t, 99, mgr.stat.get("hello/6"))
}
