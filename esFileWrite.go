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

package fwrite

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yireyun/go-flock"
)

const (
	MaxKeepDays = 3       //最大文件保持天数
	LockSuffix  = ".lock" //锁文件后缀
)

type FileWriter interface {
	//写入数据
	//in    	是输入保存数据
	//fileName  是输出文件名
	//lineNo    是输出文件行号
	//err   	是输出错误信息
	Write(in []byte) (fileName string, lineNo int64, err error)

	//写入字符串
	//s    		是输入保存数据
	//fileName  是输出文件名
	//lineNo    是输出文件行号
	//err   	是输出错误信息
	WriteString(s string) (fileName string, lineNo int64, err error)
}

//互斥写文件
type MutexWrite struct {
	sync.Mutex
	zipFile bool
	out     *os.File      //当前输出文件
	flock   flock.Flocker //当前输出文件文件锁
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
			err = mw.flock.Unlock()
			if err != nil {
				fmt.Printf("\t%s unlock '%s' error:%v\n", mw.out.Name(), err)
			}
		}

		//重命名文件
		if rename {
			err = os.Rename(mw.out.Name(), fileRename)
			if err != nil {
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

//文件配置接口
type Configer interface {

	//读取文件配置信息
	Config() *FileConfig

	//获取文件行数
	//fileName	是输入文件名
	GetFileLines(fileName string) (int64, error)

	//获取重命名文件名
	//fileName  	是输入文件名
	//fileRename	是输出重命名文件名
	//err       	是输出错误信息
	GetFileRename(fileName string) (fileRename string, err error)

	//获取文件名
	//fileName	是出文件名
	//err   	是输出错误信息
	GetFileName() (fileName string, err error)
}

type FileConfig struct {
	Name         string
	FilePrefix   string //文件名前缀
	WriteSuffix  string //正在写文件后缀
	RenameSuffix string //重命名文件后缀
	CleanSuffix  string //清理文件后缀
	FileName     string //当前文件名
	FileSync     bool   //是否同步写文件
	FileLock     bool   //是否文件锁定
	// Rotate at size
	Rotate             bool  //是否自动分割
	Dayend             bool  //文件日终
	ZipFile            bool  //压缩文件
	RotateRename       bool  //分割时是否重命名
	RotateRenameSuffix bool  //分割时是否只对后缀重命名
	MaxLines           int64 //最大行数,最小为1行
	CurLines           int64 //当前行数
	MaxSize            int64 //最大尺寸,最小为1M
	CurSize            int64 //当前尺寸

	// Rotate daily
	Cleaning          bool //清理历史
	CleanRename       bool //清理文件时是否重命名
	CleanRenameSuffix bool //清理文件时是否只对后缀重命名
	MaxDays           int  //最大天数,最小为3天
	CurDay            int  //当期天

}

//读取文件配置信息
func (c *FileConfig) Config() *FileConfig {
	return c
}

func (c *FileConfig) InitAsDefault(name string) {
	c.Name = name
	c.FilePrefix = ""            //默认为空
	c.FileSync = false           //默认为false
	c.FileLock = false           //默认为false
	c.FileName = ""              //默认为空
	c.Rotate = true              //默认为true
	c.Dayend = true              //默认为true
	c.RotateRename = true        //默认为true
	c.RotateRenameSuffix = false //默认为false
	c.MaxLines = 1000000         //默认为1000000
	c.CurLines = 0               //初始为0
	c.MaxSize = 1 << 28          //默认为256 MB
	c.CurSize = 0                //初始为0
	c.Cleaning = true            //默认为true
	c.CleanRename = false        //默认为false
	c.CleanRenameSuffix = false  //默认为false
	c.MaxDays = 7                //默认为7天
	c.CurDay = time.Now().Day()  //初始为当前日期
}

//获取文件行数
//fileName	是输入文件名
func (c *FileConfig) GetFileLines(fileName string) (int64, error) {
	fd, err := os.Open(fileName)
	if err != nil {
		return 0, err
	}
	defer fd.Close()

	buf := make([]byte, 32768) // 32k
	count := int64(0)
	lineSep := []byte{'\n'}

	for {
		c, err := fd.Read(buf)
		if err != nil && err != io.EOF {
			return count, err
		}

		count += int64(bytes.Count(buf[:c], lineSep))

		if err == io.EOF {
			break
		}
	}

	return count, nil
}

func (c *FileConfig) getFileRename(fileName string, modifyTime time.Time) (
	fileRename string, err error) {
	if fileName == "" {
		return "", fmt.Errorf("get file rename, fileName is null")
	}

	if c.RenameSuffix == "" {
		return "", fmt.Errorf("get file rename, renameSuffix is null")
	}
	//获取新文件名，如：test.log.2015-09-06.006.log，序号最大MaxInt16
	for num := 1; num <= math.MaxInt16; num++ {
		fileRename = fmt.Sprintf("%s.%s.%03d%s", fileName,
			modifyTime.Format("2006-01-02"), num, c.RenameSuffix)
		_, fileNameErr := os.Lstat(fileRename)
		_, fileZipNameErr := os.Lstat(fileRename + ".zip")
		if fileNameErr != nil && fileZipNameErr != nil {
			//文件不存在则返回
			return fileRename, nil
		}
	}

	return "", fmt.Errorf("Cannot find free file rename number:%s", fileName)
}

//获取重命名文件名
//fileName  	是输入文件名
//fileRename	是输出重命名文件名
//err       	是输出错误信息
func (c *FileConfig) GetFileRename(fileName string) (fileRename string, err error) {
	var fileInfo os.FileInfo
	fileInfo, err = os.Lstat(fileName)
	if err != nil { //文件不存在
		return "", fmt.Errorf("get file rename error:%v", err)
	}

	return c.getFileRename(fileName, fileInfo.ModTime())
}

//获取文件名
//fileName	是出文件名
//err   	是输出错误信息
func (c *FileConfig) GetFileName() (fileName string, err error) {
	if c.FilePrefix == "" {
		return "", fmt.Errorf("get file name, filePrefix is null")
	}

	if c.WriteSuffix == "" {
		return "", fmt.Errorf("get file name, writeSuffix is null")
	}

	fileName = c.FilePrefix + c.WriteSuffix

	if c.RotateRename {
		if info, e := os.Lstat(fileName); e == nil { //文件存在
			//尺寸大于0，并且人日期不等于当前，进行文件切换
			if info.Size() > 0 && info.ModTime().Day() != time.Now().Day() {
				newName, e := c.getFileRename(fileName, info.ModTime())
				if e == nil {
					if e = os.Rename(fileName, newName); e != nil {
						fmt.Printf("\t[%s] rename [%s] error: %v\n",
							fileName, newName, e)
					} else {
						go zipFile(newName)
					}
				} else {
					fmt.Printf("\t[%s] get rename [%s] error: %v\n",
						c.Name, fileName, e)
				}
			}
		}
	}

	return fileName, nil
}

func (c *FileConfig) MutexWriter(fw *FileWrite, in []byte) (int, error) {
	return fw.mw.Write(in)
}

//文件旋转检查
//size     	是输入写内容尺寸
//fileName  是输出文件名
//lineNo    是输出文件行号
func (c *FileConfig) RotateCheck(fw *FileWrite, size int) (
	fileName string, lineNo int64) {
	return fw.rotateCheck(size)
}

//只写文件记录器
type FileWrite struct {
	//对象名称
	Name string
	//互斥写文件
	mw *MutexWrite

	//配置信息
	cfg *FileConfig
	//配置接口
	cfger Configer

	//文件清理状态
	fileCleaning int32
	//锁清理状态
	lockCleaning int32

	mu sync.Mutex
}

//创建只写文件记录器，初始化输出到StdOut
//name	对象名称
func NewFileWrite(name string) *FileWrite {
	cfg := new(FileConfig)
	cfg.InitAsDefault(name)
	w := &FileWrite{
		Name:  name,
		cfg:   cfg,                         //设置配置信息
		cfger: cfg,                         //
		mw:    &MutexWrite{out: os.Stdout}, //输出到Stdout
	}
	return w
}

//创建只写文件记录器，初始化输出到StdOut
//name	对象名称
//cfger 配置接口
func NewFileWriterConfig(name string, cfger Configer) *FileWrite {
	w := &FileWrite{
		Name:  name,
		cfger: cfger,
		cfg:   cfger.Config(),              //设置配置信息
		mw:    &MutexWrite{out: os.Stdout}, //输出到Stdout
	}
	return w
}

//初始化只写文件记录器
//name	对象名称
//cfger 配置接口
func (w *FileWrite) InitFileWriter(name string, cfger Configer) {
	w.Name = name
	w.cfger = cfger
	w.cfg = cfger.Config()
	w.mw = &MutexWrite{out: os.Stdout} //输出到Stdout
}

//初始化
//fileSync  	是输入是否同步写文件
//filePrefix	是输入文件前缀
//writeSuffix   是输入正在写文件后缀
//renameSuffix  是输入重命名文件后缀
//cleanSuffix	是输入清理文件名后缀
//rotate    	是输入是否自动分割
//dayend     	是输入是否文件日终
//zipFile   	是输入是否压缩文件
//maxLines   	是输入最大行数,最小为1行
//maxSize   	是输入最大尺寸,最小为1M
//cleaning     	是输入是否清理历史
//maxDays		是输入最大天数,最小为3天
func (w *FileWrite) Init(fileSync bool, filePrefix string,
	writeSuffix, renameSuffix, cleanSuffix string, rotate, dayend, zipFile bool,
	maxLines, maxSize int64, cleaning bool, maxDays int) (string, error) {
	prefix := func(s string) string {
		s = strings.TrimSpace(s)
		if l := len(s); l > 0 && s[l-1] == '.' {
			return s[:l-1]
		} else {
			return s
		}
	}

	suffix := func(s string) string {
		s = strings.TrimSpace(s)
		if l := len(s); l > 0 && s[0] != '.' {
			return "." + s
		} else {
			return s
		}
	}
	filePrefix = prefix(filePrefix)
	if filePrefix == "" {
		return "", fmt.Errorf("filePrefix is null")
	}
	writeSuffix = suffix(writeSuffix)
	if writeSuffix == "" {
		return "", fmt.Errorf("writeSuffix is null")
	}
	renameSuffix = suffix(renameSuffix)
	if renameSuffix == "" {
		return "", fmt.Errorf("renameSuffix is null")
	}
	cleanSuffix = suffix(cleanSuffix)
	if cleanSuffix == "" {
		return "", fmt.Errorf("cleanSuffix is null")
	}
	if rotate {
		var maxSizeOk, maxLinesOk bool
		switch {
		case maxSize < 0: //maxSize非法
			return "", fmt.Errorf("maxSize not less than 0")
		case maxSize == 0:
			maxSizeOk = false
		case maxSize > 0:
			maxSizeOk = true
		}
		switch {
		case maxLines < 0: //最小行数为1行
			return "", fmt.Errorf("maxLines not less than 0")
		case maxLines == 0: //最小行数为1行
			maxLinesOk = false
		case maxLines > 0: //最小行数为1行
			maxLinesOk = true
		}
		if !maxSizeOk && !maxLinesOk {
			return "", fmt.Errorf("maxLines or maxSize is no set")
		}
	}

	if cleaning && maxDays < MaxKeepDays { //最小为3天
		return "", fmt.Errorf("maxDays not less than 3 day")
	}

	if w.cfg.FileSync == fileSync &&
		w.cfg.FilePrefix == filePrefix &&
		w.cfg.WriteSuffix == writeSuffix &&
		w.cfg.RenameSuffix == renameSuffix &&
		w.cfg.CleanSuffix == cleanSuffix &&
		w.cfg.Rotate == rotate &&
		w.cfg.Dayend == dayend &&
		w.cfg.ZipFile == zipFile &&
		w.cfg.MaxLines == maxLines &&
		w.cfg.MaxSize == maxSize &&
		w.cfg.Cleaning == cleaning &&
		w.cfg.MaxDays == maxDays {
		return w.cfg.FileName, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.cfg.FileSync = fileSync
	w.cfg.FilePrefix = filePrefix
	w.cfg.WriteSuffix = writeSuffix
	w.cfg.RenameSuffix = renameSuffix
	w.cfg.CleanSuffix = cleanSuffix
	w.cfg.Rotate = rotate
	w.cfg.Dayend = dayend
	w.cfg.ZipFile = zipFile
	w.mw.zipFile = zipFile
	if w.cfg.RotateRenameSuffix {
		w.cfg.RotateRename = writeSuffix != renameSuffix
	} else {
		w.cfg.RotateRename = true
	}
	w.cfg.MaxLines = maxLines
	w.cfg.MaxSize = maxSize
	w.cfg.Cleaning = cleaning
	if w.cfg.CleanRenameSuffix {
		w.cfg.CleanRename = writeSuffix != renameSuffix
	} else {
		w.cfg.CleanRename = false
	}
	w.cfg.MaxDays = maxDays

	if w.mw.out == os.Stdout { //首次初始化
		err := w.fileRotate()
		if err != nil {
			w.cfg.FileName = ""
			return w.cfg.FileName, err
		}
		go w.lockClean(w.cfg.FileName)
	}
	return w.cfg.FileName, nil
}

//文件旋转
func (w *FileWrite) fileRotate() (err error) {
	//获取重命名
	var fileRename string
	if w.cfg.RotateRename && w.mw.out != os.Stdout {
		if fileRename, err = w.cfger.GetFileRename(w.cfg.FileName); err != nil {
			return err
		}
	}

	//获取文件名
	var fileName string
	fileName, err = w.cfger.GetFileName()
	if err != nil {
		return err
	}

	//互斥记录器切换文件
	err = w.mw.SetFd(w.cfg.FileSync, w.cfg.FileLock, w.cfg.RotateRename,
		fileName, fileRename)
	if err != nil {
		return err
	} else {
		w.cfg.FileName = fileName
	}

	w.rotateInit()

	return
}

//文件旋转检查
//size     	是输入写内容尺寸
//fileName  是输出文件名
//lineNo    是输出文件行号
func (w *FileWrite) rotateCheck(size int) (fileName string, lineNo int64) {
	w.mu.Lock()
	now := time.Now()
	if w.cfg.Rotate && w.mw.out != os.Stdout && //未执行初始化,不切文件
		((w.cfg.MaxLines > 0 && w.cfg.CurLines >= w.cfg.MaxLines) || //最大行数触发切文件
			(w.cfg.MaxSize > 0 && w.cfg.CurSize >= w.cfg.MaxSize) || //最大尺寸触发切文件
			(w.cfg.Dayend && now.Day() != w.cfg.CurDay)) { //日期变化触发切文件
		defer w.mu.Unlock()
		if err := w.Rotate(); err != nil {
			fmt.Printf("\t%s file rotate error：%v\n", w.Name, err)
			return
		}
		w.cfg.CurLines++
		w.cfg.CurSize += int64(size)
		return w.cfg.FileName, w.cfg.CurLines
	} else {
		w.cfg.CurLines++
		w.cfg.CurSize += int64(size)
		fileName, lineNo = w.cfg.FileName, w.cfg.CurLines
		w.mu.Unlock()
		return
	}
}

//文件旋转初始化
func (w *FileWrite) rotateInit() error {
	fileInfo, err := w.mw.out.Stat()
	if err != nil {
		return fmt.Errorf("get stat err: %v\n", err)
	}

	w.cfg.CurLines = 0
	w.cfg.CurSize = int64(fileInfo.Size())
	w.cfg.CurDay = time.Now().Day()

	if fileInfo.Size() > 0 {
		count, err := w.cfger.GetFileLines(w.cfg.FileName)
		if err != nil {
			return fmt.Errorf("get file lines err: %v\n", err)
		}
		w.cfg.CurLines = count
	}
	return nil
}

func zipFile(fileName string) error {
	srcfd, err := os.Open(fileName)
	if err != nil {
		return err
	}

	fileNameZip := fileName + ".zip"
	flag := os.O_WRONLY | os.O_TRUNC | os.O_CREATE
	zipFd, err := os.OpenFile(fileNameZip, flag, 0660)
	if err != nil {
		return err
	}

	info, err := srcfd.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Method = zip.Deflate
	zipWrite := zip.NewWriter(zipFd)
	writer, err := zipWrite.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, srcfd)
	if err != nil {
		zipErr := zipWrite.Close()
		srcfd.Close()
		zipFd.Close()
		if zipErr == nil {
			return os.Remove(fileName)
		} else {
			return zipErr
		}
	} else {
		srcfd.Close()
		zipFd.Close()
		zipWrite.Close()
		return err
	}
}

//写入数据
//in    		是输入保存数据
//fileName  	是输出文件名
//lineNo    	是输出文件行号
//err   	   	是输出错误信息
func (w *FileWrite) Write(in []byte) (fileName string, lineNo int64, err error) {
	fileName, lineNo = w.rotateCheck(len(in))
	_, err = w.mw.Write(in)
	return
}

//写入字符串
//s    		是输入保存数据
//fileName  是输出文件名
//lineNo    是输出文件行号
//err   	是输出错误信息
func (w *FileWrite) WriteString(s string) (fileName string, lineNo int64, err error) {
	fileName, lineNo = w.rotateCheck(len(s))
	_, err = w.mw.WriteString(s)
	return
}

//执行文件旋转
func (w *FileWrite) Rotate() error {
	_, err := os.Lstat(w.cfg.FileName)
	if err != nil { //文件不存在
		return err
	}

	err = w.fileRotate()
	if err != nil { //文件旋转错
		return err
	}

	if w.cfg.Cleaning { //执行文件清理
		go w.fileClean(w.cfg.FileName)
	}

	return nil
}

//文件清理
func (w *FileWrite) FileClean() (error, []string) {
	_, err := os.Lstat(w.cfg.FileName)
	if err != nil { //文件不存在
		return err, nil
	}
	return w.fileClean(w.cfg.FileName)
}

func (w *FileWrite) fileClean(fileName string) (error, []string) {
	dir := filepath.Dir(fileName)
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("\t%s abs path error：%v\n", w.Name, err), nil
	}

	if atomic.CompareAndSwapInt32(&w.fileCleaning, 0, 1) {
		defer atomic.StoreInt32(&w.fileCleaning, 0)
	} else {
		return fmt.Errorf("\t%s is fileCleaning '%s'\n", w.Name, absPath), nil
	}

	now := time.Now()

	type file struct {
		Path  string
		Base  string
		Name  string      // base name of the file
		Size  int64       // length in bytes for regular files; system-dependent for others
		Mode  os.FileMode // file mode bits
		Modfy time.Time   // modification time
		ToDay int64
	}

	//计算指定日期凌晨0点时间
	truncToDay := func(t time.Time) int64 {
		return t.Unix() - int64(t.Hour())*60*60 -
			int64(t.Minute())*60 - int64(t.Second())
	}

	yesterday := truncToDay(now)   //计算今天凌晨0点时间
	files := make([]*file, 0, 256) //初始化文件数组

	//遍历目录函数函数
	cleanFunc := func(path string, info os.FileInfo, err error) (retErr error) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("\t%s clean '%s' error:%v\n", w.Name, path, r)
			}
		}()

		if err != nil {
			//fmt.Printf("\t%s walk '%s' error:%v\n", w.name, path, err)
			return nil
		}

		if !info.IsDir() {
			basePath := filepath.Base(path)
			basePrefix := filepath.Base(w.cfg.FilePrefix)

			if strings.HasPrefix(basePath, basePrefix) &&
				!strings.HasSuffix(basePath, fileName+LockSuffix) &&
				strings.HasSuffix(basePath, w.cfg.WriteSuffix+LockSuffix) {
				os.Remove(basePath)
				return
			}

			toDay := truncToDay(info.ModTime())
			if strings.HasPrefix(basePath, basePrefix) && toDay < yesterday {
				files = append(files, &file{Path: path, Name: info.Name(),
					Base: basePath, Size: info.Size(), Mode: info.Mode(),
					Modfy: info.ModTime(), ToDay: toDay})
			}
		}
		return
	}

	//读取小于max的最大时间
	maxModfy := func(max int64) (next int64) {
		for _, file := range files {
			if max > file.ToDay && file.ToDay > next {
				next = file.ToDay
			}
		}
		return
	}

	//遍类目录线的所有文件
	err = filepath.Walk(dir, cleanFunc)
	if err != nil {
		fmt.Printf("\t%s over walk error:%v\n", w.Name, err)
	}

	//结算Keep保持时间
	keepDays := w.cfg.MaxDays
	if keepDays < MaxKeepDays {
		keepDays = MaxKeepDays
	}

	//计算保留最后keepDays个文件
	var keepTime int64 = yesterday
	for i := 0; i < keepDays && keepTime > 0; i++ {
		keepTime = maxModfy(keepTime)
	}

	//取绝对失效时间
	abcTime := yesterday - 60*60*24*int64(keepDays)

	cleanFile := make([]string, 0, len(files))
	//对文件进行排序
	for _, file := range files {
		//删除过期的数据，至少保持最近3天的数据文件，增加结对时间判断防止误删除
		if file.Modfy.Unix() < abcTime && file.Modfy.Unix() < keepTime &&
			strings.HasSuffix(file.Path, w.cfg.CleanSuffix) {
			err := os.Remove(file.Path)
			if err != nil {
				fmt.Printf("\t%s %v\n", w.Name, file.Path)
			} else {
				cleanFile = append(cleanFile, file.Path)
			}
			continue
		}
		//检查并更改名称

		if w.cfg.CleanRename && w.cfg.CleanRenameSuffix &&
			file.Modfy.Unix() < yesterday &&
			strings.HasSuffix(file.Path, w.cfg.WriteSuffix) {
			newName, err := w.cfger.GetFileRename(file.Base)
			if err == nil {
				err = os.Rename(file.Path, newName)
				if err != nil {
					fmt.Printf("\t%s %v\n", w.Name, file.Path)
				} else {
					go zipFile(newName)
				}
			} else {
				fmt.Printf("\t%s %v\n", w.Name, err)
			}
		}
	}
	return nil, cleanFile
}

