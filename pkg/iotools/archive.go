package iotools

import (
	"compress/flate"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

// StreamZip open the given path (may be a file or directory), zip it, and write to the given stream
func StreamZip(paths []string, writer io.Writer) (err error) {
	filenames := make([]string, 0)
	for _, path := range paths {
		path = filepath.Clean(path)
		_filenames, _ := tree(path)
		filenames = append(filenames, _filenames...)
	}

	if len(filenames) == 0 {
		return errors.New("not found any valid file")
	}

	prefix := FileCommonPrefix(os.PathSeparator, filenames...)

	if len(paths) == 1 { // only one directory or file, we should keep the directory name
		prefix = path.Dir(paths[0])
	} // if we have more than one request path, we use the common prefix

	var arc = archiver.Zip{
		CompressionLevel:       flate.DefaultCompression,
		MkdirAll:               true,
		SelectiveCompression:   true,
		ContinueOnError:        true,
		OverwriteExisting:      false,
		ImplicitTopLevelFolder: false,
	}

	err = arc.Create(writer)
	if err != nil {
		return err
	}
	defer func(arc *archiver.Zip) {
		err := arc.Close()
		if err != nil {
			logrus.Warnf("err close arc for path: %s", prefix)
		}
	}(&arc)

	for _, path := range filenames {
		info, err := AppFs.Stat(path)
		if err != nil {
			logrus.Debugf("ignore stat error: %s", err.Error())
			continue
		}
		if info.Mode()&fs.ModeSymlink != 0 || info.Mode()&fs.ModeIrregular != 0 { // ignore symbolic link
			continue
		}

		if strings.HasPrefix(info.Name(), ".") && strings.HasSuffix(info.Name(), ".upload") { // ignore uploading file as they are incomplete
			continue
		}

		fname := filepath.Clean(path)

		file, _ := AppFs.Open(path)

		internalName := strings.TrimPrefix(fname, prefix)
		internalName = strings.TrimPrefix(internalName, "/")

		logrus.Debugf("adding file [%v] to archive ...", internalName)

		err = arc.Write(archiver.File{
			FileInfo: archiver.FileInfo{
				FileInfo:   info,
				CustomName: internalName,
			},
			ReadCloser: file,
		})
		if err != nil {
			return err
		}
		err = file.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// tree returns all the file inside a given path
func tree(base string) ([]string, error) {
	filenames := make([]string, 0)
	fi, err := AppFs.Stat(base)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		filenames = append(filenames, base)

		return filenames, nil
	}
	err = afero.Walk(AppFs, base, func(p string, info fs.FileInfo, err error) error {
		if err == nil {
			filenames = append(filenames, p)
		}

		return err
	})

	return filenames, err
}
