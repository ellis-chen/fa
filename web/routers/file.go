package routers

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	pb "github.com/ellis-chen/fa/third_party/audit/v1"

	"github.com/ellis-chen/fa/internal"
	"github.com/ellis-chen/fa/internal/app"
	"github.com/ellis-chen/fa/internal/audit"
	"github.com/ellis-chen/fa/internal/billing"
	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/internal/storage"
	"github.com/ellis-chen/fa/pkg/fsutils"
	"github.com/ellis-chen/fa/pkg/iotools"
	"golang.org/x/sync/errgroup"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
)

type (
	listFileReq struct {
		StorageID int    `json:"storage_id" form:"storage_id"` // -1 means the default store
		Path      string `json:"path" uri:"path"`

		Pattern string `json:"pattern"`
		fsutils.PageSortable

		IgnoreNotFound bool `json:"ignore_not_found"`
		OnlyDirectory  bool `json:"only_directory"`
		ListAll        bool `json:"list_all" form:"list_all"` // default false, which means . file will be ignored
	}

	newDirReq struct {
		StorageID  int    `json:"storage_id"`
		ParentPath string `json:"parent_path"`
		Name       string `json:"name" binding:"required"`
	}

	copyFileReq struct {
		SrcStorageID int    `json:"src_storage_id"`
		Src          string `json:"src_path" binding:"required"`
		DstStorageID int    `json:"dst_storage_id"`
		Dst          string `json:"dst_path"`
		Move         bool   `json:"move"`
		Sync         bool   `json:"sync"`
		DstFile      bool   `json:"dst_file"`
		Force        bool   `json:"force"`       // force mkdir dst if dst no exists
		ExcludeDir   bool   `json:"exclude_dir"` // whether exclude the src directory or not
	}

	movDirReq struct {
		SrcStorageID int    `json:"src_storage_id"`
		Src          string `json:"src_path" binding:"required"`
		DstStorageID int    `json:"dst_storage_id"`
		Dst          string `json:"dst_path" binding:"required"`
	}

	batchCopyFileReq struct {
		SrcStorageID int      `json:"src_storage_id"`
		Srcs         []string `json:"src_paths" binding:"required"`
		Dst          string   `json:"dst_path"`
		DstStorageID int      `json:"dst_storage_id"`
		Move         bool     `json:"move"`
		Sync         bool     `json:"sync"`
		Force        bool     `json:"force"` // force mkdir dst if dst no exists
	}

	renameFileReq struct {
		StorageID int    `json:"storage_id"`
		Path      string `json:"path" binding:"required"`
		Name      string `json:"name" binding:"required"`
		Force     bool   `json:"force"` // force mkdir dst if dst no exists
	}

	delFileReq struct {
		StorageID int    `json:"storage_id"`
		Path      string `json:"path" binding:"required"`
		FileOnly  bool   `json:"file_only"`
	}

	batchDeleteFileReq struct {
		Paths     []string `json:"paths" binding:"required"`
		StorageID int      `json:"storage_id"`
	}

	listFileRes struct {
		Code    int                  `json:"code"`
		Message string               `json:"message"`
		Data    *pageableListFileRes `json:"data"`
	}

	pageableListFileRes struct {
		Contents      *[]fileObject `json:"content"`
		Page          int           `json:"number"`
		Size          int           `json:"size"`
		TotalElements int           `json:"totalElements"`
		TotalPages    int           `json:"totalPages"`
	}

	fileObject struct {
		Type             FileType `json:"type"`
		Size             int64    `json:"size"`
		Name             string   `json:"name"`
		Path             string   `json:"path"`
		Parent           string   `json:"parent_path"`
		Mime             string   `json:"mime"`
		LastModifiedTime int64    `json:"last_modified_time"`
		StorageID        int      `json:"storage_id"`
	}

	// cloudStorage ...
	cloudStorage struct {
		SystemRootPrefix        string
		AuditRootPrefix         string
		AuditApprovedRootPrefix string
		ExternalMntPrefix       string
		mu                      sync.Mutex
		store                   storage.StoreManager
		ctx                     context.Context
		userPrincipal           *internal.UserPrincipal
		config                  *internal.Config
		appTransferCtx          *app.TransferJobCtx
		badFilenameRegex        *regexp.Regexp
		bc                      billing.Client
		ac                      audit.Client

		ignoreNameCheck bool // https://github.com:8443/ellis-chen/compute-cloud/-/issues/2257
		gin             *gin.Context
	}

	fileObjectSorter struct {
		fileObjects FileObjectList
		by          func(p1, p2 interface{}) bool // Closure used in the Less method.
	}

	// FileType ...
	FileType string

	// FileObjectList implements sort.Interface based on the LastModifiedTime field.
	FileObjectList []fileObject
)

