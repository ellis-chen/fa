package storage

import (
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"
)

type ApplyService interface {
	LoadTrialAppApply(string) (*AppTrialApply, error)
	LoadBootstrapRunningAppApply() ([]*AppTrialApply, error)
	AddAppTrialApply(string, string) error
	DoneAppTrialApply(string, bool) error
	DeleteAppTrialApply(string) error
	ScavengeUpload()
	ScavengeTrialApp(func(string, string) error)
}

// LoadTrialAppApply load any trial app apply based on the appname
func (sm *StoreMgr) LoadTrialAppApply(appname string) (*AppTrialApply, error) {
	var appTrialApply AppTrialApply
	if rowsAffected := sm.DB.Where("name=?", appname).Find(&appTrialApply).RowsAffected; rowsAffected > 0 {
		return &appTrialApply, nil
	}

	return nil, ErrAppNotActivated
}

// LoadBootstrapRunningAppApply load all running trial apply, should only be called at bootstrap time once
func (sm *StoreMgr) LoadBootstrapRunningAppApply() ([]*AppTrialApply, error) {
	var persistApplies []*AppTrialApply
	tx := sm.DB.Unscoped().Debug().Where("state = ? and reported = false", RUNNING).Find(&persistApplies)

	return persistApplies, tx.Error
}

// AddAppTrialApply record an app trial apply
func (sm *StoreMgr) AddAppTrialApply(appname string, path string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	res := sm.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "path", "reported", "state", "updated_at"}),
	}).Create(&AppTrialApply{
		Name:  appname,
		Path:  path,
		State: RUNNING,
	})

	return res.Error
}

// DoneAppTrialApply ...
func (sm *StoreMgr) DoneAppTrialApply(appname string, success bool) error {
	var appState = DONE
	if !success {
		appState = FAILED
	}
	res := sm.DB.Model(&AppTrialApply{}).Where("name = ?", appname).Update("state", appState)

	return res.Error
}

// DeleteAppTrialApply ...
func (sm *StoreMgr) DeleteAppTrialApply(appname string) error {
	res := sm.DB.Unscoped().Debug().Where("name = ?", appname).Delete(&AppTrialApply{})

	return res.Error
}

// ScavengeTrialApp scan the long-running trial app and handle it with the given callback
func (sm *StoreMgr) ScavengeTrialApp(f func(string, string) error) {
	if f == nil {
		f = func(name string, _ string) error {
			logrus.Infof("find abnormal trial software applicance: %s, just gonna silencely ignored", name)

			return nil
		}
	}

	var trialApplies []AppTrialApply
	sm.DB.Unscoped().Debug().Where("updated_at < ? and state = ?", time.Now().Add(time.Hour*-6), RUNNING).Find(&trialApplies)

	var err error
	for _, apply := range trialApplies {
		if err = f(apply.Name, apply.Path); err == nil {
			sm.DB.Unscoped().Debug().Delete(apply)
		}
	}
}
