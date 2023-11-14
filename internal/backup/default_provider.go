package backup

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellis-chen/fa/internal"
	"github.com/ellis-chen/fa/internal/app"
	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/internal/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	DefaultProviderName = "default_backup_provider"

	ErrJobProcessing = errors.New("job procressing")
)

// DefaultBackupProvider is a wrapper of provider, which supports more cancellation features
type DefaultBackupProvider struct {
	delegate  Provider
	jobCtx    *app.JobCtx
	backupMgr storage.BackupService
	// +checkatomic
	termState int32
	donC      chan struct{}
	mu        sync.RWMutex

	// +checklocks:mu
	cfg *Config

	parallelBackupMutex sync.Mutex
	// +checklocks:parallelBackupMutex
	currentParallelBackups int
	// +checklocks:parallelBackupMutex
	maxParallelBackups int
	// +checklocks:parallelBackupMutex
	parallelBackupsChanged *sync.Cond // condition triggered on change to currentParallelBackups or maxParallelBackups

	parallelRestoreMutex sync.Mutex
	// +checklocks:parallelRestoreMutex
	currentParallelRestores int
	// +checklocks:parallelRestoreMutex
	maxParallelRestores int
	// +checklocks:parallelRestoreMutex
	parallelRestoresChanged *sync.Cond // condition triggered on change to currentParallelRestores or maxParallelRestores
}

func NewDefault(ctx context.Context, cfg *Config) (Provider, error) {
	delegate, err := NewProvider(ctx, ProviderInfo{Type: "kopia", Config: cfg})
	if err != nil {
		return nil, err
	}
	p := &DefaultBackupProvider{
		jobCtx:              app.NewJobCtx(time.Hour * time.Duration(cfg.RepoConfig.JobTimeoutInHours)),
		backupMgr:           storage.NewStoreMgr(internal.LoadConf()),
		donC:                make(chan struct{}),
		delegate:            delegate,
		parallelBackupMutex: sync.Mutex{},
		maxParallelBackups:  cfg.RepoConfig.ParallelBackup,
		cfg:                 cfg,

		parallelRestoreMutex: sync.Mutex{},
		maxParallelRestores:  cfg.RepoConfig.ParallelRestore,
	}
	p.parallelBackupsChanged = sync.NewCond(&p.parallelBackupMutex)
	p.parallelRestoresChanged = sync.NewCond(&p.parallelRestoreMutex)

	return p, nil
}

func (dp *DefaultBackupProvider) RegisterLifecycleHook(faHook LifecycleHook) {
	dp.delegate.RegisterLifecycleHook(faHook)
}

func (dp *DefaultBackupProvider) Start(ctx context.Context) error {
	return errors.Wrap(dp.delegate.Start(ctx), "err start KopiaProvider")
}