// func (m fileObject) LastModifiedTime() time.Time {
// 	return time.Unix(0, m.Millis*int64(time.Millisecond))
// }

func (a FileObjectList) Len() int { return len(a) }
func (a FileObjectList) Less(i, j int) bool {
	return a[i].Name < a[j].Name
}
func (a FileObjectList) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// List ...
func (a FileObjectList) List(pagable fsutils.Pagable) FileObjectList {
	if pagable.Size == -1 {
		return a
	}
	off := pagable.Size * pagable.Page
	end := pagable.Size * (pagable.Page + 1)
	if off > len(a) {
		return make([]fileObject, 0)
	}
	if end > len(a) {
		end = len(a)
	}

	return a[off:end]
}

// By stands for a sortable method
type By func(p1, p2 interface{}) bool

// Sort sort the given collections based on the by method
func (by By) Sort(fileObjs []fileObject) {
	ps := &fileObjectSorter{
		fileObjects: fileObjs,
		by:          by, // The Sort method receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(ps)
}

// Len is part of sort.Interface.
func (s *fileObjectSorter) Len() int { return len(s.fileObjects) }

// Swap is part of sort.Interface.
func (s *fileObjectSorter) Swap(i, j int) {
	s.fileObjects[i], s.fileObjects[j] = s.fileObjects[j], s.fileObjects[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *fileObjectSorter) Less(i, j int) bool {
	return s.by(&s.fileObjects[i], &s.fileObjects[j])
}

func (cs *cloudStorage) listFile(req *listFileReq) (objs []fileObject, num int, err error) {
	curDir, err := cs.sanitizePath(req.Path, req.StorageID)
	if err != nil && !errors.Is(err, ErrInvalidFileName) {
		return
	}
	// curDir := strings.TrimSpace(fmt.Sprintf("%s/%s", cs.RootPrefix, req.Path))

	var fileObjects FileObjectList
	var dirObjects FileObjectList

	fileInfo, err := os.Stat(curDir)
	if err != nil {
		return nil, -1, err
	}
	if !fileInfo.IsDir() {
		mime, _ := iotools.ReadMime(curDir)
		fileObj := fileObject{
			Type:             FILE,
			Size:             fileInfo.Size(),
			Name:             fileInfo.Name(),
			Path:             req.Path,
			Parent:           filepath.Clean(req.Path),
			Mime:             mime,
			LastModifiedTime: fileInfo.ModTime().UTC().UnixMilli(),
			StorageID:        req.StorageID,
		}

		return []fileObject{fileObj}, 1, nil
	}

	entries, err := ioutil.ReadDir(curDir)
	if err != nil {
		return nil, -1, err
	}

	for _, entry := range entries {
		var fileObj *fileObject
		var curPath string
		var mime = "-"
		if strings.TrimSpace(req.Path) == "" {
			curPath = entry.Name()
		} else {
			curPath = fmt.Sprintf("%s/%s", req.Path, entry.Name())
		}

		if req.Pattern != "*" && !strings.Contains(entry.Name(), req.Pattern) {
			continue
		}

		if !req.ListAll && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		if strings.HasPrefix(entry.Name(), ".") && strings.HasSuffix(entry.Name(), ".upload") { // tmp upload file, ignore it
			continue
		}

		curPath = filepath.Clean(curPath)
		parentPath := filepath.Clean(req.Path)

		if entry.IsDir() || entry.Mode().Type() == fs.ModeSymlink {
			fileObj = &fileObject{
				Type:             DIR,
				Size:             -1,
				Name:             entry.Name(),
				Path:             curPath,
				Parent:           parentPath,
				Mime:             mime,
				LastModifiedTime: entry.ModTime().UTC().UnixNano() / int64(time.Millisecond),
				StorageID:        req.StorageID,
			}
		} else {
			mime, _ = iotools.ReadMime(fmt.Sprintf("%s/%s", cs.SystemRootPrefix, curPath))
			fileObj = &fileObject{
				Type:             FILE,
				Size:             entry.Size(),
				Name:             entry.Name(),
				Path:             curPath,
				Parent:           parentPath,
				Mime:             mime,
				LastModifiedTime: entry.ModTime().UTC().UnixNano() / int64(time.Millisecond),
				StorageID:        req.StorageID,
			}
		}

		if fileObj.Type == FILE {
			fileObjects = append(fileObjects, *fileObj)
		} else {
			dirObjects = append(dirObjects, *fileObj)
		}
	}

	if req.OnlyDirectory {
		By(fsutils.TagSortFunc(req.Sort, req.Asc)).Sort(dirObjects)

		return dirObjects.List(req.Pagable), len(dirObjects), nil
	}

	By(fsutils.TagSortFunc(req.Sort, req.Asc)).Sort(dirObjects)
	By(fsutils.TagSortFunc(req.Sort, req.Asc)).Sort(fileObjects)

	fileObjects = append(dirObjects, fileObjects...)

	return fileObjects.List(req.Pagable), len(fileObjects), nil
}

func (cs *cloudStorage) newDir(req *newDirReq) (err error) {
	uid, gid := cs.userPrincipal.LinuxUID, cs.userPrincipal.LinuxGID

	absParentDir, err := cs.sanitizePath(req.ParentPath, req.StorageID)
	if err != nil {
		return
	}

	dir := strings.TrimSpace(fmt.Sprintf("%s/%s", absParentDir, req.Name))
	logger.Infof(cs.ctx, "Creating directory: [ %s ]", dir)
	if err = iotools.EnsureOwnDirR(dir, cs.rootPath(req.StorageID), uid, gid); err != nil {
		return
	}

	evt := cs.auditEvtTemplate()
	evt.Name = pb.EventName_EVENT_NAME_STORAGE_FILE_CREATED.String()
	evt.Entities = []*pb.Entity{
		{
			Type: pb.EntityType_ENTITY_TYPE_STORAGE.String(),
			Name: req.Name,
		},
	}
	evt.Payload = fsutils.H{"storage-id": req.StorageID, "parent": req.ParentPath, "name": req.Name}.ToJson()
	if _, err := cs.ac.Create(cs.ctx, evt); err != nil {
		logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
	}

	return
}

func (cs *cloudStorage) moveDir(req *movDirReq) (err error) {
	src, err := cs.sanitizePath(req.Src, req.SrcStorageID)
	if err != nil {
		return
	}
	dst, err := cs.sanitizePath(req.Dst, req.DstStorageID)
	if err != nil {
		return
	}

	if fSrc, err := os.Lstat(src); os.IsNotExist(err) || !fSrc.IsDir() {
		return fmt.Errorf("src directory not exists or not a directory: %w", err)
	}
	if _, err := os.Lstat(dst); err == nil {
		return iotools.Move2Dir(src, dst, true)
	}

	err = os.Rename(src, dst)
	if err != nil {
		return err
	}
	evt := cs.auditEvtTemplate()
	evt.Name = pb.EventName_EVENT_NAME_STORAGE_FILE_RENAMED.String()
	evt.Entities = []*pb.Entity{
		{
			Type: pb.EntityType_ENTITY_TYPE_STORAGE.String(),
			Name: req.Src,
		},
	}
	evt.Payload = fsutils.H{"src-storage-id": req.SrcStorageID, "src": src, "dst-storage-id": req.DstStorageID, "dst": dst}.ToJson()
	if _, err := cs.ac.Create(cs.ctx, evt); err != nil {
		logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
	}

	return
}

func (cs *cloudStorage) copyFile(req *copyFileReq) (err error) {
	src, err := cs.sanitizePath(req.Src, req.SrcStorageID)
	if err != nil {
		return
	}
	dst, err := cs.sanitizePath(req.Dst, req.DstStorageID)
	if err != nil {
		return
	}
	dstRoot := cs.rootPath(req.DstStorageID)

	uid, gid := cs.userPrincipal.LinuxUID, cs.userPrincipal.LinuxGID

	if !req.DstFile {
		if fileInfo, err := os.Stat(dst); !req.Force && (os.IsNotExist(err) || !fileInfo.IsDir()) {
			return fmt.Errorf("dst directory not exists or not a directory: %w", err)
		}
	}

	if _, err = os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("src file or directory not exists: %w", err)
	}

	if isSubDir, _ := iotools.SubElem(src, dst); isSubDir {
		return fmt.Errorf("dst directory [%v] is a subdirectory of src directory [%v]. invalid operation", dst, src)
	}

	if req.Force {
		if req.DstFile {
			if err = iotools.EnsureOwnFileR(dst, dstRoot, uid, gid); err != nil {
				return
			}
		} else {
			if err = iotools.EnsureOwnDirR(dst, dstRoot, uid, gid); err != nil {
				return
			}
		}
	}
	evt := cs.auditEvtTemplate()
	evt.Name = pb.EventName_EVENT_NAME_STORAGE_FILE_COPIED.String()
	evt.Entities = []*pb.Entity{
		{
			Type: pb.EntityType_ENTITY_TYPE_STORAGE.String(),
			Name: req.Dst,
		},
	}
	evt.Payload = fsutils.H{"src-storage-id": req.SrcStorageID, "src": src, "dst-storage-id": req.DstStorageID, "dst": dst}.ToJson()

	if !req.Move {
		if req.DstFile {
			logger.Infof(cs.ctx, "Coping file [ %s ] to file: [ %s ]", src, dst)
			err = iotools.CopyFile(src, dst)
		} else {
			logger.Infof(cs.ctx, "Coping file [ %s ] to directory: [ %s ]", src, dst)
			err = iotools.Copy2Dir(src, dst, req.ExcludeDir)
			if err != nil {
				return
			}
			if err = iotools.ChownR(dst, uid, gid); err != nil {
				return
			}
		}

		if _, err := cs.ac.Create(cs.ctx, evt); err != nil {
			logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
		}
		return
	}

	evt.Name = pb.EventName_EVENT_NAME_STORAGE_FILE_MOVED.String()

	if req.DstFile {
		logger.Infof(cs.ctx, "Moving file [ %s ] to file: [ %s ]", src, dst)
		if req.SrcStorageID != req.DstStorageID { // diff storage, copy then remove
			err = iotools.CopyFile(src, dst)
			if err != nil {
				return
			}
			err = iotools.Remove(src)
			if err != nil {
				return
			}
		} else {
			err = iotools.Move2File(src, dst)
		}
	} else {
		logger.Infof(cs.ctx, "Moving file [ %s ] to directory: [ %s ]", src, dst)
		// diff storage, copy then remove;
		if req.SrcStorageID != req.DstStorageID {
			err = iotools.Copy2Dir(src, dst, req.ExcludeDir)
			if err != nil {
				return
			}
			if err = iotools.ChownR(dst, uid, gid); err != nil {
				return
			}
			err = iotools.Remove(src)
			if err != nil {
				return
			}
		} else {
			err = iotools.Move2Dir(src, dst, req.ExcludeDir) // just rename it
		}
	}
	if err != nil {
		return
	}
	if _, err := cs.ac.Create(cs.ctx, evt); err != nil {
		logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
	}
	return
}

func (cs *cloudStorage) renameFile(req *renameFileReq) (err error) {
	path, err := cs.sanitizePath(req.Path, req.StorageID)
	if err != nil {
		return
	}
	if _, err = os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("directory or file [%s] not exists: %w", req.Path, err)
	}

	dir, _ := filepath.Split(path)
	newPath := filepath.Join(dir, req.Name)
	logger.Infof(cs.ctx, "Rename file: [ %s ] to file: [ %s ]", path, newPath)
	if path == newPath {
		return nil
	}
	fi, _ := os.Lstat(newPath)
	if fi != nil {
		return fmt.Errorf("directory or file with name [%s] already exists, try give a different one instead", req.Name)
	}
	err = os.Rename(path, newPath)

	if err != nil {
		return
	}

	evt := cs.auditEvtTemplate()
	evt.Name = pb.EventName_EVENT_NAME_STORAGE_FILE_RENAMED.String()
	evt.Entities = []*pb.Entity{
		{
			Type: pb.EntityType_ENTITY_TYPE_STORAGE.String(),
			Name: path,
		},
	}
	evt.Payload = fsutils.H{"origin": path, "new": newPath}.ToJson()
	if _, err := cs.ac.Create(cs.ctx, evt); err != nil {
		logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
	}

	return
}