func (w *FileWrite) lockClean(fileName string) error {
	dir := filepath.Dir(fileName)
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("\t%s abs path error：%v\n", w.Name, err)
	}

	if atomic.CompareAndSwapInt32(&w.lockCleaning, 0, 1) {
		defer atomic.StoreInt32(&w.lockCleaning, 0)
	} else {
		return fmt.Errorf("\t%s is lockCleaning '%s'\n", w.Name, absPath)
	}

	//遍历目录函数函数
	cleanFunc := func(path string, info os.FileInfo, err error) (retErr error) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("\t%s clean '%s' error:%v\n", w.Name, path, r)
			}
		}()

		if err != nil {
			//fmt.Printf("\t%s walk '%s' error:%v\n", w.name, path, err)
			return nil
		}

		if !info.IsDir() {
			basePath := filepath.Base(path)
			basePrefix := filepath.Base(w.cfg.FilePrefix)
			if strings.HasPrefix(basePath, basePrefix) &&
				!strings.HasSuffix(basePath, fileName+LockSuffix) &&
				strings.HasSuffix(basePath, w.cfg.WriteSuffix+LockSuffix) {
				os.Remove(basePath)
			}
		}
		return
	}

	//遍类目录线的所有文件
	err = filepath.Walk(dir, cleanFunc)
	if err != nil {
		fmt.Printf("\t%s over walk error:%v\n", w.Name, err)
	}
	return nil
}

//释放所有资源
func (w *FileWrite) Destroy() {
	if w.mw.out != os.Stdout && w.mw.out != nil {
		w.mw.out.Close()
	}
}

//写入缓存数据
func (w *FileWrite) Flush() {
	if w.mw.out != os.Stdout && w.mw.out != nil {
		w.mw.out.Sync()
	}
}
