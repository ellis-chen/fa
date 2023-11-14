package routers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ellis-chen/fa/internal"
	"github.com/ellis-chen/fa/internal/backup"
	"github.com/ellis-chen/fa/internal/storage"

	"github.com/ellis-chen/fa/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type (
	// BackupReq create a backup job
	BackupReq struct {
		StorageID int    `json:"storage_id" form:"storage_id"`
		ID        int    `json:"id"`
		Path      string `json:"path"`
	}

	CancelBackupReq struct {
		ID int `json:"id"`
	}

	DelBackupReq struct {
		ID int `uri:"id"`
	}

	BackupJobStatus string

	GetBackupJobRes struct {
		ID     int             `json:"id"`
		Path   string          `json:"path"`
		Status BackupJobStatus `json:"status"`
		Stat   *BackupStat     `json:"stat"`
		Extra  string          `json:"extra"`
	}

	BackupStat struct {
		TotalSize uint64 `json:"total_size"`
		PackSize  uint64 `json:"pack_size"`
	}

	RestoreReq struct {
		StorageID int    `json:"storage_id" form:"storage_id" binding:"required"`
		RestoreID int    `json:"restore_id"`
		BackupID  int    `json:"backup_id"`
		Path      string `json:"dst_path"`
	}

	RestoreJobStatus string

	GetRestoreJobRes struct {
		RestoreID int              `json:"restore_id"`
		BackupID  int              `json:"backup_id"`
		Path      string           `json:"dst_path"`
		Status    RestoreJobStatus `json:"status"`
		Stat      *BackupStat      `json:"stat"`
		Extra     string           `json:"extra"`
	}

	CancelRestoreReq struct {
		ID int `json:"id"`
	}
)

func newBackup(c *gin.Context) {
	req := BackupReq{StorageID: -1}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)

		return
	}

	cs := getCloudStorage(c)
	absPath, err := cs.sanitizePath(req.Path, req.StorageID)
	if err != nil {
		_ = c.Error(err)

		return
	}

	req.Path = absPath
	go func() {
		ctx, cancel := context.WithTimeout(context.TODO(), time.Hour*5)
		defer cancel()
		ctx = logger.With(
			ctx,
			logger.Str(logger.Module, "new backup"),
			logger.Str(logger.Path, req.Path),
			logger.Str(logger.BackupID, strconv.Itoa(req.ID)),
		)
		_, err := getBackupProvider(c).BackupFile(ctx, &backup.Request{
			ID:       req.ID,
			Path:     req.Path,
			Username: cs.userPrincipal.UserName,
		})

		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Errorf("err new backup [ ID=%d ] due to : %v", req.ID, err)
		}
	}()

	c.JSON(http.StatusOK, DefaultSuccess)
}

func getBackup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(err)

		return
	}

	cs := getCloudStorage(c)

	logrus.Infof("loading backup [ %d ]", id)
	item, err := cs.store.LoadBackup(id)
	if err != nil {
		err = errors.Wrapf(err, "err when load backup [ %v ]", id)
		logger.Errorf(cs.ctx, err.Error())
		c.JSON(http.StatusOK, &BaseResponse{Code: 4001, Message: err.Error()})

		return
	}

	c.JSON(http.StatusOK, &BaseResponse{Code: 2001, Message: "success", Data: GetBackupJobRes{
		ID:     item.BackupID,
		Path:   item.Path,
		Status: BackupJobStatus(item.State),
		Stat: &BackupStat{
			TotalSize: item.SnapshotSize,
			PackSize:  item.ManifestSize,
		},
		Extra: item.Extra,
	}})
}

func cancelBackup(c *gin.Context) {
	var req CancelBackupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)

		return
	}

	ctx := logger.With(
		context.TODO(),
		logger.Str(logger.Module, "cancel backup"),
		logger.Str(logger.BackupID, strconv.Itoa(req.ID)),
	)
	err := getBackupProvider(c).CancelBackup(ctx, req.ID)

	if err != nil {
		_ = c.Error(errors.Wrapf(err, "err cancel backup [ ID=%d ]", req.ID))

		return
	}
	c.JSON(http.StatusOK, DefaultSuccess)
}

