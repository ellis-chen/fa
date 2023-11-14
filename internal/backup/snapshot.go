package backup

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/pkg/fsutils"
	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/repo/manifest"
	"github.com/kopia/kopia/snapshot"
	"github.com/kopia/kopia/snapshot/policy"
	"github.com/kopia/kopia/snapshot/snapshotfs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	defaultConfigFilePath = "/tmp/kopia.config"
)

func (kp *KopiaProvider) ListSnapshots(ctx context.Context, source string, tags map[string]string) ([]*snapshot.Manifest, error) {
	rep, err := kp.OpenRepository(ctx, pullRepoPurpose)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to open kopia repository")
	}

	return ListSnapshots0(ctx, rep, source, tags)
}

// DeleteSnapshot deletes Kopia snapshot with given manifest ID
func (kp *KopiaProvider) DeleteSnapshot(ctx context.Context, backupID string) error {
	rep, err := kp.OpenRepository(ctx, pullRepoPurpose)
	if err != nil {
		return errors.Wrap(err, "Failed to open kopia repository")
	}

	return DeleteSnapshot0(ctx, rep, backupID)
}

func (kp *KopiaProvider) doSnapshot(
	ctx context.Context,
	si snapshot.SourceInfo,
	description string,
	tags map[string]string,
	progressC chan uploadMetric,
) (string, int64, int64, error) {
	previous, err := findPreviousSnapshotManifest(ctx, kp.getRepository(), si, nil)
	if err != nil {
		return "", 0, 0, errors.Wrap(err, "Failed to find previous kopia manifests")
	}

	rootDir, err := getLocalFSEntry(si.Path)
	if err != nil {
		return "", 0, 0, errors.Wrap(err, "Unable to get local filesystem entry")
	}

	snapshotStartTime := time.Now()

	onUpload := func(numBytes int64) {}

	var man snapshot.Manifest
	var u *snapshotfs.Uploader
	err = repo.WriteSession(ctx, kp.getRepository(), repo.WriteSessionOptions{
		Purpose: description,
		OnUpload: func(numBytes int64) {
			onUpload(numBytes)
		},
	}, func(ctx context.Context, w repo.RepositoryWriter) error {
		_u, _ := kp.uploaderMap.LoadOrStore(si.Path, snapshotfs.NewUploader(w))
		u = _u.(*snapshotfs.Uploader)
		defer func() {
			kp.uploaderMap.Delete(si.Path)
		}()

		policyTree, err := policy.TreeForSource(ctx, w, si)
		if err != nil {
			return errors.Wrap(err, "unable to create policy getter")
		}

		countingProgress := &snapshotfs.CountingUploadProgress{}
		u.Progress = countingProgress
		var uploadMetric uploadMetric
		var counterMu sync.Mutex
		onUpload = func(numBytes int64) {
			countingProgress.UploadedBytes(numBytes)
			counterMu.Lock()
			uploadMetric.uploadedBytes = numBytes
			uploadMetric.totalUploadedBytes += numBytes
			uploadMetric.estimatedBytes = countingProgress.Snapshot().EstimatedBytes // +checklocksignore
			defer counterMu.Unlock()
			progressC <- uploadMetric
		}

		logger.Debugf(ctx, "starting upload of %v", si.Path)

		manifest, err := u.Upload(ctx, rootDir, policyTree, si, previous...)
		if err != nil {
			return errors.Wrap(err, "upload error")
		}

		if manifest.IncompleteReason != "" {
			return errors.Errorf("upload error : %v", manifest.IncompleteReason)
		}

		man = *manifest

		logger.Debugf(ctx, "finish upload of %v", si.Path)

		manifest.Description = description
		manifest.Tags = tags

		snapshotID, err := snapshot.SaveSnapshot(ctx, w, manifest)
		if err != nil {
			return errors.Wrap(err, "unable to save snapshot")
		}

		if _, err := policy.ApplyRetentionPolicy(ctx, w, si, true); err != nil {
			return errors.Wrap(err, "unable to apply retention policy")
		}

		if err = policy.SetManual(ctx, w, si); err != nil {
			return errors.Wrap(err, "Failed to set manual field in kopia scheduling policy for source")
		}

		man.ID = snapshotID

		return nil
	})

	if err != nil {
		return "", 0, 0, err
	}

	storageStats := &snapshot.StorageStats{NewData: snapshot.StorageUsageDetails{}, RunningTotal: snapshot.StorageUsageDetails{}}
	_ = snapshotfs.CalculateStorageStats(ctx, kp.getRepository(), append(previous, &man), func(m *snapshot.Manifest) error {
		storageStats = m.StorageStats

		return nil
	})

	nsud := storageStats.NewData
	rsud := storageStats.RunningTotal

	logger.Debugf(ctx, "new storage usage: [OriginalContentBytes=%s, PackedContentBytes=%s, ObjectBytes=%s, FileObjectCount=%d, ContentCount=%d, DirObjectCount=%d] ", fsutils.ToHumanBytes(nsud.OriginalContentBytes), fsutils.ToHumanBytes(nsud.PackedContentBytes), fsutils.ToHumanBytes(nsud.ObjectBytes), nsud.FileObjectCount, nsud.ContentCount, nsud.DirObjectCount)
	logger.Debugf(ctx, "running storage usage: [OriginalContentBytes=%s, PackedContentBytes=%s, ObjectBytes=%s, FileObjectCount=%d, ContentCount=%d, DirObjectCount=%d] ", fsutils.ToHumanBytes(rsud.OriginalContentBytes), fsutils.ToHumanBytes(rsud.PackedContentBytes), fsutils.ToHumanBytes(rsud.ObjectBytes), rsud.FileObjectCount, rsud.ContentCount, rsud.DirObjectCount)
	logger.Debugf(ctx, "Created snapshot with root %v and ID %v in %v\n", man.RootObjectID(), string(man.ID), time.Since(snapshotStartTime).Truncate(time.Second))

	// Even if the manifest is created, it might contain fatal errors and failed entries
	var errs []string
	if ds := man.RootEntry.DirSummary; ds != nil {
		for _, ent := range ds.FailedEntries {
			errs = append(errs, ent.Error)
		}
	}
	if len(errs) != 0 {
		return "", 0, 0, errors.New(strings.Join(errs, "\n"))
	}

	return string(man.ID), man.Stats.TotalFileSize, nsud.PackedContentBytes, err
}

