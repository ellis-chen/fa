package routers

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/internal/storage"
	"github.com/ellis-chen/fa/pkg/cipher"
	"github.com/ellis-chen/fa/pkg/fsutils"
	"github.com/ellis-chen/fa/pkg/iotools"
	pb "github.com/ellis-chen/fa/third_party/audit/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gin-gonic/gin"
)

type (
	UploadSmallReq struct {
		Path      string `uri:"path"`
		StorageID int    `form:"storage_id"`
	}
	// CreateMultipartReq ...
	CreateMultipartReq struct {
		StorageID  int    `json:"storage_id"`
		Key        string `json:"key"`
		Size       uint64 `json:"size"`
		ChunkSize  uint64 `json:"chunkSize"`
		ContentMD5 string `json:"contentMD5"`
	}

	// PartReq ...
	PartReq struct {
		Token string `uri:"token" binding:"required"`
		Index uint64 `uri:"index" `
		MD5   string `header:"X-Content-MD5"`
	}

	// CompletePartReq ...
	CompletePartReq struct {
		StorageID    int    `json:"storage_id"`
		Key          string `json:"key" binding:"required"`
		Token        string `json:"token"`
		Parts        []Part `json:"parts"`
		ModifiedTime string `header:"X-Last-Modified"`
	}

	// PartRes ...
	PartRes struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    Part   `json:"data"`
	}

	// BaseResponse ...
	BaseResponse struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data"`
	}

	// CompletePartRes ...
	CompletePartRes struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		// BaseResponse
		Data CompletePart `json:"data"`
	}

	// CompletePart ...
	CompletePart struct {
		Etag string `json:"etag"`
	}

	// Part ...
	Part struct {
		Idx  uint64 `json:"index"`
		Etag string `json:"etag"`
	}

	// InitPart ...
	InitPart struct {
		Token string `json:"token"`

		Parts []Part `json:"parts"`
	}

	// InitPartResponse ...
	InitPartResponse struct {
		Data    InitPart `json:"data"`
		Code    int      `json:"code"`
		Message string   `json:"message"`
	}

	// Parts implements sort.Interface based on the Idx field.
	Parts []Part
)

func (p *CreateMultipartReq) String() string {
	return fmt.Sprintf("Key: %s, Size: %.2fMB, ChunkSize: %dMB, ContentMD5: %s", p.Key, float64(p.Size)/float64(fsutils.MB), p.ChunkSize, p.ContentMD5)
}

func (p *PartReq) String() string {
	return fmt.Sprintf("Token: %s, Index: %d, MD5: %s", p.Token, p.Index, p.MD5)
}

func (a Parts) Len() int           { return len(a) }
func (a Parts) Less(i, j int) bool { return a[i].Idx < a[j].Idx }
func (a Parts) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func (cs *cloudStorage) initMultipartUpload(req *CreateMultipartReq) (*InitPartResponse, error) {
	store := cs.store
	token, existed, err := store.InitUpload(req.Key, req.Size, req.ChunkSize*fsutils.MB, req.StorageID)
	if existed { // remain an upload token
		if errors.Is(err, storage.ErrPartUpCorrupted) { // token exists, but client upload chunk changes
			_ = store.DestroyUploadByToken(token)
			_ = os.Remove(req.Key)
			token, _, _ := store.InitUpload(req.Key, req.Size, req.ChunkSize*fsutils.MB, req.StorageID)

			return &InitPartResponse{Code: 2003, Message: "success", Data: InitPart{Token: token}}, nil
		}

		parts, err := store.LoadParts(token)
		if err != nil {
			return nil, err
		}

		var partList = make([]Part, len(parts))
		for idx, part := range parts {
			partList[idx] = Part{Idx: part.Idx, Etag: part.MD5}
		}

		return &InitPartResponse{Code: 2002, Message: "success", Data: InitPart{Token: token, Parts: partList}}, nil
	}

	if err != nil {
		logger.Errorf(cs.ctx, "err when invoke InitUpload, details goes as: %v", err.Error())

		return nil, ErrInitPart
	}

	logger.Infof(cs.ctx, "Initialize multipart-upload for [ %s ]", req)

	return &InitPartResponse{Code: 2001, Message: "success", Data: InitPart{Token: token}}, nil
}

