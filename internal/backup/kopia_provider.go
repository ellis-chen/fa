package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ellis-chen/fa/internal"
	"github.com/ellis-chen/fa/internal/ctx"
	logCtx "github.com/ellis-chen/fa/internal/ctx"
	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/internal/storage"
	"github.com/ellis-chen/fa/pkg/fsutils"
	"github.com/gofrs/flock"
	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/repo/blob"
	"github.com/kopia/kopia/repo/blob/filesystem"
	"github.com/kopia/kopia/repo/blob/s3"
	"github.com/kopia/kopia/repo/blob/sharded"
	"github.com/kopia/kopia/repo/content"
	"github.com/kopia/kopia/repo/encryption"
	"github.com/kopia/kopia/repo/hashing"
	"github.com/kopia/kopia/repo/logging"
	"github.com/kopia/kopia/repo/maintenance"
	"github.com/kopia/kopia/repo/object"
	"github.com/kopia/kopia/repo/splitter"
	"github.com/kopia/kopia/snapshot"
	"github.com/kopia/kopia/snapshot/restore"
	"github.com/kopia/kopia/snapshot/snapshotfs"
	"github.com/kopia/kopia/snapshot/snapshotmaintenance"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// ErrBackupRunning signal a given backup is running
	ErrBackupRunning = errors.New("backup is running")

	// ErrRestoreRunning signal a given restore is running
	ErrRestoreRunning = errors.New("restore is running")

	ErrRepoHasData = errors.New("repo has data")

	ErrProviderInvalidState = errors.New("KopiaProvider went into a invalid state")

	startLockFile   = "/tmp/fa-kopia-start.lock"
	destroyLockFile = "/tmp/fa-kopia-destroy.lock"
	refreshLockFile = "/tmp/fa-kopia-refresh.lock"
)

// KopiaProvier is a provider that uses kopia to backup and restore the data.
type KopiaProvider struct {
	// +checklocks:mu
	cfg *Config
	// +checklocks:mu
	repo repo.Repository

	mu         sync.RWMutex
	backupHook LifecycleHook
	donC       chan struct{}

	// +checklocks:mu
	rootCtx context.Context
	// +checklocks:mu
	runningCtx       context.Context
	cancelRunningCtx context.CancelFunc

	// +checklocks:mu
	s3Client *minio.Client

	usageC chan int64

	uploaderMap sync.Map
	uploadWg    sync.WaitGroup

	// +checklocks:mu
	state ProviderState

	stateTransistCond *sync.Cond

	// +checklocks:mu
	innerLogger *kopiaLogger

	// +checklocks:mu
	repoStat *RepoStat

	fullMode bool
	chore    storage.Chore
}

type Option func(*KopiaProvider)

// NewKopiaProvider creates a new KopiaProvider.
func NewKopia(ctx context.Context, cfg *Config) (Provider, error) {
	p := &KopiaProvider{
		donC:     make(chan struct{}),
		state:    ProviderInitializing,
		usageC:   make(chan int64),
		repoStat: &RepoStat{},
		cfg:      cfg,
	}

	p.stateTransistCond = sync.NewCond(&p.mu)

	return p, nil
}

func NewKopiaFull(ctx context.Context, cfg *Config) (Provider, error) {
	p, err := NewKopia(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pp := p.(*KopiaProvider)
	pp.fullMode = true
	pp.chore = storage.NewStoreMgr(internal.LoadConf())

	return pp, nil
}

func newS3Client(p *Config) (*minio.Client, error) {
	endpoint := p.Location.Endpoint
	accessKeyID := p.Credential.KeyPair.AccessID
	secretAccessKey := p.Credential.KeyPair.SecretKey
	useSSL := !p.SkipSSLVerify

	var s3Client *minio.Client
	s3Client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Region: p.Location.Region,
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}

	return s3Client, nil
}

// DefaultBackupHook is a backup hook that does various hook function during the backup or restore.
type DefaultBackupHook struct {
	backupMgr storage.BackupService
}

var _ LifecycleHook = (*DefaultBackupHook)(nil)

func (fh *DefaultBackupHook) OnBackupStart(ctx context.Context, req *Request) error {
	item, _ := fh.backupMgr.LoadBackup(req.ID)
	if item != nil && item.State == storage.BackupRunning {
		return ErrBackupRunning
	}

	return fh.backupMgr.NewBackup(req.ID, req.Path, req.Username)
}

func (fh *DefaultBackupHook) OnBackupEnd(ctx context.Context, res *Response) error {
	return fh.backupMgr.DoneBackup(res.ID, &storage.BackupDetail{
		ID:           res.Detail.ID,
		SnapshotSize: res.Detail.SnapshotSize,
		ManifestSize: res.Detail.ManifestSize,
	}, storage.BackupState(res.State), res.Extra)
}

