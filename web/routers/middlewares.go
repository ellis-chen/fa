package routers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ellis-chen/fa/global"
	"github.com/ellis-chen/fa/internal"
	"github.com/ellis-chen/fa/internal/billing"
	"github.com/ellis-chen/fa/internal/download"
	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/pkg/fsutils"
	"github.com/ellis-chen/fa/pkg/iotools"
	"github.com/ellis-chen/fa/pkg/mounts"
	"github.com/ellis-chen/fa/pkg/queue"
	pb "github.com/ellis-chen/fa/third_party/audit/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type (
	//
	// APP error definition
	//
	appError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
)

func (ae appError) Error() string {
	return ae.Message
}

// JwtMiddleware return an jwt middleware
func JwtMiddleware(conf *internal.Config) gin.HandlerFunc {
	logger.WithoutContext().Infof("Activating jwt auth ...")
	jwtMiddleware, err := jwt.New(&jwt.GinJWTMiddleware{
		Realm:            conf.JwtRealm,
		SigningAlgorithm: "HS512",
		Key:              []byte(conf.JwtSecret),
		IdentityKey:      internal.IdentityKey,
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)

			tenantID := claims["tenantId"]
			if tenantID == nil {
				tenantID = float64(0)
			}
			UserID := claims["userId"]
			if UserID == nil {
				UserID = float64(0)
			}
			LinuxUID := claims["linuxUid"]
			if LinuxUID == nil {
				LinuxUID = float64(0)
			}
			LinuxGID := claims["linuxGid"]
			if LinuxGID == nil {
				LinuxGID = float64(0)
			}

			return &internal.UserPrincipal{
				UserName: claims["username"].(string),
				UserID:   int(UserID.(float64)),
				LinuxUID: int(LinuxUID.(float64)),
				LinuxGID: int(LinuxGID.(float64)),
				TenantID: int64(tenantID.(float64)),
			}
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			log.Printf("unauthorized: %v, %v", code, message)
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},
		TokenLookup: "header: Authorization, query: token, cookie: Authorization",

		// TokenHeadName is a string in the header. Default value is "Bearer"
		TokenHeadName: "Bearer",

		// TimeFunc provides the current time. You can override it to use another time value. This is useful for testing or if your server uses a different time zone than your tokens.
		TimeFunc: time.Now,
	})

	if err != nil {
		log.Fatal("JWT Error:" + err.Error())
	}

	err = jwtMiddleware.MiddlewareInit()
	if err != nil {
		log.Fatal("authMiddleware.MiddlewareInit() Error:" + err.Error())
	}

	return jwtMiddleware.MiddlewareFunc()
}

var (
	startKey = "__start"
)

