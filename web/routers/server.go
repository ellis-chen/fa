package routers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ellis-chen/fa/global"
	"github.com/ellis-chen/fa/internal"
	"github.com/ellis-chen/fa/internal/app"
	"github.com/ellis-chen/fa/internal/audit"
	"github.com/ellis-chen/fa/internal/backup"
	"github.com/ellis-chen/fa/internal/billing"
	"github.com/ellis-chen/fa/internal/download"
	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/internal/storage"
	"github.com/ellis-chen/fa/pkg/fsutils"
	"github.com/ellis-chen/fa/pkg/iotools"
	"github.com/ellis-chen/fa/pkg/queue"
	"github.com/ellis-chen/fa/pkg/sampler"
	"github.com/gofrs/flock"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	serverOnce sync.Once
	// +checklocks:serverMutex
	server      *Server
	serverMutex sync.RWMutex

	refreshLockFile = "/tmp/fa-refresh.lock"
)

const (
	ServerStarting ServerState = iota
	ServerRunning
	ServerStopping
	ServerStopped
)

type ServerState int

func (s ServerState) String() string {
	switch s {
	case ServerStarting:
		return "STARTING"
	case ServerRunning:
		return "RUNNING"
	case ServerStopping:
		return "ERROR"
	case ServerStopped:
		return "STOPPED"
	default:
		return "INVALID_STATE"
	}
}

func NewServer(ctx context.Context) *Server {
	serverOnce.Do(func() {
		createServerInstance(ctx)
	})

	serverMutex.RLock()
	defer serverMutex.RUnlock()

	return server
}

func createServerInstance(ctx context.Context) *Server {
	cfg := internal.LoadConf()

	storeMgr := storage.NewStoreMgr(cfg)
	bc := billing.NewClient(cfg.BillingAddr, storeMgr)

	ctx, cancel := context.WithCancel(ctx)

	serverMutex.Lock()
	defer serverMutex.Unlock()

	backupCfg := backup.LoadCfg()
	defaultBackupProvider, err := backup.NewProvider(ctx, backup.ProviderInfo{Type: "default", Config: backupCfg})
	if err != nil {
		logrus.Errorf("err new default backup provider: %v", err)
	}
	fullBackupProvider, err := backup.NewProvider(ctx, backup.ProviderInfo{Type: "full", Config: backupCfg})
	if err != nil {
		logrus.Errorf("err new full backup provider: %v", err)
	}

	var ac audit.Client
	if cfg.FcceMode {
		ac = audit.New(cfg.AuditAddr)
	} else {
		ac = audit.NewNullClient()
	}

	server = &Server{
		config:     cfg,
		storageMgr: storeMgr,
		sampler:    sampler.NewSampler(),
		bc:         bc,
		dlCacheMgr: download.NewDlCacheMgr(
			download.DlOption{
				DlLimit: fsutils.MB * 200,
				Timeout: 20,
				Ctx:     ctx,
			},
			bc.SendTransferUsage,
		),
		jobCtx:             app.NewAppTransferJobCtx(time.Hour * 6),
		executor:           queue.NewDispatcher(10),
		backupProvider:     defaultBackupProvider,
		fullBackupProvider: fullBackupProvider,
		rootCtx:            ctx,
		rootCancel:         cancel,
		refreshSig:         make(chan struct{}),
		state:              ServerStarting,
		dataMetric:         &DataMetric{},
		ac:                 ac,
	}

	return server
}

// Server is the wrapper of *gin.Engine
type Server struct {
	router *Router
	// +checklocks:mu
	config             *internal.Config
	storageMgr         storage.StoreManager
	sampler            *sampler.Sampler
	bc                 billing.Client
	ac                 audit.Client
	dlCacheMgr         *download.DlCacheMgr
	jobCtx             *app.TransferJobCtx
	executor           *queue.Dispatcher
	backupProvider     backup.Provider
	fullBackupProvider backup.Provider

	wg sync.WaitGroup

	rootCtx    context.Context
	rootCancel context.CancelFunc

	mu sync.RWMutex

	refreshSig chan struct{}
	// +checklocks:mu
	state      ServerState
	dataMetric *DataMetric
}

func (s *Server) setupRoute() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.router = NewRouter(s.config)
	s.router.SetupServer(s)
	s.router.Setup()
}