func ListSnapshots0(ctx context.Context, rep repo.Repository, source string, tags map[string]string) ([]*snapshot.Manifest, error) {
	manifestIDs, _, err := findManifestIDs(ctx, rep, source, tags)
	if err != nil {
		return nil, err
	}

	manifests, err := snapshot.LoadSnapshots(ctx, rep, manifestIDs)
	if err != nil {
		return nil, errors.Wrap(err, "unable to load snapshots")
	}

	return manifests, nil
}

// DeleteSnapshot deletes Kopia snapshot with given manifest ID
func DeleteSnapshot(ctx context.Context, backupID, password string) error {
	rep, err := OpenRepository(ctx, defaultConfigFilePath, password)
	if err != nil {
		return errors.Wrap(err, "Failed to open kopia repository")
	}

	return DeleteSnapshot0(ctx, rep, backupID)
}

// DeleteSnapshot deletes Kopia snapshot with given manifest ID
func DeleteSnapshot0(ctx context.Context, rep repo.Repository, backupID string) error {
	return repo.WriteSession(ctx, rep, repo.WriteSessionOptions{
		Purpose: "DeleteSnapshots",
	}, func(ctx context.Context, w repo.RepositoryWriter) error {
		m := manifest.ID(backupID)
		_, err := snapshot.LoadSnapshot(ctx, w, m)
		if err != nil {
			return errors.Wrapf(err, "Failed to load kopia snapshot with ID: %v", backupID)
		}
		if err := w.DeleteManifest(ctx, m); err != nil {
			return errors.Wrap(err, "uanble to delete snapshot")
		}

		return nil
	})
}

func findManifestIDs(ctx context.Context, rep repo.Repository, source string, tags map[string]string) ([]manifest.ID, string, error) {
	if source == "" {
		man, err := snapshot.ListSnapshotManifests(ctx, rep, nil, tags)

		return man, "", errors.Wrap(err, "error listing all snapshot manifests")
	}

	si, err := snapshot.ParseSourceInfo(source, rep.ClientOptions().Hostname, rep.ClientOptions().Username)
	if err != nil {
		return nil, "", errors.Errorf("invalid directory: '%s': %s", source, err)
	}

	manifestIDs, relPath, err := findSnapshotsForSource(ctx, rep, si, tags)
	if relPath != "" {
		relPath = "/" + relPath
	}

	return manifestIDs, relPath, err
}

func findSnapshotsForSource(ctx context.Context, rep repo.Repository, sourceInfo snapshot.SourceInfo, tags map[string]string) (manifestIDs []manifest.ID, relPath string, err error) {
	for len(sourceInfo.Path) > 0 {
		list, err := snapshot.ListSnapshotManifests(ctx, rep, &sourceInfo, tags)
		if err != nil {
			return nil, "", errors.Wrapf(err, "error listing manifests for %v", sourceInfo)
		}

		if len(list) > 0 {
			return list, relPath, nil
		}

		if len(relPath) > 0 {
			relPath = filepath.Base(sourceInfo.Path) + "/" + relPath
		} else {
			relPath = filepath.Base(sourceInfo.Path)
		}

		logrus.Debugf("No snapshots of %v@%v:%v", sourceInfo.UserName, sourceInfo.Host, sourceInfo.Path)

		parentPath := filepath.Dir(sourceInfo.Path)
		if parentPath == sourceInfo.Path {
			break
		}

		sourceInfo.Path = parentPath
	}

	return nil, "", nil
}

// findPreviousSnapshotManifest returns the list of previous snapshots for a given source,
// including last complete snapshot
func findPreviousSnapshotManifest(ctx context.Context, rep repo.Repository, sourceInfo snapshot.SourceInfo, noLaterThan *time.Time) ([]*snapshot.Manifest, error) {
	man, err := snapshot.ListSnapshots(ctx, rep, sourceInfo)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list previous kopia snapshots")
	}

	// find latest complete snapshot
	var previousComplete *snapshot.Manifest
	var result []*snapshot.Manifest

	for _, p := range man {
		if noLaterThan != nil && p.StartTime.After(*noLaterThan) {
			continue
		}

		if p.IncompleteReason == "" && (previousComplete == nil || p.StartTime.After(previousComplete.StartTime)) {
			previousComplete = p
		}
	}

	if previousComplete != nil {
		result = append(result, previousComplete)
	}

	return result, nil
}
