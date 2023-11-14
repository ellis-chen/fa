package iotools

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ellis-chen/fa/pkg/fsutils"
	"github.com/ellis-chen/fa/pkg/pool"

	"github.com/gabriel-vasile/mimetype"
	"github.com/sirupsen/logrus"
)

// Copy2Dir copy a regular file or directory to the given directory
func Copy2Dir(src, dst string, excludeDir bool) error {
	return Copy2DirWithContext(context.TODO(), src, dst, excludeDir)
}

// Move2Dir move the given file or directory to the given directory
func Move2Dir(src, dst string, excludeDir bool) (err error) {
	fSrc, err := os.Stat(src)
	if err != nil {
		return err
	}
	dstStat, err := os.Stat(dst)
	if err != nil {
		return err
	}
	if !dstStat.Mode().IsDir() {
		return fmt.Errorf("%s is not a regular directory", dst)
	}
	if excludeDir && fSrc.IsDir() {
		err = os.Rename(src, dst)

		return
	}
	_, srcLastDir := path.Split(src)
	dst = filepath.Join(dst, srcLastDir)

	err = os.Rename(src, dst)

	return
}

// Move2File move the given file to the given file
func Move2File(src, dst string) (err error) {
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if fi.Mode().IsDir() {
		return fmt.Errorf("can't move a directory [ %s ] to a file", src)
	}

	err = os.Rename(src, dst)

	return
}

// CopyDir recursively copies a src directory to a destination.
func CopyDir(src, dst string) error {
	return CopyDirWithContext(context.TODO(), src, dst)
}

// CopyFile copies a src file to a dst file where src and dst are regular files.
func CopyFile(src, dst string) error {
	return CopyFileWithContext(context.TODO(), src, dst)
}

// CopyFileChown copies a src file to a dst file where src and dst are regular files. After successfuly copy, change with uid and gid
func CopyFileChown(src, dst string, uid, gid int) (err error) {
	err = CopyFileWithContext(context.TODO(), src, dst)
	if err != nil {
		return err
	}

	return ChownR(dst, uid, gid)
}

func exists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}

	return true
}

// CreateDir create the directory with the given perm
func CreateDir(dir string, perm os.FileMode) error {
	if exists(dir) {
		return nil
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%w'", dir, err)
	}

	return nil
}

// ReplicateDir create the directory with the given perm and modTime
func ReplicateDir(dir string, perm os.FileMode, modTime time.Time) (err error) {
	if err = os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%w'", dir, err)
	}

	err = os.Chtimes(dir, modTime, modTime)
	if err != nil {
		return err
	}

	return nil
}

// CopySymLink copies a symbolic link from src to dst.
func CopySymLink(src, dst string) error {
	return CopySymLinkWithContext(context.TODO(), src, dst)
}

// Create creates the named file with mode 0666 (before umask), truncating
// it if it already exists. If successful, methods on the returned
// File can be used for I/O; the associated file descriptor has mode
// O_RDWR.
// If there is an error, it will be of type *PathError.
func Create(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0666)
}

// RemoveFileR will walk all children of the given directory, and if it find a file, delete it,
// if it find a directory, ignore it
func RemoveFileR(name string) (err error) {
	fSrc, err := os.Lstat(name)
	if err != nil {
		return nil
	}
	if !fSrc.IsDir() {
		err = os.RemoveAll(name)

		return
	}

	return filepath.Walk(name, func(name string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			err = os.Remove(name)
		}

		return err
	})
}

// RemoveAll remove all file and it's children
func RemoveAll(name string) (err error) {
	fi, err := os.Lstat(name)
	if err != nil {
		return
	}
	if !fi.IsDir() {
		return os.Remove(name)
	}
	dir, err := ioutil.ReadDir(name)
	if err != nil {
		return
	}
	for _, d := range dir {
		err = os.RemoveAll(path.Join([]string{name, d.Name()}...))
		if err != nil {
			return err
		}
	}

	return os.Remove(name)
}

// Remove try remove a directory or file
func Remove(name string) (err error) {
	_, err = os.Lstat(name)
	if err != nil {
		return nil
	}

	return os.RemoveAll(name)
}

// CreateOwn create the given file with the given uid and gid
func CreateOwn(name string, uid, gid int) (f *os.File, err error) {
	f, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return
	}
	if err = f.Chown(uid, gid); err != nil {
		return
	}

	return
}