func (s *Server) getCliVersion() string {
	return s.rootCtx.Value(global.CliVerKey).(string)
}

func (s *Server) scanUpload() {
	defer s.wg.Done()

	nextPollInterval := time.Minute * 10
	timer := time.NewTimer(nextPollInterval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			s.storageMgr.ScavengeUpload()
			timer.Reset(nextPollInterval)
		case <-s.rootCtx.Done():
			logrus.Warning("Exiting the upload scanUpload goroutine!")

			return
		}
	}
}

// should only be called at bootstrap time once
func (s *Server) scanTrialApp() {
	defer s.wg.Done()

	nextPollInterval := time.Minute * 30
	timer := time.NewTimer(nextPollInterval)
	defer timer.Stop()

	applyHandle := func(app string, appPath string) error {
		logrus.Infof("cleaning stale app [ %s ] trial apply running located at [ %s ]", app, appPath)
		defer func() {
			_ = s.GetStoreMgr().DoneAppTrialApply(app, false)
		}()

		s.jobCtx.DeRegister(app)

		err := os.RemoveAll(appPath)
		if err != nil {
			logrus.Warnf("err cleaning app [ %s ] at [ %s ]", app, appPath)
		}
		// report to regional-api the fail status
		return nil
	}

	// after bootstrap, first check any stale app applies
	staleApplies, _ := s.GetStoreMgr().LoadBootstrapRunningAppApply()
	for _, apply := range staleApplies {
		_ = applyHandle(apply.Name, apply.Path)
	}

	for {
		select {
		case <-timer.C:
			s.storageMgr.ScavengeTrialApp(applyHandle)
			timer.Reset(nextPollInterval)
		case <-s.rootCtx.Done():
			logrus.Warning("Exiting the trialApp scanUpload goroutine!")

			return
		}
	}
}

func (s *Server) scanCopyLine() {
	defer s.wg.Done()

	nextPollInterval := time.Minute * 30
	timer := time.NewTimer(nextPollInterval)
	defer timer.Stop()

	copyLineHandle := func(srcPath string, srcStorageID int, dstPath string, dstStorageID int, done bool) error {
		if !done { // copy failed for some reason
			logrus.Errorf("err when copy from [ %d:%s ] to [ %d:%s ]", srcStorageID, srcPath, dstStorageID, dstPath)
		}
		// report to regional-api / notification the status
		return nil
	}

	for {
		select {
		case <-timer.C:
			s.storageMgr.ScavengeCopyLine(copyLineHandle)
			timer.Reset(nextPollInterval)
		case <-s.rootCtx.Done():
			logrus.Warning("Exiting the copyLine scan goroutine!")

			return
		}
	}
}

func (s *Server) scanFailedUsage() {
	defer s.wg.Done()

	nextPollInterval := time.Hour
	timer := time.NewTimer(nextPollInterval)
	defer timer.Stop()

	usageHandle := func() error {
		mgr := s.storageMgr

		usages, err := mgr.LoadEfsUsages()
		if err != nil {
			return err
		}
		for _, usage := range usages {
			queue.SubmitJob(queue.Job{PayLoad: billing.EfsUsage{
				UsageID:      usage.UsageID,
				Used:         usage.Used,
				TenantID:     usage.TenantID,
				FileSystemID: usage.FileSystemID,
				CloudAccount: usage.CloudAccount,
				StartTime:    usage.StartTime,
				EndTime:      usage.EndTime,
			}, Action: s.GetBc().SendEfsUsage})
		}

		usages, err = mgr.LoadTransferUsage()
		if err != nil {
			return err
		}
		for _, usage := range usages {
			queue.SubmitJob(queue.Job{PayLoad: billing.TransferUsage{
				UsageID:   usage.UsageID,
				Size:      int64(usage.Used),
				TenantID:  usage.TenantID,
				UserID:    usage.UserID,
				Type:      billing.DOWNLOAD,
				Name:      usage.Name,
				From:      usage.From,
				To:        usage.To,
				StartTime: usage.StartTime,
				EndTime:   usage.EndTime,
				ClientIP:  usage.ClientIP,
			}, Action: s.GetBc().SendTransferUsage})
		}

		usages, err = mgr.LoadBackupUsages()
		if err != nil {
			return err
		}
		for _, usage := range usages {
			queue.SubmitJob(queue.Job{PayLoad: billing.BackupUsage{
				UsageID:   usage.UsageID,
				Used:      usage.Used,
				TenantID:  usage.TenantID,
				StartTime: usage.StartTime,
				EndTime:   usage.EndTime,
				Time:      usage.CreatedAt,
			}, Action: s.GetBc().SendBackupUsage})
		}

		return nil
	}

	for {
		select {
		case <-timer.C:
			err := usageHandle()
			if err != nil {
				logrus.Errorf("err handle failed usage: %v", err)
			}
			timer.Reset(nextPollInterval)
		case <-s.rootCtx.Done():
			logrus.Warning("Exiting the failed billing usage scan goroutine!")

			return
		}
	}
}

