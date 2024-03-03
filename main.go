package main

import (
	"fmt"
	"bufio"
	"os"
	"strings"
	"strconv"
)
type measurement struct {
	location string
	temperature float64
}

func main(){

	rawData, err := loadData("data/measurements_small.txt")
	if err != nil {
		fmt.Println("Could not open file")
	}
	
	data := processData(rawData)

	count := make(map[string] int)

	for _, m := range data {
		count[m.location] += 1
	}

	for k, v := range count {
		if v > 1 {
			fmt.Printf("%s: %d\n", k, v)
		}
	}
}


func processData(rawData []string) []measurement {
	var processedData []measurement
	for i, line := range rawData {
		parts := strings.Split(line, ";")
		temperature, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			fmt.Printf("Error parsing line %d: %s could not be parsed\n", i, parts[1])
		}
		processedData = append(processedData, measurement{location: parts[0], temperature: temperature})
	}
	
	return processedData
}
func loadData(filePath string) (data []string, err error){
	file, err := os.Open(filePath)

	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		data = append(data, scanner.Text())

	}

	return data, nil
}