// FsContextMiddleware ...
func FsContextMiddleware(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(startKey, time.Now())

		var principal = fromCtx(c, internal.IdentityKey, internal.UnkownPrincipal).(*internal.UserPrincipal)
		config := s.GetConfig()
		userHomeDir := fmt.Sprintf("%s/users/%s", config.StorePath, principal.UserName)
		auditPrefix := fmt.Sprintf("%s/users/%s", config.AuditPath, principal.UserName)
		auditApprovedPrefix := fmt.Sprintf("%s/users/%s", config.AuditApprovedPath, principal.UserName)

		if _, err := os.Stat(userHomeDir); os.IsNotExist(err) {
			logrus.Infof("home dir [ %s ] not found, creating", userHomeDir)
			_ = iotools.EnsureOwnDir(userHomeDir, principal.LinuxUID, principal.LinuxGID)
			_ = os.Chmod(userHomeDir, 0700)
		}
		// for whatever reason, the user home directory's owner is not right, we just make the syscall to ensure it right
		_ = iotools.Chown(userHomeDir, principal.LinuxUID, principal.LinuxGID)
		if _, err := os.Stat(auditPrefix); os.IsNotExist(err) {
			_ = iotools.EnsureOwnDir(auditPrefix, principal.LinuxUID, principal.LinuxGID)
			_ = os.Chmod(auditPrefix, 0755)
		}
		if _, err := os.Stat(auditApprovedPrefix); os.IsNotExist(err) {
			_ = iotools.EnsureOwnDir(auditApprovedPrefix, principal.LinuxUID, principal.LinuxGID)
			_ = os.Chmod(auditApprovedPrefix, 0700)
		}

		// copy skel to home dir
		if _, err := os.Stat(fmt.Sprintf("%s/.bashrc", userHomeDir)); os.IsNotExist(err) {
			err = iotools.CopyDir("/etc/skel/", userHomeDir)
			if err == nil {
				for _, file := range initFileList {
					_ = iotools.Chown(fmt.Sprintf("%s/%s", userHomeDir, file), principal.LinuxUID, principal.LinuxGID)
				}
			}
		}

		ctx := logger.With(
			c,
			logger.Str(logger.TenantID, fmt.Sprint(principal.TenantID)),
			logger.Str(logger.UserID, fmt.Sprint(principal.UserID)),
			logger.Str(logger.Username, fmt.Sprint(principal.UserName)),
			logger.Str(global.CliVerKey, s.getCliVersion()),
		)
		cs := &cloudStorage{
			SystemRootPrefix:        userHomeDir,
			ExternalMntPrefix:       config.StoreMgrMntRoot,
			AuditRootPrefix:         auditPrefix,
			AuditApprovedRootPrefix: auditApprovedPrefix,
			mu:                      sync.Mutex{},
			store:                   s.storageMgr,
			ctx:                     ctx,
			userPrincipal:           principal,
			config:                  config,
			appTransferCtx:          s.jobCtx,
			badFilenameRegex:        regexp.MustCompile(`[\/:*?"'<>|]`),
			bc:                      s.GetBc(),
			ac:                      s.ac,

			gin: c,
		}

		c.Set(internal.CloudStorageKey, cs)

		uri := c.Request.RequestURI
		method := c.Request.Method

		if (strings.HasPrefix(uri, "/fa/api/v0/download/") && method == "GET") || (strings.HasPrefix(uri, "/fa/api/v0/upload/") && (method == "POST" || method == "PUT")) {
			s.dataMetric.Upload(principal.TenantID, principal.UserName, int(c.Request.ContentLength))
			if config.EnableBalanceCheck {
				balance, err := billing.QueryBalance(c.GetString("JWT_TOKEN"), principal.TenantID)
				if err != nil {
					_ = c.Error(err)

					return
				}
				var balanceLimit = float32(viper.GetInt("balance_limit"))
				if balance < balanceLimit {
					_ = c.Error(appError{Code: 402, Message: fmt.Sprintf("余额少于%v，不能下载", balanceLimit)})

					return
				}
			}
		}

		if !config.FcceMode && s.GetSampler().Channel(principal.TenantID) == nil {
			err := s.RegisterEfsSampler(principal.TenantID)
			if err != nil {
				return
			}
		}

		c.Next()
	}
}

