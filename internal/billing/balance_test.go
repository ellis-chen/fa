package billing

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestLoadBalance(t *testing.T) {
	t.Skip()
	type user struct {
		name     string
		balance  float32
		token    string
		tenantID int64
	}
	var ss = map[string]*user{
		"alan": {"alan", 21, "alan", 1},
		"tom":  {"tom", 40, "tom", 2},
		"john": {"john", 50, "john", 3},
		"kate": {"kate", 31, "kate", 4},
	}

	var mu sync.RWMutex

	balanceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		br := &BalanceResponse{}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

		mu.RLock()
		balance := ss[token]
		mu.RUnlock()
		err := json.Unmarshal([]byte(fmt.Sprintf(`{"code":200,"message":"success","data":{"balance":%.2f,"validPeriodTime":"2024-09-27 23:59","consumeAmount":816.66,"expireAmount":118.00}}`, balance.balance)), br)
		if err != nil {
			return
		}

		b, _ := json.Marshal(&br)
		_, _ = io.WriteString(w, string(b))
	}))
	defer balanceServer.Close()

	viper.Set("balance_host", balanceServer.URL)
	viper.Set("balance_freq", 2)

	for _, v := range ss {
		v := v
		go func(v *user) {
			var doneC = make(chan struct{})

			rand.Seed(time.Now().UTC().UnixMicro())
			r := rand.Intn(2000)
			time.AfterFunc(time.Millisecond*time.Duration(r), func() {
				mu.Lock()
				if r&0x01 == 0 {
					v.balance += float32(r / 100)
				} else {
					v.balance -= float32(r / 100)
				}

				mu.Unlock()
				doneC <- struct{}{}
			})

			<-doneC
		}(v)

		t.Run(v.name, func(t *testing.T) {
			//t.Parallel()
			var times int

			balanceCache := newBalanceCache(v.token, v.tenantID)
			defer balanceCache.Close()
			for {
				<-time.After(time.Millisecond * 200)
				balance, err := balanceCache.Query()
				t.Logf("balance for user [ %s ] is: %v", v.name, balance)
				//require.GreaterOrEqual(t, balance, float32(10))
				times++
				require.NoError(t, err)
				if times >= 15 {
					return
				}
			}
		})
	}
}

func TestLoadBalanceExpire(t *testing.T) {
	t.Skip()
	type user struct {
		name     string
		balance  float32
		token    string
		tenantID int64
	}
	var ss = map[string]*user{
		"alan": {"alan", 21, "alan", 1},
		"tom":  {"tom", 40, "tom", 2},
		"john": {"john", 50, "john", 3},
		"kate": {"kate", 31, "kate", 4},
	}

	var mu sync.RWMutex

	balanceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		br := &BalanceResponse{}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

		mu.RLock()
		balance := ss[token]
		mu.RUnlock()
		err := json.Unmarshal([]byte(fmt.Sprintf(`{"code":200,"message":"success","data":{"balance":%.2f,"validPeriodTime":"2024-09-27 23:59","consumeAmount":816.66,"expireAmount":118.00}}`, balance.balance)), br)
		if err != nil {
			return
		}

		b, _ := json.Marshal(&br)
		_, _ = io.WriteString(w, string(b))
	}))
	defer balanceServer.Close()

	viper.Set("balance_host", balanceServer.URL)
	viper.Set("balance_freq", 2) // means that the idle-timeout should be 4000ms

	for _, v := range ss {
		v := v

		t.Run(v.name, func(t *testing.T) {
			//t.Parallel()
			var times int

			balanceCache := newBalanceCache(v.token, v.tenantID)
			defer balanceCache.Close()
			for {
				<-time.After(time.Millisecond * 200)
				balance, err := balanceCache.Query()
				t.Logf("balance for user [ %s ] is: %v", v.name, balance)
				times++

				if times >= 4000/200 { // idle-timeout occurs
					t.Logf("idle-timeout happens here: { %d }", times)
					require.Error(t, err, ErrIdleTooLong)
				} else {
					t.Logf("active times are [ %d ]", times)
					require.NoError(t, err)
				}
				if times >= 50 {
					return
				}
			}
		})
	}
}
