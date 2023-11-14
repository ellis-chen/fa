package cipher

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"sort"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	_ = 1 << (iota * 10) // ignore the first value
	// KB ...
	KB // decimal:       1024 -> binary 00000000000000000000010000000000
	// MB ...
	MB // decimal:    1048576 -> binary 00000000000100000000000000000000
	// GB ...
	GB // decimal: 1073741824 -> binary 01000000000000000000000000000000

	//DefaultPartSize stands for the default part size when split to multipart upload
	DefaultPartSize = MB << 2
)

var (
	defaultChunkSize int64 = 10 * MB
)

// GetMD5Hash get the md5 hash of the given string
func GetMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

// GetFileMD5Hash get the md5 file hash
func GetFileMD5Hash(filePath string) (string, error) {
	var returnMD5String string
	file, err := os.Open(filePath)
	if err != nil {
		return returnMD5String, err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return returnMD5String, err
	}
	return MD5ByChunk(file, fileInfo.Size(), math.MaxInt64)

}

// GetFileMD5HashByChunk get the md5 of a large file by concatrate the chunk together
func GetFileMD5HashByChunk(filePath string, chunkSize int64) (string, error) {
	var returnMD5String string
	file, err := os.Open(filePath)
	if err != nil {
		return returnMD5String, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return returnMD5String, err
	}
	return MD5ByChunkConcurrent(file, fileInfo.Size(), chunkSize)
}

// MD5Single read fix size from reader and calc the md5
func MD5Single(reader io.Reader) (string, error) {
	var returnMD5String string

	hash := md5.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return returnMD5String, nil
	}
	hashInBytes := hash.Sum(nil)
	returnMD5String = hex.EncodeToString(hashInBytes)
	return returnMD5String, nil
}

// MD5ByChunk ....
func MD5ByChunk(reader io.Reader, totalSize int64, chunkSize int64) (string, error) {
	var returnMD5String string

	hash := md5.New()

	if chunkSize >= totalSize {
		if _, err := io.Copy(hash, reader); err != nil {
			return returnMD5String, err
		}
		hashInBytes := hash.Sum(nil)
		returnMD5String = hex.EncodeToString(hashInBytes)
		return returnMD5String, nil
	}

	var md5s []byte
	var i int64
	for i = 0; i*chunkSize < totalSize; i++ {
		hash.Reset()
		var readSize int64
		if (i+1)*chunkSize < totalSize {
			readSize = chunkSize
		} else {
			readSize = totalSize - i*chunkSize
		}
		limitReader := io.LimitReader(reader, readSize)
		_, err := io.Copy(hash, limitReader)
		if err != nil {
			return returnMD5String, err
		}
		md5sum := hash.Sum(nil)[:16]
		logrus.Debugf("part md5 from read is: %s", hex.EncodeToString(md5sum))
		md5s = append(md5s, md5sum...)
	}

	hash.Reset()
	hash.Write(md5s)
	return fmt.Sprintf("%s-%d", hex.EncodeToString(hash.Sum(nil)[:16]), i), nil
}

type md5Chunk struct {
	md5 []byte
	idx int64
}

type md5ChunkList []md5Chunk

func (a md5ChunkList) Len() int           { return len(a) }
func (a md5ChunkList) Less(i, j int) bool { return a[i].idx < a[j].idx }
func (a md5ChunkList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// MD5ByChunkConcurrent concurrent calculate the md5 of the large file based on the chunkSize
func MD5ByChunkConcurrent[T interface {
	io.Reader
	io.ReaderAt
}](reader T, totalSize int64, chunkSize int64) (r string, err error) {
	h := md5.New()

	if chunkSize >= totalSize {
		if _, err = io.Copy(h, reader); err != nil {
			return
		}
		hashInBytes := h.Sum(nil)
		r = hex.EncodeToString(hashInBytes)
		return
	}

	var (
		totalChunks  = (totalSize + chunkSize - 1) / chunkSize
		md5ChunkChan = make(chan md5Chunk)
		totalMD5Chan = make(chan string)
		i            int64
		g, gCtx      = errgroup.WithContext(context.TODO())
	)

	go func() {
		var md5ChunkList = make(md5ChunkList, totalChunks)

		var j = 0
		for b := range md5ChunkChan {
			md5ChunkList[j] = b
			j++
		}
		sort.Sort(md5ChunkList)
		for _, chunk := range md5ChunkList {
			logrus.Debugf("part md5 from read is: %s", hex.EncodeToString(chunk.md5))
			h.Write(chunk.md5)
		}
		totalMD5Chan <- hex.EncodeToString(h.Sum(nil))
	}()

	for ; i*chunkSize < totalSize; i++ {
		var readSize int64
		var off = i * chunkSize
		if (i+1)*chunkSize < totalSize {
			readSize = chunkSize
		} else {
			readSize = totalSize - i*chunkSize
		}
		if gCtx.Err() != nil {
			break
		}

		var j = i

		g.Go(func() error {
			hash := md5.New()

			r := io.NewSectionReader(reader, off, readSize)

			_, err := io.Copy(hash, r)
			if err != nil {
				return err
			}
			md5sum := hash.Sum(nil)[:16]

			md5ChunkChan <- md5Chunk{md5: md5sum, idx: j}
			return nil
		})

	}

	err = g.Wait()
	if err != nil {
		return
	}
	close(md5ChunkChan)

	totalMD5 := <-totalMD5Chan

	return fmt.Sprintf("%s-%d", totalMD5, totalChunks), nil
}

// MD5ReadWrite read from the src, and write to the dst. In the mean time, cal the md5, and return
// https://github.com/michalpristas/fan-out-writes/blob/io-copy-buf/main.go
// https://rodaine.com/2015/04/async-split-io-reader-in-golang/
func MD5ReadWrite(dst io.Writer, src io.Reader, buf []byte) (string, error) {
	h := md5.New()
	_, err := io.CopyBuffer(dst, io.TeeReader(src, h), buf)
	if err != nil {
		logrus.Warnf("Error copying data: %v", err)
		return "", err
	}

	md5 := h.Sum(nil)
	// logrus.Infof("Copied %v bytes with hash %x", n, md5)

	return hex.EncodeToString(md5), nil
}
