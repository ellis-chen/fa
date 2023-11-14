package storage

import (
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BackupService interface {
	// NewBackup creates a new backup for the given filesystem.
	NewBackup(int, string, string, ...BackupState) error
	// DoneBackup marks the backup as completed.
	DoneBackup(backupID int, detail *BackupDetail, state BackupState, extra ...string) error
	// LoadBackups loads a backup with given backupID.
	LoadBackup(int) (*BackupItem, error)
	// LoadPendingOrRunningBackup loads a backup by path and BackupPending/BackupRunning state
	LoadPendingOrRunningBackup(string) (*BackupItem, error)
	// LoadBackups loads all backups with the given state.
	LoadBackups(state BackupState) ([]*BackupItem, error)
	// DeleteBackup deletes the backup with the given id.
	DeleteBackup(int) error
	// DeleteBackups will delete all the backups
	DeleteBackups() error
	// NewRestore creates a new restore for the given filesystem.
	NewRestore(restoreID, backupID int, dst string, state ...RestoreState) error
	// DoneRestore marks the restore as completed and if success, provide the actual restore size.
	DoneRestore(int, int64, RestoreState, ...string) error
	// LoadRestores loads the given restore with the given restoreID.
	LoadRestore(int) (*RestoreItem, error)
	// LoadRestores loads all restores with the given state.
	LoadRestores(state RestoreState) ([]*RestoreItem, error)
	// LoadRunningRestore load running restore of a given backupID
	LoadPendingOrRunningRestore(int) ([]*RestoreItem, error)
	// DeleteRestores will delete all the restores
	DeleteRestores() error
	// ScavengeBackup will scavenge all the long running( -12 h) backup or restore and if found any, make them failed
	ScavengeBackup() error
}

type BackupDetail struct {
	// ID is the snapshot ID produced by kopia snapshot operation
	ID string `json:"id"`
	// SnapshotSize is the size of the snapshot in bytes
	SnapshotSize int64 `json:"snapshotSize"`
	// ManifestSize is the size of the manifest in bytes
	ManifestSize int64 `json:"manifestSize"`
}

func (sm *StoreMgr) NewBackup(id int, path string, username string, backupState ...BackupState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	state := BackupRunning
	if len(backupState) > 0 {
		state = backupState[0]
	}

	res := sm.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "backup_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"path", "state", "updated_at", "extra"}),
	}).Create(&BackupItem{
		BackupID: id,
		Path:     path,
		State:    state,
		Extra:    "",
		Username: username,
	})

	return res.Error
}

func (sm *StoreMgr) DoneBackup(backupID int, snapshotInfo *BackupDetail, state BackupState, extra ...string) error {
	updated := BackupItem{
		State:        state,
		ManifestID:   snapshotInfo.ID,
		ManifestSize: uint64(snapshotInfo.ManifestSize),
		SnapshotSize: uint64(snapshotInfo.SnapshotSize),
	}
	if len(extra) > 0 {
		updated.Extra = extra[0]
	}
	res := sm.DB.Model(&BackupItem{}).Where(&BackupItem{BackupID: backupID}).Updates(updated)

	return res.Error
}

func (sm *StoreMgr) LoadBackup(id int) (*BackupItem, error) {
	var backupItem BackupItem
	if rowsAffected := sm.DB.Where(&BackupItem{BackupID: id}).Find(&backupItem).RowsAffected; rowsAffected > 0 {
		return &backupItem, nil
	}

	return nil, ErrNoSuchRecord
}

func (sm *StoreMgr) LoadPendingOrRunningBackup(path string) (*BackupItem, error) {
	var backupItem BackupItem
	if rowsAffected := sm.DB.Where(&BackupItem{Path: path, State: BackupPending}).Or(&BackupItem{Path: path, State: BackupRunning}).Find(&backupItem).RowsAffected; rowsAffected > 0 {
		return &backupItem, nil
	}

	return nil, ErrNoSuchRecord
}

func (sm *StoreMgr) LoadBackups(state BackupState) ([]*BackupItem, error) {
	var items []*BackupItem
	tx := sm.DB.Where("state = ? ", state).Find(&items)

	return items, tx.Error
}

func (sm *StoreMgr) DeleteBackup(id int) error {
	res := sm.DB.Unscoped().Debug().Where(&BackupItem{BackupID: id}).Delete(&BackupItem{})

	return res.Error
}