func (fh *DefaultBackupHook) OnRestoreStart(ctx context.Context, req *RestoreRequest) (entry *Entry, err error) {
	var errRunning = errors.New("restore running")
	defer func() {
		e := fh.backupMgr.NewRestore(req.RestoreID, req.BackupID, req.Dst) // whether backup exists or not, restore record should be inserted in order backup-scheduler to query
		if e != nil {
			logger.Errorf(ctx, "err persist new restore: %v", e)

			return
		}

		if err != nil && !errors.Is(err, errRunning) { // if we encounter any error during precheck, we mark restore FAILED in case of regional querying restore-state
			e = fh.backupMgr.DoneRestore(req.RestoreID, 0, storage.RestoreFailed, err.Error())
			if e != nil {
				logger.Errorf(ctx, "err change new restore state to [ failed ]: %v", e)

				return
			}
		}
	}()
	item, err := fh.backupMgr.LoadBackup(req.BackupID)
	if err != nil {
		err = errors.Wrap(err, "err load backup when try create a new restore")
		logger.Errorf(ctx, err.Error())

		return
	}

	restoreItem, _ := fh.backupMgr.LoadRestore(req.RestoreID)
	if restoreItem != nil && (restoreItem.State == storage.RestorePending || restoreItem.State == storage.RestoreRunning) { // silence ignore
		logger.Warnf(ctx, "restore running")
		err = errRunning

		return
	}

	entry = &Entry{Path: item.Path, SnapshotID: item.ManifestID,
		BackupID: item.BackupID, RestoreID: req.RestoreID,
		SnapshotSize: int64(item.SnapshotSize)}

	return
}

func (fh *DefaultBackupHook) OnRestoreEnd(ctx context.Context, res *RestoreResponse) error {
	return fh.backupMgr.DoneRestore(res.RestoreID, res.RestoreSize, storage.RestoreState(res.State), res.Extra)
}

func (fh *DefaultBackupHook) OnBackupDelStart(ctx context.Context, id int) (*Entry, error) {
	item, err := fh.backupMgr.LoadBackup(id)
	if err != nil {
		return nil, err
	}
	logger.Debugf(ctx, "Deleting backup with snapshotID: [ %s ]", item.ManifestID)

	return &Entry{Path: item.Path, SnapshotID: item.ManifestID, BackupID: item.BackupID}, nil
}

func (fh *DefaultBackupHook) OnBackupDelEnd(ctx context.Context, id int) error {
	return fh.backupMgr.DeleteBackup(id)
}

func (kp *KopiaProvider) RegisterLifecycleHook(backupHook LifecycleHook) {
	kp.mu.Lock()
	kp.backupHook = backupHook
	kp.mu.Unlock()
}

func (kp *KopiaProvider) SetState(state ProviderState) {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	kp.state = state
}

func (kp *KopiaProvider) GetState() ProviderState {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	return kp.state
}

func (kp *KopiaProvider) setUsageC(usageC chan int64) {
	kp.mu.Lock()
	kp.usageC = usageC
	kp.mu.Unlock()
}
func (kp *KopiaProvider) setRepoStat(repoStat *RepoStat) {
	kp.mu.Lock()
	kp.repoStat = repoStat
	kp.mu.Unlock()
}

func (kp *KopiaProvider) setCtx(ctx context.Context) {
	kp.mu.Lock()
	kp.rootCtx = ctx
	ctx, cancel := context.WithCancel(ctx)
	kp.runningCtx = ctx
	kp.cancelRunningCtx = cancel
	kp.mu.Unlock()
}

func (kp *KopiaProvider) getRunningCtx() context.Context {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	return kp.runningCtx
}

func (kp *KopiaProvider) setRepository(repository repo.Repository) {
	kp.mu.Lock()
	kp.repo = repository
	kp.mu.Unlock()
}

func (kp *KopiaProvider) getRepository() repo.Repository {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	return kp.repo
}

func (kp *KopiaProvider) setS3Client(client *minio.Client) {
	kp.mu.Lock()
	kp.s3Client = client
	kp.mu.Unlock()
}

func (kp *KopiaProvider) getS3Client() *minio.Client {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	return kp.s3Client
}

func (kp *KopiaProvider) initializeS3Client() error {
	if kp.getCfg() != nil && kp.getS3Client() == nil {
		s3Client, err := newS3Client(kp.getCfg())

		if err != nil {
			return errors.Wrap(err, "err initialize s3 client")
		}

		logrus.Debugf("successfully initialize s3 client")
		kp.setS3Client(s3Client)
	}

	return nil
}

