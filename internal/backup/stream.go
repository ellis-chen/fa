package backup

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/pkg/fn"
	"github.com/ellis-chen/fa/pkg/fsutils"

	"github.com/kopia/kopia/fs"
	"github.com/kopia/kopia/fs/localfs"
	"github.com/sirupsen/logrus"

	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/repo/compression"
	"github.com/kopia/kopia/repo/maintenance"
	"github.com/kopia/kopia/repo/manifest"
	"github.com/kopia/kopia/snapshot"
	"github.com/kopia/kopia/snapshot/policy"
	"github.com/kopia/kopia/snapshot/restore"
	"github.com/kopia/kopia/snapshot/snapshotfs"

	"github.com/pkg/errors"
)

const (
	pushRepoPurpose = "fa location push"
	pullRepoPurpose = "fa location pull"
	unlimitedDepth  = math.MaxInt32

	MIN  = 60
	HOUR = MIN * 60
	DAY  = HOUR * 24
)

// Validate validates SnapshotInfo field values
func (si *SnapshotInfo) Validate() error {
	if si == nil {
		return errors.New("kopia snapshotInfo cannot be nil")
	}
	if si.ID == "" {
		return errors.New("kopia snapshot ID cannot be empty")
	}

	return nil
}

func (kp *KopiaProvider) OpenRepository(ctx context.Context, purpose string) (repo.Repository, error) {
	r := kp.getRepository()
	if r != nil {
		return r, nil
	}

	repoConfig := kp.getCfg().RepoConfig

	rep, err := OpenRepository(ctx, repoConfig.ConfigFile, repoConfig.Password)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open kopia repository")
	}

	err = repo.WriteSession(ctx, rep, repo.WriteSessionOptions{
		Purpose: purpose,
	}, func(ctx context.Context, w repo.RepositoryWriter) error {
		err = setupMaintenanceParams(ctx, w)
		if err != nil {
			return errors.Wrap(err, "Failed to customize repo maintenance")
		}

		err = setGlobalPolicy(ctx, w, kp.fullMode)
		if err != nil {
			return errors.Wrap(err, "Failed to customize policy for repo")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	kp.setRepository(rep)

	go kp.refreshPeriodically(ctx)
	go kp.maintenanceRepository(ctx)
	go kp.scheduleUsage(ctx)
	go kp.scheduleFull(ctx)

	return rep, nil
}

type uploadMetricGather struct {
	// +checklocks:mu
	cumUploadedBytes int64
	// +checklocks:mu
	totalUploadedBytes int64
	// +checklocks:mu
	estimatedBytes int64
	// +checklocks:mu
	lastAckTime time.Time
	// +checklocks:mu
	newDataArrived bool
	mu             sync.RWMutex
}

func (umg *uploadMetricGather) reset() {
	umg.mu.Lock()
	defer umg.mu.Unlock()
	umg.cumUploadedBytes = 0
	umg.totalUploadedBytes = 0
	umg.estimatedBytes = 0
	umg.lastAckTime = time.Now()
	umg.newDataArrived = false
}

func (umg *uploadMetricGather) gather(metric uploadMetric) {
	umg.mu.Lock()
	defer umg.mu.Unlock()
	umg.cumUploadedBytes += metric.uploadedBytes
	umg.totalUploadedBytes = metric.totalUploadedBytes
	umg.estimatedBytes = metric.estimatedBytes
	umg.newDataArrived = true
}

func (umg *uploadMetricGather) setLastAckTime(t time.Time) {
	umg.mu.Lock()
	defer umg.mu.Unlock()
	umg.lastAckTime = t
}

func (umg *uploadMetricGather) getLastAckTime() time.Time {
	umg.mu.RLock()
	defer umg.mu.RUnlock()

	return umg.lastAckTime
}

func (umg *uploadMetricGather) snapshot() uploadMetricGather {
	umg.mu.RLock()
	defer umg.mu.RUnlock()

	return uploadMetricGather{
		cumUploadedBytes:   umg.cumUploadedBytes,
		totalUploadedBytes: umg.totalUploadedBytes,
		estimatedBytes:     umg.estimatedBytes,
		lastAckTime:        umg.lastAckTime,
		newDataArrived:     umg.newDataArrived,
	}
}

// CreateSnapshot creates a kopia snapshot from the given source file
func (kp *KopiaProvider) CreateSnapshot(ctx context.Context, path string, tags map[string]string) (*SnapshotInfo, error) {
	rep, err := kp.OpenRepository(ctx, pushRepoPurpose)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open kopia repository")
	}

	dir, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.Wrapf(err, "Invalid source path '%s'", path)
	}

	// Populate the source info with parent path as the source
	sourceInfo := snapshot.SourceInfo{
		UserName: rep.ClientOptions().Username,
		Host:     rep.ClientOptions().Hostname,
		Path:     filepath.Clean(dir),
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var progressC = make(chan uploadMetric)
	defer close(progressC)

	go func() {
		defer kp.uploadWg.Done()

		percent := 0.0
		startTime := time.Now()
		ackTime := time.Now()
		beater := &uploadMetricGather{lastAckTime: ackTime}

		guardC := make(chan struct{})
		guardWg := sync.WaitGroup{}

		var recoverMonitor = func() { // beater monitor in case of the zombie goroutines of kopia upload
			nextPollInterval := time.Minute
			timer := time.NewTimer(nextPollInterval)
			defer guardWg.Done()
			defer timer.Stop()

			for {
				select {
				case <-timer.C:
					if time.Since(beater.getLastAckTime()).Minutes() > 5 { // elapsed 5 minutes, still not recev any progress info, may in recover mode
						logger.Debugf(ctx, "the kopia upload goroutines recover the previous upload scene, it doesn't send the upload info for long time")
						// can't just return the for loop, since the later send to guardC may panic
					}
					timer.Reset(nextPollInterval)
				case <-guardC:
					return
				}
			}
		}

		var progressRecivier = func() {
			limiter := time.NewTicker(time.Second * 5)
			defer guardWg.Done()
			defer limiter.Stop()

			progressHandle := func() bool {
				snap := beater.snapshot()
				if !snap.newDataArrived {
					return false
				}
				if snap.estimatedBytes != 0 {
					percent = float64(snap.totalUploadedBytes) / float64(snap.estimatedBytes)
					if percent > 1.0 {
						percent = 1.0
					}
				}

				since := time.Since(snap.lastAckTime).Seconds()
				speed := float64(0)
				if since > 0 {
					speed = float64(snap.cumUploadedBytes) / since
				}
				eta := float64(0)
				if speed > 0 {
					eta = float64(snap.estimatedBytes-snap.totalUploadedBytes) / speed
				}

				ackTime = time.Now()
				beater.setLastAckTime(ackTime)

				kp.RepoStat().IncBackupBytes(snap.cumUploadedBytes)

				logger.Debugf(ctx, "upload: %s, current uploaded: %s, estimate total: %s, percent: %6.2f%%, speed: %s/s, cost: %8s, eta: %8s",
					fsutils.ToHumanBytes(snap.cumUploadedBytes), fsutils.ToHumanBytes(snap.totalUploadedBytes), fsutils.ToHumanBytes(snap.estimatedBytes), percent*100, fsutils.ToHumanBytes(int64(speed)),
					time.Second*time.Duration(time.Since(startTime).Seconds()), time.Second*time.Duration(eta))
				beater.reset()

				return true
			}

			for {
				select {
				case <-limiter.C:
					progressHandle()
				case <-guardC:
					return
				}
			}
		}

		// guard against the upload progress goroutines. Both the "recoverMonitor" and "progressRecivier" goroutines need to be released after an upload finished (wheter success or not)
		guardWg.Add(2)
		go recoverMonitor()
		go progressRecivier()

		for metric := range progressC {
			beater.gather(metric)
		}
		close(guardC)
		guardWg.Wait() // wait the spawned two routines to exit
		logger.Debugf(ctx, "progress gathering done")
	}()

	// guard against an upload. Further "OnRefresh", "DestroyBackendRepo" or "Close" need to wait the upload to finished(whether success or not)
	kp.uploadWg.Add(1)
	snapID, snapshotSize, manSize, err := kp.doSnapshot(ctx, sourceInfo, "FA Data Backup", tags, progressC)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create kopia snapshot")
	}

	snapshotInfo := &SnapshotInfo{
		ID:           snapID,
		SnapshotSize: snapshotSize,
		ManifestSize: manSize,
	}

	return snapshotInfo, nil
}

