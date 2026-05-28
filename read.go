package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/exp/mmap"
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

func readChunked(path string, size int) int {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	buf := make([]byte, size)
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

func readMMapped(path string, bufSize int) {
	reader, err := mmap.Open(path)
	if err != nil {
		log.Fatalf("failed to open file: %v", err)
	}
	defer reader.Close()

	buf := make([]byte, bufSize)
	n := 0

	for true {
		_, err := reader.ReadAt(buf, int64(n*bufSize))
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println("Successfully reached the end of the file.")
				break
			}
			log.Fatalf("Error reading file during iteration: %v", err)
		}

		n++

	}
}

func readMMappedIterate(path string, bufSize int) {
	reader, err := mmap.Open(path)
	if err != nil {
		log.Fatalf("failed to open file: %v", err)
	}
	defer reader.Close()

	buf := make([]byte, bufSize)
	n := 0

	for true {
		_, err := reader.ReadAt(buf, int64(n*bufSize))
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Println("Successfully reached the end of the file.")
				break
			}
			log.Fatalf("Error reading file during iteration: %v", err)
		}

		n++

	}
}
