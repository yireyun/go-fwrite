package fwrite

import (
	"bytes"
	"fmt"
	"os"
	"runtime/pprof"
	"testing"
	"time"
)

func TestWriteFile(t *testing.T) {
	//t.SkipNow()
	w := NewFileWrite("Journal")
	var err error
	//fileSync, filePrefix, writeSuffix, renameSuffix string,
	//rotate, dayend, zeroSize bool, maxLines, maxSize int,
	//cleaning bool, maxDays int
	_, err = w.Init(true, "test", "log", "log", "log",
		true, true, false, 10, 1<<20, true, 3)
	if err != nil {
		t.Fatal(err)
	}

	msg := []byte("012345678901234567890123456789012345678901234567890123456789")
	//	msg = append(msg, []byte("012345678901234567890123456789012345678901234567890123456789")...)
	//	msg = append(msg, []byte("012345678901234567890123456789012345678901234567890123456789")...)
	//	msg = append(msg, []byte("012345678901234567890123456789012345678901234567890123456789")...)
	//	msg = append(msg, []byte("012345678901234567890123456789012345678901234567890123456789")...)
	//	msg = append(msg, []byte("012345678901234567890123456789012345678901234567890123456789")...)
	//	msg = append(msg, []byte("012345678901234567890123456789012345678901234567890123456789")...)
	//	msg = append(msg, []byte("012345678901234567890123456789012345678901234567890123456789")...)
	//	msg = append(msg, []byte("012345678901234567890123456789012345678901234567890123456789")...)
	//	msg = append(msg, []byte("012345678901234567890123456789012345678901234567890123456789")...)
	buf := bytes.NewBuffer(make([]byte, 0, 1024))

	start := time.Now()
	var oldName, newName string
	var lineNo int64
	for i := 1; i <= 2*10; i++ {
		buf.Reset()
		buf.WriteString(fmt.Sprintf("%8d,WriteMsg:%s\n", i, msg))
		newName, lineNo, err = w.Write(buf.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		if newName != "" && newName != oldName {
			t.Logf("%05d,logFile:[%s]->[%s],%d", i, oldName, newName, lineNo)
			oldName = newName
		}
	}
	end := time.Now()
	if d := end.Sub(start); d < time.Second*10 {
		time.Sleep(d)
	}
	//w.Close()
}

var (
	write = NewFileWrite("Journal")
)

func TestBenchmarkWrite(t *testing.T) {
	t.SkipNow()
	rotatename := "testWrite.log" + fmt.Sprintf(".%s.%03d.log", time.Now().Format("2006-01-02"), 1)
	os.Remove("testWrite.log")
	os.Remove(rotatename)
	//fileSync, filePrefix, writeSuffix, renameSuffix string,
	//rotate, dayend,zeroSize bool, maxLines, maxSize int,
	//cleaning bool, maxDays int
	write.Init(true, "testWrite", "log", "log", "log", true, true, true,
		10000*100, 0, true, 3)

	//	t.SkipNow()

	os.Remove("pprofWrite")
	pproF, _ := os.Create("pprofWrite") // 创建记录文件
	pprof.StartCPUProfile(pproF)        // 开始cpu profile，结果写到文件f中
	defer pprof.StopCPUProfile()        // 结束profile

	N := 10000 * 100
	start := time.Now()
	for i := 0; i < N; i++ {
		write.WriteString(">>>>2015/10/10 16:48:32 [esLogger_test.go:151] #[L:D][M:M][A:A][T:T][S:00000005.000000][K:K]# debug\n")
	}
	end := time.Now()
	t.Logf("Cnt:%v,Use:%v", N, end.Sub(start))
}

func BenchmarkWrite(b *testing.B) {
	for i := 0; i < b.N; i++ {
		write.WriteString("debug")
	}
}