func cancelAllBackup(c *gin.Context) {
	ctx := logger.With(
		context.TODO(),
		logger.Str(logger.Module, "cancel all backups"),
	)
	err := getBackupProvider(c).CancelBackups(ctx)

	if err != nil {
		_ = c.Error(errors.Wrapf(err, "err cancel backups"))

		return
	}
	c.JSON(http.StatusOK, DefaultSuccess)
}

func destroyBackupRepo(c *gin.Context) {
	ctx := logger.With(
		context.TODO(),
		logger.Str(logger.Module, "destroy repository"),
	)

	err := getBackupProvider(c).DestroyBackendRepo(ctx)
	if err != nil {
		_ = c.Error(errors.Wrapf(err, "err destroy repository"))

		return
	}
	enableFullBackup, _ := os.LookupEnv("FB_FULLMODE")
	if enableFullBackup != "true" {
		c.JSON(http.StatusOK, DefaultSuccess)

		return
	}
	err = getBackupFullProvider(c).DestroyBackendRepo(ctx)
	if err != nil {
		_ = c.Error(errors.Wrapf(err, "err destroy repository"))

		return
	}
	c.JSON(http.StatusOK, DefaultSuccess)
}

func deleteBackup(c *gin.Context) {
	backupID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(err)

		return
	}
	ctx := logger.With(
		context.TODO(),
		logger.Str(logger.Module, "delete backup"),
		logger.Str(logger.BackupID, strconv.Itoa(backupID)),
	)

	err = getBackupProvider(c).DeleteBackup(ctx, backupID)

	if err != nil {
		err = errors.Wrapf(err, "err delete backup [ ID=%d ]", backupID)
		_ = c.Error(err)

		return
	}
	c.JSON(http.StatusOK, DefaultSuccess)
}

func newRestore(c *gin.Context) {
	req := RestoreReq{StorageID: -1}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)

		return
	}

	cs := getCloudStorage(c)
	dstPath, err := cs.sanitizePath(req.Path, req.StorageID)
	if err != nil {
		_ = c.Error(err)

		return
	}

	req.Path = dstPath

	go func() {
		ctx, cancel := context.WithTimeout(context.TODO(), time.Hour*5)
		defer cancel()
		ctx = logger.With(
			ctx,
			logger.Str(logger.Module, "new restore"),
			logger.Str(logger.BackupID, strconv.Itoa(req.BackupID)),
			logger.Str(logger.RestoreID, strconv.Itoa(req.RestoreID)),
			logger.Str(logger.Path, dstPath),
		)
		err := getBackupProvider(c).RestoreFile(ctx, req.RestoreID, req.BackupID, dstPath)

		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Errorf(ctx, "err new restore [ ID=%d ] due to: %s", req.RestoreID, err.Error())
		}
	}()
	c.JSON(http.StatusOK, DefaultSuccess)
}

func getRestore(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(err)

		return
	}

	cs := getCloudStorage(c)

	item, err := cs.store.LoadRestore(id)

	if err != nil {
		err = errors.Wrapf(err, "err when load restore [ %v ]", id)
		logger.Errorf(cs.ctx, err.Error())
		c.JSON(http.StatusOK, &BaseResponse{Code: 4001, Message: err.Error()})

		return
	}

	c.JSON(http.StatusOK, &BaseResponse{Code: 2001, Message: "success", Data: GetRestoreJobRes{
		RestoreID: item.RestoreID,
		BackupID:  item.BackupID,
		Path:      item.Dst,
		Status:    RestoreJobStatus(item.State),
		Stat:      &BackupStat{TotalSize: item.RestoreSize},
		Extra:     item.Extra,
	}})
}

func cancelRestore(c *gin.Context) {
	var req CancelBackupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)

		return
	}

	ctx := logger.With(
		context.TODO(),
		logger.Str(logger.Module, "cancel restore"),
		logger.Str(logger.BackupID, strconv.Itoa(req.ID)),
	)
	err := getBackupProvider(c).CancelRestore(ctx, req.ID)

	if err != nil {
		_ = c.Error(errors.Wrapf(err, "err cancel backup [ ID=%d ]", req.ID))

		return
	}
	c.JSON(http.StatusOK, DefaultSuccess)
}

