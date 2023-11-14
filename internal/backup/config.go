package backup

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// ProviderType enum for different providers
type ProviderType string

const (
	// ProviderTypeGCS captures enum value "GCS"
	ProviderTypeGCS ProviderType = "GCS"
	// ProviderTypeS3 captures enum value "S3"
	ProviderTypeS3 ProviderType = "S3"
	// ProviderTypeAzure captures enum value "Azure"
	ProviderTypeAzure ProviderType = "Azure"
)

// SecretType enum for different providers
type SecretType string

const (
	// SecretTypeAwsAccessKey captures enum value "AwsAccessKey"
	SecretTypeAwsAccessKey SecretType = "AwsAccessKey"
	// SecretTypeGcpServiceAccountKey captures enum value "GcpServiceAccountKey"
	SecretTypeGcpServiceAccountKey SecretType = "GcpServiceAccountKey"
	// SecretTypeAzStorageAccount captures enum value "AzStorageAccount"
	SecretTypeAzStorageAccount SecretType = "AzStorageAccount"
)

// SecretAws AWS keys
type SecretAws struct {
	// access key Id
	AccessKeyID string
	// secret access key
	SecretAccessKey string
	// session token
	SessionToken string
}

// Secret contains the credentials for different providers
type Secret struct {
	// aws
	Aws  *SecretAws
	Type SecretType
}

// ProviderConfig describes the config for the object store (which provider to use)
type ProviderConfig struct {
	// object store type
	Type ProviderType
	// Endpoint used to access the object store. It can be implicit for
	// stores from certain cloud providers such as AWS. In that case it can
	// be empty
	Endpoint string
	// Region specifies the region of the object store.
	Region string
	// If true, disable SSL verification. If false (the default), SSL
	// verification is enabled.
	SkipSSLVerify bool
}

type Location struct {
	Type       LocationType `mapstructure:"type"`
	VendorType VendorType   `mapstructure:"vendor"`
	Bucket     string       `mapstructure:"bucket"`
	Endpoint   string       `mapstructure:"endpoint"`
	Prefix     string       `mapstructure:"prefix"`
	Region     string       `mapstructure:"region"`
}

// LocationType
type LocationType string

const (
	LocationTypeS3Compliant LocationType = "s3Compliant"
	LocationTypeFilesytem   LocationType = "filesystem"
)

// VendorType
type VendorType string

const (
	VendorTypeAws    VendorType = "aws"
	VendorTypeOracle VendorType = "oracle"
	VendorTypeBaidu  VendorType = "baidu"
)

// CredentialType
type CredentialType string

const (
	CredentialTypeKeyPair CredentialType = "keyPair"
)

// Credential
type Credential struct {
	Type    CredentialType `mapstructure:"type"`
	KeyPair *KeyPair       `mapstructure:"keypair,omitempty"`
}

type KeyPair struct {
	AccessID  string `mapstructure:"key"`
	SecretKey string `mapstructure:"secret"`
}

type RepoConfig struct {
	Password          string `mapstructure:"pass"`
	CacheDir          string `mapstructure:"cachedir"`
	ConfigFile        string `mapstructure:"configfile"`
	EnableThrottle    bool   `mapstructure:"enableThrottle"`
	ParallelBackup    int    `mapstructure:"parallelBackup"`
	ParallelRestore   int    `mapstructure:"parallelRestore"`
	JobTimeoutInHours int    `mapstructure:"jobTimeoutInH"`
}

// Config
type Config struct {
	ScheduleFullInMs int         `mapstructure:"scheduleFullInMs"`
	FullMode         bool        `mapstructure:"fullMode"`
	Location         Location    `mapstructure:"loc"`
	Credential       Credential  `mapstructure:"cred"`
	SkipSSLVerify    bool        `mapstructure:"skipSSLVerify"`
	RepoConfig       *RepoConfig `mapstructure:"repo"`
}

