package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-git/go-git/v5"
)

const mapServerAddress = "127.0.0.1:7331"

type mapEvent struct {
	Revision uint64 `json:"revision"`
	Error    string `json:"error,omitempty"`
}

type mapWebServer struct {
	root     string
	repo     *git.Repository
	worktree *git.Worktree
	opener   func(string, int) error

	mu          sync.RWMutex
	snapshot    mapSnapshot
	subscribers map[chan mapEvent]struct{}
}

func runWebMap() error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not find the current folder: %w", err)
	}
	repo, err := git.PlainOpenWithOptions(workingDirectory, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return fmt.Errorf("the current folder is not inside a Git repository: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("could not read the repository worktree: %w", err)
	}
	root := worktree.Filesystem.Root()

	server, err := newMapWebServer(root, repo, worktree, openFileInVSCodeAtLine)
	if err != nil {
		return err
	}
	handler, err := server.routes()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", mapServerAddress)
	if err != nil {
		return fmt.Errorf("listen on http://%s: %w", mapServerAddress, err)
	}

	runContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go server.watch(runContext)

	httpServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-runContext.Done()
		shutdownContext, cancel := contextWithTimeout(3 * time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownContext)
	}()

	address := "http://" + mapServerAddress
	fmt.Printf("gitcs map: %s\n", address)
	if err := openBrowser(address); err != nil {
		fmt.Fprintf(os.Stderr, "gitcs map: could not open the browser: %s\n", err)
	}
	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func contextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

func newMapWebServer(
	root string,
	repo *git.Repository,
	worktree *git.Worktree,
	opener func(string, int) error,
) (*mapWebServer, error) {
	snapshot, err := buildMapSnapshot(root, repo, worktree)
	if err != nil {
		return nil, fmt.Errorf("build initial repository map: %w", err)
	}
	snapshot.Response.Revision = 1
	return &mapWebServer{
		root:        root,
		repo:        repo,
		worktree:    worktree,
		opener:      opener,
		snapshot:    snapshot,
		subscribers: make(map[chan mapEvent]struct{}),
	}, nil
}

func (server *mapWebServer) routes() (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/graph", server.handleGraph)
	mux.HandleFunc("POST /api/open", server.handleOpen)
	mux.HandleFunc("GET /events", server.handleEvents)

	assets, err := mapAssets()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", http.FileServerFS(assets))
	return mux, nil
}

func (server *mapWebServer) handleGraph(response http.ResponseWriter, _ *http.Request) {
	server.mu.RLock()
	graph := server.snapshot.Response
	server.mu.RUnlock()
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(response).Encode(graph); err != nil {
		http.Error(response, "could not encode graph", http.StatusInternalServerError)
	}
}

func (server *mapWebServer) handleOpen(response http.ResponseWriter, request *http.Request) {
	if !sameOrigin(request) {
		http.Error(response, "cross-origin request rejected", http.StatusForbidden)
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(response, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}
	var payload struct {
		ID NodeID `json:"id"`
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil || payload.ID == "" {
		http.Error(response, "invalid open request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(response, "invalid open request", http.StatusBadRequest)
		return
	}

	server.mu.RLock()
	target, exists := server.snapshot.OpenTargets[payload.ID]
	server.mu.RUnlock()
	if !exists {
		http.Error(response, "unknown file", http.StatusNotFound)
		return
	}
	if err := validateOpenTarget(server.root, target); err != nil {
		http.Error(response, err.Error(), http.StatusConflict)
		return
	}
	if err := server.opener(target.Path, max(1, target.Line)); err != nil {
		http.Error(response, "could not open VS Code", http.StatusInternalServerError)
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (server *mapWebServer) handleEvents(response http.ResponseWriter, request *http.Request) {
	flusher, supported := response.(http.Flusher)
	if !supported {
		http.Error(response, "streaming is unavailable", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")

	events := make(chan mapEvent, 2)
	server.mu.Lock()
	server.subscribers[events] = struct{}{}
	currentRevision := server.snapshot.Response.Revision
	server.mu.Unlock()
	defer func() {
		server.mu.Lock()
		delete(server.subscribers, events)
		server.mu.Unlock()
	}()

	writeServerEvent(response, "ready", mapEvent{Revision: currentRevision})
	flusher.Flush()
	for {
		select {
		case <-request.Context().Done():
			return
		case event := <-events:
			name := "graph-changed"
			if event.Error != "" {
				name = "analysis-error"
			}
			writeServerEvent(response, name, event)
			flusher.Flush()
		}
	}
}

func writeServerEvent(writer io.Writer, name string, event mapEvent) {
	encoded, _ := json.Marshal(event)
	fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", name, encoded)
}

func (server *mapWebServer) rebuild() {
	snapshot, err := buildMapSnapshot(server.root, server.repo, server.worktree)
	server.mu.Lock()
	defer server.mu.Unlock()
	if err != nil {
		server.broadcastLocked(mapEvent{
			Revision: server.snapshot.Response.Revision,
			Error:    err.Error(),
		})
		return
	}
	snapshot.Response.Revision = server.snapshot.Response.Revision + 1
	server.snapshot = snapshot
	server.broadcastLocked(mapEvent{Revision: snapshot.Response.Revision})
}

func (server *mapWebServer) broadcastLocked(event mapEvent) {
	for subscriber := range server.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (server *mapWebServer) watch(context context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		server.publishWatchError(err)
		return
	}
	defer watcher.Close()
	if err := addRepositoryWatches(watcher, server.root); err != nil {
		server.publishWatchError(err)
		return
	}

	var timer *time.Timer
	var timerChannel <-chan time.Time
	resetTimer := func() {
		if timer == nil {
			timer = time.NewTimer(250 * time.Millisecond)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(250 * time.Millisecond)
		}
		timerChannel = timer.C
	}

	for {
		select {
		case <-context.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case event, open := <-watcher.Events:
			if !open {
				return
			}
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() && shouldWatchCreatedDirectory(server.root, event.Name) {
					_ = addRepositoryWatches(watcher, event.Name)
				}
			}
			resetTimer()
		case err, open := <-watcher.Errors:
			if open {
				server.publishWatchError(err)
			}
		case <-timerChannel:
			timerChannel = nil
			server.rebuild()
		}
	}
}

func shouldWatchCreatedDirectory(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	for _, part := range strings.FieldsFunc(relative, func(character rune) bool {
		return character == '/' || character == '\\'
	}) {
		name := strings.ToLower(part)
		if name == ".git" {
			return false
		}
		if _, excluded := excludedFolders[name]; excluded {
			return false
		}
	}
	return true
}

func addRepositoryWatches(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if path != root {
			if name == ".git" {
				if err := watcher.Add(path); err != nil {
					return err
				}
				return filepath.SkipDir
			}
			if _, excluded := excludedFolders[name]; excluded {
				return filepath.SkipDir
			}
		}
		return watcher.Add(path)
	})
}

func (server *mapWebServer) publishWatchError(err error) {
	server.mu.Lock()
	server.broadcastLocked(mapEvent{Revision: server.snapshot.Response.Revision, Error: err.Error()})
	server.mu.Unlock()
}

func sameOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Host == request.Host && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func openFileInVSCodeAtLine(path string, line int) error {
	return exec.Command("code", "--goto", fmt.Sprintf("%s:%d", path, max(1, line))).Start()
}

func openBrowser(address string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", address)
	case "darwin":
		command = exec.Command("open", address)
	default:
		command = exec.Command("xdg-open", address)
	}
	return command.Start()
}
