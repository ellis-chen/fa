//go:build kopia
// +build kopia

package backup

import (
	"github.com/kopia/kopia/snapshot/snapshotfs"
	"github.com/sirupsen/logrus"
)

type FaUploadProgress struct {
	snapshotfs.NullUploadProgress
	id string
}

func (fup *FaUploadProgress) UploadStarted() {
	logrus.Infof("fa [%s] upload progress started ", fup.id)
}

func (fup *FaUploadProgress) UploadFinished() {
	logrus.Infof("fa [%s] upload progress finished ", fup.id)
}

var _ snapshotfs.UploadProgress = (*FaUploadProgress)(nil)