func (c *Config) String() string {
	var buf bytes.Buffer

	e := yaml.NewEncoder(&buf)

	if err := e.Encode(c); err != nil {
		return "unable to encode Profile as YAML: " + err.Error()
	}

	return buf.String()
}

func (c *Config) Clone() *Config {
	var buf bytes.Buffer

	e := yaml.NewEncoder(&buf)

	if err := e.Encode(c); err != nil {
		return nil
	}

	var r Config
	if err := yaml.NewDecoder(&buf).Decode(&r); err != nil {
		return nil
	}
	return &r
}

func (c *Config) ToFull() *Config {
	cfg := c.Clone()
	cfg.RepoConfig.ConfigFile = cfg.RepoConfig.ConfigFile + "-full"
	cfg.RepoConfig.CacheDir = cfg.RepoConfig.CacheDir + "-full"
	cfg.Location.Prefix = fmt.Sprintf("%s-full", strings.TrimRight(cfg.Location.Prefix, "/"))
	return cfg
}

func LoadCfg(cfgFile ...string) *Config {
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
	viper.SetEnvPrefix("fb")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	viper.SetDefault("scheduleFullInMs", "1440")
	viper.SetDefault("fullMode", false)

	viper.SetDefault("loc.type", "s3Compliant")
	if viper.Get("loc.type") == "s3Compliant" {
		viper.SetDefault("loc.vendor", "aws")
		viper.SetDefault("loc.bucket", "kopia-backup-center")
		viper.SetDefault("loc.endpoint", "s3.amazonaws.com")
		viper.SetDefault("loc.region", "cn-northwest-1")
	}

	viper.SetDefault("loc.prefix", "-")
	viper.SetDefault("loc.ns", "cntnp64de9wb") // only used on oracle object storage

	viper.SetDefault("cred.type", "keyPair")
	viper.SetDefault("cred.keypair.key", "-")
	viper.SetDefault("cred.keypair.secret", "-")

	if viper.Get("loc.type") == "s3Compliant" {
		if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
			viper.Set("cred.keypair.key", os.Getenv("AWS_ACCESS_KEY_ID"))
		}

		if os.Getenv("AWS_SECRET_ACCESS_KEY") != "" {
			viper.Set("cred.keypair.secret", os.Getenv("AWS_SECRET_ACCESS_KEY"))
		}

		if viper.Get("loc.vendor") == "oracle" {
			// https://docs.oracle.com/en-us/iaas/api/
			// https://cntnp64de9wb.compat.objectstorage.ap-tokyo-1.oraclecloud.com
			viper.Set("loc.endpoint", fmt.Sprintf("%s.compat.objectstorage.%s.oraclecloud.com", viper.Get("loc.ns"), viper.Get("loc.region")))
		}

		if viper.Get("loc.vendor") == "baidu" {
			// http://s3.${BUCKET_REGION}.bcebos.com
			viper.Set("loc.endpoint", fmt.Sprintf("s3.%s.bcebos.com", viper.Get("loc.region")))
		}
	}

	viper.SetDefault("repo.pass", "mypass")
	viper.SetDefault("repo.cachedir", "/tmp/kopia-cache")
	viper.SetDefault("repo.configfile", "/tmp/kopia.config")
	viper.SetDefault("repo.enableThrottle", false)
	viper.SetDefault("repo.parallelBackup", 5)
	viper.SetDefault("repo.parallelRestore", 1)
	viper.SetDefault("repo.jobTimeoutInH", 5)

	prefix := viper.GetString("loc.prefix")
	if prefix != "-" && !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/" //nolint
	}

	var cfg Config

	if err := viper.Unmarshal(&cfg); err != nil {
		logrus.Errorf("err unmarshal viper instance: %v", err)
	}

	//https://github.com:8443/ellis-chen/compute-cloud/-/issues/2326
	//hide the kopia password since exported env may change over time
	cfg.RepoConfig.Password = "JWTSuperSecretKeyJWTSuperSecretKeyJWTSuperSecretKeyJWTSuperSecretKey"

	return &cfg
}