func (kp *KopiaProvider) Start(ctx context.Context) (err error) {
	p := kp.getCfg()
	if !strings.HasSuffix(p.Location.Prefix, "/") {
		p.Location.Prefix = p.Location.Prefix + "/"
	}

	l := flock.New(startLockFile)

	ok, err := l.TryLock()
	if err != nil {
		return errors.Wrap(err, "error acquiring KopiaProvider lock")
	}

	if !ok {
		logger.Debugf(ctx, "KopiaProvider is starting")

		return nil
	}

	defer l.Unlock() //nolint:errcheck

	if kp.GetState() == ProviderRunning {
		return
	}

	kp.SetState(ProviderStarting)
	kp.setCtx(ctx)
	kp.setupKopiaLogger()

	defer func() {
		if err == nil { // no error detected, setup the repository to accept further request
			_, err = kp.OpenRepository(kp.getRunningCtx(), "")
			if err == nil {
				kp.SetState(ProviderRunning)
			} else {
				logrus.Errorf("err start kopia repository: %v", err)
				kp.SetState(ProviderError)
			}
		} else {
			logrus.Errorf("err start kopia repository: %v", err)
			kp.SetState(ProviderError)
		}

		kp.stateTransistCond.Broadcast()
	}()

	if kp.backupHook == nil {
		kp.RegisterLifecycleHook(&DefaultBackupHook{backupMgr: storage.NewStoreMgr(internal.LoadConf())})
	}

	err = verifyConnect(ctx, p.RepoConfig.ConfigFile, p.RepoConfig.Password)
	if err == nil { // if no error, then we can skip creating the bucket
		return
	}

	switch p.Location.Type {
	case LocationTypeS3Compliant:
		// initialize minio client
		err = kp.initializeS3Client()
		if err != nil {
			panic(err)
		}

		err = kp.createBucketIfNotExists(ctx)
		if err != nil {
			return
		}
		err = kp.initKopiaRepo(ctx)
		if err != nil {
			return
		}
	case LocationTypeFilesytem:
		err = kp.initKopiaRepo(ctx)
		if err != nil {
			return
		}
	default:
		return errors.Errorf("unsupported provider type %s", p.Location.Type)
	}

	return err
}

func (kp *KopiaProvider) BackupFile(ctx context.Context, req *Request) (snapInfo *SnapshotInfo, err error) {
	ctx = logging.WithLogger(ctx, func(module string) logging.Logger {
		return kp.getKopiaLogger().src
	})

	tags := map[string]string{"tag:author": req.Username, "tag:path": req.Path, "tag:id": strconv.Itoa(req.ID)}
	commandTags := fmt.Sprintf("--tags author:%s --tags path:%s --tags id:%d", req.Username, req.Path, req.ID)
	logger.Infof(ctx, "backup file with tags [ %s ]", commandTags)
	err = kp.backupHook.OnBackupStart(ctx, req)
	if err != nil {
		return
	}

	snapInfo = &SnapshotInfo{ID: "-", SnapshotSize: 0, ManifestSize: 0}
	backupState := "success"
	extraMsg := ""
	defer func() {
		if snapInfo == nil {
			snapInfo = &SnapshotInfo{ID: "-", SnapshotSize: 0, ManifestSize: 0}
		}
		br := &Response{Path: req.Path, ID: req.ID, Detail: snapInfo, State: backupState}
		if backupState != "success" {
			br.Extra = extraMsg
		}
		e := kp.backupHook.OnBackupEnd(ctx, br)
		if e != nil {
			logrus.Errorf("err change backup status for backup: %v", req)
			err = e
		}
	}()

	err = kp.lockUntil()
	if err != nil {
		backupState = "failed"
		extraMsg = err.Error()

		return
	}

	snapInfo, err = kp.CreateSnapshot(ctx, req.Path, tags)
	if err != nil {
		switch errors.Is(err, context.Canceled) {
		case true:
			backupState = "cancel"
		default:
			backupState = "failed"
		}
		extraMsg = err.Error()

		return
	}

	return
}

func (kp *KopiaProvider) lockUntil() (err error) {
	switch kp.GetState() { //nolint: exhaustive
	case ProviderDestroyed:
		err = kp.Start(kp.Context())
		if err != nil {
			return
		}

		return kp.lockUntil()
	case ProviderError:
		err = ErrProviderInvalidState

		return
	case ProviderStarting: // detect middle state, block to get the end state
		kp.mu.Lock()
		for kp.state == ProviderStarting {
			kp.stateTransistCond.Wait()
		}
		kp.mu.Unlock()

		return kp.lockUntil()
	case ProviderRunning: // escape successfully
	}

	return
}

func (kp *KopiaProvider) RestoreFile(ctx context.Context, restoreID, backupID int, dst string) (err error) {
	ctx = logging.WithLogger(ctx, func(module string) logging.Logger {
		return kp.getKopiaLogger().src
	})

	entry, err := kp.backupHook.OnRestoreStart(ctx, &RestoreRequest{BackupID: backupID, RestoreID: restoreID, Dst: dst})
	if err != nil {
		return errors.Wrapf(err, "err when restore file [ %d ] to dst [ %s ]", backupID, dst)
	}

	restoreState := "success"
	extraMsg := ""
	stats := restore.Stats{}
	defer func() {
		br := &RestoreResponse{BackupID: entry.BackupID, RestoreID: entry.RestoreID, State: restoreState, Dst: dst, RestoreSize: stats.RestoredTotalFileSize} // +checklocksignore
		if restoreState != "success" {
			br.Extra = extraMsg
		} else {
			br.RestoreSize = entry.SnapshotSize
		}

		e := kp.backupHook.OnRestoreEnd(ctx, br)
		if e != nil {
			logrus.Errorf("err change restore status for restore: %v", entry)
			err = e
		}
	}()

	err = kp.lockUntil()
	if err != nil {
		restoreState = "failed"
		extraMsg = err.Error()

		return
	}

	logger.Infof(ctx, "Restoring snapshot of [ %d ] with size [ %s ] to dst [ %s ] ...", entry.BackupID, fsutils.ToHumanBytes(entry.SnapshotSize), dst)
	stats, err = kp.restoreSnapshot(ctx, entry.SnapshotID, dst)
	if err != nil {
		switch errors.Is(err, context.Canceled) {
		case true:
			restoreState = "cancel"
		default:
			restoreState = "failed"
		}
		extraMsg = err.Error()
		return
	}

	return nil
}