func cancelAllRestores(c *gin.Context) {
	ctx := logger.With(
		context.TODO(),
		logger.Str(logger.Module, "cancel all restores"),
	)
	err := getBackupProvider(c).CancelRestores(ctx)

	if err != nil {
		_ = c.Error(errors.Wrapf(err, "err cancel restores"))

		return
	}
	c.JSON(http.StatusOK, DefaultSuccess)
}

func isBackupProviderDestroyed(c *gin.Context) {
	state := getBackupProvider(c).GetState()
	c.JSON(http.StatusOK, &BaseResponse{Code: 2001, Message: "success", Data: map[string]interface{}{"destroyed": state == backup.ProviderDestroyed}})
}

func isSafeMaintain(c *gin.Context) {
	stat := getBackupProvider(c).RepoStat()
	logrus.Infof("repo stat is: \n%s", stat)
	c.JSON(http.StatusOK, map[string]interface{}{"safe_upgrade": stat.IsSafeMaintain()})
}

func fullRestore(c *gin.Context) {
	dst := c.DefaultQuery("dst", "/mnt/restore")
	chore := getCloudStorage(c).store.QueryLatestChore()
	if chore == nil {
		c.JSON(http.StatusOK, &BaseResponse{Code: 4001, Message: "not found full backup"})
	}
	err := getBackupFullProvider(c).RestoreLatest(c.Request.Context(), chore.ToTags(), dst)
	if err != nil {
		c.JSON(http.StatusOK, &BaseResponse{Code: 4001, Message: "err restore latest"})
		return
	}
	c.JSON(http.StatusOK, DefaultSuccess)
}

func fullAck(c *gin.Context) {
	type arg struct {
		id int
	}
	var a arg
	if err := c.ShouldBindUri(&a); err != nil {
		_ = c.Error(err)
		return
	}
	if err := getCloudStorage(c).store.ChgBChoreState(uint(a.id), storage.ChoreStateACKED); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, DefaultSuccess)
}

func fullQuery(c *gin.Context) {
	_start := c.DefaultQuery("start", fmt.Sprint(time.Now().Add(time.Hour*24).Unix()))
	start, err := strconv.Atoi(_start)
	if err != nil {
		c.JSON(http.StatusOK, &BaseResponse{Code: 4001, Message: fmt.Sprintf("INVALID START param: %v", err)})
		return
	}
	chores := getCloudStorage(c).store.QueryBChore(uint64(start))
	c.JSON(http.StatusOK, chores)
}

// BackupMiddleware ...
func BackupMiddleware(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		uri := c.Request.RequestURI

		if strings.HasPrefix(uri, "/fa/api/v0/backup") || strings.HasPrefix(uri, "/fa/api/v0/restore") || strings.HasPrefix(uri, "/fa/public/safe-upgrade") {
			c.Set(internal.BackupProviderKey, s.GetBackupProvider())
			c.Next()
		} else {
			c.Next()

			return
		}
	}
}

// BackupFullMiddleware ...
func BackupFullMiddleware(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(internal.BackupFullProviderKey, s.fullBackupProvider)
	}
}

func BackupFullCheckMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		enableFullBackup, _ := os.LookupEnv("FB_FULLMODE")
		if enableFullBackup != "true" {
			c.JSON(http.StatusOK, &BaseResponse{Code: 4001, Message: "only support when FULLMODE activated"})
			return
		}
	}
}

func getBackupProvider(c *gin.Context) backup.Provider {
	backupProvider, exists := c.Get(internal.BackupProviderKey)
	if !exists {
		panic("not bind backup provider")
	}

	return backupProvider.(backup.Provider)
}

func getBackupFullProvider(c *gin.Context) backup.Provider {
	backupProvider, exists := c.Get(internal.BackupFullProviderKey)
	if !exists {
		panic("not bind backup provider")
	}

	return backupProvider.(backup.Provider)
}