func (cs *cloudStorage) uploadPart(req *PartReq, readSeeker io.Reader) (res *PartRes, err error) {
	uid, gid := cs.userPrincipal.LinuxUID, cs.userPrincipal.LinuxGID
	store := cs.store
	upInfo, err := store.LoadUpload(req.Token)
	if err != nil {
		return
	}
	logger.Debugf(cs.ctx, "Upload an part of file: [ %s ] with part info: [ %s ]", upInfo.Key, req)

	name := upInfo.TmpName

	err = iotools.EnsureOwnFileR(name, cs.rootPath(upInfo.StorageID), uid, gid)
	if err != nil {
		return
	}

	file, err := iotools.Create(name)
	if err != nil {
		return
	}
	defer file.Close()

	// reset the offset
	offset := upInfo.ChunkSize * req.Index
	logger.Debugf(cs.ctx, "Offset of file: [ %s ] is : %d", upInfo.Key, offset)
	_, err = file.Seek(int64(offset), 0)
	if err != nil {
		return
	}

	buffer := iotools.MyBufAllocator.GetBuffer()
	defer iotools.MyBufAllocator.PutBuffer(buffer)
	md5Calc, err := cipher.MD5ReadWrite(file, readSeeker, buffer.GetBuf())
	if err != nil {
		return
	}
	if md5Calc != req.MD5 {
		logger.Errorf(cs.ctx, "file corrupted, mismatch md5, given md5: %s, actual: %s", req.MD5, md5Calc)

		return nil, ErrPartCorrupted
	}

	_, err = store.UploadPart(upInfo.Key, req.Index, md5Calc)
	if err != nil {
		return
	}

	return &PartRes{Code: 2001, Message: "success", Data: Part{Idx: req.Index, Etag: md5Calc}}, nil
}

func (cs *cloudStorage) completePart(req *CompletePartReq) (*CompletePartRes, error) {
	store := cs.store
	uploadInfo, err := store.LoadUploadByKey(req.Key)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(req.Parts, func(i, j int) bool { return req.Parts[i].Idx < req.Parts[j].Idx })
	var byteSlice []byte
	for _, part := range req.Parts {
		decodeString, err := hex.DecodeString(part.Etag)
		if err != nil {
			return nil, ErrPartCorrupted
		}
		byteSlice = append(byteSlice, decodeString...)
	}
	clientMD5, err := cipher.MD5Single(bytes.NewReader(byteSlice))
	if err != nil {
		return nil, ErrPartCorrupted
	}
	if len(req.Parts) > 1 {
		clientMD5 = fmt.Sprintf("%s-%d", clientMD5, len(req.Parts))
	}

	err = store.CompletePart(req.Key, clientMD5)
	if err != nil {
		return nil, err
	}
	if _, err = os.Lstat(uploadInfo.TmpName); err == nil {
		err = os.Rename(uploadInfo.TmpName, uploadInfo.Key)
		if err != nil {
			return nil, err
		}
	}
	if modTime, err := http.ParseTime(req.ModifiedTime); err == nil {
		_ = iotools.Chtimes(uploadInfo.Key, modTime)
	}

	logger.Infof(cs.ctx, "Complete multipart-upload for upload: [ %s ], final file md5 are: [ %s ]", uploadInfo, clientMD5)

	return &CompletePartRes{Code: 2001, Message: "success", Data: CompletePart{Etag: clientMD5}}, nil
}

// upload a small file
func uploadSmall(c *gin.Context) {
	// single file
	file, _ := c.FormFile("file")
	rel := filepath.Clean(c.PostForm("key"))
	storageID, err := strconv.Atoi(c.DefaultPostForm("storage_id", "-1"))
	if err != nil {
		_ = c.Error(err)

		return
	}
	cs := getCloudStorage(c)

	key, err := cs.sanitizePath(rel, storageID)
	if err != nil {
		_ = c.Error(err)

		return
	}

	uid, gid := cs.userPrincipal.LinuxUID, cs.userPrincipal.LinuxGID

	logger.Infof(cs.ctx, "Upload a small file at [ %s ], rel [ %s ]", key, rel)
	err = iotools.EnsureOwnFileR(key, cs.rootPath(storageID), uid, gid)
	if err != nil {
		_ = c.Error(err)

		return
	}
	// Upload the file to specific dst.
	if err := c.SaveUploadedFile(file, key); err != nil {
		logger.Errorf(cs.ctx, "err when upload small file: [ %s ], detail is: %s", key, err.Error())
		_ = c.Error(err)

		return
	}

	if _, err := cs.ac.Create(cs.ctx, &pb.CreateEventRequest{
		Name:      pb.EventName_EVENT_NAME_STORAGE_FILE_UPLOADED.String(),
		CreatedAt: timestamppb.New(time.Now()),
		Entities: []*pb.Entity{
			{
				Type: pb.EntityType_ENTITY_TYPE_STORAGE.String(),
				Name: key,
			},
		},
		Operator: &pb.Operator{Type: pb.OperatorType_OPERATOR_TYPE_USER, Name: cs.userPrincipal.UserName},
		Payload:  fsutils.H{"files": key, "storage-id": storageID}.ToJson(),
		Device:   &pb.Device{Ip: fsutils.ReadUserIP(c.Request)},
	}); err != nil {
		logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
	}

	_, filename := filepath.Split(rel)

	if modTime, err := http.ParseTime(c.GetHeader("X-Last-Modified")); err == nil {
		_ = iotools.Chtimes(key, modTime)
	}

	c.JSON(http.StatusOK, &BaseResponse{Code: 2001, Message: "success", Data: struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}{rel, filename}})
}