func (kp *KopiaProvider) RestoreLatest(ctx context.Context, tags map[string]string, dst string) (err error) {
	err = kp.lockUntil()
	if err != nil {
		return
	}
	logger.Infof(ctx, "Restoring latest snapshot with tags [ %v ] to dst [ %s ] ...", tags, dst)
	stats, err := kp.restoreLatestSnapshot(ctx, tags, dst)
	if err != nil {
		return
	}
	logger.Infof(ctx, "Success restore to dst [ %s ], summary: [ total-size: %d ]", dst, stats.RestoredTotalFileSize)
	return nil
}

func (kp *KopiaProvider) DeleteBackup(ctx context.Context, backupID int) (err error) {
	err = kp.lockUntil()
	if err != nil {
		return
	}
	item, err := kp.backupHook.OnBackupDelStart(ctx, backupID)
	if err != nil {
		return errors.Wrapf(err, "err delete backup [ %d ]", backupID)
	}
	err = kp.DeleteSnapshot(ctx, item.SnapshotID)
	if err == nil || errors.Is(err, snapshot.ErrSnapshotNotFound) {
		return kp.backupHook.OnBackupDelEnd(ctx, backupID)
	}

	return err
}

func (kp *KopiaProvider) ListBackups(ctx context.Context, path string, tags map[string]string) (snapInfos []*SnapshotInfo, err error) {
	err = kp.lockUntil()
	if err != nil {
		return
	}

	logger.Infof(ctx, "list backups for path [ %s ] with tags [ %v ]", path, tags)
	mans, err := kp.ListSnapshots(ctx, path, tags)
	if err != nil {
		return
	}

	snapInfos = make([]*SnapshotInfo, 0)
	var previousMan *snapshot.Manifest
	manSize := 0
	for _, man := range mans {
		if previousMan == nil {
			manSize = int(man.Stats.TotalFileSize) // +checklocksignore
		} else {
			manSize = int(man.Stats.TotalFileSize - previousMan.Stats.TotalFileSize) // +checklocksignore
		}
		previousMan = man
		snapInfos = append(snapInfos, &SnapshotInfo{
			ID:           string(man.ID),
			SnapshotSize: man.Stats.TotalFileSize, // +checklocksignore
			ManifestSize: int64(manSize),
		})
	}

	return snapInfos, nil
}

func (kp *KopiaProvider) CancelBackup(ctx context.Context, backupID int) error {
	return nil
}

func (kp *KopiaProvider) CancelRestore(ctx context.Context, restoreID int) error {
	return nil
}

func (kp *KopiaProvider) CancelBackups(ctx context.Context) error {
	return nil
}

func (kp *KopiaProvider) CancelRestores(ctx context.Context) error {
	return nil
}

func (kp *KopiaProvider) DestroyBackendRepo(ctx context.Context) (err error) {
	l := flock.New(destroyLockFile)

	ok, err := l.TryLock()
	if err != nil {
		return errors.Wrap(err, "error acquiring KopiaProvider destroy lock")
	}

	if !ok {
		logger.Debugf(ctx, "KopiaProvider is destroying")

		return nil
	}

	//nolint
	defer l.Unlock()

	if kp.getRepository() != nil {
		kp.waitJobExit()
	}

	err = kp.closeKopiaReleatedResouces(ctx)
	if err != nil {
		return err
	}

	kp.setRepository(nil)

	defer kp.SetState(ProviderDestroyed)

	err = kp.cleanupRepo(ctx)
	if err != nil {
		return
	}

	return
}

func (kp *KopiaProvider) Close(ctx context.Context) (err error) {
	logrus.Info("Closing kopiaProvider ...")
	select {
	case <-kp.donC:
		// already closed
		return

	default:
	}

	switch kp.GetState() { //nolint: exhaustive
	case ProviderInitializing:
		fallthrough
	case ProviderDestroyed:
		close(kp.usageC)

		return
	}

	defer func() {
		_ = os.Remove(kp.getCfg().RepoConfig.ConfigFile)
	}()

	if kp.getRepository() != nil {
		kp.waitJobExit()
	}
	err = kp.closeKopiaReleatedResouces(ctx)
	if err != nil {
		return
	}

	kp.mu.Lock()
	close(kp.usageC)
	close(kp.donC)
	kp.mu.Unlock()
	logrus.Info("Closed KopiaProvider .")

	return
}

