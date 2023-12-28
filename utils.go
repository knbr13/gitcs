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

func daysAgo(t time.Time) int {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	hours := int(today.Sub(t).Hours())
	if hours < 0 {
		return 0
	}
	if hours%24 == 0 {
		return hours / 24
	}
	return hours/24 + 1
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