// CreateMulitpartUpload ==
func createMulitpartUpload(c *gin.Context) {
	req := &CreateMultipartReq{StorageID: -1}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)

		return
	}
	cs := getCloudStorage(c)
	path, err := cs.sanitizePath(req.Key, req.StorageID)
	if err != nil {
		_ = c.Error(err)

		return
	}
	req.Key = path

	res, err := cs.initMultipartUpload(req)
	if err != nil {
		logger.Errorf(cs.ctx, "err when create multipart[%s], detail is: %s", req.Key, err.Error())
		if errors.Is(err, ErrInitPart) {
			_ = c.Error(fmt.Errorf("error when creating multipart upload: %w", err))

			return
		}
		_ = c.Error(fmt.Errorf("error when creating multipart upload: %w", err))

		return
	}
	c.JSON(http.StatusOK, res)
}

func uploadPart(c *gin.Context) {
	partReq := PartReq{}
	if err := c.ShouldBindUri(&partReq); err != nil {
		_ = c.Error(err)

		return
	}
	if err := c.ShouldBindHeader(&partReq); err != nil {
		_ = c.Error(err)

		return
	}
	cloudStorage := getCloudStorage(c)

	res, err := cloudStorage.uploadPart(&partReq, c.Request.Body)
	if err != nil {
		logger.Errorf(cloudStorage.ctx, "err when upload part[%d], detail is: %s", partReq.Index, err.Error())
		if errors.Is(err, ErrPartCorrupted) {
			_ = c.Error(appError{Code: 612, Message: err.Error()})

			return
		}
		_ = c.Error(err)

		return
	}

	c.JSON(http.StatusOK, res)
}

func completeMultipartUpload(c *gin.Context) {
	req := CompletePartReq{StorageID: -1}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)

		return
	}
	_ = c.ShouldBindHeader(&req) // optional

	cs := getCloudStorage(c)

	path, err := cs.sanitizePath(req.Key, req.StorageID)
	if err != nil {
		_ = c.Error(err)

		return
	}
	req.Key = path

	res, err := cs.completePart(&req)
	if err != nil {
		logger.Errorf(cs.ctx, "err when complete part[%s], detail is: %s", req.Key, err.Error())
		if errors.Is(err, ErrPartCorrupted) {
			_ = c.Error(appError{Code: 612, Message: err.Error()})

			return
		}
		_ = c.Error(err)

		return
	}
	if _, err := cs.ac.Create(cs.ctx, &pb.CreateEventRequest{
		Name:      pb.EventName_EVENT_NAME_STORAGE_FILE_UPLOADED.String(),
		CreatedAt: timestamppb.New(time.Now()),
		Entities: []*pb.Entity{
			{
				Type: pb.EntityType_ENTITY_TYPE_STORAGE.String(),
				Name: path,
			},
		},
		Operator: &pb.Operator{Type: pb.OperatorType_OPERATOR_TYPE_USER, Name: cs.userPrincipal.UserName},
		Payload:  fsutils.H{"files": path, "storage-id": req.StorageID}.ToJson(),
		Device:   &pb.Device{Ip: fsutils.ReadUserIP(c.Request)},
	}); err != nil {
		logger.Errorf(cs.ctx, "err creating audit event: %s", err.Error())
	}
	c.JSON(http.StatusOK, res)
}

var (
	// ErrPartCorrupted means that the uploading part changed during transfer
	ErrPartCorrupted = errors.New("file part corrupted during upload")

	// ErrInitPart init part failed, the client need to retry
	ErrInitPart = errors.New("init part failed, maybe duplicate token detected")

	// DefaultSuccess the default success message
	DefaultSuccess = &BaseResponse{Code: 2001, Message: "success"}
)
