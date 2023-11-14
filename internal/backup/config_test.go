package backup

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCfg(t *testing.T) {
	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	// required to be set in the environment
	t.Setenv("FB_FULLMODE", "true")
	t.Setenv("FB_LOC_TYPE", "s3Compliant")
	t.Setenv("FB_LOC_VENDOR", "aws")                // aws, oracle or baidu
	t.Setenv("FB_LOC_ENDPOINT", "s3.amazonaws.com") // optional
	t.Setenv("FB_LOC_REGION", "cn-northwest-1")
	t.Setenv("FB_LOC_PREFIX", "tenant_id") // tenant id
	t.Setenv("FB_LOC_NS", "oracle ns")     // namespace, oracle must provide, use - to left empty
	t.Setenv("FB_LOC_BUCKET", "kopia-backup-center")

	// s3 credential
	t.Setenv("FB_CRED_KEYPAIR_KEY", "this is s3 access key")
	t.Setenv("FB_CRED_KEYPAIR_SECRET", "this is s3 secret key")

	// optional to be set
	t.Setenv("FB_REPO_CACHEDIR", "/tmp/kopia-cache")
	t.Setenv("FB_REPO_CONFIGFILE", "/tmp/mykopia.config")

	cfg := LoadCfg()

	require.Equal(t, true, cfg.FullMode, "fullMode not match")
	require.Equal(t, VendorType("aws"), cfg.Location.VendorType, "vendor not match")
	require.Equal(t, "this is s3 access key", cfg.Credential.KeyPair.AccessID, "accessID not match")
	require.Equal(t, "this is s3 secret key", cfg.Credential.KeyPair.SecretKey, "secretKey not match")

	require.Equal(t, "s3.amazonaws.com", cfg.Location.Endpoint, "endpoint not match")
	require.Equal(t, "kopia-backup-center", cfg.Location.Bucket, "bucket not match")
	require.Equal(t, "tenant_id", cfg.Location.Prefix, "prefix not match")
	require.Equal(t, "cn-northwest-1", cfg.Location.Region, "region not match")

	require.Equal(t, "/tmp/kopia-cache", cfg.RepoConfig.CacheDir, "cachedir not match")
	require.Equal(t, "/tmp/mykopia.config", cfg.RepoConfig.ConfigFile, "configFile not match")
}
