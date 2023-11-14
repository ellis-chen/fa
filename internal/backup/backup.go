package backup

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	ProviderInitializing ProviderState = iota
	ProviderStarting
	ProviderError
	ProviderRunning
	ProviderDestroyed
)

type ProviderState int

func (s ProviderState) String() string {
	switch s {
	case ProviderInitializing:
		return "INITIALIZING"
	case ProviderStarting:
		return "STARTING"
	case ProviderError:
		return "ERROR"
	case ProviderRunning:
		return "RUNNING"
	case ProviderDestroyed:
		return "DESTORYED"
	default:
		return "INVALID_STATE"
	}
}

type Provider interface {
	// RegisterLifecycleHook registers a function that is called between lifecycle of backup or restore.
	RegisterLifecycleHook(backupHook LifecycleHook)
	// Start try to create a bucket and a repo for the given provider type and tenantID
	Start(ctx context.Context) error
	// BackupFile accept a request and backup the given file or directory to remote described by tags
	BackupFile(ctx context.Context, req *Request) (*SnapshotInfo, error)
	// RestoreFile restores the given file or directory from the given backup
	RestoreFile(ctx context.Context, restoreID, backupID int, dst string) error
	// RestoreLatest restores the latest backup if possible to the specific path
	RestoreLatest(ctx context.Context, tags map[string]string, dst string) error
	// DeleteBackup deletes the given backup
	DeleteBackup(ctx context.Context, backupID int) error
	// ListBackups returns a list of snapshots for the given path and tags
	ListBackups(ctx context.Context, path string, tags map[string]string) ([]*SnapshotInfo, error)
	// CancelBackup cancels the given backup
	CancelBackup(ctx context.Context, backupID int) error
	// CancelRestore cancels the given restore
	CancelRestore(ctx context.Context, restoreID int) error
	// CancelBackups cancels all running backups
	CancelBackups(ctx context.Context) error
	// CancelRestores cancels all running restores
	CancelRestores(ctx context.Context) error
	// DestroyBackendRepo destroys the given repo
	DestroyBackendRepo(ctx context.Context) error
	// Close stop the provider
	Close(ctx context.Context) error
	// RepoUsage return a channel of repo usage size
	RepoUsage() chan int64
	// RepoStat return statistics of the job status
	RepoStat() *RepoStat
	// Context return the root context
	Context() context.Context
	// OnRefresh will refresh the repository
	OnRefresh(ctx context.Context) error
	// SetState change state to given state
	SetState(state ProviderState)
	// GetState return current provider's state
	GetState() ProviderState
}

// LifecycleHook provides various hook function to preserve backup or restore state or change it
type LifecycleHook interface {
	// OnBackupStart is called before backup starts
	OnBackupStart(context.Context, *Request) error
	// OnBackupEnd is called after backup ends
	OnBackupEnd(context.Context, *Response) error
	// OnRestoreStart is called before restore starts
	OnRestoreStart(context.Context, *RestoreRequest) (*Entry, error)
	// OnRestoreEnd is called after restore ends
	OnRestoreEnd(context.Context, *RestoreResponse) error
	// OnBackupDelStart is called when backup is goint to be deleted
	OnBackupDelStart(context.Context, int) (*Entry, error)
	// OnBackupDelEnd is called when backup is deleted
	OnBackupDelEnd(context.Context, int) error
}

type Entry struct {
	Path         string
	BackupID     int // regional backup id
	RestoreID    int //regional restore id
	SnapshotID   string
	SnapshotSize int64 // the actual size of the backup
}

// SnapshotInfo tracks kopia snapshot information
type SnapshotInfo struct {
	// ID is the snapshot ID produced by kopia snapshot operation
	ID string `json:"id"`
	// SnapshotSize is the size of the snapshot in bytes
	SnapshotSize int64 `json:"snapshotSize"`
	// ManifestSize is the size of the manifest in bytes
	ManifestSize int64 `json:"manifestSize"`
}

func (si SnapshotInfo) String() string {
	return fmt.Sprintf("[ id=%s, logicalSize=%d, physicalSize=%d ]", si.ID, si.SnapshotSize, si.ManifestSize)
}

type Request struct {
	ID       int    `json:"id"`
	Path     string `json:"path"`
	Username string `json:"username"`
}

func (r Request) GetJobID() string {
	return fmt.Sprintf("backup:%d", r.ID)
}

