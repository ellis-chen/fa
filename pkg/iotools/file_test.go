package iotools

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadDir(t *testing.T) {
	f, e := os.Open("../../")
	if e != nil {
		panic(e)
	}
	files, _ := f.ReadDir(-1)
	for _, file := range files {
		log.Println(file.Name())
	}
}

func TestCreateZip(t *testing.T) {
	name, err := CreateZip("../")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("zip file is : %v", name)
}

func TestRel(t *testing.T) {
	path1 := "../hello"
	path2 := "../hello/world/again"
	t.Log(filepath.Rel(path1, path2))
}

func TestFilePathClean(t *testing.T) {
	path1 := "hello//world"
	path2 := "hello/world"
	assert.Equal(t, path2, filepath.Clean(path1))
}

func TestFilePathClean2(t *testing.T) {
	path1 := "hello///world"
	path2 := "hello/world"
	assert.Equal(t, path2, filepath.Clean(path1))
}

func TestRelPath(t *testing.T) {
	path1 := "/users/test/hello/hello.txt"
	prefix := "/users/test/"
	rel, err := filepath.Rel(prefix, path1)
	assert.NoError(t, err)
	assert.Equal(t, "hello/hello.txt", rel)
}

func TestSubElem(t *testing.T) {
	path1 := "/users/test/hello/hello.txt"
	prefix := "/users/test/"
	rel, err := SubElem(prefix, path1)
	assert.NoError(t, err)
	assert.True(t, rel, "not sub elem")

	prefix = "/users/test/aaa"
	rel, err = SubElem(prefix, path1)
	assert.NoError(t, err)
	assert.False(t, rel, "not sub elem")
}

func TestDirList(t *testing.T) {
	path := "/users/test/hello/hello.txt"
	prefix := "/users/test"

	list := subPathList(path, prefix)
	assert.Equal(t, 2, len(list))
}

func TestFileList(t *testing.T) {
	path := "/users/test/hello.txt"
	prefix := "/users/test"

	list := subPathList(path, prefix)
	assert.Equal(t, 1, len(list))
}

func TestCommonPrefix(t *testing.T) {
	c := FileCommonPrefix(os.PathSeparator,
		//"/home/user1/tmp",
		"/home/user1/tmp/coverage/test",
		"/home/user1/tmp/covert/operator",
		"/home/user1/tmp/coven/members",
		"/home//user1/tmp/coventry",
		"/home/user1//",
		"/home/user1/././tmp/covertly/foo",
		"/home/bob/../user1/tmp/coved/bar",
	)
	require.Equal(t, c, "/home/user1")
}
