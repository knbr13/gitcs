package main

import (
	"log"
	"math"
	"os"
	"time"
)

func isValidFolderPath(folder string) bool {
	// Check if the folder exists and is a directory
	info, err := os.Stat(folder)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		log.Fatal(err)
	}

	return info.IsDir()
}

var today = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

func daysAgo(t time.Time) int {
	milliSeconds := int(today.Sub(t).Milliseconds()) // milliseconds to days: 1000 * 60 * 60 * 24
	if milliSeconds < 0 {
		return 0
	}
	if milliSeconds%(1000*60*60*24) == 0 {
		return milliSeconds / (1000 * 60 * 60 * 24)
	}
	return milliSeconds/(1000*60*60*24) + 1
}

func getMaxValue(m map[int]int) int {
	max := math.MinInt
	for _, v := range m {
		if v > max {
			max = v
		}
	}
	return max
}
