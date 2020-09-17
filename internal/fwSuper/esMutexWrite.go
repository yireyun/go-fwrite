// Copyright 2014 beego Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// fork from github.com\astaxie\beego\logs

package fwSuper

import (
	"fmt"
	"os"
	"sync"

	"github.com/yireyun/go-flock"
)

//互斥写文件
type MutexWrite struct {
	sync.Mutex
	out   *os.File      //当前输出文件
	flock flock.Flocker //当前输出文件文件锁
}

//以WriteOnly和Append打开文件，不存在则创建
func (mw *MutexWrite) openFile(fileName string, fielSync bool) (*os.File, error) {
	flag := os.O_WRONLY | os.O_APPEND | os.O_CREATE
	if fielSync {
		flag |= os.O_SYNC
	}
	fd, err := os.OpenFile(fileName, flag, 0660) //建议同步写
	return fd, err
}

//互斥写数据
func (mw *MutexWrite) Write(b []byte) (int, error) {
	mw.Lock()
	defer mw.Unlock()

	return mw.out.Write(b)
}

//互斥写字符串
func (mw *MutexWrite) WriteString(s string) (int, error) {
	mw.Lock()
	defer mw.Unlock()

	return mw.out.WriteString(s)
}

//设置互斥文件
func (mw *MutexWrite) SetFd(fileSync, fileLock, rename bool,
	fileName, fileRename string) (err error) {
	if fileName == "" {
		return fmt.Errorf("fileName equi or is null")
	}
	if rename && mw.out != os.Stdout && (fileRename == "" || fileRename == fileName) {
		return fmt.Errorf("fileRename equi fileName or is null")
	}
	mw.Lock()
	defer mw.Unlock()

	if mw.out != nil && mw.out != os.Stdout {
		//关闭文件
		err = mw.out.Close()
		if err != nil {
			return
		}

		//解除锁定
		if mw.flock != nil {
			err = mw.flock.Unlock() // ▲ 解锁当前文件锁
			if err != nil {
				fmt.Printf("\t%s unlock '%s' error:%v\n", mw.out.Name(), err)
			}
		}

		//重命名文件
		if rename {
			if err = os.Rename(mw.out.Name(), fileRename); err != nil {
				return
			} else if mw.zipFile {
				go zipFile(fileRename)
			}
		}

	}
	//锁定文件
	if fileLock {
		mw.flock = flock.NewFlock(fileName + LockSuffix)
		err = mw.flock.NBLock()
		if err != nil {
			fmt.Printf("\t%s lock '%s' error:%v\n", fileName, err)
		}
	}

	//以append方式打开文件，不存在则创建
	var fd *os.File
	fd, err = mw.openFile(fileName, fileSync)
	if err != nil {
		return err
	}
	mw.out = fd
	return nil
}
