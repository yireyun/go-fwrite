// fileList
package fwrite

import (
	"os"
	"sync"
)

type fileLock struct {
	files map[string]*os.File
	mu    sync.Mutex
}

func (f *fileLock) DoLock(file *os.File) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.files[file.Name()]; !ok {
		f.files[file.Name()] = file
		return true
	} else {
		return false
	}
}

func (f *fileLock) Switch(oldFile, newFile *os.File) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.files[oldFile.Name()]; ok {
		delete(f.files, oldFile.Name())
	} else {
		return false
	}

	if _, ok := f.files[newFile.Name()]; !ok {
		f.files[newFile.Name()] = newFile
		return true
	} else {
		return false
	}
}

func (f *fileLock) Unlock(file *os.File) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.files[file.Name()]; ok {
		delete(f.files, file.Name())
		return ok
	} else {
		return ok
	}
}

func (f *fileLock) Exists(name string) bool {
	f.mu.Lock()
	_, ok := f.files[name]
	f.mu.Unlock()

	return ok
}
