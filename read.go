package main

import (
	"bufio"
	"io"
	"os"
)

func readBufioScanner(path string) int {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count
}

func readBufio4KB(path string) int  { return readBufioSize(path, 4*1024) }
func readBufio1MB(path string) int  { return readBufioSize(path, 1024*1024) }
func readBufio16MB(path string) int { return readBufioSize(path, 16*1024*1024) }

func readBufioSize(path string, bufSize int) int {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, bufSize)
	count := 0
	for {
		_, err := reader.ReadSlice('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			if err == bufio.ErrBufferFull {
				continue
			}
			panic(err)
		}
		count++
	}
	return count
}

func readChunked(path string) int {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	buf := make([]byte, 16*1024*1024)
	count := 0
	for {
		n, err := file.Read(buf)
		for _, b := range buf[:n] {
			if b == '\n' {
				count++
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
	}
	return count
}
