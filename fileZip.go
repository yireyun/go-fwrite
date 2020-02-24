// fileZip
package fwrite

import (
	"archive/zip"
	"io"
	"os"
	"runtime/debug"
)

const (
	zipFileSuffix = ".zip"
)

func zipLogFile(fileName string) error {
	defer func() {
		if x := recover(); x != nil {
			errorf("zipFile [%v] Error: %v\n", fileName, x)
			errorf("zipFile Stack => %s\n", debug.Stack())
		}
	}()
	srcfd, err := os.Open(fileName)
	if err != nil {
		return err
	}

	fileNameZip := fileName + zipFileSuffix
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
	if err == nil {
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