func (dp *DefaultBackupProvider) BackupFile(ctx context.Context, req *Request) (si *SnapshotInfo, err error) {
	backupKey := fmt.Sprintf("backup:%d", req.ID)
	_ctx, _, _ := dp.jobCtx.GetJobCtx(backupKey)
	if _ctx != nil {
		logger.Infof(ctx, "job [ %s ] processing", backupKey)

		return nil, ErrJobProcessing
	}

	var lockSuccess, haveWait bool
	if dp.getCfg().RepoConfig.EnableThrottle {
		lockSuccess, haveWait = dp.tryLockBackup(ctx, req)
		if !lockSuccess {
			return nil, errors.New("err accquering lock to backup")
		}
		defer dp.unlockBackup(ctx)

		var cancel context.CancelFunc
		if haveWait { // reset ctx
			ctx, cancel = context.WithTimeout(context.TODO(), time.Hour*3)
			defer cancel()
		}
	}

	backupCtx, cancel, successC, errC := dp.jobCtx.RegisterJob(ctx, backupKey, func(ctx0 context.Context) error {
		logger.Debugf(ctx, "backup begining ...")

		if haveWait {
			dp.delegate.RepoStat().P2RBackups()
		} else {
			dp.delegate.RepoStat().IncRunningBackups()
		}

		si, err = dp.delegate.BackupFile(ctx0, req)
		if err != nil {
			logger.Errorf(ctx, "err backup file [ %s ] due to : %v", req.Path, err)

			return err
		}

		logger.Debugf(ctx, "backup done.")

		return nil
	})

	defer func() {
		cancel()
		if err == nil {
			dp.delegate.RepoStat().R2SBackups()
		} else {
			dp.delegate.RepoStat().R2FBackups()
		}
	}()

	select {
	case <-successC:
		logger.Infof(ctx, "job [ %s ] success", backupKey)
	case err = <-errC:
		logger.Errorf(ctx, "job [ %s ] failed due to: %v", backupKey, errors.Unwrap(err))

		return nil, err
	case <-backupCtx.Done(): // job context timeout
		err = backupCtx.Err()

		return
	case <-ctx.Done(): // outer context timeout
		err = ctx.Err()

		return
	}

	return si, nil
}

func (dp *DefaultBackupProvider) RestoreFile(ctx context.Context, restoreID, backupID int, dst string) (err error) {
	restoreKey := fmt.Sprintf("restore:%d", restoreID)
	_ctx, _, _ := dp.jobCtx.GetJobCtx(restoreKey)
	if _ctx != nil {
		logger.Infof(ctx, "job [ %s ] processing", restoreKey)

		return nil
	}

	var lockSuccess, haveWait bool
	if dp.getCfg().RepoConfig.EnableThrottle {
		req := &RestoreRequest{RestoreID: restoreID, BackupID: backupID, Dst: dst}

		lockSuccess, haveWait = dp.tryLockRestore(ctx, req)
		if !lockSuccess {
			return errors.New("err accquering lock to restore")
		}
		defer dp.unlockRestore(ctx)

		var cancel context.CancelFunc
		if haveWait { // reset ctx
			ctx, cancel = context.WithTimeout(context.TODO(), time.Hour*3)
			defer cancel()
		}
	}

	restoreCtx, cancel, successC, errC := dp.jobCtx.RegisterJob(ctx, restoreKey, func(ctx0 context.Context) error {
		logger.Debugf(ctx, "restore file begining ...")

		if haveWait {
			dp.delegate.RepoStat().P2RRestores()
		} else {
			dp.delegate.RepoStat().IncRunningRestores()
		}

		err := dp.delegate.RestoreFile(ctx0, restoreID, backupID, dst)
		if err != nil {
			logger.Errorf(ctx, "err restore due to : %v", err)

			return err
		}
		logger.Debugf(ctx, "restore file done.")

		return nil
	})

	defer func() {
		cancel()
		if err == nil {
			dp.delegate.RepoStat().R2SRestores()
		} else {
			dp.delegate.RepoStat().R2FRestores()
		}
	}()

	select {
	case <-successC:
		logger.Infof(ctx, "job [ %s ] success", restoreKey)
	case err = <-errC:
		logger.Errorf(ctx, "job [ %s ] failed due to: %v", restoreKey, errors.Unwrap(err))

		return
	case <-restoreCtx.Done():
		err = restoreCtx.Err()

		return
	case <-ctx.Done():
		err = ctx.Err()

		return
	}

	return nil
}

func (dp *DefaultBackupProvider) RestoreLatest(ctx context.Context, tags map[string]string, dst string) error {
	return dp.delegate.RestoreLatest(ctx, tags, dst)
}

