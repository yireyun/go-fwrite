// mutexNewFD
package fwrite

import (
	"os"

	flock "github.com/yireyun/go-flock"
)

//互斥切换文件（Go程安全）
func (mw *MutexWrite) SwitchFD() (err error) {
	if mw == nil {
		return ErrFileNil
	}

	fileSync := mw.cfger.IsFileSync()
	fileLock := mw.cfger.IsFileLock()
	rename := mw.cfger.IsRename()
	fileEof := mw.cfger.GetFileEof()

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
			mw.closed = true
			if err != nil {
				printf(" <ERROR>[%s] %s close \"%s\" error:%v\n",
					logTime(), mw._Name_, curName, err)
			}
		}

		//解除锁定
		if mw.flock != nil {
			err = mw.flock.Unlock()
			mw.flock = nil
			if err != nil {
				printf(" <ERROR>[%s] %s unlock \"%s\" error:%v\n",
					logTime(), mw._Name_, curName, err)
			}
		}

		//重命名文件
		if rename && !mw.renamed {

			if curName == "" {
				printf(" <ERROR>[%s] %s rename old file error:%v\n",
					logTime(), mw._Name_, ErrNameEmpty)
				goto NEWFILE
			}

			fileRename, renameErr := mw.cfger.GetFileRename(curName)
			if renameErr != nil {
				printf(" <ERROR>[%s] %s rename \"%s\" error:%v\n",
					logTime(), mw._Name_, curName, renameErr)
				goto NEWFILE
			}

			if fileRename == "" || fileRename == curName {
				printf(" <ERROR>[%s] %s rename \"%s\" -> \"%s\"  error:%v\n",
					logTime(), mw._Name_, curName, fileRename, ErrNameSame)
				goto NEWFILE
			}

			if e := os.Rename(curName, fileRename); e != nil {
				printf(" <ERROR>[%s] %s rename \"%s\" -> \"%s\" error:%v\n",
					logTime(), mw._Name_, curName, fileRename, e)
				goto NEWFILE
			}
			mw.renamed = true
		}
	}

NEWFILE:
	for {
		fileName, fileErr := mw.cfger.GetNewFileName()
		if fileErr != nil {
			return fileErr
		}

		//以append方式打开文件，不存在则创建
		var fd *os.File
		fd, err = openFileWithCreateAppend(fileName, fileSync)
		if err != nil {
			return err
		}
		fs, fe := fd.Stat()
		if fe != nil {
			fd.Close()
			continue
		}
		if mw.cfger.IsZeroSize() && fs.Size() > 0 {
			fd.Close()

			fileRename, renameErr := mw.cfger.GetFileRename(fileName)
			if renameErr != nil {
				printf(" <ERROR>[%s] %s rename \"%s\" error:%v\n",
					logTime(), mw._Name_, fileName, renameErr)
				continue
			}

			if fileRename == "" || fileRename == fileName {
				printf(" <ERROR>[%s] %s rename \"%s\" -> \"%s\"  error:%v\n",
					logTime(), mw._Name_, fileName, fileRename, ErrNameSame)
				continue
			}

			if e := os.Rename(fileName, fileRename); e != nil {
				printf(" <ERROR>[%s] %s rename \"%s\" -> \"%s\" error:%v\n",
					logTime(), mw._Name_, fileName, fileRename, e)
				continue
			}
			mw.renamed = true
			continue
		}
		mw.file, mw.closed, mw.renamed, mw.stdout = fd, false, false, false
		mw.cfger.setCurFileName(fileName, fs.Size())
		fLocks.DoLock(mw.file)

		//锁定文件
		if fileLock {
			mw.flock = flock.NewFlock(fileName + LockSuffix)
			err = mw.flock.NBLock()
			if err != nil {
				printf(" <ERROR>[%s] %s lock \"%s\" error:%v\n",
					logTime(), mw._Name_, fileName, err)
			}
		}
		return nil
	}
}