func (sm *StoreMgr) DeleteBackups() error {
	res := sm.DB.Unscoped().Debug().Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&BackupItem{})

	return res.Error
}

func (sm *StoreMgr) NewRestore(restoreID, backupID int, path string, restoreState ...RestoreState) (err error) {
	manifestID := ""
	item, err := sm.LoadBackup(backupID)
	if err == nil {
		manifestID = item.ManifestID
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	state := RestoreRunning
	if len(restoreState) > 0 {
		state = restoreState[0]
	}

	res := sm.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "restore_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"backup_id", "manifest_id", "state", "updated_at"}),
	}).Create(&RestoreItem{
		RestoreID:   restoreID,
		BackupID:    backupID,
		Dst:         path,
		RestoreSize: uint64(0),
		ManifestID:  manifestID,
		State:       state,
		Extra:       "",
	})

	return res.Error
}

func (sm *StoreMgr) DoneRestore(restoreID int, resoreSize int64, state RestoreState, extra ...string) error {
	updated := RestoreItem{State: state, RestoreSize: uint64(resoreSize)}
	if len(extra) > 0 {
		updated.Extra = extra[0]
	}
	res := sm.DB.Model(&RestoreItem{}).Where(&RestoreItem{RestoreID: restoreID}).Updates(updated)

	return res.Error
}

func (sm *StoreMgr) LoadRestores(state RestoreState) ([]*RestoreItem, error) {
	var items []*RestoreItem
	tx := sm.DB.Where("state = ? ", state).Find(&items)

	return items, tx.Error
}

func (sm *StoreMgr) LoadRestore(id int) (*RestoreItem, error) {
	var restoreItem RestoreItem
	if rowsAffected := sm.DB.Where(&RestoreItem{RestoreID: id}).Find(&restoreItem).RowsAffected; rowsAffected > 0 {
		return &restoreItem, nil
	}

	return nil, ErrNoSuchRecord
}

func (sm *StoreMgr) LoadPendingOrRunningRestore(backupID int) ([]*RestoreItem, error) {
	var restoreItems []*RestoreItem
	if rowsAffected := sm.DB.Where(&RestoreItem{BackupID: backupID, State: RestorePending}).Or(&RestoreItem{BackupID: backupID, State: RestoreRunning}).Find(&restoreItems).RowsAffected; rowsAffected > 0 {
		return restoreItems, nil
	}

	return nil, ErrNoSuchRecord
}

func (sm *StoreMgr) DeleteRestores() error {
	res := sm.DB.Unscoped().Debug().Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&RestoreItem{})

	return res.Error
}

func (sm *StoreMgr) ScavengeBackup() error {
	var backupItems []BackupItem
	sm.DB.Model(&BackupItem{}).Where("state = ? and updated_at < ?", BackupRunning, time.Now().Add(time.Hour*-12)).Find(&backupItems)

	for _, item := range backupItems {
		logrus.Warnf("Update long running backup [ %s ] with backup_id [ %v ] to failed", item.Path, item.BackupID)
		item.State = BackupFailed
		item.Extra = "scavenge canceled"
		res := sm.DB.Model(&BackupItem{}).Where("id = ?", item.ID).Updates(item)
		if res.Error != nil {
			logrus.Errorf("err update backupItem to failed due to: %v", res.Error.Error())
		}
	}

	var restoreItems []RestoreItem
	sm.DB.Model(&RestoreItem{}).Where("state = ? and updated_at < ?", RestoreRunning, time.Now().Add(time.Hour*-12)).Find(&restoreItems)

	for _, item := range restoreItems {
		logrus.Warnf("Update long running restore [ %s ] with restore_id [ %v ] and backup_id [ %v ] to failed", item.Dst, item.RestoreID, item.BackupID)
		item.State = RestoreFailed
		item.Extra = "scavenge canceled"
		res := sm.DB.Model(&RestoreItem{}).Where("id = ?", item.ID).Updates(item)
		if res.Error != nil {
			logrus.Errorf("err update restoreItem to failed due to: %v", res.Error.Error())
		}
	}

	return nil
}

var (
	ErrNoSuchRecord = errors.New("no such record")
)

var _ BackupService = (*StoreMgr)(nil)
