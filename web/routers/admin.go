package routers

import (
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/internal/storage"
	"github.com/ellis-chen/fa/pkg/fsutils"
	"github.com/ellis-chen/fa/pkg/iotools"
	"github.com/ellis-chen/fa/pkg/queue"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type (
	injectKeyReq struct {
		PubKey string `json:"pub_key" binding:"required"`
	}
	trialAppReq struct {
		AppName string `json:"app_name" form:"app_name" binding:"required"`
	}
)

func cleanUploadStatus(c *gin.Context) {
	cloudStorage := getCloudStorage(c)
	err := cloudStorage.store.Clean()
	if err != nil {
		_ = c.Error(err)

		return
	}
}

func injectPubKey(c *gin.Context) {
	injectKeyReq := injectKeyReq{}
	if err := c.ShouldBindJSON(&injectKeyReq); err != nil {
		_ = c.Error(err)

		return
	}
	cloudStorage := getCloudStorage(c)
	script := fsutils.Render(injectPubKeyTemplate, &map[string]string{
		"homedir": cloudStorage.SystemRootPrefix,
		"pub_key": injectKeyReq.PubKey,
		"uid":     fmt.Sprint(cloudStorage.userPrincipal.LinuxUID),
		"gid":     fmt.Sprint(cloudStorage.userPrincipal.LinuxGID),
	})

	logger.Debugf(cloudStorage.ctx, "inject public key: \n%v\n", script)

	_, _, err := fsutils.ExecCmdInShell(script)
	if err != nil {
		logger.Errorf(cloudStorage.ctx, "err when inject public key: %s", err.Error())
		_ = c.Error(err)

		return
	}
	c.JSON(http.StatusOK, DefaultSuccess)
}

func activateTrialApp(c *gin.Context) {
	trialAppReq := trialAppReq{}
	if err := c.ShouldBindJSON(&trialAppReq); err != nil {
		_ = c.Error(err)

		return
	}
	cs := getCloudStorage(c)
	err := cs.activateTrialApp(&trialAppReq)
	if err != nil {
		logger.Errorf(cs.ctx, "err when activate trial app [ %v ], detail is: %s", err.Error())
		_ = c.Error(err)

		return
	}
	c.JSON(http.StatusOK, DefaultSuccess)
}

func deactivateTrialApp(c *gin.Context) {
	trialAppReq := trialAppReq{}
	if err := c.ShouldBindJSON(&trialAppReq); err != nil {
		_ = c.Error(err)

		return
	}
	cs := getCloudStorage(c)
	err := cs.deactivateTrialApp(&trialAppReq)
	if err != nil {
		logger.Errorf(cs.ctx, "err when deactivate trial app [ %v ], detail is: %s", err.Error())
		_ = c.Error(err)

		return
	}
	c.JSON(http.StatusOK, DefaultSuccess)
}

func trialProgress(c *gin.Context) {
	req := trialAppReq{}
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(err)

		return
	}
	cs := getCloudStorage(c)
	trialApply, err := cs.trialProgress(&req)
	if err != nil {
		errMsg := fmt.Sprintf("err when query trial app [ %v ] progress, detail is: %s", req.AppName, err.Error())
		logger.Errorf(cs.ctx, errMsg)
		c.JSON(http.StatusOK, &BaseResponse{Code: 4001, Message: errMsg})

		return
	}
	c.JSON(http.StatusOK, &BaseResponse{Code: 2001, Message: "success", Data: gin.H{"status": trialApply.State}})
}