func (cs *cloudStorage) deleteFile(req *delFileReq) (err error) {
	if strings.TrimSpace(req.Path) == "" {
		return nil
	}
	path, err := cs.sanitizePath(req.Path, req.StorageID)
	if err != nil {
		return
	}

	fSrc, err := os.Lstat(path)
	if err != nil {
		return err
	}

	doCleanUpload := func() {
		tmpFileList, _ := cs.store.DestroyUploadByKey(path)
		for _, tmpFile := range tmpFileList {
			logrus.Warnf("Delete file of uploading [ %v ] when delete path [ %v ]", tmpFile, path)
			_ = os.RemoveAll(tmpFile)
		}
	}

	if req.FileOnly {
		if fSrc.IsDir() {
			doCleanUpload()
		}
		logger.Infof(cs.ctx, "Delete file in [ %s ]", path)

		return iotools.RemoveFileR(path)
	}

	if fSrc.IsDir() {
		doCleanUpload()
	}

	logrus.Infof("Delete file or dir named [ %s ]", path)
	err = os.RemoveAll(path)
	if err != nil {
		logrus.Errorf("error when try deleting file or dir named [ %s ], details are: %v", path, err)

		return
	}

	evt := cs.auditEvtTemplate()
	evt.Name = pb.EventName_EVENT_NAME_STORAGE_FILE_DELETED.String()
	evt.Entities = []*pb.Entity{
		{
			Type: pb.EntityType_ENTITY_TYPE_STORAGE.String(),
			Name: path,
		},
	}
	evt.Payload = fsutils.H{"file": path, "storage-id": req.StorageID}.ToJson()
	if _, err := cs.ac.Create(cs.ctx, evt); err != nil {
		logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
	}

	return
}