// restoreSnapshot restores a kopia snapshot with the given ID to the given target
func (kp *KopiaProvider) restoreSnapshot(ctx context.Context, snapshotID, target string) (stats restore.Stats, err error) {
	rep, err := kp.OpenRepository(ctx, pullRepoPurpose)
	if err != nil {
		err = errors.Wrap(err, "Failed to open kopia repository")

		return
	}

	stats, err = doRestore(ctx, rep, snapshotID, target)

	kp.RepoStat().IncRestoreBytes(stats.RestoredTotalFileSize)

	return
}

func (kp *KopiaProvider) restoreLatestSnapshot(ctx context.Context, tags map[string]string, target string) (stats restore.Stats, err error) {
	rep, err := kp.OpenRepository(ctx, pullRepoPurpose)
	if err != nil {
		err = errors.Wrap(err, "Failed to open kopia repository")

		return
	}

	labels := map[string]string{
		"type": "snapshot",
	}
	for key, value := range tags {
		labels[key] = value
	}

	entries, err := rep.FindManifests(ctx, labels)
	if err != nil || len(entries) == 0 {
		err = errors.Wrap(err, "unable to find snapshot manifests")

		return
	}

	rootID := manifest.PickLatestID(entries)
	stats, err = doRestore(ctx, rep, string(rootID), target)
	kp.RepoStat().IncRestoreBytes(stats.RestoredTotalFileSize)

	return
}

