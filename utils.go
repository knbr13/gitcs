package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"time"
)

func isValidFolderPath(folder string) bool {
	// Check if the folder exists and is a directory
	info, err := os.Stat(folder)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		fmt.Fprintf(os.Stderr, "gitcs: path %q: error: %s\n", folder, err.Error())
		os.Exit(1)
	}

	return info.IsDir()
}

var today = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 99999999, now.Location())

func daysAgo(t time.Time) int {
	milliSeconds := int(today.Sub(t).Milliseconds()) // milliseconds to days: 1000 * 60 * 60 * 24
	if milliSeconds < 0 {
		return -1
	}
	if milliSeconds/(1000*60*60) < 24 {
		return 0
	}
	return milliSeconds / (1000 * 60 * 60 * 24)
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

func getGlobalEmailFromGit() string {
	localEmail, err := exec.Command("git", "config", "--global", "user.email").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gitcs: unable to retrieve your global Git email: %s", err.Error())
		os.Exit(1)
	}

	return string(localEmail)
}