func (kp *KopiaProvider) OnRefresh(ctx context.Context) (err error) {
	l := flock.New(refreshLockFile)

	ok, err := l.TryLock()
	if err != nil {
		return errors.Wrap(err, "error acquiring KopiaProvider OnRefresh lock")
	}

	if !ok {
		logger.Debugf(ctx, "KopiaProvider is refreshing")

		return nil
	}
	//nolint
	defer l.Unlock()

	if kp.getRepository() != nil {
		kp.waitJobExit()
	}

	err = kp.closeKopiaReleatedResouces(ctx)
	if err != nil {
		return err
	}

	oldState := kp.GetState()
	profile := LoadCfg()

	kp.mu.Lock() // reset everything backup to the initial state
	kp.repo = nil
	kp.state = ProviderInitializing
	kp.usageC = make(chan int64)
	kp.donC = make(chan struct{})
	kp.repoStat = &RepoStat{}
	kp.backupHook = nil
	kp.cfg = profile
	kp.mu.Unlock()

	if oldState == ProviderInitializing {
		return
	}

	_ = os.RemoveAll(kp.getCfg().RepoConfig.CacheDir)

	err = kp.Start(ctx)
	if err != nil {
		return
	}

	err = kp.maintain(ctx)
	if err != nil {
		return
	}

	return
}

func (kp *KopiaProvider) closeKopiaReleatedResouces(ctx context.Context) (err error) {
	rep := kp.getRepository()
	if rep != nil {
		err = rep.Close(ctx)
		if err != nil {
			return
		}
	}

	innerLogger := kp.getKopiaLogger()
	if innerLogger != nil {
		err = innerLogger.close()

		return
	}

	return
}

func (kp *KopiaProvider) refreshPeriodically(ctx context.Context) {
	nextPollInterval := time.Minute * 10
	timer := time.NewTimer(nextPollInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Warnf(ctx, "Exit refreshPeriodically ...")

			return
		case <-timer.C:
			if err := kp.getRepository().Refresh(ctx); err != nil {
				if kp.getRepository() != nil {
					logrus.Errorf("error refreshing repository: %v", err)
					kp.mu.Lock()
					defer kp.mu.Unlock()
					kp.state = ProviderError
					kp.repo = nil

					return
				}
			}
			timer.Reset(nextPollInterval)
		}
	}
}

func (kp *KopiaProvider) maintenanceRepository(ctx context.Context) {
	rand.Seed(time.Now().UnixNano())
	r := rand.Intn(600)

	select {
	case <-ctx.Done():
		return
	case <-time.After(time.Duration(r) * time.Second): // make the following invocation of maintain jitter
	}

	pollInterval := time.Minute * 30
	timer := time.NewTimer(pollInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Warnf(ctx, "Exit maintenanceRepository ...")

			return
		case <-timer.C:
			err := kp.maintain(ctx)
			if err != nil {
				logrus.Errorf("err maintaining kopia repository: %v", err)
			}
			timer.Reset(pollInterval)
		}
	}
}

func (kp *KopiaProvider) maintain(ctx context.Context) (err error) {
	rep := kp.getRepository()
	if rep == nil {
		return
	}

	dr, ok := rep.(repo.DirectRepository)
	if !ok {
		logrus.Errorf("not a direct repository")

		return
	}

	logger.Infof(ctx, "maintaining repository ...")

	ctx = logging.WithLogger(ctx, func(module string) logging.Logger {
		return kp.getKopiaLogger().src
	})
	safeParam := maintenance.SafetyParameters{
		// the max time that a part can be preserved(if too small, will corrupt the running backup. If too large, will waste storage)
		// we choose 5 hour because the max timeout of a upload is 5 hours
		BlobDeleteMinAge:                time.Hour * time.Duration(kp.getCfg().RepoConfig.JobTimeoutInHours),
		DropContentFromIndexExtraMargin: time.Minute * 30,
		MarginBetweenSnapshotGC:         time.Hour,
		MinContentAgeSubjectToGC:        time.Hour,
		RewriteMinAge:                   time.Hour,
		SessionExpirationAge:            time.Hour * time.Duration(kp.getCfg().RepoConfig.JobTimeoutInHours),
		RequireTwoGCCycles:              true,
		MinRewriteToOrphanDeletionDelay: time.Minute * 30,
	}
	err = repo.DirectWriteSession(ctx, dr, repo.WriteSessionOptions{
		Purpose: "periodicMaintenanceOnce",
	}, func(ctx context.Context, w repo.DirectRepositoryWriter) error {
		return snapshotmaintenance.Run(ctx, w, maintenance.ModeAuto, true, safeParam)
	})

	return
}

