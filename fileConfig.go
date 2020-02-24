// fileConfig
package fwrite

import (
	"bytes"
	"io"
	"math"
	"os"
	"time"
)

//文件配置接口
type Configer interface {

	//继承互斥配置接口
	MutexConfiger

	//读取文件配置信息
	Config() *FileConfig

	//获取文件行数
	//fileName	是输入文件名
	//int64  	是输出记录行数
	//error 	操作结果
	GetFileLines(fileName string) (int64, error)
}

type FileConfig struct {
	Name         string
	FilePrefix   string //文件名前缀
	WriteSuffix  string //正在写文件后缀
	RenameSuffix string //重命名文件后缀
	CleanSuffix  string //清理文件后缀
	FileName     string //当前文件名
	FileEof      []byte //文件结束填充
	FileSync     bool   //是否同步写文件
	FileLock     bool   //是否文件锁定
	FileZip      bool   //是否压缩文件

	// Rotate at size
	Rotate             bool  //是否自动分割
	Dayend             bool  //文件日终切换
	ZeroSize           bool  //新文件零尺寸
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
	c.ZeroSize = false           //默认为false
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

//设置文件结束填充
func (c *FileConfig) SetFileEof(fileEof []byte) {
	c.FileEof = fileEof
}

//是否启用同步写
func (c *FileConfig) IsFileSync() bool {
	return c.FileSync
}

//是否启用文件锁
func (c *FileConfig) IsFileLock() bool {
	return c.FileLock
}

//新文件零尺寸
func (c *FileConfig) IsZeroSize() bool {
	return c.ZeroSize
}

//是否启用重命名
func (c *FileConfig) IsRename() bool {
	return c.RotateRename
}

func (c *FileConfig) IsFileZip() bool {
	return c.FileZip
}

//获取文件结束填充
func (c *FileConfig) GetFileEof() []byte {
	return c.FileEof
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
		return "", errorf("get file rename, fileName is null")
	}

	if c.RenameSuffix == "" {
		return "", errorf("get file rename, renameSuffix is null")
	}
	//获取新文件名，如：test.log.2015-09-06.006.log，序号最大MaxInt16
	for num := 1; num <= math.MaxInt16; num++ {
		fileRename = sprintf("%s.%s.%03d%s", fileName,
			modifyTime.Format("2006-01-02"), num, c.RenameSuffix)
		if !FileExist(fileRename) && !FileExist(fileRename+zipFileSuffix) {
			//文件不存在则返回
			return fileRename, nil
		}
	}

	return "", errorf("Cannot find free file rename number:%s", fileName)
}

//获取重命名文件名
//fileName  	是输入文件名
//fileRename	是输出重命名文件名
//err       	是输出错误信息
func (c *FileConfig) GetFileRename(fileName string) (fileRename string, err error) {
	fileInfo, exist, _, err := FileInfo(fileName)
	if !exist { //文件不存在
		return "", errorf("get file rename error: %v ", err)
	}
	return c.getFileRename(fileName, fileInfo.ModTime())
}

//获取文件名
//fileName	是出文件名
//err   	是输出错误信息
func (c *FileConfig) GetNewFileName() (fileName string, err error) {
	if c.FilePrefix == "" {
		return "", errorf("get file name, filePrefix is null")
	}

	if c.WriteSuffix == "" {
		return "", errorf("get file name, writeSuffix is null")
	}

	fileName = c.FilePrefix + c.WriteSuffix

	if c.RotateRename {
		if info, exist, locked, _ := FileInfo(fileName); exist && !locked { //文件存在
			//尺寸大于0，并且人日期不等于当前，进行文件切换
			if info.Size() > 0 && info.ModTime().Day() != time.Now().Day() {
				newName, e := c.getFileRename(fileName, info.ModTime())
				if e == nil {
					if e = os.Rename(fileName, newName); e != nil {
						printf(" <ERROR>[%s] %s rename [%s] error: %v\n\n",
							logTime(), c.Name, fileName, e)
					} else if c.IsFileZip() {
						go zipLogFile(newName)
					}
				} else {
					printf(" <ERROR>[%s] %s get rename [%s] error: %v\n\n",
						logTime(), c.Name, fileName, e)
				}
			}
		}
	}

	return fileName, nil
}

func (c *FileConfig) setCurFileName(fileName string, fileSize int64) {
	c.FileName = fileName
	c.CurSize = fileSize
	c.CurLines = 0
	c.CurDay = time.Now().Day()
}

func (c *FileConfig) MutexWriter(fw *FileWrite, in []byte) (int, error) {
	return fw.muwt.Write(in)
}

//文件旋转检查
//size     	是输入写内容尺寸
//fileName  是输出文件名
//lineNo    是输出文件行号
func (c *FileConfig) RotateCheck(fw *FileWrite, size int) (
	fileName string, lineNo int64) {
	return fw.rotateCheck(size)
}
