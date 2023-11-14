package backup

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/ellis-chen/fa/pkg/fsutils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
)

var (
	bucketPath  = "/tmp/bucket"
	dataPath    = "/tmp/test-backup"
	restorePath = "/tmp/restore"
	backupID    = 1
	restoreID   = 1
)

type backupHarness struct {
	suite.Suite

	provider Provider
	ctx      context.Context
	cancel   context.CancelFunc

	username string
}

func (h *backupHarness) SetupSuite() {
	err := os.RemoveAll(bucketPath)
	h.NoError(err)

	err = os.RemoveAll(dataPath)
	h.NoError(err)

	err = os.RemoveAll(restorePath)
	h.NoError(err)

	err = os.MkdirAll(dataPath, 0777)
	h.NoError(err)

	err = os.MkdirAll(restorePath, 0777)
	h.NoError(err)

	logrus.SetLevel(logrus.DebugLevel)

	cfg := LoadCfg()
	cfg.Location.Type = LocationTypeFilesytem
	cfg.Location.Bucket = bucketPath
	cfg.RepoConfig.Password = "pass"

	h.ctx, h.cancel = context.WithCancel(context.TODO())

	h.provider, err = NewProvider(h.ctx, ProviderInfo{Type: "kopia", Config: cfg})
	h.NoError(err)

	err = h.provider.Start(h.ctx)
	h.NoError(err)

	user, ok := os.LookupEnv("USER")
	if !ok {
		h.username = "unknown"
	}
	h.username = user
}

func (h *backupHarness) TeardownSuite() {
	h.cancel()

	h.provider.Close(h.ctx)
}

func TestHarness(t *testing.T) {
	suite.Run(t, new(backupHarness))
}

func (h *backupHarness) TestBackup() {
	tmpFile, err := os.CreateTemp(dataPath, "data")
	h.NoError(err)

	_, err = io.CopyN(tmpFile, rand.New(rand.NewSource(time.Now().Unix())), 128*1024*1024)
	h.NoError(err)

	path := tmpFile.Name()
	req := &Request{ID: backupID, Path: path, Username: h.username}
	si, err := h.provider.BackupFile(h.ctx, req)
	h.NoError(err)
	h.NotNil(si)

	tags := req.Tags()
	sis, err := h.provider.ListBackups(h.ctx, path, tags)
	h.Nil(err)
	h.Len(sis, 1)

	h.Equal(si.SnapshotSize, sis[0].SnapshotSize)
}

