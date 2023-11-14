package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ellis-chen/fa/internal"
	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/pkg/iotools"
	"github.com/ellis-chen/fa/web/routers"

	"github.com/sirupsen/logrus"
)

// Serve ...
func Serve(ctx context.Context) {
	migrateDB(ctx) // try best effort to migrate the old db to new

	server := routers.NewServer(ctx)
	server.Start()

	config := server.GetConfig()
	port := config.Port

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: server,
	}

	logrus.Infof("Listening on port [%d]", port)

	err := iotools.EnsureDir(fmt.Sprintf("%s/users", config.StorePath))
	if err != nil {
		logrus.Errorf("err when ensure the users base directory, detail goes as: %s", err.Error())

		return
	}

	err = iotools.EnsureDir(fmt.Sprintf("%s/softwares", config.StorePath))
	if err != nil {
		logrus.Errorf("err when ensure the tenant level software base directory, detail goes as: %s", err.Error())

		return
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			logger.WithoutContext().Warnf("listen: %s\n", err)
		}
	}()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGUSR1)

		for {
			select {
			case <-c:
				err := server.OnRefresh(context.TODO())
				if err != nil {
					logrus.Errorf("err refreshing fa: %v", err)
				}
			case <-server.GetRootCtx().Done():
				logrus.Warning("Exiting the server.OnRefresh goroutine!")

				return
			}
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.WithoutContext().Infof("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	isShutdown := make(chan struct{}, 1)

	go func(ctx context.Context) {
		server.Stop(ctx)

		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal("Server forced to shutdown:", err)
		}
		isShutdown <- struct{}{}
	}(ctx)

	select {
	case <-ctx.Done():
		logger.WithoutContext().Errorf("Graceful shutdown timeout of %v seconds exceeded, terminating the server forcefully", 5)
	case <-isShutdown:
		logger.WithoutContext().Infof("Server exited gracefully")
	}

	_ = logger.Rotate()
}

// https://github.com:8443/ellis-chen/cce-project/-/issues/4076
func migrateDB(ctx context.Context) {
	cfg := internal.LoadConf()
	oldDbname := fmt.Sprintf("%s/sully.db", cfg.StorePath)
	newDbname := fmt.Sprintf("%s/%s", cfg.StorePath, cfg.Dbname)

	if _, err := os.Lstat(oldDbname); err == nil {
		_ = os.Rename(oldDbname, newDbname)
	}
}
