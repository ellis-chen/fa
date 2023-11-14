package storage

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ellis-chen/fa/pkg/cipher"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"
)

type UploadService interface {
	InitUpload(key string, _size uint64, _chunkSize uint64, storageID int) (string, bool, error)
	LoadUpload(token string) (*Upload, error)
	LoadUploadByKey(key string) (*Upload, error)
	LoadUploads() ([]Upload, error)
	UploadPart(key string, idx uint64, md5 string) (string, error)
	LoadParts(token string) ([]UploadPart, error)
	CompletePart(key string, actualMD5 string) error
	DestroyUploadByToken(token string) error
	DestroyUploadByKey(key string) ([]string, error)
	Clean() error
}

// InitUpload ...
func (sm *StoreMgr) InitUpload(key string, _size uint64, _chunkSize uint64, storageID int) (string, bool, error) {
	var up Upload
	if rowsAffected := sm.DB.Where("key=?", key).Find(&up).RowsAffected; rowsAffected > 0 {
		if up.ChunkSize != _chunkSize || up.Size != _size {
			return up.Token, true, ErrPartUpCorrupted
		}
		if _, err := os.Lstat(up.TmpName); os.IsNotExist(err) && up.State == RUNNING {
			// tmp file not found, but upload state running
			return up.Token, true, ErrPartUpCorrupted
		}
		if up.State == DONE {
			fi, err := os.Lstat(up.Key)
			if os.IsNotExist(err) {
				return up.Token, true, ErrPartUpCorrupted
			}
			if fi.Size() != int64(up.Size) {
				return up.Token, true, ErrPartUpCorrupted
			}
		}

		return up.Token, true, nil
	}
	parent, file := filepath.Split(key)
	sm.mu.Lock()
	token, _ := GenerateID()
	res := sm.DB.Create(&Upload{
		Key:       key,
		Token:     token,
		Size:      _size,
		ChunkSize: _chunkSize,
		StorageID: storageID,
		TmpName:   fmt.Sprintf("%s/.%s.upload", parent, file),
	})
	sm.mu.Unlock()

	if res.Error != nil {
		return "", false, res.Error
	}

	return token, false, nil
}

// LoadUpload ...
func (sm *StoreMgr) LoadUpload(token string) (*Upload, error) {
	var persistUpload Upload
	if rowsAffected := sm.DB.Where("token=?", token).Find(&persistUpload).RowsAffected; rowsAffected > 0 {
		return &persistUpload, nil
	}

	return nil, ErrUploadNotFound
}

// LoadUploadByKey ...
func (sm *StoreMgr) LoadUploadByKey(key string) (*Upload, error) {
	var persistUpload Upload
	if rowsAffected := sm.DB.Where("key=?", key).Find(&persistUpload).RowsAffected; rowsAffected > 0 {
		return &persistUpload, nil
	}

	return nil, ErrUploadNotFound
}

// LoadUploads ..
func (sm *StoreMgr) LoadUploads() ([]Upload, error) {
	var persistUploads []Upload
	sm.DB.Unscoped().Find(&persistUploads)

	return persistUploads, nil
}

// UploadPart ...
func (sm *StoreMgr) UploadPart(key string, idx uint64, md5 string) (string, error) {
	var upload Upload
	res := sm.DB.Where("key=?", key).First(&upload)
	if res.Error != nil {
		return "", ErrUploadNotFound
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.DB.Debug().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "idx"}, {Name: "token"}},
		DoUpdates: clause.AssignmentColumns([]string{"token", "idx", "md5", "updated_at", "deleted_at"}),
	}).Create(&UploadPart{
		Token: upload.Token,
		Idx:   idx,
		MD5:   md5,
	})

	return "", nil
}

// LoadParts ...
func (sm *StoreMgr) LoadParts(token string) ([]UploadPart, error) {
	var upload Upload
	res := sm.DB.Where("token=?", token).Preload("Parts").Find(&upload)
	if res.Error != nil {
		return nil, ErrUploadNotFound
	}

	return upload.Parts, nil
}