// EnsureFile ensure the file exists
func EnsureFile(filename string) error {
	dir, _ := path.Split(filename)
	err := EnsureDir(dir)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if _, err = Create(filename); err != nil {
			return err
		}
	}

	return nil
}

// EnsureOwnFile make sure the directory exists. if not, create it, and chown it with given uid and gid
func EnsureOwnFile(filename string, uid, gid int) error {
	dir, _ := path.Split(filename)
	err := EnsureOwnDir(dir, uid, gid)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if _, err = CreateOwn(filename, uid, gid); err != nil {
			return err
		}
	}

	return nil
}

// EnsureOwnFileR make sure the given file exists no matter how deep it is
// it also change the owner of all the directory with uid and gid that traverse from the parent
func EnsureOwnFileR(filename string, root string, uid, gid int) error {
	dir, _ := path.Split(filename)
	err := EnsureOwnDirR(dir, root, uid, gid)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if _, err = CreateOwn(filename, uid, gid); err != nil {
			return err
		}
	}

	return nil
}

// EnsureDir make sure the directory exists. if not, create it
func EnsureDir(name string) error {
	err := os.MkdirAll(name, os.ModePerm)

	if err == nil || os.IsExist(err) {
		return nil
	}

	return err
}

// EnsureOwnDir ...
func EnsureOwnDir(name string, uid, gid int) (err error) {
	return EnsureOwnDirR(name, name, uid, gid)
}

// EnsureOwnDirR make sure the given directory exists no matter how deep it is
// it also change the owner of all the directory with uid and gid that traverse from the parent
func EnsureOwnDirR(dir, parent string, uid, gid int) (err error) {
	dir = filepath.Clean(dir)
	parent = filepath.Clean(parent)

	err = os.MkdirAll(dir, os.ModePerm)

	if err != nil {
		return fmt.Errorf("creating directory [ %s ] failed for %w", dir, err)
	}

	if dir == parent { // parent and dir are same directory, need chown the parent too
		err = os.Chown(parent, uid, gid)
		if err != nil {
			return
		}
	}

	parentValid, err := SubElem(parent, dir)
	if err != nil {
		return err
	}
	if !parentValid {
		return fmt.Errorf("invalid parent path [ %s ], children path [ %s ]", parent, dir)
	}

	if err = ChownDirR(parent, dir, uid, gid); err != nil {
		return
	}

	return
}

// ChownDirR chown the children and its descendent's directory owner with given uid and gid
func ChownDirR(parent, dir string, uid, gid int) (err error) {
	rel, _ := filepath.Rel(parent, dir)
	pathSegs := strings.Split(rel, "/")
	seg := ""
	for _, p := range pathSegs {
		seg = seg + "/" + p
		err = os.Chown(parent+seg, uid, gid)
		if err != nil {
			return
		}
	}

	return
}

// ChownR chown the children and its descendent's file or directory owner with given uid and gid
func ChownR(path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err == nil {
			err = os.Lchown(name, uid, gid)
		}

		return err
	})
}

// Chown change the ownership of the file to given uid and gid
// or the diretory based on the recursive stragety
func Chown(src string, uid int, gid int) error {
	var err error
	_, err = os.Stat(src)
	if err != nil {
		return err
	}
	if err = os.Chown(src, uid, gid); err != nil {
		return err
	}

	return err
}

// ChownTimes chown the src with the given uid and gid, it also change the access time and modTime of the src file
func ChownTimes(src string, uid int, gid int, time time.Time) (err error) {
	_, err = os.Stat(src)
	if err != nil {
		return
	}
	if err = os.Chown(src, uid, gid); err != nil {
		return
	}
	err = os.Chtimes(src, time, time)
	if err != nil {
		return
	}

	return
}

// Chtimes change the given file with the given modTime
func Chtimes(src string, time time.Time) (err error) {
	_, err = os.Stat(src)
	if err != nil {
		return
	}

	err = os.Chtimes(src, time, time)
	if err != nil {
		return
	}

	return
}