func (s *Server) scavengeAbormalBackup() {
	defer s.wg.Done()
	select {
	case <-s.rootCtx.Done():
		logrus.Warning("Exiting the scavengeAbormalBackup goroutine!")

		return
	default:
	}

	mgr := s.GetStoreMgr()
	items, _ := mgr.LoadBackups(storage.BackupRunning)
	for _, item := range items {
		err := mgr.DoneBackup(item.BackupID, &storage.BackupDetail{}, storage.BackupFailed, "fa container terminated abormally")
		if err != nil {
			logrus.Errorf("err done backup [ %d ]", item.BackupID)
		}
	}

	restoreItems, _ := mgr.LoadRestores(storage.RestoreRunning)
	for _, item := range restoreItems {
		err := mgr.DoneRestore(item.RestoreID, 0, storage.RestoreFailed, "fa container terminated abormally")
		if err != nil {
			logrus.Errorf("err done restore [ %d ]", item.RestoreID)
		}
	}
}

func (s *Server) scanLongBackup() {
	defer s.wg.Done()

	nextPollInterval := time.Minute * 10
	timer := time.NewTimer(nextPollInterval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			_ = s.storageMgr.ScavengeBackup()
			timer.Reset(nextPollInterval)
		case <-s.rootCtx.Done():
			logrus.Warning("Exiting the scanLongBackup goroutine!")

			return
		}
	}
}

func (s *Server) cleanChores() {
	defer s.wg.Done()

	nextPollInterval := time.Hour * 24
	timer := time.NewTimer(nextPollInterval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			if err := s.storageMgr.ScavengeChores(); err != nil {
				logrus.Error("err done cleanChores: ", err)
			}
			timer.Reset(nextPollInterval)
		case <-s.rootCtx.Done():
			logrus.Warning("Exiting the cleanChores goroutine!")

			return
		}
	}
}

func (s *Server) Start() {
	s.mu.Lock()
	s.executor.Start()
	s.sampler.Start()
	s.sampler.Handle(s.sampleEfsUsage)

	backupCfg := backup.LoadCfg()
	if backupCfg.FullMode {
		err := s.fullBackupProvider.Start(s.rootCtx)
		if err != nil {
			logrus.Errorf("err start backupProvider: %v", err)
		}
	}

	if !s.config.FcceMode {
		err := s.backupProvider.Start(s.rootCtx)
		if err != nil {
			logrus.Errorf("err start backupProvider: %v", err)
		}

		tenantID, err := strconv.Atoi(backupCfg.Location.Prefix)
		if err != nil {
			tenantID = 0
		}
		_ = s.RegisterEfsSampler(int64(tenantID))
	}

	s.mu.Unlock()

	s.setupRoute()

	s.wgWrapper(s.scanUpload)
	s.wgWrapper(s.scanTrialApp)
	s.wgWrapper(s.scanCopyLine)
	s.wgWrapper(s.scanFailedUsage)
	s.wgWrapper(s.onRefreshBackupProvider)
	s.wgWrapper(s.ensureBackupOnline)
	s.wgWrapper(s.scanLongBackup)
	s.wgWrapper(s.scavengeAbormalBackup)
	s.wgWrapper(s.sampleS3Usage)
	s.wgWrapper(s.cleanChores)

	s.setState(ServerRunning)
}

func (s *Server) setState(state ServerState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

func (s *Server) GetState() ServerState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.state
}

