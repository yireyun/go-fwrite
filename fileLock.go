// fileList
package fwrite

import (
	"os"
	"strings"
	"sync"
	"time"
)

const (
	traceLock = true
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
		if traceLock {
			printf("<TRACE>[%s] $fileLock.DoLock \"%s\" Success.\n\n",
				time.Now().Format(logFormat), file.Name())
		}
		return true
	} else {
		if traceLock {
			printf("<TRACE>[%s] $fileLock.DoLock \"%s\" IsExist.\n\n",
				time.Now().Format(logFormat), file.Name())
		}
		return false
	}
}

func (f *fileLock) Unlock(file *os.File) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.files[file.Name()]; ok {
		delete(f.files, file.Name())
		if traceLock {
			printf("<TRACE>[%s] $fileLock.Unlock \"%s\" Success.\n\n",
				time.Now().Format(logFormat), file.Name())
		}
		return ok
	} else {
		if traceLock {
			printf("<TRACE>[%s] $fileLock.Unlock \"%s\" Not Exist.\n\n",
				time.Now().Format(logFormat), file.Name())
		}
		return ok
	}
}

func (f *fileLock) Exists(name string) bool {

	name = strings.Replace(name, `\`, `/`, -1)

	ok := func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()

		_, ok := f.files[name]
		return ok
	}()

	if ok {
		if traceLock {
			printf("<TRACE>[%s] $fileLock.Exists \"%s\" Is Locked.\n\n",
				time.Now().Format(logFormat), name)
		}
	} else {
		if traceLock {
			printf("<TRACE>[%s] $fileLock.Exists \"%s\" Not Exist.\n\n",
				time.Now().Format(logFormat), name)
		}
	}

	return ok
}
