package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGraphHandlerReturnsJSONWithoutAbsoluteOpenTargets(t *testing.T) {
	root := t.TempDir()
	server := &mapWebServer{
		root: root,
		snapshot: mapSnapshot{
			Response: mapResponse{
				Repository: "example",
				Revision:   4,
				Nodes: []mapNodeResponse{{
					ID:       "main.go",
					Label:    "main.go",
					Language: "Go",
					Openable: true,
				}},
			},
			OpenTargets: map[NodeID]openTarget{
				"main.go": {Path: filepath.Join(root, "main.go"), Openable: true},
			},
		},
		subscribers: make(map[chan mapEvent]struct{}),
	}
	recorder := httptest.NewRecorder()
	server.handleGraph(recorder, httptest.NewRequest(http.MethodGet, "/api/graph", nil))

	if recorder.Code != http.StatusOK || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("graph response = %d %q", recorder.Code, recorder.Header())
	}
	if strings.Contains(recorder.Body.String(), root) {
		t.Fatalf("graph response leaks absolute root: %s", recorder.Body.String())
	}
	var response mapResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Revision != 4 || len(response.Nodes) != 1 || response.Nodes[0].ID != "main.go" {
		t.Fatalf("decoded graph = %#v", response)
	}
}

func TestOpenHandlerValidatesBoundaryAndUsesStoredLine(t *testing.T) {
	root := t.TempDir()
	wantPath := filepath.Join(root, "main.go")
	if err := os.WriteFile(wantPath, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var openedPath string
	var openedLine int
	server := &mapWebServer{
		root: root,
		opener: func(path string, line int) error {
			openedPath, openedLine = path, line
			return nil
		},
		snapshot: mapSnapshot{OpenTargets: map[NodeID]openTarget{
			"main.go": {Path: wantPath, Line: 17, Openable: true},
		}},
		subscribers: make(map[chan mapEvent]struct{}),
	}

	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7331/api/open", strings.NewReader(`{"id":"main.go"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://127.0.0.1:7331")
	recorder := httptest.NewRecorder()
	server.handleOpen(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("open response = %d: %s", recorder.Code, recorder.Body.String())
	}
	if openedPath != wantPath || openedLine != 17 {
		t.Fatalf("opened %q:%d, want %q:17", openedPath, openedLine, wantPath)
	}
}

func TestOpenHandlerRejectsUnknownCrossOriginAndWrongContentType(t *testing.T) {
	server := &mapWebServer{
		root:        t.TempDir(),
		opener:      func(string, int) error { return nil },
		snapshot:    mapSnapshot{OpenTargets: make(map[NodeID]openTarget)},
		subscribers: make(map[chan mapEvent]struct{}),
	}

	tests := []struct {
		name        string
		contentType string
		origin      string
		body        string
		want        int
	}{
		{name: "wrong content type", contentType: "text/plain", body: `{}`, want: http.StatusUnsupportedMediaType},
		{name: "cross origin", contentType: "application/json", origin: "https://example.com", body: `{"id":"main.go"}`, want: http.StatusForbidden},
		{name: "unknown", contentType: "application/json", body: `{"id":"missing.go"}`, want: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:7331/api/open", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			recorder := httptest.NewRecorder()
			server.handleOpen(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, test.want, recorder.Body.String())
			}
		})
	}
}

func TestShouldWatchCreatedDirectorySkipsRepositoryInternalsAndBuildOutput(t *testing.T) {
	root := filepath.Clean(`C:\repo`)
	tests := []struct {
		path string
		want bool
	}{
		{path: filepath.Join(root, "internal", "newpackage"), want: true},
		{path: filepath.Join(root, ".git", "objects", "ab"), want: false},
		{path: filepath.Join(root, "frontend", "node_modules", "package"), want: false},
		{path: filepath.Join(root, "frontend", "dist", "assets"), want: false},
		{path: filepath.Join(filepath.Dir(root), "outside"), want: false},
	}
	for _, test := range tests {
		if got := shouldWatchCreatedDirectory(root, test.path); got != test.want {
			t.Errorf("shouldWatchCreatedDirectory(%q) = %t, want %t", test.path, got, test.want)
		}
	}
}