func (cs *cloudStorage) activateTrialApp(req *trialAppReq) error {
	appName := req.AppName

	bussSoftBasePath := cs.config.BussSoftPath
	logger.Debugf(cs.ctx, "Activating trial app: [ %v ] which located at [ %v ] ", appName, bussSoftBasePath)

	bussAppPath := filepath.Join(bussSoftBasePath, appName)
	if _, err := os.Stat(bussAppPath); os.IsNotExist(err) {
		return fmt.Errorf("can't find given trial app: [ %v ] at given directory: [ %v ]", appName, bussSoftBasePath)
	}

	tenantAppBasePath := fmt.Sprintf("%s/softwares", cs.config.StorePath)
	tenantAppPath := fmt.Sprintf("%s/%s", tenantAppBasePath, appName)
	userHome := cs.SystemRootPrefix

	var handle = func() (err error) {
		err = cs.store.AddAppTrialApply(appName, tenantAppPath)
		if err != nil {
			return err
		}

		uploadMgr := cs.store

		_, _, err = cs.appTransferCtx.RegisterJobWithTimeout(appName, time.Hour*6, func(ctx context.Context, _ interface{}) {
			var err error
			defer func() {
				if err != nil {
					logrus.Errorf("error transfering software [ %s ], detail goes as: %v", appName, err)
					// clean the failed scene, we choose to ignore the error here, since we belive that next time the same trial app apply
					// will remove the failed scene too
					// comment out this line if you want to review the scene to find any error
					_ = os.RemoveAll(tenantAppPath)
					_ = uploadMgr.DoneAppTrialApply(appName, false)
				} else {
					_ = uploadMgr.DoneAppTrialApply(appName, true)
				}
			}()

			err = os.RemoveAll(tenantAppPath) // clean the scene first
			if err != nil {
				return
			}

			start := time.Now()
			err = iotools.Copy2DirWithContext(ctx, bussAppPath, tenantAppBasePath, false)
			if err != nil {
				return
			}

			entries, err := ioutil.ReadDir(tenantAppPath)
			if err != nil {
				return
			}

			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".desktop") {
					desktopName := strings.TrimSuffix(entry.Name(), ".desktop")
					desktopFolder := fmt.Sprintf("%s/Desktop", userHome)
					destFile := fmt.Sprintf("%s/%s-b.desktop", desktopFolder, desktopName)
					err = iotools.EnsureDir(desktopFolder)
					if err != nil {
						logrus.Errorf("err when ensure desktop folder: %v", err)
					}
					err = iotools.CopyFileChown(
						fmt.Sprintf("%s/%s", tenantAppPath, entry.Name()), destFile,
						cs.userPrincipal.LinuxUID, cs.userPrincipal.LinuxGID)
					if err != nil {
						logrus.Errorf("err when copy desktop file: %v", err)
					}
				}
			}

			logrus.Infof("Finish the activation of trial app [ %v ], cost time: %v (s)", appName, time.Since(start).Seconds())
		})

		return err
	}

	appFInfo, err := os.Stat(tenantAppPath)
	if err == nil && appFInfo.Mode() == fs.ModeSymlink { // symlink, delete it (/opt/community/xxx)
		_ = os.Remove(tenantAppPath)
	}

	trialApply, err := cs.store.LoadTrialAppApply(appName)
	if err != nil { // not receive apply activation request yet
		return handle()
	}

	_, err = os.Stat(tenantAppPath)

	if trialApply.State == storage.DONE && err == nil {
		logger.Infof(cs.ctx, "app: [ %v ] has already been activated, no need to activate again", appName)

		return nil
	}

	if trialApply.State == storage.FAILED {
		return handle()
	}

	ctx, _ := cs.appTransferCtx.GetJobCtx(appName)
	if ctx == nil { // we found the running apply but missing ctx (which means we suffer from a restart)
		_ = cs.store.DeleteAppTrialApply(appName)

		return handle()
	}

	logger.Debugf(cs.ctx, "app [ %v ] activating, be patient", appName)

	return nil
}

func (cs *cloudStorage) deactivateTrialApp(req *trialAppReq) error {
	appName := req.AppName
	tenantAppPath := filepath.Join(cs.config.StorePath, "softwares", appName)
	logger.Debugf(cs.ctx, "Deactivating trial app: [ %v ] which located at [ %v ] ", appName, tenantAppPath)

	// first de-register the possible running context
	cs.appTransferCtx.DeRegister(appName)

	uploadMgr := cs.store

	if _, err := os.Stat(tenantAppPath); os.IsNotExist(err) {
		logger.Warnf(cs.ctx, "can't find given trial app: [ %v ] at given directory: [ %v ]", appName, tenantAppPath)
		_ = uploadMgr.DeleteAppTrialApply(appName) // anyway clean the mess

		return nil //ignore directory not found
	}

	trialApply, err := uploadMgr.LoadTrialAppApply(appName)
	if err != nil {
		logger.Warnf(cs.ctx, "err load the trial app: [ %v ] at given directory: [ %v ] from db", appName, tenantAppPath)

		return err
	}

	queue.SubmitJob(queue.Job{ // use a dedicated pool to handle job in case of abnormal termination
		PayLoad: map[string]string{"path": trialApply.Path, "appname": appName},
		Action: func(_ context.Context, m interface{}) {
			mm := m.(map[string]string)
			start := time.Now()

			defer func() {
				err = uploadMgr.DeleteAppTrialApply(mm["appname"])
				if err != nil {
					logrus.Errorf("err when deleting the trial apply from db for app [ %s ], detail is : %v", mm["appname"], err)

					return
				}
				logrus.Infof("Finish the deactivation of trial app [ %v ], cost time: %v (s)", mm["appname"], time.Since(start).Seconds())
			}()
			logrus.Infof("Delete file [ %v ] for app [ %v ]", mm["path"], mm["appname"])
			err = os.RemoveAll(mm["path"])
			if err != nil {
				logrus.Errorf("err when removing the trial app directory [ %s ], detail is : %v", mm["path"], err)

				return
			}
		},
	})

	return nil
}

func (cs *cloudStorage) trialProgress(req *trialAppReq) (*storage.AppTrialApply, error) {
	appName := req.AppName

	return cs.store.LoadTrialAppApply(appName)
}

var injectPubKeyTemplate = `
mkdir -p {{ .homedir }}/.ssh

chown {{ .uid }}:{{ .gid }} {{ .homedir }}

authorized_keys="{{ .homedir }}/.ssh/authorized_keys"

echo "{{ .pub_key }}" >> "$authorized_keys"

sort "$authorized_keys"  | uniq > {{ .homedir }}/.ssh/authorized_keys.uniq
mv {{ .homedir }}/.ssh/authorized_keys{.uniq,}

chown -R {{ .uid }}:{{ .gid }}  {{ .homedir }}/.ssh
chmod 600 "$authorized_keys"
`
