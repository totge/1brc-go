package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
)

func TestProcess(t *testing.T) {
	midExpected, err := os.Open("./output/expected/result_mid.csv")

	if err != nil {
		t.Errorf("Error when opening comparison file.")
		return
	}

	expectedHash := sha256.New()
	if _, err := io.Copy(expectedHash, midExpected); err != nil {
		log.Fatal(err)
	}

	//fmt.Printf("%x", expectedHash.Sum(nil))

	process("data/measurements_mid.txt")

	midResult, err := os.Open("output/result.csv")
	
	if err != nil {
		t.Errorf("Error when opening result file.")
		return
	}

	resultHash := sha256.New()
	if _, err := io.Copy(resultHash, midResult); err != nil {
		log.Fatal(err)
	}

	//fmt.Printf("%x", resultHash.Sum(nil))

	if fmt.Sprintf("%x", resultHash.Sum(nil)) != fmt.Sprintf("%x", expectedHash.Sum(nil)) {
		t.Errorf("Expected hash: %x, result hash:%x\n",expectedHash.Sum(nil), resultHash.Sum(nil))
	}
}
