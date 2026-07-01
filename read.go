package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"syscall"

	"golang.org/x/exp/mmap"
	"golang.org/x/sys/unix"
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
		count += bytes.Count(buf[:n], []byte{'\n'})

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

func readMMappedNoCopy(path string) int {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Get file size
	info, err := file.Stat()
	if err != nil {
		panic(err)
	}
	size := info.Size()

	// Map the entire file into memory as a single giant []byte
	// MAP_SHARED means changes are shared (we only read, so it doesn't matter)
	// PROT_READ means we only want read access
	data, err := syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		panic(err)
	}
	// Make sure we unmap when done to free OS resources
	defer syscall.Munmap(data)

	err = unix.Madvise(data, unix.MADV_SEQUENTIAL)
	if err != nil {
		panic(err)
	}
	// Now we have ZERO COPY. 'data' is pointing directly to the OS cache!
	count := 0
	// for _, b := range data {
	// 	if b == '\n' {
	// 		count++
	// 	}
	// }

	for i := range len(data) {
		if data[i] == '\n' {
			count++
		}
	}

	return count
}

type Chunk struct {
	start int64
	end   int64
}

func calculateChunkBoundaries(file *os.File, numChunks int) []Chunk {
	chunks := make([]Chunk, numChunks)

	// Get file size
	info, err := file.Stat()
	if err != nil {
		panic(err)
	}
	fileSize := info.Size()

	baseChunkSize := fileSize / int64(numChunks)
	var offset int64
	// station name is 100 bytes max, measurement is another ~10 bytes
	// so 128 is enough fit on record
	peekBuf := make([]byte, 128)

	for i := range numChunks {
		chunks[i].start = offset

		targetEnd := offset + baseChunkSize // for the last chunk this coulb go over the file length
		// handling going over file length
		if targetEnd >= fileSize {
			chunks[i].end = fileSize
			break
		}

		n, err := file.ReadAt(peekBuf, targetEnd)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				// fmt.Println("Successfully reached the end of the file.")
				log.Fatalf("Error reading file during iteration: %v", err)
			}
		}

		for correction := range n {
			if peekBuf[correction] == '\n' {
				offset = targetEnd + int64(correction) + 1
				break
			}
		}

		chunks[i].end = offset

	}

	return chunks
}

func processChunk(file *os.File, chunk Chunk, bufferSize int, c chan int) {
	var offset int64 = int64(chunk.start)
	buffer := make([]byte, bufferSize)
	counter := 0

	for offset < chunk.end {

		n, _ := file.ReadAt(buffer, offset)
		// if err != nil {
		// 	if !errors.Is(err, io.EOF) {
		// 		// fmt.Println("Successfully reached the end of the file.")
		// 		log.Fatalf("Error reading file during iteration: %v", err)
		// 	}
		// }

		// only read until chunk end if buffer read data over it
		bytesLeft := chunk.end - offset
		readSize := min(int64(n), bytesLeft)

		counter += bytes.Count(buffer[:readSize], []byte{'\n'})

		offset += int64(n)
	}

	c <- counter
}

func readChunkedConcurrent(path string, numChunks int, buffSize int) int {

	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	chunks := calculateChunkBoundaries(file, numChunks)
	c := make(chan int, numChunks)

	for _, chunk := range chunks {
		go processChunk(file, chunk, buffSize, c)
	}

	total := 0
	for range numChunks {
		total += <-c
	}

	fmt.Printf("Found %d records.\n", total)
	return total
}

func readChunkedWorkerPool(path string, numChunks int, numWorkers int, buffSize int) int {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	chunks := calculateChunkBoundaries(file, numChunks)

	// 2. Put all chunks into a buffered channel
	chunkChan := make(chan Chunk, numChunks)
	for _, chunk := range chunks {
		chunkChan <- chunk
	}
	close(chunkChan) // Close it so workers know when to stop

	resultChan := make(chan int, numWorkers)

	for range numWorkers {
		go func() {
			// Allocate the buffer ONCE per worker, reuse it for all chunks!
			buffer := make([]byte, buffSize)
			workerTotal := 0

			// The worker constantly pulls chunks until the channel is empty
			for chunk := range chunkChan {
				workerTotal += processSingleChunk(file, chunk, buffer)
			}
			resultChan <- workerTotal
		}()
	}

	// 4. Aggregate results from the workers
	total := 0
	for range numWorkers {
		total += <-resultChan
	}

	return total
}

// Notice we pass the pre-allocated buffer in, rather than recreating it!
func processSingleChunk(file *os.File, chunk Chunk, buffer []byte) int {
	offset := chunk.start
	counter := 0
	bufferSize := len(buffer)

	for offset < chunk.end {
		bytesLeft := chunk.end - offset
		readSize := min(int64(bufferSize), bytesLeft)

		n, _ := file.ReadAt(buffer[:readSize], offset)

		counter += bytes.Count(buffer[:n], []byte{'\n'})

		offset += int64(n)
	}

	return counter
}