func (kp *KopiaProvider) scheduleUsage(ctx context.Context) { // usage writer side
	for {
		select {
		case <-ctx.Done():
			logger.Warnf(ctx, "Exit scheduleUsage ...")

			return

		case <-time.After(round2Interval(5)):
			if kp.GetState() == ProviderRunning { // only check usage when in active session
				if err := kp.checkUsage(ctx); err != nil {
					logger.Errorf(ctx, "error checking repository usage: %v", err)
				}
			}
		}
	}
}

func round2Interval(round int) time.Duration {
	_, min, _ := time.Now().Clock()

	interval := round
	if min+round > 60 {
		interval = 60 - min
	}

	return time.Minute * time.Duration(interval)
}

func (kp *KopiaProvider) scheduleFull(ctx context.Context) {
	if !kp.fullMode {
		return
	}
	prefix := strings.Trim(kp.getCfg().Location.Prefix, "/")
	prefix = strings.TrimSuffix(prefix, "-full")
	tenantID, err := strconv.ParseUint(prefix, 10, 64)
	if err != nil {
		logrus.Errorf("err parse tenantID from full: %v, fallback to 0", err)
		tenantID = 0
	}

	triggerH := tenantID % 6  // range from 0-5h
	triggerM := tenantID % 60 // range from 0-59m

	nextPollInterval := time.Second * 30
	timer := time.NewTimer(nextPollInterval)
	defer timer.Stop()

	var running bool
	for {
		timer.Reset(nextPollInterval)

		select {
		case <-ctx.Done():
			logger.Warnf(ctx, "Exit scheduleFull ...")
			return
		case <-timer.C:
			if running {
				continue
			}
			now := time.Now()
			if int(triggerH) != now.Hour() {
				continue
			}
			if int(triggerM) != now.Minute() {
				continue
			}
			logrus.Infof("tenant [ %d ] begin full backup at [ %d:%d ]", tenantID, triggerH, triggerM)
			running = true //nolint:ineffassign,staticcheck
			if err := kp.doScheduleFull(logCtx.NewLogContext(ctx)); err != nil {
				logrus.Errorf("err schedule full: %v", err)
			}
			running = false
		}
	}
}

func (kp *KopiaProvider) doScheduleFull(ctx ctx.Context) (err error) {
	if !kp.fullMode {
		return errors.New("schedule full only supported when fullMode is enabled")
	}

	err = kp.lockUntil()
	if err != nil {
		return err
	}

	path := "/ellis-chen/users"
	username := "auto"

	chore := &storage.BackupChore{
		Path:     path,
		Username: username,
		State:    storage.ChoreStateRUNNING,
	}
	if err = kp.chore.NewBChore(chore); err != nil {
		return errors.Wrapf(err, "err new backup chore")
	}
	chore = kp.chore.QueryLatestChore()

	var si *SnapshotInfo

	defer func() {
		if err != nil && !strings.Contains(err.Error(), "unknown or unsupported entry type") {
			kp.repoStat.FullBackupFinish(ctx.Duration().Seconds(), false)
			chore.State = storage.ChoreStateFAILED
			chore.Extra = err.Error()
			_ = kp.chore.UpdateChore(chore)
		} else {
			kp.repoStat.FullBackupFinish(ctx.Duration().Seconds(), true)
			chore.State = storage.ChoreStateFINISHED
			if si != nil {
				chore.SnapshotSize = uint64(si.SnapshotSize)
				chore.ManifestSize = uint64(si.ManifestSize)
				chore.ManifestID = si.ID
			}
			_ = kp.chore.UpdateChore(chore)
		}
	}()

	kp.repoStat.FullBackupRunning()
	logrus.Infof("snapshoting path: %s", path)
	ctx0 := logging.WithLogger(ctx, func(module string) logging.Logger {
		return kp.getKopiaLogger().src
	})
	si, err = kp.CreateSnapshot(ctx0, path, chore.ToTags())
	if err != nil {
		return errors.Wrap(err, "err do schedule full")
	}

	logrus.Infof("success created snapshot: %v", si)
	return nil
}

func (kp *KopiaProvider) checkUsage(ctx context.Context) (err error) {
	p := kp.getCfg()
	if kp.fullMode {
		logger.Debugf(ctx, "full mode not check usage")
		return
	}
	if p.Location.Type != LocationTypeS3Compliant {
		return
	}
	objInfos := kp.getS3Client().ListObjects(ctx, p.Location.Bucket, minio.ListObjectsOptions{Prefix: p.Location.Prefix, Recursive: true})

	var totalSize int64
	for objInfo := range objInfos {
		// ignore kopia config file and xw file
		// see explanation of prefix on https://kopia.io/docs/advanced/architecture/
		if strings.HasPrefix(objInfo.Key, p.Location.Prefix+"kopia") ||
			strings.HasPrefix(objInfo.Key, p.Location.Prefix+"xw") ||
			strings.HasPrefix(objInfo.Key, p.Location.Prefix+"xn") ||
			strings.HasPrefix(objInfo.Key, p.Location.Prefix+"q") ||
			strings.HasPrefix(objInfo.Key, p.Location.Prefix+"_log") {
			continue
		}
		totalSize += objInfo.Size
	}
	if totalSize <= 1024 { // less than 1K, just ignore it
		return
	}

	if kp.GetState() == ProviderRunning {
		logger.Debugf(ctx, "gather usage: %s", fsutils.ToHumanBytes(totalSize))
		kp.RepoUsage() <- totalSize
	}

	return
}

