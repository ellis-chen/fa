package cipher

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMD5Single(t *testing.T) {
	bs := bytes.Repeat([]byte{1}, (10 * 1024 * 1024))
	rd := bytes.NewReader(bs)
	md5, err := MD5Single(rd)
	require.NoError(t, err)
	expectMD5 := "76027b5c660dbba173c28588441749d3"
	assert.Equal(t, expectMD5, md5)
}

func TestGetFileMD5Hash(t *testing.T) {
	bs := bytes.Repeat([]byte{1}, (10 * 1024 * 1024))
	r := bytes.NewBuffer(bs)
	f, _ := ioutil.TempFile("", "md5test")
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	_, _ = io.CopyBuffer(f, r, make([]byte, 1024*1024))
	md5, err := GetFileMD5Hash(f.Name())
	require.NoError(t, err)
	assert.Equal(t, "76027b5c660dbba173c28588441749d3", md5)
}

func TestGetFileMD5HashByChunk(t *testing.T) {
	var totalSize int64 = 50 * 1024 * 1024
	bs := bytes.Repeat([]byte{1}, int(totalSize))
	r := bytes.NewBuffer(bs)
	f, _ := ioutil.TempFile("", "md5test")
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	_, _ = io.CopyBuffer(f, r, make([]byte, 1024*1024))
	_, _ = f.Seek(0, 0)
	md5, err := MD5ByChunk(f, totalSize, defaultChunkSize)
	require.NoError(t, err)
	assert.Equal(t, "d7f44d20b4c330619472741e75274e03-5", md5)
}

func TestGetFileMD5HashByChunkConcurrent(t *testing.T) {
	bs := bytes.Repeat([]byte{1}, (50 * 1024 * 1024))
	r := bytes.NewBuffer(bs)
	f, _ := ioutil.TempFile("", "md5test")
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()
	_, _ = io.CopyBuffer(f, r, make([]byte, 1024*1024))
	md5, err := GetFileMD5HashByChunk(f.Name(), defaultChunkSize)
	require.NoError(t, err)
	assert.Equal(t, "d7f44d20b4c330619472741e75274e03-5", md5)
}

func TestRange(t *testing.T) {

	var intChan = make(chan int)
	go func() {

		for in := range intChan {
			t.Logf("value of in: %v", in)
		}
		t.Log("iterate intChan done.")
		t.Log("exit iterate intChan")
	}()

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		j := i
		go func() {
			defer wg.Done()

			intChan <- j
		}()
	}
	wg.Wait()
	// close(intChan)
	t.Log("exit main")
}

func TestChanClose(t *testing.T) {
	jobs := make(chan int, 5)
	done := make(chan bool)

	// go func() {
	// 	for {
	// 		j, more := <-jobs
	// 		if more {
	// 			fmt.Println("received job", j)
	// 		} else {
	// 			fmt.Println("received all jobs")
	// 			done <- true
	// 			return
	// 		}
	// 	}
	// }()

	go func() {
		for job := range jobs {
			fmt.Println("received job", job)
		}
		fmt.Println("received all jobs")
		done <- true
	}()

	for j := 1; j <= 3; j++ {
		jobs <- j
		fmt.Println("sent job", j)
	}
	close(jobs)
	fmt.Println("sent all jobs")

	<-done
}