func listFile(c *gin.Context) {
	req := listFileReq{StorageID: -1}
	if err := c.ShouldBind(&req); err != nil {
		_ = c.Error(err)

		return
	}

	if req.Size == 0 {
		req.Size = 10
	}
	if strings.TrimSpace(req.Pattern) == "" {
		req.Pattern = "*"
	}
	if req.Sort == "" {
		req.Sort = "name"
		req.Asc = true
	}

	err := fsutils.ValidateSort(&fileObject{}, req.Sortable)
	if err != nil {
		_ = c.Error(err)

		return
	}

	cloudStorage := getCloudStorage(c)

	fileObjs, totalElements, err := cloudStorage.listFile(&req)
	if err != nil {
		if os.IsNotExist(err) && req.IgnoreNotFound {
			c.JSON(http.StatusOK, &listFileRes{Code: 2001, Message: "success", Data: &pageableListFileRes{&fileObjs,
				req.Page,
				req.Size, 0, 0}})

			return
		}
		logger.Errorf(cloudStorage.ctx, "err when list file: %s", err.Error())
		_ = c.Error(err)

		return
	}
	totalPages := (totalElements + req.Size - 1) / req.Size

	dir, _ := cloudStorage.sanitizePath(req.Path, req.StorageID)

	logger.Debugf(cloudStorage.ctx, "List directory: [ %s ], total elements: %v, page size: %d, total pages: %d", dir, totalElements, req.Size, totalPages)
	if fileObjs == nil {
		fileObjs = make([]fileObject, 0)
	}
	c.JSON(http.StatusOK, &listFileRes{Code: 2001, Message: "success", Data: &pageableListFileRes{&fileObjs,
		req.Page,
		req.Size,
		totalElements,
		totalPages,
	}})
}

