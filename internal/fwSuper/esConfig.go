package fwSuper

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"time"
)

const (
	MaxKeepDays = 3 //最大文件保持天数
)

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
		if fileNameErr != nil {
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
						print(fmt.Sprintf("\t[%s] rename [%s] error:%v\n",
							fileName, newName, e))
					} else if mw.zipFile {
						go zipFile(fileRename)
					}
				} else {
					print(fmt.Sprintf("\t[%s] get rename [%s] error:%v\n",
						c.Name, fileName, e))
				}
			}
		}
	}

	return fileName, nil
}
