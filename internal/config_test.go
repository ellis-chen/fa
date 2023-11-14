package internal

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfDefault(t *testing.T) {
	conf := ReloadConf()
	assert.True(t, conf.ActivateJWT)
}

func TestLoadenvBool(t *testing.T) {
	t.Setenv("FA_ACTIVATE_JWT", "false")
	conf := ReloadConf()
	assert.False(t, conf.ActivateJWT)
}

func TestLoadenvString(t *testing.T) {
	t.Setenv("FA_STORE_PATH", "/ellis-chen/users/")
	conf := loadFaConf()
	assert.Equal(t, "/ellis-chen/users/", conf.StorePath)
}

func TestLoadConfigByFile(t *testing.T) {
	var yamlExample = []byte(`
activate_jwt: false
store_path: /ellis-chen/users/

`)
	f, _ := ioutil.TempFile("", "conf*.yaml")
	_, err := io.Copy(f, bytes.NewReader(yamlExample))
	require.NoError(t, err)
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	conf := loadFaConf(f.Name())
	assert.Equal(t, "/ellis-chen/users/", conf.StorePath)
}

func TestLog(t *testing.T) {
	t.Setenv("FA_LOG_LEVEL", "INFO")
	t.Setenv("FA_BALANCE_FREQ", "120")
	t.Setenv("FA_LOG_MAX_BACKUPS", "1024")
	t.Setenv("FA_ENABLE_FCCE_MODE", "true")
	t.Setenv("FA_BILLING_ADDR", "localhost:50041")
	t.Setenv("FA_AUDIT_ADDR", "localhost:40021")

	conf := loadFaConf()
	assert.Equal(t, "INFO", conf.Log.Level)
	assert.Equal(t, 1024, conf.Log.MaxBackups)

	assert.Equal(t, 30, conf.BalanceLimit)
	assert.Equal(t, 120, conf.BalanceFreq)
	assert.Equal(t, "/ellis-chen-mnt", conf.StoreMgrMntRoot)
	assert.Equal(t, "localhost:50041", conf.BillingAddr)
	assert.Equal(t, "localhost:40021", conf.AuditAddr)

	t.Setenv("FA_STORE_MGR_ROOT", "/ellis-chen-mnt1")
	conf = ReloadConf()
	assert.Equal(t, "/ellis-chen-mnt1", conf.StoreMgrMntRoot)
}

type model struct {
	userName string
}

func TestMapStructure(t *testing.T) {
	// This input can come from anywhere, but typically comes from
	// something like decoding JSON where we're not quite sure of the
	// struct initially.
	input := map[string]interface{}{
		"username": "Mitchell",
	}

	var result model

	err := mapstructure.Decode(input, &result)
	if err != nil {
		panic(err)
	}

	t.Logf("%#v", result)
}
