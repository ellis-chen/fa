package backup

import (
	"bytes"
	"encoding/json"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type uploadMetric struct {
	totalUploadedBytes int64
	uploadedBytes      int64
	estimatedBytes     int64
}

var (
	metricPendingBackupCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_pending_backup_count",
		Help: "Number of backups that were in pending state",
	})

	metricRunningBackupCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_running_backup_count",
		Help: "Number of backups that were in running state",
	})

	metricFullBackupCost = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "fa_full_backup_cost",
		Help:    "Number of seconds that full backup cost",
		Buckets: prometheus.ExponentialBuckets(1, 1.5, 30),
	})

	metricRunningFullBackup = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_running_full_backup_count",
		Help: "Number of backups that were in running state",
	})

	metricFailedFullBackup = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_failed_full_backup_count",
		Help: "Number of full backups that were in failed state",
	})

	metricSuccesFullBackup = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_success_full_backup_count",
		Help: "Number of full backups that were in success state",
	})

	metricSuccessBackupCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_success_backup_count",
		Help: "Number of backups that were in success state",
	})

	metricFailedBackupCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_failed_backup_count",
		Help: "Number of backups that were in failed state",
	})

	metricPendingRestoreCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_pending_restore_count",
		Help: "Number of restores that were in pending state",
	})

	metricRunningRestoreCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_running_restore_count",
		Help: "Number of restores that were in running state",
	})

	metricSuccessRestoreCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_success_restore_count",
		Help: "Number of restores that were in success state",
	})

	metricFailedRestoreCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_failed_restore_count",
		Help: "Number of restores that were in failed state",
	})

	metricBackupBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_backup_bytes",
		Help: "Number of backups that were in running state",
	})

	metricRestoreBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fa_restore_bytes",
		Help: "Number of backups that were in running state",
	})
)

type RepoStat struct {
	// +checkatomic
	PendingBackups int32 `json:"pendingBackups"`
	// +checkatomic
	PendingRestores int32 `json:"pendingRestores"`
	// +checkatomic
	RunningBackups int32 `json:"runningBackups"`
	// +checkatomic
	RunningRestores int32 `json:"runningRestores"`
	// +checkatomic
	SuccessBackups int32 `json:"successBackups"`
	// +checkatomic
	SuccessRestores int32 `json:"successRestores"`
	// +checkatomic
	FailedBackups int32 `json:"failedBackups"`
	// +checkatomic
	FailedRestores int32 `json:"failedRestores"`
	// +checkatomic
	BackupBytes int64 `json:"backupBytes"`
	// +checkatomic
	RestoreBytes int64 `json:"restoreBytes"`
}

func (rs *RepoStat) IsSafeMaintain() bool {
	return atomic.LoadInt32(&rs.RunningBackups) == 0 && atomic.LoadInt32(&rs.RunningRestores) == 0
}

func (rs *RepoStat) IncPendingBackups() {
	metricPendingBackupCount.Inc()
	atomic.AddInt32(&rs.PendingBackups, 1)
}

func (rs *RepoStat) IncRunningBackups() {
	metricRunningBackupCount.Inc()
	atomic.AddInt32(&rs.RunningBackups, 1)
}

func (rs *RepoStat) P2RBackups() {
	rs.DecrPendingBackups()
	rs.IncRunningBackups()
}

func (rs *RepoStat) P2RRestores() {
	rs.DecrPendingRestores()
	rs.IncRunningRestores()
}

func (rs *RepoStat) R2SBackups() {
	rs.DecrRunningBackups()
	rs.IncSuccessBackups()
}

func (rs *RepoStat) R2SRestores() {
	rs.DecrRunningRestores()
	rs.IncSuccessRestores()
}

func (rs *RepoStat) R2FBackups() {
	rs.DecrRunningBackups()
	rs.IncFailedBackups()
}

func (rs *RepoStat) R2FRestores() {
	rs.DecrRunningRestores()
	rs.IncFailedRestores()
}

func (rs *RepoStat) IncPendingRestores() {
	metricPendingRestoreCount.Inc()
	atomic.AddInt32(&rs.PendingBackups, 1)
}

func (rs *RepoStat) IncRunningRestores() {
	metricRunningRestoreCount.Inc()
	atomic.AddInt32(&rs.RunningRestores, 1)
}

func (rs *RepoStat) DecrPendingBackups() {
	metricPendingBackupCount.Dec()
	atomic.AddInt32(&rs.PendingBackups, -1)
}

func (rs *RepoStat) DecrPendingRestores() {
	metricPendingRestoreCount.Dec()
	atomic.AddInt32(&rs.PendingBackups, -1)
}

func (rs *RepoStat) DecrRunningBackups() {
	metricRunningBackupCount.Dec()
	atomic.AddInt32(&rs.RunningBackups, -1)
}

func (rs *RepoStat) DecrRunningRestores() {
	metricRunningRestoreCount.Dec()
	atomic.AddInt32(&rs.RunningRestores, -1)
}

func (rs *RepoStat) IncSuccessBackups() {
	metricSuccessBackupCount.Inc()
	atomic.AddInt32(&rs.SuccessBackups, 1)
}

func (rs *RepoStat) IncSuccessRestores() {
	metricSuccessRestoreCount.Inc()
	atomic.AddInt32(&rs.SuccessRestores, 1)
}

func (rs *RepoStat) IncFailedBackups() {
	metricFailedBackupCount.Inc()
	atomic.AddInt32(&rs.FailedBackups, 1)
}

func (rs *RepoStat) IncFailedRestores() {
	metricFailedRestoreCount.Inc()
	atomic.AddInt32(&rs.FailedRestores, 1)
}

func (rs *RepoStat) IncBackupBytes(length int64) {
	metricBackupBytes.Add(float64(length))
	atomic.AddInt64(&rs.BackupBytes, length)
}

func (rs *RepoStat) IncRestoreBytes(length int64) {
	metricRestoreBytes.Add(float64(length))
	atomic.AddInt64(&rs.RestoreBytes, length)
}

func (rs *RepoStat) FullBackupRunning() {
	metricRunningFullBackup.Add(1)
}

func (rs *RepoStat) FullBackupFinish(seconds float64, success bool) {
	metricRunningFullBackup.Add(-1)
	metricFullBackupCost.Observe(seconds)
	if success {
		metricSuccesFullBackup.Inc()
		metricFailedFullBackup.Set(0)
	} else {
		metricFailedFullBackup.Inc()
	}
}

func (rs *RepoStat) String() string {
	var buf bytes.Buffer

	e := json.NewEncoder(&buf)
	e.SetIndent("", "  ")

	if err := e.Encode(rs); err != nil {
		return "unable to encode RepoStat as JSON: " + err.Error()
	}

	return buf.String()
}
