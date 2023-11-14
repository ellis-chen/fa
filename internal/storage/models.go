package storage

import (
	"fmt"
	"time"

	"github.com/ellis-chen/fa/pkg/fsutils"
	"gorm.io/gorm"
)

type (
	// Upload reprensents an multipart upload
	Upload struct {
		gorm.Model
		Token     string `gorm:"unique"`
		Key       string `gorm:"unique"`
		TmpName   string
		StorageID int
		Size      uint64
		ChunkSize uint64
		MD5       string
		Parts     []UploadPart `gorm:"foreignKey:Token;references:Token;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
		State     UploadState  `sql:"type:upload_state" gorm:"default:running"`
	}

	// UploadThin a thin wrapper of the Upload
	UploadThin struct {
		Token   string
		Key     string
		TmpName string
		State   UploadState
	}

	// UploadPart represents an part upload
	UploadPart struct {
		gorm.Model
		Token string `gorm:"uniqueIndex:part_idx"`
		Idx   uint64 `gorm:"uniqueIndex:part_idx"`
		MD5   string
	}

	// UploadParts implements sort.Interface based on the Idx field.
	UploadParts []UploadPart

	// UploadState ...
	// https://github.com/go-gorm/gorm/issues/1978#issuecomment-476673540
	UploadState string

	// AppTrialApply ...
	AppTrialApply struct {
		gorm.Model
		Name     string      `gorm:"uniqueIndex:appnameIdx"`
		Path     string      `gorm:"uniqueIndex:pathIdx"`
		State    UploadState `sql:"type:upload_state" gorm:"default:running"`
		Reported bool
	}

	// CopyLine represents a copy from src_path:src_storage_id (hello.txt:-1) to dst_storage_id (hello_dir:1)
	CopyLine struct {
		gorm.Model
		Group   string // unique identify a group of copy
		SrcPath string
		DstPath string
		SrcID   int         `gorm:"default:-1;default:(-)"` // default:(-) tell the migrator not migrate such default value (-1) for such table
		DstID   int         `gorm:"default:-1;default:(-)"`
		State   UploadState `sql:"type:upload_state" gorm:"default:running"`
		Extra   string      // error message etc
	}

	BackupState string

	BackupItem struct {
		gorm.Model
		BackupID     int `gorm:"uniqueIndex:backupIdIdx"` // unique identity of a backup action
		Path         string
		ManifestID   string      // kopia or other backup provider specific id
		ManifestSize uint64      // size of the manifest
		SnapshotSize uint64      // snapshoft size
		State        BackupState `sql:"type:backup_state" gorm:"default:running"`
		Extra        string      // error message etc
		Username     string
	}

	RestoreState string

	RestoreItem struct {
		gorm.Model
		RestoreID   int          `gorm:"uniqueIndex:restoreIdIdx"` // unique identity of a backup action
		BackupID    int          // identity of a backup action
		Dst         string       // destination path
		ManifestID  string       // kopia or other backup provider specific id
		RestoreSize uint64       // size of the restore
		State       RestoreState `sql:"type:restore_state" gorm:"default:running"`
		Extra       string       // error message etc
	}

	UsageType string

	UsageItem struct {
		gorm.Model
		UsageID      string `gorm:"uniqueIndex:usageIdIdx"`
		Used         uint64
		TenantID     int64
		UserID       int
		FileSystemID string
		CloudAccount string
		StartTime    time.Time
		EndTime      time.Time
		Name         string
		From         string
		To           string
		ClientIP     string

		UsageType UsageType `sql:"type:usage_type"`
	}
)

func (p *Upload) String() string {
	return fmt.Sprintf("Key: %s, TmpName: %s, Size: %.2fMB, ChunkSize: %.2fMB, Token: %s", p.Key, p.TmpName, float64(p.Size)/float64(fsutils.MB), float64(p.ChunkSize)/float64(fsutils.MB), p.Token)
}

func (a UploadParts) Len() int           { return len(a) }
func (a UploadParts) Less(i, j int) bool { return a[i].Idx < a[j].Idx }
func (a UploadParts) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

const (
	// RUNNING upload running
	RUNNING UploadState = "running"
	// DONE upload finish successfully
	DONE UploadState = "done"
	// FAILED upload failed
	FAILED UploadState = "failed"

	BackupPending BackupState = "pending"
	// BackupRunning backup running
	BackupRunning BackupState = "running"
	BackupSuccess BackupState = "success"
	BackupFailed  BackupState = "failed"
	BackupCancel  BackupState = "cancel"

	RestorePending RestoreState = "pending"
	// RestoreRunning restore running
	RestoreRunning RestoreState = "running"
	RestoreSuccess RestoreState = "success"
	RestoreFailed  RestoreState = "failed"
	RestoreCancel  RestoreState = "cancel"

	UsageTypeEfs      UsageType = "efs-usage"
	UsageTypeBackup   UsageType = "backup-usage"
	UsageTypeTransfer UsageType = "transfer-usage"
)