// ChownAfter chown after the given prefix, util the end of the last
func ChownAfter(src string, prefix string, uid int, gid int) error {
	var err error
	_, err = os.Stat(src)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(src, prefix) {
		return nil
	}

	subDirs := subPathList(src, prefix)
	for _, dir := range subDirs {
		if err = os.Chown(dir, uid, gid); err != nil {
			return err
		}
	}

	return err
}

// SubElem check if the `sub` dir is the children of `parent` dir
func SubElem(parent, sub string) (bool, error) {
	up := ".." + string(os.PathSeparator)

	// path-comparisons using filepath.Abs don't work reliably according to docs (no unique representation).
	rel, err := filepath.Rel(parent, sub)
	if err != nil {
		return false, err
	}
	if !strings.HasPrefix(rel, up) && rel != ".." {
		return true, nil
	}

	return false, nil
}

func subPathList(path string, prefix string) []string {
	remain, _ := filepath.Rel(prefix, path)
	list := strings.Split(remain, string(os.PathSeparator))

	var res []string
	for i := 0; i < len(list); i++ {
		curPath := filepath.Join(prefix, list[i])
		res = append(res, curPath)
		prefix = curPath
	}

	return res
}

// ChownRecursive ...
func ChownRecursive(src string, uid int, gid int) error {
	var err error
	srcStat, err := os.Stat(src)
	if err != nil {
		return err
	}
	err = os.Chown(src, uid, gid)
	if !srcStat.IsDir() {
		return err
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		subfile := filepath.Join(src, entry.Name())
		if err = ChownRecursive(subfile, uid, gid); err != nil {
			return err
		}
	}

	return nil
}

// FileCommonPrefix return the longest directory level among a group of directories
func FileCommonPrefix(sep byte, paths ...string) string {
	// Handle special cases.
	switch len(paths) {
	case 0:
		return ""
	case 1:
		return path.Clean(paths[0])
	}

	// Note, we treat string as []byte, not []rune as is often
	// done in Go. (And sep as byte, not rune). This is because
	// most/all supported OS' treat paths as string of non-zero
	// bytes. A filename may be displayed as a sequence of Unicode
	// runes (typically encoded as UTF-8) but paths are
	// not required to be valid UTF-8 or in any normalized form
	// (e.g. "é" (U+00C9) and "é" (U+0065,U+0301) are different
	// file names.
	c := []byte(path.Clean(paths[0]))

	// We add a trailing sep to handle the case where the
	// common prefix directory is included in the path list
	// (e.g. /home/user1, /home/user1/foo, /home/user1/bar).
	// path.Clean will have cleaned off trailing / separators with
	// the exception of the root directory, "/" (in which case we
	// make it "//", but this will get fixed up to "/" bellow).
	c = append(c, sep)

	// Ignore the first path since it's already in c
	for _, v := range paths[1:] {
		// Clean up each path before testing it
		v = path.Clean(v) + string(sep)

		// Find the first non-common byte and truncate c
		if len(v) < len(c) {
			c = c[:len(v)]
		}
		for i := 0; i < len(c); i++ {
			if v[i] != c[i] {
				c = c[:i]

				break
			}
		}
	}

	// Remove trailing non-separator characters and the final separator
	for i := len(c) - 1; i >= 0; i-- {
		if c[i] == sep {
			c = c[:i]

			break
		}
	}

	return string(c)
}

// CreateZip create a zip file based on the file or the directory
func CreateZip(path string) (string, error) {
	archive, err := ioutil.TempFile("", "archive.zip")
	if err != nil {
		panic(err)
	}
	defer func(archive *os.File) {
		_ = archive.Close()
	}(archive)
	zipWriter := zip.NewWriter(archive)
	defer func(zipWriter *zip.Writer) {
		err := zipWriter.Close()
		if err != nil {
			logrus.Warnf("err close zipWriter for path: %s", path)
		}
	}(zipWriter)

	err = append2Zip(zipWriter, path, path)

	return archive.Name(), err
}