func (kp *KopiaProvider) RepoUsage() chan int64 {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	return kp.usageC
}

func (kp *KopiaProvider) RepoStat() *RepoStat {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	return kp.repoStat
}

func (kp *KopiaProvider) Context() context.Context {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	return kp.rootCtx
}

func (kp *KopiaProvider) getCfg() *Config {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	return kp.cfg
}

func (kp *KopiaProvider) setCfg(p *Config) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	kp.cfg = p
}

// waitJobExit block and wait all the pending or running job (or spawning goroutines) exit
func (kp *KopiaProvider) waitJobExit() {
	kp.cancelRunningCtx() // cancel the spawn goroutines

	kp.uploaderMap.Range(func(k, v any) bool {
		logrus.Warnf("cancel upload for key: %v", k)
		v.(*snapshotfs.Uploader).Cancel()

		return true
	})

	kp.uploadWg.Wait() // wait all the upload process to exit
}

func (kp *KopiaProvider) createBucketIfNotExists(ctx context.Context) error {
	p := kp.getCfg()

	logrus.Infof("Ensuring bucket [ %s ] exists", p.Location.Bucket)
	bucketExists, err := kp.getS3Client().BucketExists(ctx, p.Location.Bucket)
	if err != nil {
		return errors.Wrapf(err, "err check bucket [ %s ] exists", p.Location.Bucket)
	}
	if bucketExists {
		logrus.Infof("bucket [ %s ] exists", p.Location.Bucket)

		return nil
	}

	err = kp.getS3Client().MakeBucket(ctx, p.Location.Bucket, minio.MakeBucketOptions{Region: p.Location.Region, ObjectLocking: false})
	if err != nil {
		logrus.Errorf("err make bucket: %v", err)

		return errors.Wrap(err, "err create bucket")
	}
	logrus.Infof("Successfully created bucket [ %s ].", p.Location.Bucket)

	return nil
}

func (kp *KopiaProvider) initKopiaRepo(ctx context.Context) error {
	p := kp.getCfg()

	var (
		storage blob.Storage
		err     error
	)

	switch p.Location.Type {
	case LocationTypeS3Compliant:
		storage, err = s3.New(ctx, &s3.Options{
			Endpoint:        p.Location.Endpoint,
			BucketName:      p.Location.Bucket,
			Prefix:          p.Location.Prefix,
			Region:          p.Location.Region,
			AccessKeyID:     p.Credential.KeyPair.AccessID,
			SecretAccessKey: p.Credential.KeyPair.SecretKey,
		})
	case LocationTypeFilesytem:
		storage, err = filesystem.New(ctx, &filesystem.Options{
			Path:    p.Location.Bucket,
			Options: sharded.Options{},
		}, true)
	}

	logrus.Debugf("endpoint: %s, bucket: %s, prefix: %s, region: %s, accessID: %s, secret: %s", p.Location.Endpoint, p.Location.Bucket, p.Location.Prefix, p.Location.Region, "-", "-")
	if err != nil {
		return errors.Wrap(err, "err when connecting to s3")
	}
	logrus.Debugf("display name is %s", storage.DisplayName())

	return doInitialize(ctx, storage, p.RepoConfig.ConfigFile, p.RepoConfig.Password, p.RepoConfig.CacheDir)
}

func doInitialize(ctx context.Context, storage blob.Storage, cfgFile, password, cacheDir string) error {
	err := ensureEmpty(ctx, storage)
	if err != nil && !errors.Is(err, ErrRepoHasData) {
		logrus.Errorf("err ensure repository empty")

		return errors.Wrap(err, "unable to get repository storage")
	}

	var retentionMode string
	if storage.ConnectionInfo().Type != "filesystem" {
		retentionMode = blob.Governance.String()
	}

	options := repo.NewRepositoryOptions{
		BlockFormat: content.FormattingOptions{
			MutableParameters: content.MutableParameters{
				Version: content.FormatVersion(0),
			},
			Hash:       hashing.DefaultAlgorithm,
			Encryption: encryption.DefaultAlgorithm,
		},
		ObjectFormat: object.Format{
			Splitter: splitter.DefaultAlgorithm,
		},

		RetentionMode: blob.RetentionMode(retentionMode),
	}

	logrus.Debugf("Initializing repository with:")

	if options.BlockFormat.Version != 0 {
		logrus.Debugf("  format version:      %v", options.BlockFormat.Version)
	}

	logrus.Debugf("  block hash:          %v", options.BlockFormat.Hash)
	logrus.Debugf("  encryption:          %v", options.BlockFormat.Encryption)
	logrus.Debugf("  splitter:            %v", options.ObjectFormat.Splitter)

	err = repo.Initialize(ctx, storage, &options, password)
	if err != nil && !errors.Is(err, repo.ErrAlreadyInitialized) {
		return errors.Wrap(err, "err initialize repository")
	}

	//  connect to the existing repository
	err = repo.Connect(ctx, cfgFile, storage, password, &repo.ConnectOptions{
		CachingOptions: content.CachingOptions{
			CacheDirectory:            cacheDir,
			MaxCacheSizeBytes:         128 << 20,
			MaxMetadataCacheSizeBytes: 128 << 20,
			MaxListCacheDuration:      content.DurationSeconds(30),
			MinContentSweepAge:        content.DurationSeconds(30),
		},
		ClientOptions: repo.ClientOptions{
			Hostname:      "fcc.ellis-chentech.com",
			Username:      "fa",
			ReadOnly:      false,
			Description:   "fa connect repo",
			EnableActions: true,
		},
	})
	if err != nil {
		return errors.Wrap(err, "err connect to repository")
	}

	logrus.Infof("Successfully initialized repository.")

	return nil
}

