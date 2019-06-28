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
		printf(" <Trace> fileLock.DoLock \"%s\" Success.", file.Name())
		return true
	} else {
		printf(" <Trace> fileLock.DoLock \"%s\" Exist.", file.Name())
		return false
	}
}

func (f *fileLock) Switch(oldFile, newFile *os.File) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.files[oldFile.Name()]; ok {
		delete(f.files, oldFile.Name())
		printf(" <Trace> fileLock.Switch Old \"%s\" Success.", oldFile)
	} else {
		printf(" <Trace> fileLock.Switch Old \"%s\" Not Exist.", oldFile)
		return false
	}

	if _, ok := f.files[newFile.Name()]; !ok {
		f.files[newFile.Name()] = newFile
		printf(" <Trace> fileLock.Switch New \"%s\" Success.", newFile)
		return true
	} else {
		printf(" <Trace> fileLock.Switch New \"%s\" Is Exist.", newFile)
		return false
	}
}

func (f *fileLock) Unlock(file *os.File) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.files[file.Name()]; ok {
		delete(f.files, file.Name())
		printf(" <Trace> fileLock.Unlock \"%s\" Success.", file.Name())
		return ok
	} else {
		printf(" <Trace> fileLock.Unlock \"%s\" Not Exist.", file.Name())
		return ok
	}
}

func (f *fileLock) Exists(name string) bool {
	f.mu.Lock()
	_, ok := f.files[name]
	f.mu.Unlock()
	if ok {
		printf(" <Trace> fileLock.Exists \"%s\" Is Exist.", name)
	} else {
		printf(" <Trace> fileLock.Exists \"%s\" Not Exist.", name)
	}

	return ok
}
