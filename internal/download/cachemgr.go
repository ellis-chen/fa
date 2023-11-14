package download

import (
	"context"
	"sync"
	"time"

	"github.com/ellis-chen/fa/internal/billing"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
)

var (
	// Cache ...
	// +checklocks:dlcMutex
	dcm *DlCacheMgr

	dlcOnce sync.Once

	dlcMutex sync.RWMutex
)

type (
	trasferCtxKey int
	// DlCacheMgr ...
	DlCacheMgr struct {
		wg sync.WaitGroup

		drainC chan struct{}
		doneC  chan struct{}

		option  DlOption
		handler func(context.Context, interface{})

		recvC chan Dl
		once  sync.Once

		stat dlStat
	}

	// DlOption ...
	DlOption struct {
		DlLimit int64
		Timeout int
		Ctx     context.Context
	}

	// DlRecorder  ...
	DlRecorder struct {
		PathKey   string
		Path      string
		CumSize   int64
		TenantID  int64
		UserID    int
		From      string
		To        string
		clientIP  string
		StartTime time.Time
	}

	// Dl an download request
	Dl struct {
		PathKey   string    `json:"PathKey"`
		Path      string    `json:"Path"`
		Size      int64     `json:"Size"`
		From      string    `json:"From"`
		To        string    `json:"To"`
		TenantID  int64     `json:"TenantID"`
		UserID    int       `json:"UserID"`
		ClientIP  string    `json:"ClientIP"`
		StartTime time.Time `json:"StartTime"`
	}

	dlStat struct {
		// +checklocks:mu
		stat map[string]int
		mu   sync.Mutex
	}
)

// NewDlCacheMgr ...
func NewDlCacheMgr(option DlOption, callfunc func(context.Context, interface{})) *DlCacheMgr {
	dlcOnce.Do(func() {
		ReloadDlCacheMgr(option, callfunc)
	})

	dlcMutex.RLock()
	defer dlcMutex.RUnlock()

	return dcm
}

func ReloadDlCacheMgr(option DlOption, callfunc func(context.Context, interface{})) *DlCacheMgr {
	dlcMutex.Lock()
	defer dlcMutex.Unlock()
	dcm = &DlCacheMgr{
		option:  option,
		handler: callfunc,
		drainC:  make(chan struct{}),
		doneC:   make(chan struct{}),
		recvC:   make(chan Dl),
		stat:    dlStat{stat: make(map[string]int)},
	}

	go dcm.serve()

	return dcm
}

// Record ...
func (dcm *DlCacheMgr) Record(dl Dl) {
	if dl.Size <= 0 {
		return
	}

	dcm.recvC <- dl
}

type dlCache map[string]*DlRecorder

func (dc dlCache) Clone() dlCache {
	snapshot := make(dlCache, len(dc))
	maps.Copy(snapshot, dc)

	return snapshot
}

func (dc dlCache) Clear() {
	maps.Clear(dc)
}

func (dc dlCache) Delete(key string) {
	maps.DeleteFunc(dc, func(k string, _ *DlRecorder) bool { return k == key })
}

func (dc dlCache) Len() int {
	return len(dc)
}

func (dcm *DlCacheMgr) serve() {
	cache := make(dlCache)
	defer func() {
		dcm.wg.Add(cache.Len())
		dcm.publishBatch(cache)

		dcm.wg.Wait()
		dcm.doneC <- struct{}{}
	}()

	for {
		select {
		case dl := <-dcm.recvC:
			dr := cache[dl.PathKey]
			if dr != nil {
				dr.CumSize += dl.Size
			} else {
				dr = &DlRecorder{
					PathKey:   dl.PathKey,
					Path:      dl.Path,
					CumSize:   dl.Size,
					TenantID:  dl.TenantID,
					UserID:    dl.UserID,
					From:      dl.From,
					To:        dl.To,
					clientIP:  dl.ClientIP,
					StartTime: time.Now(),
				}
				cache[dl.PathKey] = dr
			}

			if dr.CumSize > dcm.option.DlLimit {
				cache.Delete(dl.PathKey)

				dcm.wg.Add(1)
				go dcm.publishOne(dr)
			}

		case <-time.After(time.Second * time.Duration(dcm.option.Timeout)):
			snapshot := cache.Clone()
			cache.Clear()

			dcm.wg.Add(snapshot.Len())
			go dcm.publishBatch(snapshot)
		case <-dcm.drainC:

			return
		}
	}
}

func (dcm *DlCacheMgr) publishBatch(recorders map[string]*DlRecorder) {
	for _, dl := range recorders {
		dcm.publishOne(dl)
	}
}

func (dcm *DlCacheMgr) publishOne(dl *DlRecorder) {
	defer dcm.wg.Done()
	u4 := uuid.New().String()
	usage := billing.TransferUsage{
		UsageID:   u4,
		TenantID:  dl.TenantID,
		UserID:    dl.UserID,
		Name:      dl.Path,
		Size:      dl.CumSize,
		Type:      billing.DOWNLOAD,
		From:      dl.From,
		To:        dl.clientIP,
		StartTime: dl.StartTime,
		EndTime:   time.Now(),
		ClientIP:  dl.clientIP,
	}

	dcm.stat.add(dl.PathKey, dl.CumSize)

	dcm.handler(dcm.option.Ctx, usage)
}

func (ds *dlStat) add(path string, size int64) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.stat[path] += int(size)
}

func (ds *dlStat) get(path string) int {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	return ds.stat[path]
}

// Close ...
func (dcm *DlCacheMgr) Close() {
	select {
	case <-dcm.doneC:
		return
	default:
	}

	logrus.Warnf("Closing the DlCacheMgr ...")

	dcm.once.Do(func() {
		dcm.drainC <- struct{}{}
		<-dcm.doneC
		close(dcm.drainC)
		close(dcm.doneC)
		close(dcm.recvC)

		logrus.Warnf("Successfully close the DlCacheMgr")
	})
}
