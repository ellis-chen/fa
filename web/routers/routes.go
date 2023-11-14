package routers

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ellis-chen/fa/internal"
	"github.com/ellis-chen/fa/pkg/prom"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"

	"github.com/gin-gonic/gin"
)

type Router struct {
	*gin.Engine
	Config *internal.Config
	Server *Server

	mu sync.Mutex
}

func NewRouter(config *internal.Config) *Router {
	return &Router{
		gin.Default(),
		config,
		&Server{},
		sync.Mutex{},
	}
}

func (r *Router) SetupServer(s *Server) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Server = s
}

func (r *Router) Setup() {
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  false,
		AllowMethods:     []string{"PUT", "PATCH", "POST", "DELETE"},
		AllowHeaders:     []string{"Content-Type", "Accept", "Authorization", "X-Content-MD5", "X-Last-Modified", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
		AllowOriginFunc: func(origin string) bool {
			return true
		},
	}))

	r.Engine.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "UP")
	})

	r.registerAuthorizedHandlers()
	r.registerAnonHandlers()
}

func (r *Router) registerAuthorizedHandlers() {
	authorized := r.Group("/fa/api/v0")
	authorized.Use(ErrorHandleMiddleware())
	if r.Config.ActivateJWT {
		authorized.Use(JwtMiddleware(r.Config))
	}

	authorized.Use(ErrorHandleMiddleware()) // we want the error handler to be last processed
	authorized.Use(FsContextMiddleware(r.Server))
	authorized.Use(DownloadMiddleware(r.Server))

	if !r.Config.FcceMode {
		authorized.Use(BackupMiddleware(r.Server))
		authorized.Use(BackupFullMiddleware(r.Server))
	}

	registerUploadHandlers(authorized)
	registerDownloadHandlers(authorized)
	registerFileHandlers(authorized)
	registerRcloneHandlers(authorized)
	registerAdminHandlers(authorized)
	registerInternalHandlers(authorized)
	if !r.Config.FcceMode {
		registerBackupHandlers(authorized)
	}
}

func (r *Router) registerAnonHandlers() {
	rg := r.Group("/fa")

	p := prom.NewPrometheus("gin")
	p.ReqCntURLLabelMappingFn = downloadReqCntFunc
	p.Use(r.Engine)

	if r.Config.Debug {
		pprof.Register(r.Engine, "/fa/debug/pprof")
	}

	publicGroup := rg.Group("/public")
	{
		publicGroup.Use(ErrorHandleMiddleware())
		publicGroup.GET("/safe-upgrade", BackupMiddleware(r.Server), isSafeMaintain)
		publicGroup.GET("/full-restore", BackupFullMiddleware(r.Server), BackupFullCheckMiddleware(), FsContextMiddleware(r.Server), fullRestore)
		publicGroup.GET("/full-query", BackupFullMiddleware(r.Server), BackupFullCheckMiddleware(), FsContextMiddleware(r.Server), fullQuery)
		publicGroup.PUT("/full-ack/:id", BackupFullMiddleware(r.Server), BackupFullCheckMiddleware(), FsContextMiddleware(r.Server), fullAck)
	}
}

func downloadReqCntFunc(c *gin.Context) string {
	url := c.Request.URL.Path
	for _, p := range c.Params {
		if p.Key == "path" && c.Request.Method == "GET" && strings.Contains(c.Request.RequestURI, "download") {
			url = strings.Replace(url, p.Value, ":path", 1)

			break
		}
		if p.Key == "token" && c.Request.Method == "PUT" && strings.Contains(c.Request.RequestURI, "upload/upload_part") {
			// strings.SplitAfter(sampler string, sep string)
			idx := strings.LastIndex(url, "upload/upload_part")
			url = url[:idx+len("upload/upload_part")]

			break
		}
	}

	return url
}

func registerUploadHandlers(rg *gin.RouterGroup) {
	uploadGroup := rg.Group("/upload")
	{
		uploadGroup.POST("/", uploadSmall)
		uploadGroup.POST("/init_part", createMulitpartUpload)
		uploadGroup.PUT("/upload_part/:token/:index", uploadPart)
		uploadGroup.POST("/complete_part", completeMultipartUpload)
	}
}

func registerDownloadHandlers(rg *gin.RouterGroup) {
	downloadGroup := rg.Group("/download")

	downloadGroup.GET("/*path", downloadFiles)
}

func registerFileHandlers(rg *gin.RouterGroup) {
	fileGroup := rg.Group("/file")

	fileGroup.POST("/list", listFile)
	fileGroup.POST("/new", newDir)
	fileGroup.POST("/copy", copyFile)
	fileGroup.POST("/movedir", moveDir)
	fileGroup.POST("/rename", renameFile)
	fileGroup.POST("/delete", deleteFile)
	fileGroup.POST("/batch-delete", batchDelFile)
	fileGroup.POST("/batch-copy", batchCopyFile)
	fileGroup.GET("/preview/*path", previewFile)
	fileGroup.GET("/tail/*path", tailFile)
	fileGroup.GET("/progress", copyLineProcess)
	fileGroup.GET("/used", used)
	fileGroup.HEAD("/:path", infoFile)
}

func registerRcloneHandlers(rg *gin.RouterGroup) {
	fileGroup := rg.Group("/rclone")

	fileGroup.GET("/*path", rListOrDownload)
	fileGroup.HEAD("/*path", rInfoFile)
}

func registerAdminHandlers(rg *gin.RouterGroup) {
	adminGroup := rg.Group("/admin")
	adminGroup.GET("/cleanup", cleanUploadStatus)
}

func registerInternalHandlers(rg *gin.RouterGroup) {
	internalGroup := rg.Group("/internal")
	{
		internalGroup.POST("/inject-id", injectPubKey)
		internalGroup.POST("/activate-app", activateTrialApp)
		internalGroup.POST("/deactivate-app", deactivateTrialApp)
		internalGroup.GET("/trial-progress", trialProgress)
	}
}

func registerBackupHandlers(rg *gin.RouterGroup) {
	backupGroup := rg.Group("/backup")
	{
		backupGroup.POST("/new", newBackup)
		backupGroup.POST("/cancel", cancelBackup)
		backupGroup.POST("/cancel-all", cancelAllBackup)
		backupGroup.POST("/destroy-repo", destroyBackupRepo)
		backupGroup.GET("/repo-destroyed", isBackupProviderDestroyed)
		backupGroup.GET("/:id", getBackup)
		backupGroup.DELETE("/:id", deleteBackup)
	}
	restoreGroup := rg.Group("/restore")
	{
		restoreGroup.POST("/new", newRestore)
		restoreGroup.GET("/:id", getRestore)
		restoreGroup.POST("/cancel", cancelRestore)
		restoreGroup.POST("/cancel-all", cancelAllRestores)
	}
}