func (dp *DefaultBackupProvider) DeleteBackup(ctx context.Context, backupID int) (err error) {
	logger.Infof(ctx, "delete backup")

	err = dp.CancelBackup(ctx, backupID)
	if err != nil {
		logger.Errorf(ctx, " err cancel running backup")
	}

	err = dp.delegate.DeleteBackup(ctx, backupID)
	if err != nil {
		return
	}
	restores, _ := dp.backupMgr.LoadPendingOrRunningRestore(backupID)
	for _, restore := range restores {
		logger.Infof(ctx, "cancel restore of [ restore_id: %d,  dst: %s ]", restore.RestoreID, restore.Dst)
		restoreKey := fmt.Sprintf("restore:%d", restore.RestoreID)
		dp.jobCtx.DeRegister(restoreKey)
	}

	return
}
func (dp *DefaultBackupProvider) ListBackups(ctx context.Context, path string, tags map[string]string) ([]*SnapshotInfo, error) {
	si, err := dp.delegate.ListBackups(ctx, path, tags)

	return si, errors.Wrap(err, "err using kopiaProvider to listBackups")
}

func (dp *DefaultBackupProvider) CancelBackup(ctx context.Context, backupID int) error {
	item, err := dp.backupMgr.LoadBackup(backupID)
	if err != nil {
		return err
	}
	if item == nil {
		return storage.ErrNoSuchRecord
	}
	backupKey := fmt.Sprintf("backup:%d", backupID)
	dp.jobCtx.DeRegister(backupKey)

	return nil
}

func (dp *DefaultBackupProvider) CancelRestore(ctx context.Context, restoreID int) error {
	item, err := dp.backupMgr.LoadRestore(restoreID)
	if err != nil {
		return err
	}
	if item == nil {
		return storage.ErrNoSuchRecord
	}
	restoreKey := fmt.Sprintf("restore:%d", restoreID)
	dp.jobCtx.DeRegister(restoreKey)

	return nil
}

func (dp *DefaultBackupProvider) CancelBackups(ctx context.Context) error {
	backupItems, err := dp.backupMgr.LoadBackups(storage.BackupRunning)
	if err != nil {
		return errors.Wrap(err, "failed to load backup items")
	}
	for _, item := range backupItems {
		err = dp.CancelBackup(ctx, item.BackupID)
		if err != nil {
			return errors.Wrap(err, "failed to cancel backup")
		}
	}
	// still find RUNNING state, make it canceled
	backupItems, _ = dp.backupMgr.LoadBackups(storage.BackupRunning)
	for _, item := range backupItems {
		logger.Warnf(ctx, "Cancel backup [ %d ] of [ %s ]", item.BackupID, item.Path)
		err = dp.backupMgr.DoneBackup(item.BackupID, &storage.BackupDetail{}, storage.BackupCancel, "canceled passive")
		if err != nil {
			return err
		}
	}

	return nil
}

func (dp *DefaultBackupProvider) CancelRestores(ctx context.Context) error {
	restoreItems, err := dp.backupMgr.LoadRestores(storage.RestoreRunning)
	if err != nil {
		return errors.Wrap(err, "failed to load restore items")
	}
	for _, item := range restoreItems {
		err = dp.CancelRestore(ctx, item.RestoreID)
		if err != nil {
			return errors.Wrap(err, "failed to cancel restore")
		}
	}
	// still find RUNNING state, make it canceled
	restoreItems, _ = dp.backupMgr.LoadRestores(storage.RestoreRunning)
	for _, item := range restoreItems {
		logger.Warnf(ctx, "Cancel restore [ %d ] to dst [ %s ]", item.RestoreID, item.Dst)
		err = dp.backupMgr.DoneRestore(item.RestoreID, 0, storage.RestoreCancel, "canceled passive")
		if err != nil {
			return errors.Wrap(err, "failed to cancel restore")
		}
	}

	return nil
}