func DownloadMiddleware(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		var (
			principal = fromCtx(c, internal.IdentityKey, internal.UnkownPrincipal).(*internal.UserPrincipal)
			startTime = fromCtx(c, startKey, time.Time{}).(time.Time)
			config    = s.GetConfig()
			uri       = c.Request.RequestURI
			method    = c.Request.Method
			cs        = getCloudStorage(c)
		)

		if !config.FcceMode && (strings.HasPrefix(uri, "/fa/api/v0/download/") || (strings.HasPrefix(uri, "/fa/api/v0/rclone/") && c.Request.FormValue("action") == "download")) && method == "GET" {
			resSz := c.Writer.Size()
			if resSz < 0 {
				return
			}
			s.dataMetric.Download(principal.TenantID, principal.UserName, resSz)
			path := strings.TrimLeft(c.Params.ByName("path"), "/")
			logrus.Debugf("download file: [ %v ], res size is: [ %v ]", path, resSz)
			storageID, err := strconv.Atoi(c.DefaultQuery("storage_id", "-1"))
			if err != nil {
				_ = c.Error(err)

				return
			}
			absPath, err := cs.sanitizePath(path, storageID)
			if err != nil {
				_ = c.Error(err)

				return
			}
			fileInfo, err := os.Stat(absPath)
			if os.IsNotExist(err) {
				_ = c.Error(err)

				return
			}
			var totalSize = fileInfo.Size()
			var ip = fsutils.ReadUserIP(c.Request)
			if totalSize < 20*fsutils.MB && float64(resSz)/float64(totalSize) > 0.1 {
				queue.SubmitJob(queue.Job{PayLoad: billing.TransferUsage{
					UsageID:   uuid.New().String(),
					TenantID:  principal.TenantID,
					UserID:    principal.UserID,
					Name:      path,
					Size:      int64(resSz),
					Type:      billing.DOWNLOAD,
					From:      config.NFSID,
					To:        ip,
					StartTime: startTime,
					EndTime:   time.Now(),
					ClientIP:  ip,
				}, Action: s.GetBc().SendTransferUsage})

				return
			}
			s.GetDlCacheMgr().Record(download.Dl{
				PathKey:   fmt.Sprintf("%s/%s", principal.UserName, path),
				Path:      path,
				Size:      int64(resSz),
				From:      config.NFSID,
				To:        ip,
				TenantID:  principal.TenantID,
				UserID:    principal.UserID,
				ClientIP:  ip,
				StartTime: startTime,
			})
		}
	}
}

// ErrorHandleMiddleware general error handling middleware
// user can specific their own appError instead of the default 400 status code
func ErrorHandleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		detectedErrors := c.Errors.ByType(gin.ErrorTypeAny)

		if len(detectedErrors) > 0 {
			err := detectedErrors[0].Err
			logrus.Errorf(err.Error())

			var parsedError *appError

			switch {
			case errors.As(err, &parsedError):
			default:
				parsedError = &appError{
					Code:    http.StatusBadRequest,
					Message: err.Error(),
				}
			}
			c.AbortWithStatusJSON(parsedError.Code, parsedError)

			return
		}
	}
}

func fromCtx(c *gin.Context, key string, defaultVal interface{}) interface{} {
	v, ok := c.Get(key)
	if ok {
		return v
	}

	return defaultVal
}

func getMountUsage(mountPoint string) (uint64, string, error) {
	m, _, err := mounts.Mounts()
	if err != nil {
		return 0, "", err
	}
	for _, m := range m {
		if m.Mountpoint == mountPoint {
			return m.Used, m.Device, nil
		}
	}

	return 0, "", ErrNoSuchMount
}

func getCloudStorage(c *gin.Context) *cloudStorage {
	cs, exists := c.Get(internal.CloudStorageKey)
	if !exists {
		panic("not bind cloudStorage")
	}

	return cs.(*cloudStorage)
}

func (cs *cloudStorage) auditEvtTemplate() *pb.CreateEventRequest {
	return &pb.CreateEventRequest{
		Name:      pb.EventName_EVENT_NAME_STORAGE_FILE_PREVIEWD.String(),
		CreatedAt: timestamppb.New(time.Now()),
		Operator:  &pb.Operator{Type: pb.OperatorType_OPERATOR_TYPE_USER, Name: cs.userPrincipal.UserName},
		Device:    &pb.Device{Ip: fsutils.ReadUserIP(cs.gin.Request)},
	}
}

var (
	// ErrNoSuchMount means that the mount point does not exist
	ErrNoSuchMount = errors.New("no such mount")

	initFileList = []string{".bashrc", ".bash_profile", ".bash_logout"}
)
