package fwrite

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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

//只写文件记录器
type FileWrite struct {
	//对象名称
	_Name_ string

	//互斥写文件
	muwt *MutexWrite

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
		_Name_: name,
		cfg:    cfg, //设置配置信息
		cfger:  cfg, //信息接口
		muwt: &MutexWrite{
			_Name_: name + ".MWrite",
			stdout: true,      //输出到Stdout
			file:   os.Stdout, //输出到Stdout
			cfger:  cfg,       //信息接口
		},
	}
	return w
}

//创建只写文件记录器，初始化输出到StdOut
//name	对象名称
//cfger 配置接口
func NewFileWriterConfig(name string, cfger Configer) *FileWrite {
	if cfger == nil {
		panic("Configer Is't Nil")
	}
	w := &FileWrite{
		_Name_: name,
		cfger:  cfger,
		cfg:    cfger.Config(), //设置配置信息
		muwt: &MutexWrite{
			_Name_: name + ".MWrite",
			stdout: true,      //输出到Stdout
			file:   os.Stdout, //输出到Stdout
			cfger:  cfger,     //输入配置
		},
	}
	return w
}

//创建只写文件记录器，初始化输出到StdOut
//name	对象名称
//cfger 配置接口
func (w *FileWrite) InitFileWriter(name string, cfger Configer) {
	if cfger == nil {
		panic("Configer Is't Nil")
	}
	w._Name_ = name
	w.cfger = cfger
	w.cfg = cfger.Config() //设置配置信息
	w.muwt = &MutexWrite{
		_Name_: name + ".mw",
		stdout: true,      //输出到Stdout
		file:   os.Stdout, //输出到Stdout
		cfger:  cfger,     //输入配置
	}
}

//初始化
//fileSync  	是输入是否同步写文件
//filePrefix	是输入文件前缀
//writeSuffix   是输入正在写文件后缀
//renameSuffix  是输入重命名文件后缀
//cleanSuffix	是输入清理文件名后缀
//rotate    	是输入是否自动分割
//dayend     	是输入是否文件日终
//zeroSize  	是输入是否新文件零尺寸
//maxLines   	是输入最大行数,最小为1行
//maxSize   	是输入最大尺寸,最小为1M
//cleaning     	是输入是否清理历史
//maxDays		是输入最大天数,最小为3天
func (w *FileWrite) Init(fileSync bool, filePrefix string,
	writeSuffix, renameSuffix, cleanSuffix string,
	rotate, dayend, zeroSize bool, maxLines, maxSize int64,
	cleaning bool, maxDays int) (string, error) {

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
		return "", errorf("filePrefix is null")
	}
	writeSuffix = suffix(writeSuffix)
	if writeSuffix == "" {
		return "", errorf("writeSuffix is null")
	}
	renameSuffix = suffix(renameSuffix)
	if renameSuffix == "" {
		return "", errorf("renameSuffix is null")
	}
	cleanSuffix = suffix(cleanSuffix)
	if cleanSuffix == "" {
		return "", errorf("cleanSuffix is null")
	}
	if rotate {
		var maxSizeOk, maxLinesOk bool
		switch {
		case maxSize < 0: //maxSize非法
			return "", errorf("maxSize not less than 0")
		case maxSize == 0:
			maxSizeOk = false
		case maxSize > 0:
			maxSizeOk = true
		}
		switch {
		case maxLines < 0: //最小行数为1行
			return "", errorf("maxLines not less than 0")
		case maxLines == 0: //最小行数为1行
			maxLinesOk = false
		case maxLines > 0: //最小行数为1行
			maxLinesOk = true
		}
		if !maxSizeOk && !maxLinesOk {
			return "", errorf("maxLines or maxSize is no set")
		}
	}

	if cleaning && maxDays < MaxKeepDays { //最小为3天
		return "", errorf("maxDays not less than 3 day")
	}

	if w.cfg.FileSync == fileSync && w.cfg.FilePrefix == filePrefix &&
		w.cfg.WriteSuffix == writeSuffix && w.cfg.RenameSuffix == renameSuffix &&
		w.cfg.CleanSuffix == cleanSuffix && w.cfg.Rotate == rotate &&
		w.cfg.Dayend == dayend && w.cfg.ZeroSize == zeroSize &&
		w.cfg.MaxLines == maxLines && w.cfg.MaxSize == maxSize &&
		w.cfg.Cleaning == cleaning && w.cfg.MaxDays == maxDays {
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
	w.cfg.ZeroSize = zeroSize
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

	if w.muwt.IsStdout() { //首次初始化
		err := w.fileRotate(w.cfger.GetFileEof())
		if err != nil {
			w.cfg.FileName = ""
			return w.cfg.FileName, err
		}
		go w.lockClean(w.cfg.FileName)
	}
	return w.cfg.FileName, nil
}

