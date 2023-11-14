package storage

import "gorm.io/gorm/clause"

type BillManager interface {
	NewUsage(usage *UsageItem) error

	LoadEfsUsages() ([]*UsageItem, error)
	LoadBackupUsages() ([]*UsageItem, error)
	LoadTransferUsage() ([]*UsageItem, error)

	DeleteUsage(usageID string) error
}

func (sm *StoreMgr) NewUsage(usage *UsageItem) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	res := sm.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "usage_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"used", "tenant_id",
			"user_id", "file_system_id",
			"cloud_account", "name",
			"start_time", "end_time",
			"from", "to",
			"updated_at", "client_ip",
		}),
	}).Create(usage)

	return res.Error
}

func (sm *StoreMgr) LoadEfsUsages() ([]*UsageItem, error) {
	var items []*UsageItem
	tx := sm.DB.Where("usage_type = ? ", UsageTypeEfs).Find(&items)

	return items, tx.Error
}

func (sm *StoreMgr) LoadBackupUsages() ([]*UsageItem, error) {
	var items []*UsageItem
	tx := sm.DB.Where("usage_type = ? ", UsageTypeBackup).Find(&items)

	return items, tx.Error
}

func (sm *StoreMgr) LoadTransferUsage() ([]*UsageItem, error) {
	var items []*UsageItem
	tx := sm.DB.Where("usage_type = ? ", UsageTypeTransfer).Find(&items)

	return items, tx.Error
}

func (sm *StoreMgr) DeleteUsage(usageID string) error {
	var usageItem UsageItem
	if rowsAffected := sm.DB.Where(&UsageItem{UsageID: usageID}).Find(&usageItem).RowsAffected; rowsAffected > 0 {
		res := sm.DB.Unscoped().Delete(&usageItem)

		return res.Error
	}

	return nil
}

var _ BillManager = (*StoreMgr)(nil)
