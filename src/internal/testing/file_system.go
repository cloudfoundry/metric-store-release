package testing

import (
	"io/ioutil"
	"os"
)

type TempStorage struct {
	path string
}

func NewTempStorage() TempStorage {
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

func (s TempStorage) FileNames() []string {
	files, err := ioutil.ReadDir(s.path)
	if err != nil {
		panic(err)
	}

	fileNames := []string{}
	for _, file := range files {
		fileNames = append(fileNames, file.Name())
	}

	return fileNames
}