func md5Compute(r io.Reader) (string, error) {
	hash := md5.New()
	_, err := io.Copy(hash, r)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func md5ComputeFile(p string) (string, error) {
	file, err := os.Open(p)
	if err != nil {
		return "", nil
	}
	return md5Compute(file)
}

func (h *backupHarness) TestRestoreFile() {
	var (
		md5Val string
		err    error
	)

	bs := bytes.Repeat([]byte{1}, (128 * 1024 * 1024))
	r := bytes.NewBuffer(bs)

	origFile, err := os.CreateTemp(dataPath, "data")
	h.NoError(err)

	tr := io.TeeReader(r, origFile)
	md5Val, err = md5Compute(tr)
	h.NoError(err)

	h.Equal("5be54a3ca1c0eb6d6286d67d82021035", md5Val)

	path := origFile.Name()
	req := &Request{ID: backupID, Path: path, Username: h.username}
	si, err := h.provider.BackupFile(h.ctx, req)
	h.NoError(err)
	h.NotNil(si)

	restoreFile, err := ioutil.TempFile(restorePath, "fa-kopia-restore")
	h.NoError(err)

	err = h.provider.RestoreFile(h.ctx, restoreID, backupID, restoreFile.Name())
	h.NoError(err)

	logrus.Infof("Restored filename : %v", restoreFile.Name())

	md5Val, err = md5Compute(restoreFile)
	h.NoError(err)
	h.Equal("5be54a3ca1c0eb6d6286d67d82021035", md5Val)
}

func (h *backupHarness) TestWriteTwice() {
	var (
		md5Val string
		err    error
	)

	bs := bytes.Repeat([]byte{1}, (128 * 1024 * 1024))
	r := bytes.NewBuffer(bs)

	origFile, err := os.CreateTemp(dataPath, "data")
	h.NoError(err)

	tr := io.TeeReader(r, origFile)
	md5Val, err = md5Compute(tr)
	h.NoError(err)
	fi, err := origFile.Stat()
	h.NoError(err)
	logrus.Infof("name is %s, size is %s", origFile.Name(), fsutils.ToHumanBytes(fi.Size()))

	h.Equal("5be54a3ca1c0eb6d6286d67d82021035", md5Val)

	bs = bytes.Repeat([]byte{1}, (128 * 1024 * 1024))
	r = bytes.NewBuffer(bs)

	tr = io.TeeReader(r, origFile)
	md5Val, err = md5Compute(tr)
	h.NoError(err)
	h.Equal("5be54a3ca1c0eb6d6286d67d82021035", md5Val)

	fi, err = origFile.Stat()
	h.NoError(err)
	logrus.Infof("name is %s, size is %s", origFile.Name(), fsutils.ToHumanBytes(fi.Size()))

	md5Val, err = md5ComputeFile(origFile.Name())
	h.NoError(err)
	logrus.Infof("md5 is %s", md5Val)
	h.Equal("e947fc62b011b8e33fe5c91f9f857217", md5Val)
}

func (h *backupHarness) TestRestoreLatest() {
	var (
		md5Val string
		err    error
	)

	bs := bytes.Repeat([]byte{1}, (128 * 1024 * 1024))
	r := bytes.NewBuffer(bs)

	origFile, err := os.CreateTemp(dataPath, "data")
	h.NoError(err)

	tr := io.TeeReader(r, origFile)
	md5Val, err = md5Compute(tr)
	h.NoError(err)

	h.Equal("5be54a3ca1c0eb6d6286d67d82021035", md5Val)

	path := origFile.Name()
	req := &Request{ID: backupID, Path: path, Username: h.username}
	si, err := h.provider.BackupFile(h.ctx, req)
	h.NoError(err)
	h.NotNil(si)

	bs = bytes.Repeat([]byte{1}, (128 * 1024 * 1024))
	r = bytes.NewBuffer(bs)

	tr = io.TeeReader(r, origFile)
	md5Val, err = md5Compute(tr)
	h.NoError(err)
	h.Equal("5be54a3ca1c0eb6d6286d67d82021035", md5Val) // 128M md5

	md5Val, err = md5Compute(origFile)
	h.NoError(err)
	h.Equal("d41d8cd98f00b204e9800998ecf8427e", md5Val) // 256M md5

	si, err = h.provider.BackupFile(h.ctx, req)
	h.NoError(err)
	h.NotNil(si)

	restoreFile, err := ioutil.TempFile(restorePath, "fa-kopia-restore")
	h.NoError(err)

	err = h.provider.RestoreLatest(h.ctx, req.Tags(), restoreFile.Name())
	h.NoError(err)

	logrus.Infof("Restored filename : %v", restoreFile.Name())

	md5Val, err = md5ComputeFile(restoreFile.Name())
	h.NoError(err)
	h.Equal("e947fc62b011b8e33fe5c91f9f857217", md5Val)
}

func (h *backupHarness) TestDeleteBackup() {
	var (
		err error
	)

	bs := bytes.Repeat([]byte{1}, (128 * 1024 * 1024))
	r := bytes.NewBuffer(bs)

	origFile, err := os.CreateTemp(dataPath, "data")
	h.NoError(err)

	_, err = io.Copy(origFile, r)
	h.NoError(err)

	path := origFile.Name()
	req := &Request{ID: backupID, Path: path, Username: h.username}
	si, err := h.provider.BackupFile(h.ctx, req)
	h.NoError(err)
	h.NotNil(si)

	tags := req.Tags()
	sis, err := h.provider.ListBackups(h.ctx, path, tags)
	h.Nil(err)
	h.Len(sis, 1)

	err = h.provider.DeleteBackup(h.ctx, backupID)
	h.NoError(err)

	sis, err = h.provider.ListBackups(h.ctx, path, tags)
	h.Nil(err)
	h.Len(sis, 0)
}

func (h *backupHarness) TestKopiaMaintenance() {

}

func (h *backupHarness) TestKopiaDeleteManifest() {
}