func append2Zip(writer *zip.Writer, root string, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			logrus.Warnf("err close file: %s", f.Name())
		}
	}(f)
	stat, _ := f.Stat()

	if stat.Mode()&fs.ModeSymlink != 0 || stat.Mode()&fs.ModeIrregular != 0 { // ignore symbolic link
		return nil
	}

	if !stat.IsDir() {
		filename := strings.TrimPrefix(path, root)

		if filename == "" {
			filename = f.Name()
		}
		filename = strings.TrimPrefix(filename, "/")

		logrus.Debugf("adding file [%v] to archive ...", filename)
		w, err := writer.Create(filename)
		if err != nil {
			return err
		}
		BufCopy(w, f)

		return nil
	}

	entries, err := f.Readdir(-1)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		err := append2Zip(writer, root, fmt.Sprintf("%s/%s", f.Name(), entry.Name()))
		if err != nil {
			return err
		}
	}

	return nil
}

// ReadMime detect the type of a given file
func ReadMime(fname string) (mimeType string, err error) {
	mime, err := mimetype.DetectFile(fname)

	return mime.String(), err
}

// RecycleMimeReader returns the MIME type of input and a new reader
// containing the whole data from input.
func RecycleMimeReader(input io.Reader) (mimeType string, recycled io.Reader, err error) {
	// header will store the bytes mimetype uses for detection.
	header := bytes.NewBuffer(nil)

	// After DetectReader, the data read from input is copied into header.
	mtype, err := mimetype.DetectReader(io.TeeReader(input, header))

	// Concatenate back the header to the rest of the file.
	// recycled now contains the complete, original data.
	recycled = io.MultiReader(header, input)

	return mtype.String(), recycled, err
}

// TailFile tail of the given file at given offset and ifle
func TailFile(fname string, pagable *PagableRead) ([]byte, int64, error) {
	file, err := os.Open(fname)
	if err != nil {
		panic(err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			logrus.Warnf("err close file: %s", file.Name())

			return
		}
	}(file)

	fi, err := file.Stat()
	if err != nil {
		logrus.Error(err)
	}

	fileSize := fi.Size()
	offset := pagable.Offset
	size := pagable.Size

	if offset < 0 {
		if size < fileSize {
			offset = fileSize - size
		} else {
			offset = 0
		}
	}

	if offset > fileSize {
		return nil, -1, fmt.Errorf("invalid offset {%v}, file size {%v}", pagable.Offset, fileSize)
	}
	if (offset + size) > fileSize {
		size = fileSize - offset
	}

	defaultTailSize := 10 * fsutils.MB
	if size > int64(defaultTailSize) {
		return nil, -1, fmt.Errorf("tail size too large (%v), max size (%v), try give a lower one", size, defaultTailSize)
	}

	buf := make([]byte, size)

	n, err := file.ReadAt(buf, offset)
	if err != nil {
		logrus.Errorf("error when read: %v", err)
	}
	buf = buf[:n]

	return buf, offset + int64(n), nil
}

// DiskUsage ...
func DiskUsage(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}

		return err
	})

	return size, err
}

// BufCopy ...
func BufCopy(dst io.Writer, src io.Reader) {
	buffer := MyBufAllocator.GetBuffer()
	defer MyBufAllocator.PutBuffer(buffer)
	_, _ = io.CopyBuffer(dst, src, buffer.buf)
}

func (buf *buffer) GetBuf() []byte {
	return buf.buf
}

// PutBuffer return the buffer to the pool (clearing it)
func (a *bufAllocator) PutBuffer(b *buffer) {
	bufferPool.Put(b.buf)
	b.buf = nil
}

// GetBuffer get a buffer from the pool
func (a *bufAllocator) GetBuffer() *buffer {
	bufferPoolOnce.Do(func() {
		// Initialise the buffer pool when used
		bufferPool = pool.New(bufferCacheFlushTime, BufferSize, bufferCacheSize, false)
	})

	return &buffer{
		buf: bufferPool.Get(),
	}
}

type (
	buffer struct {
		buf []byte
	}
	bufAllocator struct{}
	// PagableRead ...
	PagableRead struct {
		Offset int64
		Size   int64
	}
)

const (
	// BufferSize is the default size of the async buffer
	BufferSize           = 1024 * 1024
	bufferCacheSize      = 64              // max number of buffers to keep in cache
	bufferCacheFlushTime = 5 * time.Second // flush the cached buffers after this long
)

var (
	ErrFileExists = errors.New("file or directory exists")

	// bufferPool is a global pool of buffers
	bufferPool     *pool.Pool
	bufferPoolOnce sync.Once

	// MyBufAllocator ...
	MyBufAllocator = new(bufAllocator)
)