func newDir(c *gin.Context) {
	newDirReq := newDirReq{StorageID: -1}
	if err := c.ShouldBindJSON(&newDirReq); err != nil {
		_ = c.Error(err)

		return
	}

	cloudStorage := getCloudStorage(c)
	err := cloudStorage.newDir(&newDirReq)
	if err != nil {
		logger.Errorf(cloudStorage.ctx, "err when mkdir: %s", err.Error())
		_ = c.Error(err)

		return
	}

	c.JSON(http.StatusOK, DefaultSuccess)
}

func copyFile(c *gin.Context) {
	req := copyFileReq{SrcStorageID: -1, DstStorageID: -1}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)

		return
	}

	cs := getCloudStorage(c)

	doCopy := func(group string) error {
		err := cs.copyFile(&req)
		if group != "" {
			if err != nil {
				_ = cs.store.DoneCopyLineG(req.Src, req.SrcStorageID, req.Dst, req.DstStorageID, false, group, err.Error())

				return err
			}
			_ = cs.store.DoneCopyLineG(req.Src, req.SrcStorageID, req.Dst, req.DstStorageID, true, group, "")
		}

		return nil
	}

	if req.Sync { // sync mode: ignore process handling
		err := doCopy("")
		if err != nil {
			_ = c.Error(err)

			return
		}
		c.JSON(http.StatusOK, DefaultSuccess)

		return
	}

	uid := uuid.New().String()
	err := cs.store.AddCopyLineG(req.Src, req.SrcStorageID, req.Dst, req.DstStorageID, uid)
	if err != nil {
		_ = c.Error(err)

		return
	}
	go func() {
		_ = doCopy(uid)
	}()
	c.JSON(http.StatusOK, &BaseResponse{Code: 2001, Message: "success", Data: uid})
}

