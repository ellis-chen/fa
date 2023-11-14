package storage

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ellis-chen/fa/internal"

	"github.com/bwmarrin/snowflake"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (

	// ErrPartUpCorrupted file size changed or part size changed
	ErrPartUpCorrupted = errors.New("file changed or chunkSize changed since last upload")

	// ErrUploadNotFound ...
	ErrUploadNotFound = errors.New("not found given upload")

	// ErrCompletePart ...
	ErrCompletePart = errors.New("complete part error, md5 mismatch")

	// ErrInvalidMD5 ...
	ErrInvalidMD5 = errors.New("invalid part md5 found")

	storeMgrOnce sync.Once

	// +checklocks:storeMgrMutex
	storeMgr StoreManager

	storeMgrMutex sync.RWMutex
)

// StoreManager represents an generate for multipart upload
type StoreManager interface {
	GetConfig() *internal.Config
	Close() error
	UploadService
	CopyLineService
	ApplyService
	BackupService
	BillManager
	Chore
}

// StoreMgr manage the upload of file
type StoreMgr struct {
	*gorm.DB
	mu sync.RWMutex
	// +checklocks:mu
	conf *internal.Config
}

// NewStoreMgr return a StoreManager instance
func NewStoreMgr(conf *internal.Config) StoreManager {
	storeMgrOnce.Do(func() {
		ReloadStoreMgr(conf)
	})

	storeMgrMutex.RLock()
	defer storeMgrMutex.RUnlock()

	return storeMgr
}

func ReloadStoreMgr(conf *internal.Config) StoreManager {
	storeMgrMutex.Lock()
	defer storeMgrMutex.Unlock()

	storeMgr = createStoreManager(conf)

	return storeMgr
}

func createStoreManager(conf *internal.Config) StoreManager {
	dbname := conf.Dbname
	db, err := gorm.Open(sqlite.Open(dbname), &gorm.Config{
		PrepareStmt: true,
	})
	if err != nil {
		panic("failed to connect database")
	}
	// https://www.alexedwards.net/blog/configuring-sqldb
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	err = db.AutoMigrate(&Upload{}, &UploadPart{}, &AppTrialApply{}, &CopyLine{}, &BackupItem{}, &RestoreItem{}, &UsageItem{}, &BackupChore{})

	if err != nil {
		logrus.Errorf("err when migrating the database schema: %v", err)

		return nil
	}

	return &StoreMgr{
		DB:   db,
		conf: conf}
}

// Close ...
func (sm *StoreMgr) GetConfig() *internal.Config {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.conf
}

// Close ...
func (sm *StoreMgr) Close() error {
	logrus.Warnf("Closing StorageManager ...")
	sqlDB, _ := sm.Debug().DB()
	_ = sqlDB.Close()

	return nil
}

// GenerateID generate id
func GenerateID() (string, error) {
	node, err := snowflake.NewNode(rand.Int63n(1024))
	if err != nil {
		logrus.Error(err)

		return "", err
	}

	// Generate a snowflake ID.
	id := node.Generate()

	return fmt.Sprint(id), nil
}

var (
	_                  StoreManager = (*StoreMgr)(nil)
	ErrAppNotActivated              = errors.New("app apply not activated")
)
