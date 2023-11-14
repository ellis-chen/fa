package storage

import (
	"time"

	"github.com/sirupsen/logrus"
)

type CopyLineService interface {
	AddCopyLine(string, int, string, int) error
	AddCopyLineG(string, int, string, int, string) error
	DoneCopyLine(string, int, string, int, bool) error
	DoneCopyLineG(string, int, string, int, bool, string, string) error
	LoadCopyLines(string, int, string, int) ([]CopyLine, error)
	LoadCopyLinesByG(string) ([]CopyLine, error)
	ScavengeCopyLine(func(string, int, string, int, bool) error)
}

func (sm *StoreMgr) AddCopyLine(srcPath string, srcStorageID int, dstPath string, dstStorageID int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	res := sm.DB.Create(&CopyLine{
		SrcPath: srcPath,
		SrcID:   srcStorageID,
		DstPath: dstPath,
		DstID:   dstStorageID,
		State:   RUNNING,
		Group:   "",
	})

	return res.Error
}

func (sm *StoreMgr) AddCopyLineG(srcPath string, srcStorageID int, dstPath string, dstStorageID int, group string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	res := sm.DB.Create(&CopyLine{
		SrcPath: srcPath,
		SrcID:   srcStorageID,
		DstPath: dstPath,
		DstID:   dstStorageID,
		State:   RUNNING,
		Group:   group,
	})

	return res.Error
}

func (sm *StoreMgr) DoneCopyLine(srcPath string, srcStorageID int, dstPath string, dstStorageID int, success bool) error {
	var state UploadState
	if success {
		state = DONE
	} else {
		state = FAILED
	}
	res := sm.DB.Debug().Model(&CopyLine{}).Where(&CopyLine{
		SrcPath: srcPath,
		SrcID:   srcStorageID,
		DstPath: dstPath,
		DstID:   dstStorageID,
	}).Update("state", state)

	return res.Error
}

func (sm *StoreMgr) DoneCopyLineG(srcPath string, srcStorageID int, dstPath string, dstStorageID int, success bool, group string, extra string) error {
	var state UploadState
	if success {
		state = DONE
	} else {
		state = FAILED
	}
	res := sm.DB.Debug().Model(&CopyLine{}).Where(&CopyLine{
		SrcPath: srcPath,
		SrcID:   srcStorageID,
		DstPath: dstPath,
		DstID:   dstStorageID,
		Group:   group,
	}).Updates(CopyLine{State: state, Extra: extra})

	return res.Error
}

func (sm *StoreMgr) LoadCopyLines(srcPath string, srcStorageID int, dstPath string, dstStorageID int) ([]CopyLine, error) {
	var copyLines []CopyLine
	res := sm.DB.Debug().Where(&CopyLine{
		SrcPath: srcPath,
		SrcID:   srcStorageID,
		DstPath: dstPath,
		DstID:   dstStorageID,
	}).Find(&copyLines)

	return copyLines, res.Error
}

func (sm *StoreMgr) LoadCopyLinesByG(group string) ([]CopyLine, error) {
	var copyLines []CopyLine
	res := sm.DB.Debug().Where(&CopyLine{
		Group: group,
	}).Find(&copyLines)

	return copyLines, res.Error
}

func (sm *StoreMgr) ScavengeCopyLine(f func(string, int, string, int, bool) error) {
	if f == nil {
		f = func(srcPath string, srcStorageID int, dstPath string, dstStorageID int, done bool) error {
			if !done {
				logrus.Errorf("err when copy from [ %d:%s ] to [ %d:%s ]", srcStorageID, srcPath, dstStorageID, dstPath)
			}

			return nil
		}
	}

	var err error
	var copyLines []CopyLine

	// running --> last for 12 hours
	sm.DB.Unscoped().Where("created_at < ? and state = ?", time.Now().Add(time.Hour*-12), RUNNING).Find(&copyLines)

	for _, copy := range copyLines {
		if err = f(copy.SrcPath, copy.SrcID, copy.DstPath, copy.DstID, false); err == nil {
			sm.DB.Unscoped().Debug().Delete(copy)
		}
	}

	// end state --> passed 6 hours
	sm.DB.Unscoped().Where("updated_at < ? and state != ?", time.Now().Add(time.Hour*-6), RUNNING).Find(&copyLines)
	for _, copy := range copyLines {
		if err = f(copy.SrcPath, copy.SrcID, copy.DstPath, copy.DstID, copy.State == DONE); err == nil {
			sm.DB.Unscoped().Debug().Delete(copy)
		}
	}
}