func (s *Server) ensureBackupOnline() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			backupProvider := s.GetBackupProvider()
			if backupProvider.GetState() != backup.ProviderError {
				continue
			}

			logrus.Warnf("restarting backupProvider, possible error detected")
			err := backupProvider.OnRefresh(s.GetRootCtx())
			if err != nil {
				logrus.Errorf("refresh repository error: %v", err)
			}
		case <-s.GetRootCtx().Done():
			logrus.Warning("Exiting the ensureBackupOnline goroutine!")

			return
		}
	}
}

func (s *Server) onRefreshBackupProvider() {
	defer s.wg.Done()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR2)

	for {
		select {
		case <-c:
			logrus.Infof("refresh BackupProvider ...")
			backupProvider := s.GetBackupProvider()
			if backupProvider.GetState() == backup.ProviderInitializing {
				continue
			}

			s.refreshSig <- struct{}{}

			err := backupProvider.OnRefresh(s.GetRootCtx())
			if err != nil {
				logrus.Errorf("refresh repository error: %v", err)
			}

			s.wgWrapper(s.sampleS3Usage)
		case <-s.GetRootCtx().Done():
			logrus.Warning("Exiting the onRefreshBackupProvider goroutine!")

			return
		}
	}
}

/*
Stop will stop all server components
*/
func (s *Server) Stop(ctx context.Context) {
	s.setState(ServerStopping)

	s.rootCancel() // cancel the start context

	s.jobCtx.Stop()
	s.dlCacheMgr.Close()
	s.sampler.StopAll()
	s.bc.Stop()
	s.executor.Close()

	s.backupProvider.Close(ctx)
	s.fullBackupProvider.Close(ctx)
	close(s.refreshSig)
	s.wg.Wait() // wait for all the spawning goroutines exit

	err := s.storageMgr.Close()
	if err != nil {
		logrus.Errorf("err when shutdown the uploadMgr, detial goes as: %s", err.Error())
	}

	s.setState(ServerStopped)
}

/*
OnRefesh will try reload the config, and restart all the server components
Current it's not safe to refresh the server context when there's any active requests handling (part becasuse of previous flawed design)
*/
func (s *Server) OnRefresh(ctx context.Context) error {
	l := flock.New(refreshLockFile)

	ok, err := l.TryLock()
	if err != nil {
		return errors.Wrap(err, "error acquiring OnRefresh lock")
	}

	if !ok {
		logger.Debugf(ctx, "fa is refreshing")

		return nil
	}

	defer l.Unlock() //nolint:errcheck

	s.Stop(ctx)

	conf := internal.ReloadConf()
	s.setConf(conf)

	storeMgr := storage.ReloadStoreMgr(conf)
	bc := billing.ReloadClient(conf.BillingAddr, storeMgr)

	if conf.FcceMode {
		s.ac = audit.New(conf.AuditAddr)
	} else {
		s.ac = audit.NewNullClient()
	}

	ctx, cancel := context.WithCancel(ctx)
	s.rootCtx = ctx
	s.rootCancel = cancel

	s.storageMgr = storeMgr
	s.sampler = sampler.NewSampler()
	s.bc = bc
	s.dlCacheMgr = download.ReloadDlCacheMgr(
		download.DlOption{
			DlLimit: fsutils.MB * 200,
			Timeout: 20,
		},
		bc.SendTransferUsage,
	)
	s.jobCtx = app.ReloadAppAppTransferJobCtx(time.Hour * 6)
	s.executor = queue.ReloadDispatcher(10)

	err = s.backupProvider.OnRefresh(ctx)
	if err != nil {
		logrus.Errorf("err refresh backupProvider: %v", err)
	}

	s.refreshSig = make(chan struct{})

	s.Start()

	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.GetState() == ServerRunning {
		s.router.ServeHTTP(w, r)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(appError{Code: 400, Message: "server not in RUNNING state"}) //nolint:errchkjson
	}
}

