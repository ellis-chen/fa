package iotools

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

type readerFunc func(p []byte) (n int, err error)

// SleepLimitedReader reads from R but limits the amount of
// data returned to just N bytes. Each call to Read
// updates N to reflect the new amount remaining.
// Read returns EOF when N <= 0 or when the underlying R returns EOF.
type SleepLimitedReader struct {
	R     io.Reader // underlying reader
	N     int64     // max bytes remaining
	U     int64
	Sleep time.Duration
}

func (rf readerFunc) Read(p []byte) (n int, err error) { return rf(p) }

// Copy wrap the io.copy() to provide a context aware copy
func Copy(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	n, err := io.Copy(dst, readerFunc(func(p []byte) (int, error) {
		deadline, ok := ctx.Deadline()
		if ok && time.Since(deadline) > 0 {
			return 0, context.DeadlineExceeded
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			return src.Read(p)
		}
	}))

	return n, err
}

// Copy2DirWithContext copy a regular file or directory to the given directory
func Copy2DirWithContext(ctx context.Context, src, dst string, excludeDir bool) error {
	fSrc, err := os.Stat(src)
	if err != nil {
		return err
	}
	fDst, err := os.Stat(dst)
	if err != nil {
		return err
	}
	if !fDst.Mode().IsDir() {
		return fmt.Errorf("%s is not a regular directory", dst)
	}

	if fSrc.Mode().IsDir() {
		if excludeDir {
			return CopyDirWithContext(ctx, src, dst)
		}
		_, srcLastDir := path.Split(src)
		dst = filepath.Join(dst, srcLastDir)
		err := EnsureDir(dst)
		if err != nil {
			return err
		}

		return CopyDirWithContext(ctx, src, dst)
	}

	dstFileName := fmt.Sprintf("%s/%s", dst, fSrc.Name())

	return CopyFileWithContext(ctx, src, dstFileName)
}

// CopyDirWithContext recursively copies a src directory's content to a destination directory.
func CopyDirWithContext(ctx context.Context, src, dst string) error {
	logrus.Debugf("copy directory from [ %s ] to [ %s ]", src, dst)
	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dst, entry.Name())

		fSrc, err := os.Lstat(srcPath)
		if err != nil {
			return err
		}

		switch fSrc.Mode() & os.ModeType { // nolint:exhaustive
		case os.ModeDir:
			if err := ReplicateDir(destPath, 0755, fSrc.ModTime()); err != nil {
				return err
			}
			if err := CopyDirWithContext(ctx, srcPath, destPath); err != nil {
				return err
			}
		case os.ModeSymlink:
			if err := CopySymLinkWithContext(ctx, srcPath, destPath); err != nil {
				return err
			}
		default:
			if err := CopyFileWithContext(ctx, srcPath, destPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// CopyFileWithContext copies a src file to a dst file where src and dst are regular files.
func CopyFileWithContext(ctx context.Context, src, dst string) (err error) {
	fSrc, err := os.Stat(src)
	if err != nil {
		return
	}

	if !fSrc.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return
	}
	defer func(srcFile *os.File) {
		err := srcFile.Close()
		if err != nil {
			logrus.Warnf("err close file: %s", srcFile.Name())
		}
	}(srcFile)

	dstFile, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func(dstFile *os.File) {
		err := dstFile.Close()
		if err != nil {
			logrus.Warnf("err close file: %s", dstFile.Name())
		}
	}(dstFile)
	_, err = Copy(ctx, dstFile, srcFile)
	if err != nil {
		return
	}
	err = os.Chtimes(dst, fSrc.ModTime(), fSrc.ModTime())
	if err != nil {
		return
	}

	if err = os.Chmod(dst, fSrc.Mode()); err != nil {
		return
	}

	return nil
}

// CopySymLinkWithContext copies a symbolic link from src to dst.
func CopySymLinkWithContext(ctx context.Context, src, dst string) error {
	link, err := os.Readlink(src)
	if err != nil {
		return nil
	}
	if _, err := os.Lstat(dst); err == nil {
		if err := os.RemoveAll(dst); err != nil {
			return fmt.Errorf("failed to unlink: %+w", err)
		}
	}

	return os.Symlink(link, dst)
}

// SleepLimitReader returns a Reader that reads from r
// but stops with EOF after n bytes.
// The underlying implementation is a *SleepLimitedReader.
func SleepLimitReader(r io.Reader, n int64, u int64, sleep time.Duration) io.Reader {
	return &SleepLimitedReader{r, n, u, sleep}
}

func (l *SleepLimitedReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, io.EOF
	}

	deadline := time.Now().Add(l.Sleep)
	if int64(len(p)) > l.U {
		p = p[0:l.U]
	}
	n, err = l.R.Read(p)
	l.N -= int64(n)

	logrus.Infof("copied %d bytes", n)
	remaining := time.Until(deadline)
	if remaining > 0 {
		time.Sleep(remaining)
	}

	return
}
