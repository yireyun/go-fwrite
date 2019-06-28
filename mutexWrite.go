// mutexWrite
package fwrite

import (
	"os"
	"sync"

	flock "github.com/yireyun/go-flock"
)

type MutexConfiger interface {

	//是否启用同步写
	IsFileSync() bool

	//是否启用文件锁
	IsFileLock() bool

	//新文件零尺寸
	IsZeroSize() bool

	//是否启用重命名
	IsRename() bool

	//获取文件名
	//fileName	是出文件名
	//err   	是输出错误信息
	GetNewFileName() (fileName string, err error)

	//设置文件名
	setCurFileName(fileName string, fileSize int64)

	//获取重命名文件名
	//fileName  	是输入文件名
	//fileRename	是输出重命名文件名
	//err       	是输出错误信息
	GetFileRename(fileName string) (fileRename string, err error)

	//获取文件结束填充
	GetFileEof() []byte
}

//互斥写文件
type MutexWrite struct {
	_Name_  string
	mutex   sync.Mutex
	file    *os.File      //当前输出文件
	cfger   MutexConfiger //配置信息接口
	flock   flock.Flocker //当前输出文件文件锁
	stdout  bool          //当前输出文件是控制台
	closed  bool          //当前输出文件是否被关闭
	renamed bool          //当前输出文件是否已重命名
}

func NewMutexWrite(cfger MutexConfiger) *MutexWrite {
	if cfger == nil {
		panic("MutexConfiger Is Nil")
	}
	return &MutexWrite{file: os.Stdout, stdout: true, cfger: cfger}
}

func (mw *MutexWrite) IsStdout() bool {
	if mw == nil {
		return false
	}

	return mw.stdout
}

func (mw *MutexWrite) IsOpen() bool {
	if mw == nil {
		return false
	}
	if mw.stdout {
		return false
	}

	mw.mutex.Lock()
	opened := mw.file != nil && mw.file != os.Stdout
	mw.mutex.Unlock()

	return opened
}

func (mw *MutexWrite) FileStat() (os.FileInfo, error) {
	if mw == nil {
		return nil, ErrFileNil
	}

	mw.mutex.Lock()
	if mw.closed || mw.file == os.Stdout {
		mw.mutex.Unlock()
		return nil, ErrFileClosed
	}
	stat, err := mw.file.Stat()
	mw.mutex.Unlock()

	return stat, err
}

//互斥写数据（Go程安全）
func (mw *MutexWrite) Write(b []byte) (int, error) {
	if mw == nil {
		return 0, ErrFileNil
	}

	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	if mw.closed {
		return 0, ErrFileClosed
	}

	return mw.file.Write(b)
}

//互斥写字符串（Go程安全）
func (mw *MutexWrite) WriteString(s string) (int, error) {
	if mw == nil {
		return 0, ErrFileNil
	}

	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	if mw.closed {
		return 0, ErrFileClosed
	}

	return mw.file.WriteString(s)
}

//写入缓存数据
func (mw *MutexWrite) Flush() {
	if mw == nil {
		return
	}

	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	if mw.file != os.Stdout && mw.file != nil && !mw.closed {
		mw.file.Sync()
	}
}

//互斥关闭文件（Go程安全）
func (mw *MutexWrite) Close() (err error) {
	if mw == nil {
		return ErrFileNil
	}

	if mw.stdout {
		return nil
	}

	rename := mw.cfger.IsRename()
	fileEof := mw.cfger.GetFileEof()

	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	if mw.file == os.Stdout {
		return nil
	}

	if mw.closed {
		return ErrFileClosed
	}

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
				printf(" <ERROR>[%s] %s close \"%s\" error:%v\n\n",
					logTime(), mw._Name_, curName, err)
			}
		}

		//解除锁定
		if mw.flock != nil {
			err = mw.flock.Unlock()
			mw.flock = nil
			if err != nil {
				printf(" <ERROR>[%s] %s unlock \"%s\" error:%v\n\n",
					logTime(), mw._Name_, curName, err)
			}
		}

		//重命名文件
		if rename && !mw.renamed {

			if curName == "" {
				printf(" <ERROR>[%s] %s rename old file error:%v\n\n",
					logTime(), mw._Name_, ErrNameEmpty)
				return ErrNameEmpty
			}

			fileRename, renameErr := mw.cfger.GetFileRename(curName)
			if renameErr != nil {
				printf(" <ERROR>[%s] %s rename \"%s\" error:%v\n\n",
					logTime(), mw._Name_, curName, renameErr)
				return renameErr
			}

			if fileRename == "" || fileRename == curName {
				printf(" <ERROR>[%s] %s rename \"%s\" -> \"%s\" error: %v\n\n",
					logTime(), mw._Name_, curName, fileRename, ErrNameSame)
				return ErrNameSame
			}

			if e := os.Rename(curName, fileRename); e != nil {
				printf(" <ERROR>[%s] %s rename \"%s\" -> \"%s\" error: %v\n\n",
					logTime(), mw._Name_, curName, fileRename, e)
				return e
			}
			mw.renamed = true
		}
	}
	return nil
}