func (s *Server) sampleEfsUsage(ch <-chan interface{}) {
	var peekSize int64 // peekSize is the max size of a given period
	var startTime = time.Now()

	for v := range ch {
		efsUsage, ok := v.(billing.EfsUsage)
		if !ok {
			continue
		}
		used := int64(efsUsage.Used)
		s.GetBc().UpdateEfsUsage(used)
		if used > peekSize {
			peekSize = used
		}

		if time.Now().Minute() == 0 {
			efsUsage.EndTime = efsUsage.StartTime
			efsUsage.StartTime = startTime
			efsUsage.Used = uint64(peekSize)
			queue.SubmitJob(queue.Job{PayLoad: efsUsage, Action: s.GetBc().SendEfsUsage})
			startTime = time.Now()
			peekSize = 0
		}
	}
	logrus.Infof("Exiting the %v job iteration", 1)
}

func (s *Server) sampleS3Usage() {
	defer s.wg.Done()

	var peekSize int64 // peekSize is the max size of a given period
	var startTime = time.Now()

	profile := backup.LoadCfg()
	tenantID, err := strconv.Atoi(strings.TrimSuffix(profile.Location.Prefix, "/")) // prefix is the tenantID
	if err != nil {
		logger.WithoutContext().Warnf("can't extract tenantID, fallback to 0")
		tenantID = 0
	}

	for {
		select {
		case used, ok := <-s.backupProvider.RepoUsage():
			if !ok {
				continue
			}
			if used > peekSize {
				peekSize = used
			}
			logger.WithoutContext().Debugf("recv usage: %s, peek usage: %s", fsutils.ToHumanBytes(used), fsutils.ToHumanBytes(peekSize))
			_, min, _ := time.Now().Clock()
			if min == 0 || min == 1 {
				// generally, this is not possible, the sender has guaranteed the last accept minutes to be at xx:00:00 ~ xx:00:59
				// but it may cost some time to do the usage gattering, so we also accept xx:01:00 ~ xx:01:59 to minimize the loss ratio of usage
				backupUsage := billing.BackupUsage{
					UsageID:   uuid.New().String(),
					StartTime: startTime,
					EndTime:   time.Now(),
					TenantID:  int64(tenantID),
					Used:      uint64(peekSize),
				}
				logger.WithoutContext().Infof("send backup-usage:  %s", fsutils.ToHumanBytes(peekSize))
				queue.SubmitJob(queue.Job{PayLoad: backupUsage, Action: s.GetBc().SendBackupUsage})
				startTime = time.Now()
				peekSize = 0
			}
		case <-s.rootCtx.Done():
			logrus.Warning("Exiting the sampleS3Usage goroutine!")

			return
		case <-s.refreshSig:
			return
		}
	}
}

func (s *Server) RegisterEfsSampler(tenantID int64) error {
	config := s.config
	err := s.sampler.Register(tenantID, func(tenant int64) interface{} {
		diskUsage, filesystem, err := getMountUsage(config.StorePath)
		if err != nil {
			logrus.Warnf("Not detect the mount point, use the plain recursive scan to check disk usage because of : %v", err)
			du, _ := iotools.DiskUsage(config.StorePath)
			diskUsage = uint64(du)
		}
		filesystemID := config.NFSID
		if filesystemID == "-" && err == nil {
			filesystemID = strings.TrimRight(filesystem, ":/")
			config.NFSID = filesystemID
		}

		return billing.EfsUsage{
			UsageID:      uuid.New().String(),
			Used:         diskUsage,
			TenantID:     tenant,
			FileSystemID: filesystemID,
			CloudAccount: config.CloudAccountName,
			StartTime:    time.Now(),
		}
	}, time.NewTicker(time.Second*time.Duration(config.SampleIntervalInSec)))
	return err
}

func (s *Server) wgWrapper(f func()) {
	s.wg.Add(1)
	go f()
}

func (s *Server) setConf(c *internal.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = c
}

func (s *Server) GetConfig() *internal.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config
}

func (s *Server) GetStoreMgr() storage.StoreManager {
	return s.storageMgr
}

func (s *Server) GetSampler() *sampler.Sampler {
	return s.sampler
}

func (s *Server) GetJobCtx() *app.TransferJobCtx {
	return s.jobCtx
}

func (s *Server) GetDlCacheMgr() *download.DlCacheMgr {
	return s.dlCacheMgr
}

func (s *Server) GetBc() billing.Client {
	return s.bc
}

func (s *Server) GetBackupProvider() backup.Provider {
	return s.backupProvider
}

func (s *Server) GetRootCtx() context.Context {
	return s.rootCtx
}
