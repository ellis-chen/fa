package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ellis-chen/fa/internal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type (
	// TenantBalance ...
	TenantBalance struct {
		Balance         float32 `json:"balance"`
		ConsumeAmount   float32 `json:"consumeAmount"`
		ExpireAmount    float32 `json:"expireAmount"`
		ValidPeriodTime string  `json:"validPeriodTime"`
	}

	// BalanceResponse ...
	BalanceResponse struct {
		Code    int           `json:"code"`
		Message string        `json:"message"`
		Data    TenantBalance `json:"data"`
	}

	cacheStatus int

	// BalanceCache the cache of the tenant's balance
	BalanceCache struct {
		// +checklocks:mu
		balance float32
		// +checklocks:mu
		tenantID  int64
		ticker    *time.Ticker
		beatTimer *time.Timer
		mu        sync.RWMutex
		// +checklocks:mu
		token string
		done  chan struct{}
		// +checklocks:mu
		status cacheStatus
		beatC  chan struct{}
		config *internal.Config
	}
)

const (
	// PENDING balance cache to be update
	PENDING = iota
	// LIVE balance cache in a valid window
	LIVE
	// STALED balance cache stale
	STALED
)

func (bc *BalanceCache) loop() {
	var ctx context.Context
	var cancelFunc context.CancelFunc

	defer bc.ticker.Stop()
	for {
		ctx, cancelFunc = context.WithCancel(context.TODO())

		select {
		case <-bc.ticker.C:
			if bc.isAlive() { // only query when in active session
				ctx, cancelFunc = context.WithTimeout(ctx, time.Second*5)
				bc.query(ctx, cancelFunc)
			}
		case <-bc.done:
			cancelFunc() // by cancel the context, even if we are in the middle of query the balance, we drop it immediately and return

			return
		}
	}
}

func (bc *BalanceCache) query(ctx context.Context, cancelFunc context.CancelFunc) {
	defer cancelFunc()
	err := bc.doQueryBalance(ctx)
	if err != nil {
		logrus.Errorf("err do query balance: %v", err)
		bc.mu.Lock()
		bc.balance = 0
		bc.mu.Unlock()

		return
	}
}

// Close close the balanceCache
func (bc *BalanceCache) Close() {
	logrus.Warnf("BalanceCache closed")
	close(bc.done)
}

// getBalanceCache construct a unique balanceCache for later use. Notice even if you call multiple times, you
// will still receive only one instance
func getBalanceCache(token string, tenantID int64) *BalanceCache {
	balanceOnce.Do(func() {
		bc = newBalanceCache(token, tenantID)
	})

	return bc
}

func newBalanceCache(token string, tenantID int64) *BalanceCache {
	config := internal.LoadConf()
	aliveDuration := time.Second * time.Duration(config.BalanceFreq*2)

	bc := BalanceCache{
		balance:   -1,
		tenantID:  tenantID,
		ticker:    time.NewTicker(time.Second * time.Duration(config.BalanceFreq)),
		token:     token,
		done:      make(chan struct{}),
		status:    PENDING,
		beatTimer: time.NewTimer(aliveDuration),
		beatC:     make(chan struct{}),
		config:    config,
	}

	exitChan = make(chan os.Signal, 1)
	signal.Notify(exitChan, exitSignals...)
	go func() {
		var sig os.Signal
		select {
		case sig = <-exitChan:
			if sig == nil {
				return
			}
		case <-bc.done:
			return
		}
		logrus.Warnf("Signal received: %s", sig)
		bc.Close()
		logrus.Warnf("Exiting...")
	}()

	go func() {
		defer bc.beatTimer.Stop()
		for {
			select {
			case <-bc.done:
				return
			case <-bc.beatTimer.C:
				// mark the stop of the loop
				bc.iamGone()
			case <-bc.beatC:
				bc.iamAlive()
				bc.beatTimer.Reset(aliveDuration)
			}
		}
	}()

	ctx, cancelFunc := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancelFunc()

	err := bc.doQueryBalance(ctx)
	if err != nil {
		logrus.Errorf("err do query balance: %v", err)
	}
	go bc.loop()

	return &bc
}

func (bc *BalanceCache) iamAlive() {
	bc.mu.Lock()
	if bc.status == STALED {
		bc.status = PENDING
	}
	bc.mu.Unlock()
}

func (bc *BalanceCache) iamGone() {
	bc.mu.Lock()
	bc.status = STALED
	bc.mu.Unlock()
}

func (bc *BalanceCache) isAlive() bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.status != STALED
}

func (bc *BalanceCache) isGone() bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.status == STALED
}

// Query get the balance from local cache
func (bc *BalanceCache) Query() (balance float32, err error) {
	bc.mu.RLock()
	balance = bc.balance
	status := bc.status
	bc.mu.RUnlock()

	if status == STALED {
		err = ErrIdleTooLong
	}

	return balance, err
}

// QueryBalance ...
func QueryBalance(token string, tenantID int64) (float32, error) {
	getBalanceCache(token, tenantID)

	go func() {
		bc.beatC <- struct{}{}
	}()

	bc.mu.Lock()
	bc.token = token // always update the token to the latest in case of loop to use the valid token
	bc.mu.Unlock()

	if bc.isGone() {
		ctx, cancelFunc := context.WithTimeout(context.TODO(), time.Second*5)
		go bc.query(ctx, cancelFunc)
		<-ctx.Done()
	}

	bc.mu.RLock()
	balance := bc.balance
	bc.mu.RUnlock()

	return balance, nil
}

func (bc *BalanceCache) doQueryBalance(ctx context.Context) error {
	bc.mu.RLock()
	tenantID := bc.tenantID
	token := bc.token
	bc.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf(`%s/balance/api/v1/subAccount`, bc.config.BalanceHost), nil)
	if err != nil {
		return errors.Wrap(err, "err new GET request")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "err do the GET request")
	}
	defer resp.Body.Close()

	// b := []byte(`{"code":200,"message":"success","data":{"balance":153040.43,"validPeriodTime":"2024-09-27 23:59","consumeAmount":816.66,"expireAmount":118.00}}`)
	bytes, _ := io.ReadAll(resp.Body)

	balance := BalanceResponse{}
	err = json.Unmarshal(bytes, &balance)

	if err != nil {
		return fmt.Errorf("err when query the balance for tenant [ %v ] due to : %w", tenantID, err)
	}

	if balance.Code != 200 {
		logrus.Warnf("Illegal balance response: %s", string(bytes))

		return ErrBalanceQuery
	}

	bc.mu.Lock()
	bc.balance = balance.Data.Balance
	bc.status = LIVE
	bc.mu.Unlock()

	logrus.Infof("Balance for tenant [ %v ] is: %v", tenantID, balance.Data.Balance)

	return nil
}

var (
	// ErrBalanceQuery ...
	ErrBalanceQuery = errors.New("not able to get the detail balance")

	// ErrIdleTooLong ...
	ErrIdleTooLong = errors.New("balance query idle too long")

	bc *BalanceCache

	balanceOnce sync.Once

	exitChan    chan os.Signal
	exitSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
)
