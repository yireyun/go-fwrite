// fileUtil
package fwrite

import (
	"fmt"
	"os"
	"time"
)

var (
	ErrFileMiss   = fmt.Errorf("file is missing")
	ErrFileNil    = fmt.Errorf("file is nil")
	ErrFileClosed = fmt.Errorf("file is closed")
	ErrFileOpened = fmt.Errorf("file is opened")
	ErrNameSame   = fmt.Errorf("file name is same or nil")
	ErrNameEmpty  = fmt.Errorf("file name is empty or nil")
)

const (
	logFormat = "2006-01-02T15:04:05.000000000Z07:00"
	FileEof   = "EOF"
)

var (
	fLocks = &fileLock{files: make(map[string]*os.File)}
)

func logTime() string {
	return time.Now().Format(logFormat)
}

//判斷文件是否存在
//	Lstat写法存读不到信息的BUG, 使用OpenFile来判断
func FileExist(fileName string) bool {
	//本写法不能准确判断文件是否存在
	stat, err := os.Lstat(fileName)
	if stat != nil && err == nil {
		return true
	}

	//补救措施: 采用打开文件方式判断
	if file, e := os.OpenFile(fileName, os.O_RDONLY, 0666); e == nil {
		file.Close()
		return true
	} else {
		return os.IsExist(e)
	}
}

//判斷文件是否锁定
//	Lstat写法存读不到信息的BUG, 使用OpenFile来判断
func FileLocked(fileName string) bool {
	//本写法不能准确判断文件是否存在
	stat, err := os.Lstat(fileName)
	if stat != nil && err == nil {
		return fLocks.Exists(stat.Name())
	}

	//补救措施: 采用打开文件方式判断
	if file, e := os.OpenFile(fileName, os.O_RDONLY, 0666); e == nil {
		file.Close()
		return fLocks.Exists(file.Name())
	} else if os.IsExist(e) {
		return fLocks.Exists(fileName)
	} else {
		return false
	}
}

//获取文件信息和是否存在
func FileInfo(fileName string) (stat os.FileInfo, exist, locked bool, err error) {
	//本写法不能准确判断文件是否存在
	stat, err = os.Lstat(fileName)
	if stat != nil && err == nil {
		return stat, true, fLocks.Exists(stat.Name()), err
	}

	//补救措施: 采用打开文件方式判断
	file, err := os.OpenFile(fileName, os.O_RDONLY, 0666)
	if err == nil {
		tryTimes := 0
		stat, err = file.Stat()
		for stat == nil && tryTimes < 100 {
			stat, err = file.Stat()
			tryTimes++
		}
		file.Close()
		if stat != nil {
			return stat, true, fLocks.Exists(stat.Name()), err
		} else {
			printf(" <ERROR>[%s] FileInfo \"%v\" By os.OpenFile() Error: %v \n",
				logTime(), fileName, err)
		}
	}

	if exist = os.IsExist(err); exist {
		tryTimes := 0
		stat, err = os.Lstat(fileName)
		for stat == nil && tryTimes < 100 {
			stat, err = os.Lstat(fileName)
			tryTimes++
		}
		if stat != nil {
			return stat, true, fLocks.Exists(stat.Name()), err
		} else {
			printf(" <ERROR>[%s] %s FileInfo \"%v\" By os.IsExist() Error: %v \n",
				logTime(), fileName, err)
		}
	}
	return nil, false, fLocks.Exists(fileName), ErrFileMiss
}

//以WriteOnly和Append打开文件，不存在则创建
func openFileWithCreateAppend(fileName string, fielSync bool) (*os.File, error) {
	flag := os.O_APPEND | os.O_CREATE
	if fielSync {
		flag |= os.O_SYNC
	}
	fd, err := os.OpenFile(fileName, flag, 0660) //建议同步写
	return fd, err
}

func printf(format string, args ...interface{}) {
	fmt.Fprintf(output(), format, args...)
}

func sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

func errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