func (kp *KopiaProvider) cleanupRepo(ctx context.Context) error {
	err := os.RemoveAll(kp.getCfg().RepoConfig.ConfigFile)
	if err != nil {
		return err
	}

	err = kp.cleanupObjectStorage(ctx)

	return err
}

func (kp *KopiaProvider) cleanupObjectStorage(ctx context.Context) (err error) {
	p := kp.getCfg()

	objInfos := kp.getS3Client().ListObjects(ctx, p.Location.Bucket, minio.ListObjectsOptions{Prefix: p.Location.Prefix, Recursive: true})

	for objInfo := range objInfos {
		logrus.Infof("removing object [ %s ] from bucket [ %s ]", objInfo.Key, p.Location.Bucket)
		err = kp.getS3Client().RemoveObject(ctx, p.Location.Bucket, objInfo.Key, minio.RemoveObjectOptions{ForceDelete: true})
		if err != nil {
			logrus.Errorf("err remove object: %v", err)

			return errors.Wrap(err, "err remove object")
		}
	}

	return nil
}

func (kp *KopiaProvider) setupKopiaLogger() {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	if kp.innerLogger == nil {
		kp.innerLogger = newKopiaLogger()
	}
}

func (kp *KopiaProvider) getKopiaLogger() *kopiaLogger {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	return kp.innerLogger
}

func ensureEmpty(ctx context.Context, s blob.Storage) error {
	hasDataError := errors.Errorf("has data")

	err := s.ListBlobs(ctx, "", func(cb blob.Metadata) error {
		return hasDataError
	})

	if errors.Is(err, hasDataError) {
		return ErrRepoHasData
	}

	return errors.Wrap(err, "error listing blobs")
}

func toRepoConnectOptions(repoConfig *RepoConfig) *repo.ConnectOptions {
	return &repo.ConnectOptions{
		CachingOptions: content.CachingOptions{
			CacheDirectory:            repoConfig.CacheDir,
			MaxCacheSizeBytes:         128 << 20,
			MaxMetadataCacheSizeBytes: 128 << 20,
			MaxListCacheDuration:      content.DurationSeconds(30),
			MinContentSweepAge:        content.DurationSeconds(30),
		},
		ClientOptions: repo.ClientOptions{
			Hostname:      "fcc.ellis-chentech.com",
			Username:      "fa",
			ReadOnly:      false,
			Description:   "fa connect repo",
			EnableActions: true,
		},
	}
}

func verifyConnect(ctx context.Context, configFile, password string) error {
	if _, err := os.Stat(configFile); err != nil {
		return err
	}
	// now verify that the repository can be opened with the provided config file.
	r, err := repo.Open(ctx, configFile, password, nil)
	if err != nil {
		// we failed to open the repository after writing the config file,
		// remove the config file we just wrote and any caches.
		if derr := repo.Disconnect(ctx, configFile); derr != nil {
			logrus.Errorf("unable to disconnect after unsuccessful opening: %v", derr)
		}

		return err
	}

	return errors.Wrap(r.Close(ctx), "error closing repository")
}

// MarshalKopiaSnapshot encodes kopia SnapshotInfo struct into a string
func MarshalKopiaSnapshot(snapInfo *SnapshotInfo) (string, error) {
	if err := snapInfo.Validate(); err != nil {
		return "", err
	}
	snap, err := json.Marshal(snapInfo)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal kopia snapshot information")
	}

	return string(snap), nil
}

var _ Provider = (*KopiaProvider)(nil)

func init() {
	AddSupportedProvider("kopia", func() interface{} { return &Config{} }, func(ctx context.Context, o interface{}) (Provider, error) {
		return NewKopia(ctx, o.(*Config)) //nolint:forcetypeassert
	})

	AddSupportedProvider("full", func() interface{} { return &Config{} }, func(ctx context.Context, o interface{}) (Provider, error) {
		cfg := o.(*Config)
		return NewKopiaFull(ctx, cfg.ToFull()) //nolint:forcetypeassert
	})
}