func (r Request) String() string {
	return fmt.Sprintf("[ backupID=%d, backupPath=%s ]", r.ID, r.Path)
}

func (r Request) Tags() map[string]string {
	return map[string]string{"tag:author": r.Username, "tag:path": r.Path, "tag:id": strconv.Itoa(r.ID)}
}

type Response struct {
	ID     int           `json:"id"`
	Path   string        `json:"path"`
	State  string        `json:"state"`
	Detail *SnapshotInfo `json:"detail"`
	Extra  string        `json:"extra"`
}

func (br Response) String() string {
	return fmt.Sprintf("[ backupID=%d, backupPath=%s, success=%s, detail={id=%s, snapshotSize=%d, manifestSize=%d} ]", br.ID, br.Path, br.State, br.Detail.ID, br.Detail.SnapshotSize, br.Detail.ManifestSize)
}

type RestoreRequest struct {
	BackupID  int    `json:"backupID"`
	RestoreID int    `json:"restoreID"`
	Dst       string `json:"dst"`
}

func (br RestoreRequest) String() string {
	return fmt.Sprintf("[ RestoreID=%d, backupID=%d, backupPath=%s ]", br.RestoreID, br.BackupID, br.Dst)
}

type RestoreResponse struct {
	BackupID    int    `json:"backup_id"`
	RestoreID   int    `json:"restore_id"`
	Dst         string `json:"dst"`
	RestoreSize int64  `json:"restore_size"`
	State       string `json:"state"`
	Extra       string `json:"extra"`
}

func (rr RestoreResponse) String() string {
	return fmt.Sprintf("[ backupID=%d, restoreID=%d, dst=%s, restoreSize=%d, state=%s, extra=%s ]", rr.BackupID, rr.RestoreID, rr.Dst, rr.RestoreSize, rr.State, rr.Extra)
}

type NullBackupProvider struct{}

func NewNull(ctx context.Context, cfg *Config) (Provider, error) {
	return &NullBackupProvider{}, nil
}

func (nbp *NullBackupProvider) RegisterLifecycleHook(backupHook LifecycleHook) {
}

func (nbp *NullBackupProvider) Start(ctx context.Context) error {
	return nil
}

func (nbp *NullBackupProvider) BackupFile(ctx context.Context, req *Request) (*SnapshotInfo, error) {
	logrus.Infof("dryrun backup file [ %s ]", req.Path)

	return nil, errors.New("not implemented")
}

func (nbp *NullBackupProvider) RestoreFile(ctx context.Context, restoreID, backupID int, path string) error {
	return nil
}

func (nbp *NullBackupProvider) RestoreLatest(ctx context.Context, tags map[string]string, dst string) error {
	return nil
}

func (nbp *NullBackupProvider) DeleteBackup(ctx context.Context, backupID int) error {
	return nil
}
func (nbp *NullBackupProvider) ListBackups(ctx context.Context, path string, tags map[string]string) ([]*SnapshotInfo, error) {
	return nil, errors.New("not implemented")
}

func (nbp *NullBackupProvider) CancelBackup(ctx context.Context, backupID int) error {
	return nil
}
func (nbp *NullBackupProvider) CancelRestore(ctx context.Context, restoreID int) error {
	return nil
}

func (nbp *NullBackupProvider) CancelBackups(ctx context.Context) error {
	return nil
}

func (nbp *NullBackupProvider) CancelRestores(ctx context.Context) error {
	return nil
}

func (nbp *NullBackupProvider) DestroyBackendRepo(ctx context.Context) error {
	return nil
}

func (nbp *NullBackupProvider) Close(ctx context.Context) error {
	return nil
}

func (nbp *NullBackupProvider) RepoUsage() chan int64 {
	return make(chan int64)
}

func (nbp *NullBackupProvider) RepoStat() *RepoStat {
	return &RepoStat{}
}

func (nbp *NullBackupProvider) Context() context.Context {
	return context.TODO()
}

func (nbp *NullBackupProvider) OnRefresh(ctx context.Context) error {
	return nil
}

func (nbp *NullBackupProvider) SetState(state ProviderState) {

}

func (nbp *NullBackupProvider) GetState() ProviderState {
	return ProviderRunning
}

var _ Provider = (*NullBackupProvider)(nil)

func init() {
	AddSupportedProvider("null", func() interface{} { return &Config{} }, func(ctx context.Context, o interface{}) (Provider, error) {
		return NewNull(ctx, o.(*Config)) //nolint:forcetypeassert
	})
}