func setupMaintenanceParams(ctx context.Context, rep repo.RepositoryWriter) (err error) {
	p, err := maintenance.GetParams(ctx, rep)
	if err != nil {
		return errors.Wrap(err, "unable to get current parameters")
	}
	p.Owner = rep.ClientOptions().UsernameAtHost()
	p.FullCycle = maintenance.CycleParams{
		Enabled:  true,
		Interval: time.Hour,
	}
	p.QuickCycle = maintenance.CycleParams{
		Enabled:  true,
		Interval: time.Minute * 30,
	}
	p.LogRetention = maintenance.LogRetentionOptions{
		MaxTotalSize: 1 << 20,
		MaxAge:       24 * time.Hour,
		MaxCount:     1000,
	}
	if err := maintenance.SetParams(ctx, rep, p); err != nil {
		return errors.Wrap(err, "unable to set params")
	}

	return nil
}

func setGlobalPolicy(ctx context.Context, rep repo.RepositoryWriter, fullMode bool) error {
	dp := *policy.DefaultPolicy
	maxParallelFileReads := policy.OptionalInt(16)
	maxParallelSnapshots := policy.OptionalInt(2)

	var (
		schedulePolicy policy.SchedulingPolicy
		keeps          = math.MaxInt32
	)
	if fullMode {
		logrus.Info("fullMode detected, set the keeps to 1")
		keeps = 1
	} else {
		schedulePolicy.Manual = true
	}

	pol := &policy.Policy{
		UploadPolicy: policy.UploadPolicy{
			MaxParallelFileReads: &maxParallelFileReads,
			MaxParallelSnapshots: &maxParallelSnapshots,
		},
		CompressionPolicy: policy.CompressionPolicy{
			CompressorName: compression.Name("deflate-best-compression"),
		},
		RetentionPolicy: policy.RetentionPolicy{ // leave it very large to disable kopia retetion policy
			KeepLatest:  fn.Ptr(policy.OptionalInt(keeps)),
			KeepHourly:  fn.Ptr(policy.OptionalInt(keeps)),
			KeepDaily:   fn.Ptr(policy.OptionalInt(keeps)),
			KeepWeekly:  fn.Ptr(policy.OptionalInt(keeps)),
			KeepMonthly: fn.Ptr(policy.OptionalInt(keeps)),
			KeepAnnual:  fn.Ptr(policy.OptionalInt(keeps)),
		},
		FilesPolicy:      dp.FilesPolicy,
		SchedulingPolicy: schedulePolicy,
	}

	return policy.SetPolicy(ctx, rep, policy.GlobalPolicySourceInfo, pol)
}

func getLocalFSEntry(path0 string) (fs.Entry, error) {
	path, err := resolveSymlink(path0)
	if err != nil {
		return nil, errors.Wrap(err, "resolveSymlink")
	}

	e, err := localfs.NewEntry(path)
	if err != nil {
		return nil, errors.Wrap(err, "can't get local fs entry")
	}

	return e, nil
}

func resolveSymlink(path string) (string, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return "", errors.Wrap(err, "stat")
	}

	if (st.Mode() & os.ModeSymlink) == 0 {
		return path, nil
	}

	return filepath.EvalSymlinks(path)
}

// doRestore restores a kopia snapshot with the given ID to the given target
func doRestore(ctx context.Context, rep repo.Repository, snapshotID, target string) (stats restore.Stats, err error) {
	rootEntry, err := snapshotfs.FilesystemEntryFromIDWithPath(ctx, rep, snapshotID, false)
	if err != nil {
		err = errors.Wrap(err, "Unable to get filesystem entry")

		return
	}

	p, err := filepath.Abs(target)
	if err != nil {
		err = errors.Wrap(err, "Unable to resolve path")

		return
	}

	output := &restore.FilesystemOutput{
		TargetPath:             p,
		OverwriteDirectories:   true,
		OverwriteFiles:         true,
		OverwriteSymlinks:      true,
		IgnorePermissionErrors: true,
	}

	if err = output.Init(); err != nil {
		err = errors.Wrap(err, "unable to create output file")

		return
	}

	stats, err = restore.Entry(ctx, rep, output, rootEntry, restore.Options{
		Parallel:               8,
		RestoreDirEntryAtDepth: unlimitedDepth,
		ProgressCallback: func(ctx context.Context, s restore.Stats) {
			logger.Infof(ctx, "restore total file size summerized: [ %s / %s (%6.2f%%)] ",
				fsutils.ToHumanBytes(s.RestoredTotalFileSize),
				fsutils.ToHumanBytes(s.EnqueuedTotalFileSize),
				float64(s.RestoredTotalFileSize*100)/float64(s.EnqueuedTotalFileSize))
		},
	})
	if err != nil {
		err = errors.Wrap(err, "Failed to copy snapshot data to the target")

		return
	}

	return
}
