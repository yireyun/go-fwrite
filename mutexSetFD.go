// mutexSetFD
package fwrite

import (
	"os"

	flock "github.com/yireyun/go-flock"
)

//互斥设置文件（Go程安全）
//fileSync  	输入新创建文件是否同步文件
//fileLock  	输入新创建文件是否加锁文件
//rename    	输入是否重命名当前文件
//fileName  	输入当前文件名
//fileRename	输入重命名文件
//fileEof   	输入文件结束标志
func (mw *MutexWrite) setFd(fileSync, fileLock, rename bool,
	fileName, fileRename string, fileEof []byte) (err error) {
	if mw == nil {
		return ErrFileNil
	}

	if fileName == "" {
		return ErrNameEmpty
	}
	if rename && mw.file != os.Stdout && (fileRename == "" || fileRename == fileName) {
		return ErrNameSame
	}
	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	if mw.file != nil && mw.file != os.Stdout {

		curName := mw.file.Name()

		if !mw.closed && len(fileEof) > 0 {
			mw.file.Write(fileEof)
		}
		//关闭文件
		if !mw.closed {
			err = mw.file.Close()
			fLocks.Unlock(mw.file)
			mw.closed = true
			if err != nil {
				printf(" <ERROR> MutexWrite: close \"%s\" error:%v\n", curName, err)
			}
		}

		//解除锁定
		if mw.flock != nil {
			err = mw.flock.Unlock()
			mw.flock = nil
			if err != nil {
				printf(" <ERROR> MutexWrite: unlock \"%s\" error:%v\n", curName, err)
			}
		}

		//重命名文件
		if rename && !mw.renamed {

			if curName == "" {
				printf(" <ERROR> MutexWrite: rename old file error:%v\n", ErrNameEmpty)
				goto NEWFILE
			}
			if e := os.Rename(curName, fileRename); e != nil {
				printf(" <ERROR> MutexWrite: file rename \"%s\" -> \"%s\" error:%v\n",
					curName, fileRename, e)
				goto NEWFILE
			}
			mw.renamed = true
		}
	}

NEWFILE:
	for {

		//以append方式打开文件，不存在则创建
		var fd *os.File
		fd, err = openFileWithCreateAppend(fileName, fileSync)
		if err != nil {
			return err
		}
		fs, fe := fd.Stat()
		var fileSize int64
		if fe != nil {
			fileSize = 0
		} else {
			fileSize = fs.Size()
		}
		mw.file, mw.closed, mw.renamed, mw.stdout = fd, false, false, false
		mw.cfger.setCurFileName(fileName, fileSize)
		fLocks.DoLock(mw.file)

		//锁定文件
		if fileLock {
			mw.flock = flock.NewFlock(fileName + LockSuffix)
			err = mw.flock.NBLock()
			if err != nil {
				printf(" <ERROR> MutexWrite: lock \"%s\" error:%v\n", fileName, err)
			}
		}
		return nil
	}
}
