package storage

import (
	"strconv"
	"time"

	"gorm.io/gorm"
)

type Chore interface {
	NewBChore(c *BackupChore) error
	QueryOneChore(id int) *BackupChore
	QueryLatestChore() *BackupChore
	QueryBChore(start uint64) []*BackupChore
	UpdateChore(u *BackupChore) error
	ChgBChoreState(id uint, state ChoreState) error
	ScavengeChores() error
}

var _ Chore = (*StoreMgr)(nil)

type BackupChore struct {
	gorm.Model
	Path         string
	ManifestID   string // kopia or other backup provider specific id
	ManifestSize uint64 // size of the manifest
	SnapshotSize uint64 // snapshoft size
	State        ChoreState
	Extra        string // error message etc
	Username     string
}

func (c *BackupChore) ToTags() map[string]string {
	return map[string]string{"tag:author": c.Username, "tag:path": c.Path, "tag:id": strconv.Itoa(int(c.ID))}
}

//go:generate go run github.com/abice/go-enum -f=$GOFILE --marshal

// ChoreState represent the state of chore backup
/** ENUM(
      INVALID,
      RUNNING, // backup running
	  FAILED, // backup failed
      FINISHED, // backup finished
      ACKED, // backup acked
)
*/
type ChoreState int

func (sm *StoreMgr) NewBChore(c *BackupChore) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.DB.Create(c).Error
}
func (sm *StoreMgr) QueryOneChore(id int) *BackupChore {
	var r BackupChore
	if res := sm.DB.Where("id = ?", id).First(&r); res.Error != nil {
		return nil
	}
	return &r
}

func (sm *StoreMgr) QueryLatestChore() *BackupChore {
	var r BackupChore
	if res := sm.DB.Model(&BackupChore{}).Where("id>0").Last(&r); res.Error != nil {
		return nil
	}
	return &r
}

func (sm *StoreMgr) QueryBChore(start uint64) []*BackupChore {
	var r []*BackupChore
	sm.DB.Where("created_at > ?", start).Find(&r)
	return r
}
func (sm *StoreMgr) UpdateChore(u *BackupChore) error {
	return sm.DB.Model(&BackupChore{}).Where("id = ?", u.ID).Save(u).Error
}
func (sm *StoreMgr) ChgBChoreState(id uint, state ChoreState) error {
	res := sm.DB.Model(&BackupChore{}).Where("id = ?", id).Update("state", state)
	return res.Error
}

func (sm *StoreMgr) ScavengeChores() error {
	return sm.DB.Model(&BackupChore{}).Where("updated_at < ?", time.Now().Add(-time.Hour*24*30)).Delete(&BackupChore{}).Error
}
