package testing

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

type TempStorage struct {
	path string
	fileCleanup []string
}

func NewTempStorage(name string) TempStorage {
	path, err := ioutil.TempDir("", "metric-store")
	if err != nil {
		panic(err)
	}

	return TempStorage{
		path: path,
	}
}

func (s TempStorage) Cleanup() {
	os.RemoveAll(s.path)
}

func (s TempStorage) Path() string {
	return s.path
}

func (s TempStorage) Directories() []string {
	directories, err := ioutil.ReadDir(s.path)
	if err != nil {
		panic(err)
	}

	directoriesNames := []string{}
	for _, directory := range directories {
		if directory.IsDir() {
			directoriesNames = append(directoriesNames, directory.Name())
		}
	}

	return directoriesNames
}

func (s TempStorage) FileNames() []string {
	files, err := ioutil.ReadDir(s.path)
	if err != nil {
		panic(err)
	}

	fileNames := []string{}
	for _, file := range files {
		if !file.IsDir() {
			fileNames = append(fileNames, file.Name())
		}
	}

	return fileNames
}

func (s TempStorage) Directory(p string) []string {
	files, err := ioutil.ReadDir(filepath.Join(s.path, p))
	if err != nil {
		panic(err)
	}

	fileNames := []string{}
	for _, file := range files {
		if !file.IsDir() {
			fileNames = append(fileNames, file.Name())
		}
	}

	return fileNames
}

func (s TempStorage) CreateFile(name string, content []byte) string {
	tmpfile, err := ioutil.TempFile(s.Path(), name)
	if err != nil {
		panic(err)
	}

	if _, err := tmpfile.Write(content); err != nil {
		panic(err)
	}
	if err := tmpfile.Close(); err != nil {
		panic(err)
	}

	return tmpfile.Name()
}
