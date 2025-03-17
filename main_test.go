package main

import (
	"os"
	"testing"
)

func TestMainFunction(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	os.Args = []string{"cmd", "-path=./test_data", "-email=tester@test.com"}

	exitCode := 0
	func() {
		defer func() {
			if r := recover(); r != nil {
				if code, ok := r.(int); ok {
					exitCode = code
				} else {
					t.Fatalf("unexpected panic: %v", r)
				}
			}
		}()
		main()
	}()

	if exitCode != 0 {
		t.Errorf("main() exited with code %d; expected 0", exitCode)
	}
}
