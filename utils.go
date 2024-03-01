package main

import (
	"fmt"
	"math"
	"net/mail"
	"os"
	"os/exec"
	"time"
)

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

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

func setTimeFlags(sinceflag, untilflag string) (*Boundary, error) {
	var err error
	var boundary Boundary
	if untilflag != "" {
		boundary.Until, err = time.Parse("2006-01-02", untilflag)
		if err != nil {
			return nil, fmt.Errorf("invalid 'until' date format. please use the format: 2006-01-02")
		}
		if boundary.Until.After(now) {
			boundary.Until = now
		}
	} else {
		boundary.Until = now
	}
	if sinceflag != "" {
		boundary.Since, err = time.Parse("2006-01-02", sinceflag)
		if err != nil {
			return nil, fmt.Errorf("invalid 'since' date format. please use the format: 2006-01-02")
		}
	} else {
		boundary.Since = time.Date(boundary.Until.Year(), boundary.Until.Month(), boundary.Until.Day(), 0, 0, 0, 0, boundary.Until.Location()).AddDate(0, 0, -sixMonthsInDays)
	}

	return &boundary, nil
}
