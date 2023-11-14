package iotools

import (
	"os"
	"testing"

	"github.com/spf13/afero"
)

func TestStreamZip(t *testing.T) {
	appFS := afero.NewMemMapFs()
	// create test files and directories
	_ = appFS.MkdirAll("src/a", 0755)
	_ = afero.WriteFile(appFS, "src/a/b", []byte("file a b"), 0644)
	_ = appFS.MkdirAll("src/c", 0755)
	_ = afero.WriteFile(appFS, "src/c/a", []byte("file c a"), 0644)
	_ = afero.WriteFile(appFS, "src/c/b", []byte("file c b"), 0644)
	_ = afero.WriteFile(appFS, "src/c/c", []byte("file c c"), 0644)
	_ = appFS.MkdirAll("src/d/a", 0755)
	_ = afero.WriteFile(appFS, "src/d/a/a", []byte("file d a a"), 0644)
	_ = afero.WriteFile(appFS, "src/d/a/b", []byte("file d a a"), 0644)

	name := "src/c"
	_, err := appFS.Stat(name)
	if os.IsNotExist(err) {
		t.Errorf("file \"%s\" does not exist.\n", name)
	}

	AppFs = appFS

	_ = StreamZip([]string{"src/c", "src/a", "src/b", "src/d/a"}, os.Stdout)
}