func moveDir(c *gin.Context) {
	movDirReq := movDirReq{SrcStorageID: -1, DstStorageID: -1}
	if err := c.ShouldBindJSON(&movDirReq); err != nil {
		_ = c.Error(err)

		return
	}

	cloudStorage := getCloudStorage(c)

	doMove := func() error {
		err := cloudStorage.moveDir(&movDirReq)
		if err != nil {
			logger.Errorf(cloudStorage.ctx, "err when move dir: %s", err.Error())
		}

		return err
	}

	var err = doMove()

	if err != nil {
		_ = c.Error(err)

		return
	}
	c.JSON(http.StatusOK, DefaultSuccess)
}

func renameFile(c *gin.Context) {
	renameFileReq := renameFileReq{StorageID: -1}
	if err := c.ShouldBindJSON(&renameFileReq); err != nil {
		_ = c.Error(err)

		return
	}

	cloudStorage := getCloudStorage(c)
	err := cloudStorage.renameFile(&renameFileReq)
	if err != nil {
		logger.Errorf(cloudStorage.ctx, "err when rename file: %s", err.Error())
		_ = c.Error(err)

		return
	}

	c.JSON(http.StatusOK, DefaultSuccess)
}

func deleteFile(c *gin.Context) {
	delFileReq := delFileReq{StorageID: -1}
	if err := c.ShouldBindJSON(&delFileReq); err != nil {
		_ = c.Error(err)

		return
	}
	cloudStorage := getCloudStorage(c)
	cloudStorage.ignoreNameCheck = true // https://github.com:8443/ellis-chen/compute-cloud/-/issues/2257
	err := cloudStorage.deleteFile(&delFileReq)
	if err != nil {
		logger.Errorf(cloudStorage.ctx, "err when delete file: %s", err.Error())
		_ = c.Error(err)

		return
	}

	c.JSON(http.StatusOK, DefaultSuccess)
}

func batchDelFile(c *gin.Context) {
	req := batchDeleteFileReq{StorageID: -1}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)

		return
	}
	var tryErr error

	cloudStorage := getCloudStorage(c)
	cloudStorage.ignoreNameCheck = true // https://github.com:8443/ellis-chen/compute-cloud/-/issues/2257
	for _, path := range req.Paths {
		_ = multierror.Append(tryErr, cloudStorage.deleteFile(&delFileReq{Path: path, StorageID: req.StorageID}))
	}

	if tryErr != nil {
		logger.Errorf(cloudStorage.ctx, "err when batch delete: %s", tryErr.Error())
		_ = c.Error(tryErr)

		return
	}

	c.JSON(http.StatusOK, DefaultSuccess)
}

func batchCopyFile(c *gin.Context) {
	req := batchCopyFileReq{SrcStorageID: -1, DstStorageID: -1}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)

		return
	}

	var tryErr error
	cs := getCloudStorage(c)

	doCopy := func(group string) error {
		g, _ := errgroup.WithContext(context.TODO())

		for _, src := range req.Srcs {
			src := src
			g.Go(func() error {
				err := cs.copyFile(&copyFileReq{
					SrcStorageID: req.SrcStorageID,
					Src:          src,
					DstStorageID: req.DstStorageID,
					Dst:          req.Dst,
					Move:         req.Move,
					Force:        req.Force,
				})
				if group != "" {
					if err != nil {
						_ = cs.store.DoneCopyLineG(src, req.SrcStorageID, req.Dst, req.DstStorageID, false, group, err.Error())

						return err
					}
					_ = cs.store.DoneCopyLineG(src, req.SrcStorageID, req.Dst, req.DstStorageID, true, group, "")
				}

				return nil
			})
		}

		tryErr = g.Wait()
		if tryErr != nil {
			logger.Errorf(cs.ctx, "err when batch copy: %s", tryErr.Error())

			return tryErr
		}

		return nil
	}

	if req.Sync { // sync mode: ignore process handling
		err := doCopy("")
		if err != nil {
			_ = c.Error(err)

			return
		}
		c.JSON(http.StatusOK, DefaultSuccess)

		return
	}

	uid := uuid.New().String()
	for _, src := range req.Srcs {
		err := cs.store.AddCopyLineG(src, req.SrcStorageID, req.Dst, req.DstStorageID, uid)
		if err != nil {
			_ = c.Error(err)

			return
		}
	}
	go func() {
		_ = doCopy(uid)
	}()
	c.JSON(http.StatusOK, &BaseResponse{Code: 2001, Message: "success", Data: uid})
}