// CompletePart ...
func (sm *StoreMgr) CompletePart(key string, actualMD5 string) error {
	var upload Upload
	res := sm.DB.Where("key=?", key).Preload("Parts").Find(&upload)
	if res.Error != nil {
		return ErrUploadNotFound
	}

	parts := upload.Parts
	sort.SliceStable(parts, func(i, j int) bool { return parts[i].Idx < parts[j].Idx })
	var byteSlice []byte
	for _, part := range parts {
		logrus.Debugf("part md5 from stored is: %v", part.MD5)
		// part.MD5
		decodeString, err := hex.DecodeString(part.MD5)
		if err != nil {
			return ErrInvalidMD5
		}
		byteSlice = append(byteSlice, decodeString...)
	}
	expectMD5, err := cipher.MD5Single(bytes.NewReader(byteSlice))
	if err != nil {
		return err
	}
	var decoratedExpectMD5 string
	if len(parts) == 0 {
		logrus.Errorf("no part preserved in db")

		return ErrCompletePart
	} else if len(parts) == 1 {
		decoratedExpectMD5 = parts[0].MD5
	} else {
		decoratedExpectMD5 = fmt.Sprintf("%s-%d", expectMD5, len(parts))
	}
	if decoratedExpectMD5 != actualMD5 {
		logrus.Errorf("expect MD5: %s; actual MD5: %s", decoratedExpectMD5, actualMD5)

		return ErrCompletePart
	}

	upload.MD5 = decoratedExpectMD5
	upload.State = DONE
	sm.Save(&upload)
	//um.DB.Unscoped().Debug().Where("token = ?", upload.Token).Delete(&UploadPart{})
	return nil
}

// DestroyUploadByToken ...
func (sm *StoreMgr) DestroyUploadByToken(token string) error {
	res := sm.DB.Unscoped().Debug().Where("Token = ?", token).Delete(&UploadPart{})
	if res.Error != nil {
		return res.Error
	}

	var up Upload
	sm.DB.Unscoped().Debug().Where("token=?", token).Find(&up)
	logrus.Infof("Delete file of the upload file [ %v ] with token [ %v ]", up.TmpName, token)
	_ = os.Remove(up.TmpName)
	res = sm.DB.Unscoped().Debug().Delete(&up)

	return res.Error
}

// DestroyUploadByKey ...
func (sm *StoreMgr) DestroyUploadByKey(key string) ([]string, error) {
	var uploads []Upload
	var uploadPath = make([]string, 0)

	// delete file(folder) and its descedent
	res := sm.DB.Debug().Where("key LIKE ?", fmt.Sprintf("%s%%", key)).Find(&uploads)
	if res.Error != nil {
		return uploadPath, res.Error
	}
	for _, upload := range uploads {
		uploadPath = append(uploadPath, upload.TmpName)
		sm.DB.Unscoped().Debug().Where("token=?", upload.Token).Delete(&UploadPart{})
		res = sm.DB.Unscoped().Debug().Delete(upload)
		if res.Error != nil {
			return uploadPath, res.Error
		}
	}

	return uploadPath, nil
}

// Clean delete the upload with done status, leave the unfinished upload intact
func (sm *StoreMgr) Clean() error {
	var persistUploads []Upload
	sm.DB.Unscoped().Debug().Where("state=?", DONE).Find(&persistUploads)

	for _, upload := range persistUploads {
		sm.DB.Unscoped().Debug().Where("token=?", upload.Token).Delete(&UploadPart{})
		sm.DB.Unscoped().Debug().Delete(upload)
	}

	return nil
}

// ScavengeUpload scan the long-running upload, and delete it and its' uploadPart
// it also clean the already finished uploads
func (sm *StoreMgr) ScavengeUpload() {
	var persistUploads []UploadThin
	sm.DB.Unscoped().Model(&Upload{}).Where("created_at < ?", time.Now().Add(time.Hour*-24)).Find(&persistUploads)

	for _, upload := range persistUploads {
		logrus.Infof("Delete file of the long running (or finished) upload file [ %s ] with key [ %s ] and status [ %v ]", upload.TmpName, upload.Key, upload.State)
		_ = os.Remove(upload.TmpName)
		sm.DB.Unscoped().Debug().Where("token=?", upload.Token).Delete(&UploadPart{})
		sm.DB.Unscoped().Debug().Where("token=?", upload.Token).Delete(&Upload{})
	}
}