func (dp *DefaultBackupProvider) DestroyBackendRepo(ctx context.Context) (err error) {
	defer func() {
		e := dp.backupMgr.DeleteBackups()
		if e != nil {
			logger.Errorf(ctx, "err clean backup records from db")
		}
		e = dp.backupMgr.DeleteRestores()
		if e != nil {
			logger.Errorf(ctx, "err clean restore records from db")
		}
	}()
	err = dp.CancelBackups(ctx)
	if err != nil {
		logrus.Errorf("err cancel running backups, details goes as: %v", err)
	}
	err = dp.CancelRestores(ctx)
	if err != nil {
		logrus.Errorf("err cancel running restores, details goes as: %v", err)
	}
	err = dp.delegate.DestroyBackendRepo(ctx)

	return
}

func (dp *DefaultBackupProvider) Close(ctx context.Context) (err error) {
	logrus.Infof("Closing DefaultProvider ...")
	if atomic.LoadInt32(&dp.termState) == 1 {
		return
	}

	err = dp.CancelBackups(ctx)
	if err != nil {
		logrus.Errorf("err cancel running backups, details goes as: %v", err)
	}
	err = dp.CancelRestores(ctx)
	if err != nil {
		logrus.Errorf("err cancel running restores, details goes as: %v", err)
	}
	err = dp.delegate.Close(ctx)
	if err != nil {
		logrus.Errorf("err close delegate provider, details goes as: %v", err)

		return
	}

	dp.jobCtx.Drain() // drain the jobCtx first, since the previous cancellation is async
	dp.jobCtx.Stop()

	close(dp.donC)
	atomic.StoreInt32(&dp.termState, 1)
	logrus.Infof("Closed DefaultProvider ...")

	return
}

func (dp *DefaultBackupProvider) RepoUsage() chan int64 {
	return dp.delegate.RepoUsage()
}

func (dp *DefaultBackupProvider) RepoStat() *RepoStat {
	return dp.delegate.RepoStat()
}

func (dp *DefaultBackupProvider) Context() context.Context {
	return dp.delegate.Context()
}

func (dp *DefaultBackupProvider) OnRefresh(ctx context.Context) error {
	cfg := LoadCfg()

	logrus.Infof("Refresh DefaultBackupProvider ...")
	atomic.StoreInt32(&dp.termState, 0)

	dp.mu.Lock()
	dp.cfg = cfg
	dp.backupMgr = storage.NewStoreMgr(internal.LoadConf())
	dp.donC = make(chan struct{})
	if dp.jobCtx.IsClosed() {
		logrus.Infof("Reload JobCtx ...")
		dp.jobCtx = app.ReloadJobCtx(time.Hour * time.Duration(cfg.RepoConfig.JobTimeoutInHours))
	}
	dp.mu.Unlock()

	dp.setMaxParallelBackupsLocked(cfg.RepoConfig.ParallelBackup)
	dp.setMaxParallelRestoresLocked(cfg.RepoConfig.ParallelRestore)

	return dp.delegate.OnRefresh(ctx)
}

func (dp *DefaultBackupProvider) SetState(state ProviderState) {
	dp.delegate.SetState(state)
}

func (dp *DefaultBackupProvider) GetState() ProviderState {
	return dp.delegate.GetState()
}

func (dp *DefaultBackupProvider) getCfg() *Config {
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	return dp.cfg
}

func (dp *DefaultBackupProvider) setCfg(cfg *Config) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.cfg = cfg
}

func (dp *DefaultBackupProvider) setMaxParallelBackupsLocked(max int) {
	dp.parallelBackupMutex.Lock()
	defer dp.parallelBackupMutex.Unlock()

	dp.maxParallelBackups = max
	dp.parallelBackupsChanged.Broadcast()
}

func (dp *DefaultBackupProvider) setMaxParallelRestoresLocked(max int) {
	dp.parallelRestoreMutex.Lock()
	defer dp.parallelRestoreMutex.Unlock()

	dp.maxParallelRestores = max
	dp.parallelRestoresChanged.Broadcast()
}

