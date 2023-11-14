package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

const (
	DefaultStoreMgrRoot      = "/ellis-chen-mnt"
	DefaultAuditPath         = "/ellis-chen-auditing"
	DefaultAuditApprovedPath = "/ellis-chen-audited"
)

var (
	// +checklocks:faConfMutex
	faConf     *Config
	faConfOnce sync.Once
	//Declare a global mutex
	faConfMutex sync.RWMutex
)

// Config represent the configuration of whole app
type Config struct {
	ActivateJWT         bool `mapstructure:"activate_jwt"`
	Debug               bool
	EnableBalanceCheck  bool `mapstructure:"enable_balance_check"`
	FcceMode            bool `mapstructure:"enable_fcce_mode"`
	Port                int
	SampleIntervalInSec int    `mapstructure:"sample_interval_insec"`
	SampleIntervalCount int    `mapstructure:"sample_interval_count"`
	BalanceLimit        int    `mapstructure:"balance_limit"`
	BalanceFreq         int    `mapstructure:"balance_freq"`
	JwtSecret           string `mapstructure:"jwt_secret"`
	JwtRealm            string `mapstructure:"jwt_realm"`
	StorePath           string `mapstructure:"store_path"`          // default: /ellis-chen, storage_id: -1
	StoreMgrMntRoot     string `mapstructure:"store_mgr_root"`      // default: /ellis-chen-mnt, storage_id: 1,2,3, .., n
	AuditPath           string `mapstructure:"audit_path"`          // default: /ellis-chen-auditing, storage_id: -99, meant for internal access only
	AuditApprovedPath   string `mapstructure:"audit_approved_path"` // default: /ellis-chen-audited, storage_id: -2
	Dbname              string `mapstructure:"dbname"`

	BillingAddr string `mapstructure:"billing_addr"`
	AuditAddr   string `mapstructure:"audit_addr"`
	BalanceHost string `mapstructure:"balance_host"`

	TenantID         string `mapstructure:"tenantid"`
	NS               string `mapstructure:"ns"`
	NFSID            string `mapstructure:"nfs_id"`
	CloudAccountName string `mapstructure:"cloud_account_name"`
	BussSoftPath     string `mapstructure:"soft_path"`

	Log *struct {
		TermMode   bool `mapstructure:"term_mode"`
		Level      string
		Path       string
		MaxSize    int `mapstructure:"max_size"`
		MaxBackups int `mapstructure:"max_backups"`
		MaxAge     int `mapstructure:"max_age"`
	}
}

func (c *Config) String() string {
	var buf bytes.Buffer

	e := yaml.NewEncoder(&buf)

	if err := e.Encode(c); err != nil {
		return "unable to encode Profile as YAML: " + err.Error()
	}

	return buf.String()
}

// LoadConf load the config using viper
func LoadConf(cfgFile ...string) *Config {
	faConfOnce.Do(func() {
		ReloadConf(cfgFile...)
	})

	faConfMutex.RLock()
	defer faConfMutex.RUnlock()

	return faConf
}

func ReloadConf(cfgFile ...string) *Config {
	faConfMutex.Lock()
	defer faConfMutex.Unlock()
	faConf = loadFaConf(cfgFile...)

	return faConf
}

func loadFaConf(cfgFile ...string) *Config {
	if len(cfgFile) == 1 {
		viper.SetConfigFile(cfgFile[0])
	} else {
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName(".fa")
	}

	viper.AutomaticEnv() // read in environment variables that match
	viper.SetEnvPrefix("fa")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	viper.SetDefault("activate_jwt", true)
	viper.SetDefault("enable_fcce_mode", false)
	viper.SetDefault("store_path", "/ellis-chen")
	viper.SetDefault("jwt_secret", "changeme")
	viper.SetDefault("jwt_realm", "file access")
	viper.SetDefault("debug", false)
	viper.SetDefault("port", 8080)
	viper.SetDefault("dbname", ".fadb")
	viper.SetDefault("sample_interval_insec", "60")
	viper.SetDefault("sample_interval_count", "60")
	viper.SetDefault("billing_addr", "fs-resource-usage.prod.svc:5001")
	viper.SetDefault("audit_addr", "")
	viper.SetDefault("ns", "prod")
	viper.SetDefault("nfs_id", "-")
	viper.SetDefault("cloud_account_name", "")
	viper.SetDefault("tenantid", "-")
	viper.SetDefault("soft_path", "/opt/ellis-chen/softwares")
	viper.SetDefault("balance_host", "https://fcc.ellis-chentech.com")
	viper.SetDefault("balance_limit", 30)
	viper.SetDefault("balance_freq", 30)
	viper.SetDefault("enable_balance_check", true)
	viper.SetDefault("log.term_mode", true)
	viper.SetDefault("log.path", "/logs/fa.log")
	viper.SetDefault("log.level", "DEBUG")
	viper.SetDefault("log.max_size", 10)
	viper.SetDefault("log.max_backups", 10)
	viper.SetDefault("log.max_age", 7)

	if viper.GetString("ns") != "prod" && viper.GetString("billing_addr") == "fs-resource-usage.prod.svc:5001" {
		viper.Set("billing_addr", fmt.Sprintf("fs-resource-usage.%s.svc:5001", viper.GetString("ns")))
	}

	if viper.GetBool("enable_fcce_mode") { // fcce mode equalivent settings
		viper.SetDefault("store_mgr_root", DefaultStoreMgrRoot)
		viper.SetDefault("audit_path", DefaultAuditPath)
		viper.SetDefault("audit_approved_path", DefaultAuditApprovedPath)
		viper.Set("enable_balance_check", false)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		fmt.Fprintln(os.Stderr, "err unmarshal cfg:", err)
	}

	return &cfg
}

func DumpConf(config any, w io.Writer) error {
	out, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	_, err = w.Write(out)

	return err
}
