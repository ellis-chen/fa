package routers

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/pkg/fsutils"
	pb "github.com/ellis-chen/fa/third_party/audit/v1"
	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func rListOrDownload(c *gin.Context) {
	req := listFileReq{StorageID: -1}
	if err := c.ShouldBindUri(&req); err != nil {
		_ = c.Error(err)

		return
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(err)

		return
	}

	action := c.DefaultQuery("action", "list")

	// disable page support
	req.Page = -1
	req.Size = -1
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

	cs := getCloudStorage(c)

	fileObjs, totalElements, err := cs.listFile(&req)

	if err != nil {
		logger.Errorf(cs.ctx, "err when list file: %s", err.Error())

		if os.IsNotExist(err) {
			_ = c.Error(&appError{Code: 404,
				Message: fmt.Sprintf("not found given file or directory: %s", req.Path)})

			return
		}
		_ = c.Error(err)

		return
	}

	if len(fileObjs) == 1 && fileObjs[0].Type == FILE && action == "download" {
		file := fileObjs[0]
		path, _ := cs.sanitizePath(file.Path, req.StorageID)
		c.Header("Last-Modified", time.UnixMilli(file.LastModifiedTime).UTC().Format(http.TimeFormat))
		c.Header("Content-Length", fmt.Sprint(file.Size))
		c.Header("Accept-Ranges", "bytes")
		c.FileAttachment(path, file.Name)

		if _, err := cs.ac.Create(cs.ctx, &pb.CreateEventRequest{
			Name:      pb.EventName_EVENT_NAME_STORAGE_FILE_DOWNLOADED.String(),
			CreatedAt: timestamppb.New(time.Now()),
			Entities: []*pb.Entity{
				{
					Type: pb.EntityType_ENTITY_TYPE_STORAGE.String(),
					Name: file.Name,
				},
			},
			Operator: &pb.Operator{Type: pb.OperatorType_OPERATOR_TYPE_USER, Name: cs.userPrincipal.UserName},
			Payload:  fsutils.H{"files": file.Name, "storage-id": req.StorageID, "source": "rclone"}.ToJson(),
			Device:   &pb.Device{Ip: fsutils.ReadUserIP(c.Request)},
		}); err != nil {
			logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
		}
		return
	}

	if action == "download" && strings.HasSuffix(req.Path, "index.html") { // handle xxx/index.html special case
		path, err := cs.sanitizePath(req.Path, req.StorageID)
		if err != nil {
			_ = c.Error(err)

			return
		}

		fi, err := os.Stat(fmt.Sprintf("%s/index.html", path))
		if err == nil {
			c.FileAttachment(path, fi.Name())

			return
		}
		_ = c.Error(err)

		return
	}

	if fileObjs == nil {
		fileObjs = make([]fileObject, 0)
	}
	c.JSON(http.StatusOK, &listFileRes{Code: 2001, Message: "success", Data: &pageableListFileRes{&fileObjs,
		req.Page,
		req.Size,
		totalElements,
		-1,
	}})
}

func rInfoFile(c *gin.Context) {
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

	if fileObj.Type == DIR {
		_ = c.Error(&appError{Code: http.StatusNotAcceptable,
			Message: fmt.Sprintf("directory %s found", req.Path)})

		return
	}

	c.Header("Last-Modified", time.UnixMilli(fileObj.LastModifiedTime).UTC().Format(http.TimeFormat))
	c.Header("Content-Length", fmt.Sprint(fileObj.Size))
	c.Header("Content-Type", fileObj.Mime)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Disposition", fileObj.Name)
	c.Writer.Flush()
}
