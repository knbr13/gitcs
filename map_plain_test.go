package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintPlainMapIncludesRootedAndOtherFilesWithoutANSI(t *testing.T) {
	graph := graphForLearningTest(
		[]Node{
			{ID: "main.go", IsRoot: true},
			{ID: "service.go"},
			{ID: "unused.go"},
		},
		[]Edge{{From: "main.go", To: "service.go", Kind: EdgeKindCalls}},
	)
	var output bytes.Buffer

	if err := printPlainMap(&output, `C:\repo`, graph); err != nil {
		t.Fatalf("printPlainMap() error = %v", err)
	}

	text := output.String()
	for _, expected := range []string{
		"gitcs project map: C:\\repo",
		"3 files, 1 connections",
		"- main.go",
		"  - service.go",
		"Other files:",
		"- unused.go",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("plain output does not contain %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatalf("plain output contains ANSI escapes: %q", text)
	}
}

func TestPrintPlainMapHandlesEmptyGraph(t *testing.T) {
	var output bytes.Buffer

	if err := printPlainMap(&output, "repo", NewGraph()); err != nil {
		t.Fatalf("printPlainMap() error = %v", err)
	}
	if !strings.Contains(output.String(), "No supported source files found.") {
		t.Fatalf("empty graph output = %q", output.String())
	}
}
