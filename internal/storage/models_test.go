package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestUpload(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("foo.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	_ = db.AutoMigrate(&Upload{})

	// Create
	db.Create(&Upload{Key: "tmp/hello.tgz", Token: "hello"})

	// Read
	var upload Upload
	db.First(&upload, 1) // find product with integer primary key
	t.Logf("upload is %v", upload)
	db.First(&upload, "token = ?", "hello") // find product with code D42
	t.Logf("upload is %v", upload)

	// Update - update product's price to 200
	db.Model(&upload).Update("Size", 200)
	// Update - update multiple fields
	db.Model(&upload).Updates(Upload{ChunkSize: 5, MD5: "F42"}) // non-zero fields
	t.Logf("upload is %v", upload)

	db.Model(&upload).Updates(map[string]interface{}{"ChunkSize": 8, "MD5": "F4242"})
	t.Logf("upload is %v", upload)
	// Delete - delete product
	db.Delete(&upload, 1)

	db.Create(&Upload{
		Key: "tmp/hello.tgz", Token: "hello",
		Parts: []UploadPart{{}},
	})
}

func TestAssoication(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("foo.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	_ = db.AutoMigrate(&Upload{}, &UploadPart{})

	db.Delete(&Upload{}, 1)

	db.Where("token=?", "hello").Delete(&Upload{})

	db.Create(&Upload{
		Key: "tmp/hello.tgz", Token: "hello",
		Parts: []UploadPart{{
			Idx: 1,
			MD5: "md5-1",
		}, {
			Idx: 2,
			MD5: "md5-2",
		}, {
			Idx: 3,
			MD5: "md5-3",
		}},
	})
	var upload Upload
	db.First(&upload, 1) // find product with integer primary key
	t.Logf("upload is %v", upload)
	t.Logf("upload parts is %v", upload.Parts)

	db.Preload("Parts").Find(&upload)
	t.Logf("upload is %v", upload)

	// db.Where("token=?", "hello").Delete(&UploadPart{})

	var uploads []Upload
	// preload all the associations
	db.Preload(clause.Associations).Find(&uploads)
	t.Logf("uploads is %v", uploads)

	// customized function preload
	db.Preload("Parts", func(db *gorm.DB) *gorm.DB {
		return db.Order("Idx DESC")
	}).Find(&uploads)
	t.Logf("uploads is %v", uploads)

	db.Where("token=?", "hello").Preload("Parts").Find(&uploads)
	t.Logf("uploads is %v", uploads)
}

func TestDelete(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("foo.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	_ = db.AutoMigrate(&Upload{}, &UploadPart{})

	// tx := db.Begin()

	// // Delete - delete user
	// //tx.Delete(&user)

	// if err := tx.Delete(&Upload{}, 1).Error; err != nil {
	// 	fmt.Print(err)
	// 	tx.Rollback()
	// } else {
	// 	fmt.Printf("Rows affected: %d", tx.RowsAffected) // Always returns 0
	// 	tx.Commit()
	// }

	// db.Delete(&Upload{}, 1)
	db.Where("Token = ?", "hello").Delete(&Upload{})
	db.Unscoped().Where("Token = ?", "hello").Delete(&Upload{})
}

func TestToken(t *testing.T) {
	var mm = make(map[string]bool, 1_000_000)
	for i := 0; i < len(mm); i++ {
		k, e := GenerateID()
		if e != nil {
			t.Fatal(e)
		}
		if mm[k] == true {
			t.Fatalf("duplicated key detected: %v", k)
		}
		mm[k] = true
	}
}

func TestChore(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("foo.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	_ = db.AutoMigrate(&BackupChore{})

	// Create
	db.Create(&BackupChore{Path: "/abc", Username: "auto"})

	chore := BackupChore{}
	db.First(&chore, "path = ?", "/abc")

	chore.SnapshotSize = 1024
	db.Model(&BackupChore{}).Where("id = ?", chore.ID).Save(chore)

	chores := make([]*BackupChore, 0)
	err = db.Model(&BackupChore{}).Where("path =?", "/abc").Find(&chores).Error
	require.Nil(t, err)
	require.Len(t, chores, 1)
}