func copyLineProcess(c *gin.Context) {
	copyGroupID := c.Query("id")
	if copyGroupID == "" {
		c.JSON(http.StatusOK, &BaseResponse{Code: 4001, Message: "not found given progess id"})

		return
	}

	cs := getCloudStorage(c)
	copyLines, _ := cs.store.LoadCopyLinesByG(copyGroupID)

	if len(copyLines) == 0 {
		c.JSON(http.StatusOK, &BaseResponse{Code: 4001, Message: fmt.Sprintf("not found given progess id [ %v ]", copyGroupID)})

		return
	}

	success := true
	failed := false
	extra := ""
	for _, cl := range copyLines {
		success = success && (cl.State == storage.DONE)
		if cl.State == storage.FAILED {
			failed = true
			extra = cl.Extra

			break
		}
	}
	status := "RUNNING"
	if success {
		status = "SUCCESS"
	}
	if failed {
		status = "FAILED"
	}
	c.JSON(http.StatusOK, &BaseResponse{Code: 2001, Message: "success", Data: gin.H{"status": status, "detail": copyLines, "extra": extra}})
}

func used(c *gin.Context) {
	cs := getCloudStorage(c)
	c.JSON(http.StatusOK, &BaseResponse{Code: 2001, Message: "success", Data: cs.bc.GetEfsUsage()})
}

func (cs *cloudStorage) sanitizePath(path string, storeID int) (p string, err error) {
	_, filename := filepath.Split(path)
	if !cs.ignoreNameCheck && cs.badFilenameRegex.MatchString(filename) {
		err = ErrInvalidFileName

		return
	}
	switch storeID {
	case -1:
		p = filepath.Clean(strings.TrimSpace(fmt.Sprintf("%s/%s", cs.SystemRootPrefix, path)))
		if isSubDir, _ := iotools.SubElem(cs.SystemRootPrefix, p); !isSubDir {
			err = ErrInvalidFilePath
		}
	case -99:
		p = filepath.Clean(strings.TrimSpace(fmt.Sprintf("%s/%s", cs.AuditRootPrefix, path)))
		if isSubDir, _ := iotools.SubElem(cs.AuditRootPrefix, p); !isSubDir {
			err = ErrInvalidFilePath
		}
	case -2:
		p = filepath.Clean(strings.TrimSpace(fmt.Sprintf("%s/%s", cs.AuditApprovedRootPrefix, path)))
		if isSubDir, _ := iotools.SubElem(cs.AuditApprovedRootPrefix, p); !isSubDir {
			err = ErrInvalidFilePath
		}
	default:
		p = filepath.Clean(strings.TrimSpace(fmt.Sprintf("%s/%d/%s", cs.ExternalMntPrefix, storeID, path)))
		if isSubDir, _ := iotools.SubElem(cs.ExternalMntPrefix, p); !isSubDir {
			err = ErrInvalidFilePath
		}
	}

	return
}

func (cs *cloudStorage) rootPath(storeID int) (p string) {
	switch storeID {
	case -1:
		p = cs.SystemRootPrefix
	case -99:
		p = cs.AuditRootPrefix
	case -2:
		p = cs.AuditApprovedRootPrefix
	default:
		p = filepath.Clean(strings.TrimSpace(fmt.Sprintf("%s/%d", cs.ExternalMntPrefix, storeID)))
	}

	return
}

const (
	// DIR ...
	DIR FileType = "DIR"
	// FILE ...
	FILE FileType = "FILE"
)

var (
	// ErrInvalidFilePath user try to access directory or files that do not belong to him
	ErrInvalidFilePath = errors.New("invalid path, must not operate on other user's directory or file")

	ErrInvalidFileName = errors.New("invalid file or directory name, specical chars like [ / \\ \" \\' : < ? > ] are not allowed as filename")
)