//文件旋转
func (w *FileWrite) fileRotate(fileEof []byte) (err error) {

	//互斥记录器切换文件
	err = w.muwt.SwitchFD()

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
	if w.cfg.Rotate && !w.muwt.IsStdout() && //未执行初始化,不切文件
		((w.cfg.MaxLines > 0 && w.cfg.CurLines >= w.cfg.MaxLines) || //最大行数触发切文件
			(w.cfg.MaxSize > 0 && w.cfg.CurSize >= w.cfg.MaxSize) || //最大尺寸触发切文件
			(w.cfg.Dayend && now.Day() != w.cfg.CurDay)) { //日期变化触发切文件
		defer w.mu.Unlock()
		if err := w.Rotate(); err != nil {
			printf(" <ERROR>[%s] %s rotate error：%v\n",
				logTime(), w._Name_, err)
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

	if !w.cfg.ZeroSize {
		if w.cfg.CurSize > 0 {
			count, err := w.cfger.GetFileLines(w.cfg.FileName)
			if err != nil {
				return errorf("get file lines err: %v\n", err)
			}

			w.cfg.CurLines = count
		} else {
			w.cfg.CurLines = 0
		}
	} else {
		w.cfg.CurLines = 0
	}
	return nil
}

//写入数据
//in    		是输入保存数据
//fileName  	是输出文件名
//lineNo    	是输出文件行号
//err   	   	是输出错误信息
func (w *FileWrite) Write(in []byte) (fileName string, lineNo int64, err error) {
	fileName, lineNo = w.rotateCheck(len(in))
	_, err = w.muwt.Write(in)
	return
}

//写入字符串
//s    		是输入保存数据
//fileName  是输出文件名
//lineNo    是输出文件行号
//err   	是输出错误信息
func (w *FileWrite) WriteString(s string) (fileName string, lineNo int64, err error) {
	fileName, lineNo = w.rotateCheck(len(s))
	_, err = w.muwt.WriteString(s)
	return
}

//执行文件旋转
func (w *FileWrite) Rotate() error {
	err := w.fileRotate(w.cfger.GetFileEof())
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
	if !FileExist(w.cfg.FileName) { //文件不存在
		return ErrFileMiss, nil
	}
	return w.fileClean(w.cfg.FileName)
}

func (w *FileWrite) fileClean(fileName string) (error, []string) {
	dir := filepath.Dir(fileName)
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return errorf(" <ERROR> %s abs path error：%v\n", w._Name_, err), nil
	}

	if atomic.CompareAndSwapInt32(&w.fileCleaning, 0, 1) {
		defer atomic.StoreInt32(&w.fileCleaning, 0)
	} else {
		return errorf(" <ERROR> %s is fileCleaning \"%s\"\n", w._Name_, absPath), nil
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
				printf(" <ERROR>[%s] %s clean \"%s\" panic:%v\n",
					logTime(), w._Name_, path, r)
			}
		}()

		if err != nil {
			//fmt.Printf(" <ERROR> %s walk \"%s\" error:%v\n", w.name, path, err)
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
		printf(" <ERROR>[%v] %s over walk error: %v\n",
			logTime(), w._Name_, err)
	}

	//结算Keep保持时间
	keepDays := w.cfg.MaxDays
	if keepDays < MaxKeepDays {
		keepDays = MaxKeepDays
	}
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
				printf(" <ERROR>[%s] %s os.remove %v, err : %v\n",
					logTime(), w._Name_, file.Path, err)
			} else {
				cleanFile = append(cleanFile, file.Path)
			}
			continue
		}

		//检查并更改名称
		if w.cfg.CleanRename && w.cfg.CleanRenameSuffix &&
			file.Modfy.Unix() < yesterday &&
			strings.HasSuffix(file.Path, w.cfg.WriteSuffix) &&
			!FileLocked(file.Base) {
			newName, err := w.cfger.GetFileRename(file.Base)
			if err == nil {
				err = os.Rename(file.Base, newName)
				if err != nil {
					printf(" <ERROR>[%s] %s os.rename %v -> %v , err : %v \n",
						logTime(), w._Name_, file.Base, newName, err)
				}
			} else {
				printf(" <ERROR>[%s] %s get rename %v -> %v , err : %v \n",
					logTime(), w._Name_, file.Base, newName, err)
			}
		}
	}
	return nil, cleanFile
}

func (w *FileWrite) lockClean(fileName string) error {
	dir := filepath.Dir(fileName)
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return errorf("\t%s abs path error：%v\n", w._Name_, err)
	}

	if atomic.CompareAndSwapInt32(&w.lockCleaning, 0, 1) {
		defer atomic.StoreInt32(&w.lockCleaning, 0)
	} else {
		return errorf("%s is cleaning \"%s\"\n", w._Name_, absPath)
	}

	//遍历目录函数函数
	cleanFunc := func(path string, info os.FileInfo, err error) (retErr error) {
		defer func() {
			if x := recover(); x != nil {
				printf(" <ERROR>[%s] %s clean \"%s\" panic :%v\n",
					logTime(), w._Name_, path, x)
			}
		}()

		if err != nil {
			//printf(" <ERROR> %s walk \"%s\" error:%v\n", w.Name, path, err)
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
		printf(" <ERROR>[%s] %s over walk error:%v\n",
			logTime(), w._Name_, err)
	}
	return nil
}

//释放所有资源
func (w *FileWrite) Destroy() {
	w.muwt.Close()
}

//释放所有资源
func (w *FileWrite) Close() error {
	return w.muwt.Close()
}

//写入缓存数据
func (w *FileWrite) Flush() {
	w.muwt.Flush()
}