func (dp *DefaultBackupProvider) tryLockBackup(ctx context.Context, req *Request) (lockSuccess bool, awakeFromWait bool) {
	dp.parallelBackupMutex.Lock()
	defer dp.parallelBackupMutex.Unlock()

	backup, _ := dp.backupMgr.LoadPendingOrRunningBackup(req.Path)
	if backup != nil { // exists at least one backup with the same path which state is BackupPending(in queue) or BackupRunning(processing)
		logger.Errorf(ctx, "backup [ %d ] processing or waiting for processing", backup.BackupID)

		return
	}

	for dp.currentParallelBackups >= dp.maxParallelBackups && ctx.Err() == nil {
		err := dp.backupMgr.NewBackup(req.ID, req.Path, req.Username, storage.BackupPending) // store in db in case of regional-api querying backup progress
		if err != nil {
			logger.Errorf(ctx, "err try lock backup")

			return
		}
		dp.delegate.RepoStat().IncPendingBackups()
		dp.parallelBackupsChanged.Wait()
		awakeFromWait = true
	}

	if ctx.Err() != nil {
		// context closed
		err := dp.backupMgr.DoneBackup(req.ID, &storage.BackupDetail{}, storage.BackupFailed, fmt.Sprintf("lock backup context canceled: %v", ctx.Err()))
		if err != nil {
			logger.Errorf(ctx, "lock backup context canceled: %v", ctx.Err())
		}

		return
	}

	dp.currentParallelBackups++
	lockSuccess = true

	return
}

func (dp *DefaultBackupProvider) unlockBackup(ctx context.Context) {
	dp.parallelBackupMutex.Lock()
	defer dp.parallelBackupMutex.Unlock()

	dp.currentParallelBackups--

	// notify one of the waiters
	dp.parallelBackupsChanged.Signal()
}

func (dp *DefaultBackupProvider) tryLockRestore(ctx context.Context, req *RestoreRequest) (lockSuccess bool, awakeFromWait bool) {
	dp.parallelRestoreMutex.Lock()
	defer dp.parallelRestoreMutex.Unlock()

	restores, _ := dp.backupMgr.LoadPendingOrRunningRestore(req.BackupID)
	if restores != nil { // exists at least one restore with the same source which state is RestorePending(in queue) or RestoreRunning(processing)
		logger.Errorf(ctx, "restore from backup [ %d ] processing or waiting for processing", req.BackupID)

		return
	}

	for dp.currentParallelRestores >= dp.maxParallelRestores && ctx.Err() == nil {
		err := dp.backupMgr.NewRestore(req.RestoreID, req.BackupID, req.Dst, storage.RestorePending) // store in db in case of regional-api querying restore progress
		if err != nil {
			logger.Errorf(ctx, "err try lock restore")

			return
		}
		dp.delegate.RepoStat().IncPendingRestores()
		dp.parallelRestoresChanged.Wait()
		awakeFromWait = true
	}

	if ctx.Err() != nil {
		// context closed
		err := dp.backupMgr.DoneRestore(req.RestoreID, 0, storage.RestoreFailed, fmt.Sprintf("lock restore context canceled: %v", ctx.Err()))
		if err != nil {
			logger.Errorf(ctx, "lock restore context canceled: %v", ctx.Err())
		}

		return
	}

	dp.currentParallelRestores++
	lockSuccess = true

	return
}

func (dp *DefaultBackupProvider) unlockRestore(ctx context.Context) {
	dp.parallelRestoreMutex.Lock()
	defer dp.parallelRestoreMutex.Unlock()

	dp.currentParallelRestores--

	// notify one of the waiters
	dp.parallelRestoresChanged.Signal()
}

var _ Provider = (*DefaultBackupProvider)(nil)

func init() {
	AddSupportedProvider("default", func() interface{} { return &Config{} }, func(ctx context.Context, o interface{}) (Provider, error) {
		return NewDefault(ctx, o.(*Config)) //nolint:forcetypeassert
	})
}
