package routers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/pkg/fsutils"
	"github.com/ellis-chen/fa/pkg/iotools"
	pb "github.com/ellis-chen/fa/third_party/audit/v1"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type (
	downloadReq struct {
		Path      string   `uri:"path"`
		Paths     []string `form:"p"`
		StorageID int      `form:"storage_id"`
	}
)

func infoFile(c *gin.Context) {
	req := downloadReq{StorageID: -1}
	if err := c.ShouldBindUri(&req); err != nil {
		_ = c.Error(err)

		return
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(err)

		return
	}

	cloudStorage := getCloudStorage(c)
	fileObj, err := cloudStorage.infoFile(&req)
	if err != nil {
		_ = c.Error(err)

		return
	}

	c.Header("Last-Modified", fsutils.DateFormat("", float64(fileObj.LastModifiedTime)))
	c.Header("Content-Length", fmt.Sprint(fileObj.Size))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Disposition", fileObj.Name)
	c.Writer.Flush()
}

func previewFile(c *gin.Context) {
	req := downloadReq{StorageID: -1}
	if err := c.ShouldBindUri(&req); err != nil {
		_ = c.Error(err)

		return
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(err)

		return
	}
	cs := getCloudStorage(c)
	fileObj, err := cs.infoFile(&req)
	if err != nil {
		_ = c.Error(err)

		return
	}
	f, err := os.Open(fileObj.Path)
	if err != nil {
		logrus.Error(err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			return
		}
	}()
	mimeType, r, _ := iotools.RecycleMimeReader(f)
	c.Writer.Header().Add("Content-Type", mimeType)
	iotools.BufCopy(c.Writer, r)

	evt := cs.auditEvtTemplate()
	evt.Name = pb.EventName_EVENT_NAME_STORAGE_FILE_PREVIEWD.String()
	evt.Entities = []*pb.Entity{
		{
			Type: pb.EntityType_ENTITY_TYPE_STORAGE.String(),
			Name: fileObj.Path,
		},
	}
	evt.Payload = fsutils.H{"file": fileObj.Path, "storage-id": req.StorageID}.ToJson()

	if _, err := cs.ac.Create(cs.ctx, evt); err != nil {
		logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
	}
}

func tailFile(c *gin.Context) {
	req := downloadReq{StorageID: -1}
	if err := c.ShouldBindUri(&req); err != nil {
		_ = c.Error(err)

		return
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(err)

		return
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "-1"))
	if err != nil {
		_ = c.Error(err)

		return
	}
	size, err := strconv.Atoi(c.DefaultQuery("size", "51200"))
	if err != nil {
		_ = c.Error(err)

		return
	}

	cs := getCloudStorage(c)
	fileObj, err := cs.infoFile(&downloadReq{Path: req.Path, StorageID: req.StorageID})
	if err != nil {
		_ = c.Error(err)

		return
	}
	bytes, lastread, err := iotools.TailFile(fileObj.Path, &iotools.PagableRead{Offset: int64(offset), Size: int64(size)})
	if err != nil {
		_ = c.Error(err)

		return
	}
	c.JSON(200, &BaseResponse{Code: 2001, Message: "success", Data: struct {
		Content string `json:"content"`
		Pointer int64  `json:"pointer"`
	}{Content: string(bytes), Pointer: lastread}})
}

func downloadFiles(c *gin.Context) {
	req := downloadReq{StorageID: -1}
	if err := c.ShouldBindUri(&req); err != nil {
		_ = c.Error(err)

		return
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(err)

		return
	}

	cs := getCloudStorage(c)

	singleDownload := false
	if req.Path != "/" {
		singleDownload = true
	}
	if len(req.Paths) == 1 {
		req.Path = req.Paths[0]
		singleDownload = true
	}

	evt := cs.auditEvtTemplate()
	evt.Name = pb.EventName_EVENT_NAME_STORAGE_FILE_DOWNLOADED.String()

	if singleDownload {
		fileObj, err := cs.infoFile(&req)
		if err != nil {
			logger.Errorf(cs.ctx, "err when download file: %s", err.Error())
			_ = c.Error(err)

			return
		}
		evt.Entities = []*pb.Entity{
			{
				Type: pb.EntityType_ENTITY_TYPE_STORAGE.String(),
				Name: fileObj.Path,
			},
		}
		evt.Payload = fsutils.H{"files": fileObj.Path, "storage-id": req.StorageID}.ToJson()

		if fileObj.Type == FILE {
			logger.Debugf(cs.ctx, "Downloading file: [ %s ]", fileObj.Path)
			c.Header("Last-Modified", time.UnixMilli(fileObj.LastModifiedTime).UTC().Format(http.TimeFormat))
			c.Header("Content-Length", fmt.Sprint(fileObj.Size))
			c.Header("Accept-Ranges", "bytes")
			c.FileAttachment(fileObj.Path, fileObj.Name)

			if _, err := cs.ac.Create(cs.ctx, evt); err != nil {
				logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
			}
			return
		}

		logger.Debugf(cs.ctx, "Downloading directory as zip file: [ %s ]", fileObj.Path)
		fi, err := os.Lstat(fileObj.Path)
		if err != nil {
			logger.Errorf(cs.ctx, "err when download file: %s", err.Error())
			_ = c.Error(err)

			return
		}
		c.Writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fmt.Sprintf("%s.zip", fi.Name())))
		err = iotools.StreamZip([]string{fileObj.Path}, c.Writer)
		if err != nil {
			logger.Errorf(cs.ctx, "err when download file: %s", err.Error())
			_ = c.Error(err)
		}
		if _, err := cs.ac.Create(cs.ctx, evt); err != nil {
			logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
		}
		return
	}

	paths := make([]string, 0)
	for _, p := range req.Paths {
		path, err := cs.sanitizePath(p, req.StorageID)
		if err != nil {
			continue
		}
		_, err = os.Lstat(path)
		if err != nil {
			continue
		}
		paths = append(paths, path)
	}

	if len(paths) == 0 { // nothing valid found
		return
	}

	logger.Debugf(cs.ctx, "Downloading [%d] file/dir as zip file: archive.zip", len(paths))
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", "archive.zip"))
	if err := iotools.StreamZip(paths, c.Writer); err != nil {
		logger.Errorf(cs.ctx, "err when download file: %s", err.Error())
		_ = c.Error(err)
	}
	evt.Entities = []*pb.Entity{
		{
			Type: pb.EntityType_ENTITY_TYPE_STORAGE.String(),
			Name: strings.Join(paths, ";"),
		},
	}
	evt.Payload = fsutils.H{"files": strings.Join(paths, ";"), "storage-id": req.StorageID}.ToJson()
	if _, err := cs.ac.Create(cs.ctx, evt); err != nil {
		logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
	}
}

func (cs *cloudStorage) infoFile(req *downloadReq) (fileObj *fileObject, err error) {
	path, err := cs.sanitizePath(req.Path, req.StorageID)
	if err != nil {
		return
	}
	fileInfo, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			return nil, &appError{Code: 404,
				Message: fmt.Sprintf("not found given file or directory: %s", req.Path)}
		}

		return nil, err
	}

	var ft = FILE
	if fileInfo.IsDir() {
		ft = DIR
	}
	fileObj = &fileObject{
		Type:             ft,
		Size:             fileInfo.Size(),
		LastModifiedTime: fileInfo.ModTime().UnixMilli(),
		Name:             fileInfo.Name(),
		Path:             path,
		Parent:           filepath.Dir(path),
	}

	return
}
