package helper

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"vibecode-dev-notify/internal/storage"
)

type App struct {
	ID                 string
	Name               string
	CWD                string
	Command            []string
	HealthMode         string
	HealthURL          string
	HealthCommand      []string
	StartupTimeoutSecs int
	SoundRepeatCount   int
	CodexEnvironment   string
	RunnerEnvironment  string

	Status          string
	RunnerConnected bool
	RestartVersion  int

	PromptCount         int
	PromptActiveStarted time.Time
	PromptPermissions   int
	PromptFinishedAt    time.Time
	PendingPromptSecs   float64
	TotalWaitSeconds    float64
	MinWaitSeconds      float64
	MaxWaitSeconds      float64
	LastWaitSeconds     float64
	LastRestartResult   string
	LastNotification    string
	LastEvent           string
	LastEventAt         time.Time
	RestartRequestedAt  time.Time
	RestartStartedAt    time.Time
	RestartFinishedAt   time.Time
	HealthyAt           time.Time
	FailureReason       string
	ActivePromptID      int64
	ActiveRestartID     int64
	RecentPrompts       []PromptSummary
	RecentEvents        []EventSummary
}

type PromptSummary struct {
	Index       int
	PromptSecs  float64
	WaitSecs    float64
	Result      string
	FinishedAt  time.Time
	Permissions int
}

type EventSummary struct {
	At     time.Time
	Type   string
	Detail string
}

type SoundEvent struct {
	Type     string `json:"type"`
	AppID    string `json:"app_id"`
	AppName  string `json:"app_name"`
	Repeat   int    `json:"repeat"`
	SoundURL string `json:"sound_url"`
}

type HealthCheckEvent struct {
	RequestID      string `json:"request_id"`
	AppID          string `json:"app_id"`
	AppName        string `json:"app_name"`
	URL            string `json:"url"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type HealthCheckResult struct {
	RequestID string `json:"request_id"`
	OK        bool   `json:"ok"`
	Detail    string `json:"detail"`
	Status    int    `json:"status"`
}

type Notifier struct {
	mu            sync.Mutex
	soundClients  map[chan SoundEvent]struct{}
	healthClients map[chan HealthCheckEvent]struct{}
}

type Store struct {
	mu       sync.Mutex
	apps     map[string]*App
	notifier *Notifier
	db       *storage.DB
}

type Server struct {
	store        *Store
	notifier     *Notifier
	tmpl         *template.Template
	healthMu     sync.Mutex
	healthChecks map[string]chan HealthCheckResult
}

func Serve(addr string) error {
	notifier := newNotifier()
	db, err := storage.Open(storage.DefaultPath())
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	server := &Server{
		notifier:     notifier,
		store:        newStore(notifier, db),
		healthChecks: map[string]chan HealthCheckResult{},
		tmpl: template.Must(template.New("dashboard").Funcs(template.FuncMap{
			"avg":      averageDuration,
			"command":  commandLine,
			"duration": formatDuration,
			"join":     strings.Join,
			"statusClass": func(value string) string {
				return strings.ReplaceAll(strings.ToLower(value), " ", "-")
			},
			"shortTime": func(value time.Time) string {
				if value.IsZero() {
					return "-"
				}
				return value.Format("15:04:05")
			},
		}).Parse(dashboardTemplate)),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleDashboard)
	mux.HandleFunc("/apps", server.handleApps)
	mux.HandleFunc("/apps/", server.handleApps)
	mux.HandleFunc("/events", server.handleEvents)
	mux.HandleFunc("/sound/", server.handleSound)
	mux.HandleFunc("/browser-health-result", server.handleBrowserHealthResult)

	log.Printf("vibecode-helper storage: %s", storage.DefaultPath())
	log.Printf("vibecode-helper listening on http://%s", addr)
	return http.ListenAndServe(addr, mux)
}

func newNotifier() *Notifier {
	return &Notifier{
		soundClients:  map[chan SoundEvent]struct{}{},
		healthClients: map[chan HealthCheckEvent]struct{}{},
	}
}

func newStore(notifier *Notifier, db *storage.DB) *Store {
	store := &Store{apps: map[string]*App{}, notifier: notifier, db: db}
	records, err := db.LoadApps()
	if err != nil {
		log.Printf("load apps: %v", err)
	}
	for _, record := range records {
		app := appFromRecord(record)
		store.loadAppHistory(app)
		store.apps[app.ID] = app
	}
	if len(store.apps) > 0 {
		return store
	}

	app := &App{
		ID:                 "my-app",
		Name:               "My App",
		CWD:                ".",
		Command:            []string{"sh", "-lc", "docker compose up --build"},
		HealthMode:         "url",
		HealthURL:          "http://localhost:3000",
		StartupTimeoutSecs: 60,
		SoundRepeatCount:   1,
		CodexEnvironment:   "host",
		RunnerEnvironment:  "host",
		Status:             "runner disconnected",
		LastEvent:          "created",
		LastEventAt:        time.Now(),
	}
	store.apps[app.ID] = app
	if err := db.UpsertApp(recordFromApp(app)); err != nil {
		log.Printf("seed app: %v", err)
	}
	return store
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	apps := s.store.listApps()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, apps); err != nil {
		log.Printf("render dashboard: %v", err)
	}
}

func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 1 && parts[0] == "apps" {
		s.handleCreateApp(w, r)
		return
	}
	if len(parts) < 2 || parts[0] != "apps" {
		http.NotFound(w, r)
		return
	}

	appID := parts[1]
	if len(parts) == 2 {
		http.NotFound(w, r)
		return
	}

	action := strings.Join(parts[2:], "/")
	switch action {
	case "hooks.json":
		s.handleHooks(w, r, appID)
	case "runner.sh":
		s.handleRunnerScript(w, r, appID)
	case "runner/config":
		s.handleRunnerConfig(w, r, appID)
	case "runner/next-action":
		s.handleRunnerNextAction(w, r, appID)
	case "runner/events":
		s.handleRunnerEvent(w, r, appID)
	case "browser-health":
		s.handleBrowserHealth(w, r, appID)
	case "update":
		s.handleUpdateApp(w, r, appID)
	case "delete":
		s.handleDeleteApp(w, r, appID)
	case "restart":
		s.handleRestartApp(w, r, appID)
	case "test_sound":
		s.handleTestSound(w, r, appID)
	case "prompt_started":
		s.handlePromptStarted(w, r, appID)
	case "permission_asked":
		s.handlePermissionAsked(w, r, appID)
	case "prompt_finished":
		s.handlePromptFinished(w, r, appID)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	soundEvents := s.notifier.SubscribeSounds()
	defer s.notifier.UnsubscribeSounds(soundEvents)
	healthEvents := s.notifier.SubscribeHealthChecks()
	defer s.notifier.UnsubscribeHealthChecks(healthEvents)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	_, _ = fmt.Fprintf(w, "event: status\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		case event := <-soundEvents:
			payload, err := json.Marshal(event)
			if err != nil {
				log.Printf("marshal sound event: %v", err)
				continue
			}
			_, _ = fmt.Fprintf(w, "event: sound\ndata: %s\n\n", payload)
			flusher.Flush()
		case event := <-healthEvents:
			payload, err := json.Marshal(event)
			if err != nil {
				log.Printf("marshal health event: %v", err)
				continue
			}
			_, _ = fmt.Fprintf(w, "event: health_check\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

func (s *Server) handleSound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	soundType := strings.TrimPrefix(r.URL.Path, "/sound/")
	sourceURL, ok := soundSources[soundType]
	if !ok {
		http.NotFound(w, r)
		return
	}

	path, err := cachedSoundPath(soundType, sourceURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "audio/ogg")
	http.ServeFile(w, r, path)
}

func (s *Server) handleCreateApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	app := appFromForm(r, "")
	if app.ID == "" {
		http.Error(w, "app name is required", http.StatusBadRequest)
		return
	}
	s.store.upsertApp(app)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleUpdateApp(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	app := appFromForm(r, appID)
	s.store.upsertApp(app)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleDeleteApp(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	s.store.deleteApp(appID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleRestartApp(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	s.store.requestRestart(appID, "manual restart requested")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleTestSound(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	soundType := r.FormValue("type")
	if _, ok := soundSources[soundType]; !ok {
		http.Error(w, "unknown sound type", http.StatusBadRequest)
		return
	}
	app, ok := s.store.getApp(appID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	s.notifier.Broadcast(SoundEvent{
		Type:     soundType,
		AppID:    app.ID,
		AppName:  app.Name,
		Repeat:   repeatOrDefault(app.SoundRepeatCount),
		SoundURL: "/sound/" + soundType,
	})
	if s.store.db != nil {
		if err := s.store.db.InsertNotification(app.ID, soundType, repeatOrDefault(app.SoundRepeatCount), time.Now()); err != nil {
			log.Printf("store test notification: %v", err)
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleHooks(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	base := publicBaseURL(r)
	hooks := map[string]any{
		"hooks": map[string]any{
			"UserPromptSubmit": []any{map[string]any{"hooks": []any{hookCommand(base, appID, "prompt_started", "Prompt started")}}},
			"PermissionRequest": []any{map[string]any{"hooks": []any{hookCommand(base, appID, "permission_asked", "Permission asked")}}},
			"Stop": []any{map[string]any{
				"matcher": "*",
				"hooks":   []any{hookCommand(base, appID, "prompt_finished", "Prompt finished")},
			}},
		},
	}
	writeJSON(w, hooks)
}

func hookCommand(base, appID, action, status string) map[string]any {
	url := fmt.Sprintf("%s/apps/%s/%s", strings.TrimRight(base, "/"), appID, action)
	return map[string]any{
		"type":          "command",
		"command":       fmt.Sprintf("curl -s -X POST %s >/dev/null", url),
		"timeout":       5,
		"statusMessage": status,
	}
}

func (s *Server) handleRunnerScript(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	helperURL := publicBaseURL(r)
	body := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

HELPER_URL="${HELPER_URL:-%s}"
exec vibecode-helper runner --app %s --helper-url "$HELPER_URL"
`, helperURL, shellQuote(appID))

	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	_, _ = w.Write([]byte(body))
}

func (s *Server) handleRunnerConfig(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	app, ok := s.store.getApp(appID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, map[string]any{
		"id":                     app.ID,
		"name":                   app.Name,
		"cwd":                    app.CWD,
		"command":                app.Command,
		"health_mode":            app.HealthMode,
		"health_url":             app.HealthURL,
		"health_command":         app.HealthCommand,
		"startup_timeout_seconds": app.StartupTimeoutSecs,
		"sound_repeat_count":     app.SoundRepeatCount,
	})
}

func (s *Server) handleRunnerNextAction(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	since, _ := strconv.Atoi(r.URL.Query().Get("since"))
	deadline := time.NewTimer(25 * time.Second)
	defer deadline.Stop()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		app, ok := s.store.getApp(appID)
		if !ok {
			http.NotFound(w, r)
			return
		}
		if app.RestartVersion > since {
			writeJSON(w, map[string]any{"action": "restart", "version": app.RestartVersion})
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-deadline.C:
			writeJSON(w, map[string]any{"action": "wait", "version": since})
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) handleRunnerEvent(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var payload struct {
		Type     string `json:"type"`
		Detail   string `json:"detail"`
		ExitCode int    `json:"exit_code"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	if payload.ExitCode != 0 {
		payload.Detail = strings.TrimSpace(fmt.Sprintf("%s exit_code=%d", payload.Detail, payload.ExitCode))
	}
	s.store.recordRunnerEvent(appID, payload.Type, payload.Detail)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleBrowserHealth(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	app, ok := s.store.getApp(appID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if app.HealthURL == "" {
		writeJSON(w, HealthCheckResult{OK: true, Detail: "no health URL configured"})
		return
	}

	timeoutSeconds, _ := strconv.Atoi(r.URL.Query().Get("timeout"))
	if timeoutSeconds <= 0 {
		timeoutSeconds = 3
	}

	requestID := fmt.Sprintf("%s-%d", appID, time.Now().UnixNano())
	resultCh := make(chan HealthCheckResult, 1)
	s.healthMu.Lock()
	s.healthChecks[requestID] = resultCh
	s.healthMu.Unlock()
	defer func() {
		s.healthMu.Lock()
		delete(s.healthChecks, requestID)
		s.healthMu.Unlock()
	}()

	s.notifier.BroadcastHealthCheck(HealthCheckEvent{
		RequestID:      requestID,
		AppID:          app.ID,
		AppName:        app.Name,
		URL:            app.HealthURL,
		TimeoutSeconds: timeoutSeconds,
	})

	select {
	case result := <-resultCh:
		writeJSON(w, result)
	case <-time.After(time.Duration(timeoutSeconds+2) * time.Second):
		writeJSON(w, HealthCheckResult{
			RequestID: requestID,
			OK:        false,
			Detail:    fmt.Sprintf("browser check timed out after %ds", timeoutSeconds+2),
		})
	case <-r.Context().Done():
		return
	}
}

func (s *Server) handleBrowserHealthResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var result HealthCheckResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, "invalid result", http.StatusBadRequest)
		return
	}
	if result.RequestID == "" {
		http.Error(w, "request_id is required", http.StatusBadRequest)
		return
	}

	s.healthMu.Lock()
	resultCh, ok := s.healthChecks[result.RequestID]
	s.healthMu.Unlock()
	if ok {
		select {
		case resultCh <- result:
		default:
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handlePromptStarted(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	s.store.promptStarted(appID)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handlePermissionAsked(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	s.store.recordEvent(appID, "permission_asked", "permission sound requested")
	// Browser sound delivery will be implemented by the sound bridge slice.
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handlePromptFinished(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	s.store.promptFinished(appID)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Store) listApps() []*App {
	s.mu.Lock()
	defer s.mu.Unlock()

	apps := make([]*App, 0, len(s.apps))
	for _, app := range s.apps {
		apps = append(apps, cloneApp(app))
	}
	return apps
}

func (s *Store) getApp(appID string) (*App, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.apps[appID]
	if !ok {
		return nil, false
	}
	return cloneApp(app), true
}

func (s *Store) promptStarted(appID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app := s.ensureApp(appID)
	now := time.Now()
	app.PromptActiveStarted = now
	app.PromptPermissions = 0
	app.PendingPromptSecs = 0
	app.Status = "prompt active"
	app.LastEvent = "prompt_started"
	app.LastEventAt = now
	if s.db != nil {
		promptID, err := s.db.InsertPromptStarted(app.ID, app.PromptCount+1, now)
		if err != nil {
			log.Printf("store prompt start: %v", err)
		} else {
			app.ActivePromptID = promptID
		}
	}
	prependEvent(app, "prompt_started", "")
	s.persistEventLocked(app, "prompt_started", "", now)
}

func (s *Store) promptFinished(appID string) {
	now := time.Now()
	s.mu.Lock()
	app := s.ensureApp(appID)
	app.PromptFinishedAt = now
	app.PromptCount++
	if !app.PromptActiveStarted.IsZero() {
		app.PendingPromptSecs = now.Sub(app.PromptActiveStarted).Seconds()
	}
	s.requestRestartLocked(app, "restart requested")
	s.mu.Unlock()
}

func (s *Store) requestRestart(appID, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app := s.ensureApp(appID)
	s.requestRestartLocked(app, detail)
}

func (s *Store) requestRestartLocked(app *App, detail string) {
	now := time.Now()
	app.RestartVersion++
	app.Status = "restart requested"
	app.RestartRequestedAt = now
	app.LastEvent = "prompt_finished"
	if detail != "restart requested" {
		app.LastEvent = "restart_requested"
	}
	app.LastEventAt = now
	prependEvent(app, app.LastEvent, detail)
	s.persistEventLocked(app, app.LastEvent, detail, now)
	if s.db != nil {
		restartID, err := s.db.InsertRestartRequested(app.ID, app.ActivePromptID, now)
		if err != nil {
			log.Printf("store restart request: %v", err)
		} else {
			app.ActiveRestartID = restartID
		}
	}
}

func (s *Store) recordRunnerEvent(appID, eventType, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app := s.ensureApp(appID)
	now := time.Now()
	app.LastEvent = eventType
	app.LastEventAt = now
	prependEvent(app, eventType, detail)

	switch eventType {
	case "runner_connected":
		app.RunnerConnected = true
		app.Status = "running"
	case "runner_disconnected":
		app.RunnerConnected = false
		app.Status = "runner disconnected"
	case "command_started":
		app.Status = "running"
	case "restart_started":
		app.Status = "restarting"
		app.RestartStartedAt = now
		app.FailureReason = ""
		if s.db != nil {
			if err := s.db.MarkRestartStarted(app.ActiveRestartID, now); err != nil {
				log.Printf("store restart start: %v", err)
			}
		}
	case "restart_finished":
		app.Status = "checking health"
		app.RestartFinishedAt = now
	case "health_ok":
		app.Status = "healthy"
		app.LastRestartResult = "success"
		app.LastNotification = "restarted successfully"
		app.HealthyAt = now
		app.FailureReason = ""
		s.finishPromptLocked(app, "success", now)
		finalizePrompt(app, "success", now)
		s.finishRestartLocked(app, "success", "")
		s.notifyLocked(app, "success")
	case "health_failed", "restart_failed":
		app.Status = "failed"
		app.LastRestartResult = "failure"
		app.LastNotification = "failed to become healthy"
		app.FailureReason = detail
		s.finishPromptLocked(app, "failure", now)
		finalizePrompt(app, "failure", now)
		s.finishRestartLocked(app, "failure", detail)
		s.notifyLocked(app, "failure")
	case "command_exited":
		app.Status = "failed"
	case "command_failed":
		app.Status = "failed"
		app.LastRestartResult = "failure"
	}
	s.persistEventLocked(app, eventType, detail, now)
}

func (s *Store) recordEvent(appID, eventType, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app := s.ensureApp(appID)
	app.LastEvent = eventType
	app.LastEventAt = time.Now()
	if eventType == "permission_asked" {
		app.Status = "permission asked"
		app.LastNotification = "needs permission"
		if !app.PromptActiveStarted.IsZero() {
			app.PromptPermissions++
		}
		s.notifyLocked(app, "permission")
	}
	prependEvent(app, eventType, detail)
	s.persistEventLocked(app, eventType, detail, app.LastEventAt)
}

func (s *Store) notifyLocked(app *App, soundType string) {
	if s.notifier == nil {
		return
	}
	s.notifier.Broadcast(SoundEvent{
		Type:     soundType,
		AppID:    app.ID,
		AppName:  app.Name,
		Repeat:   repeatOrDefault(app.SoundRepeatCount),
		SoundURL: "/sound/" + soundType,
	})
	if s.db != nil {
		if err := s.db.InsertNotification(app.ID, soundType, repeatOrDefault(app.SoundRepeatCount), time.Now()); err != nil {
			log.Printf("store notification: %v", err)
		}
	}
}

func finalizePrompt(app *App, result string, finishedAt time.Time) {
	if app.PromptActiveStarted.IsZero() {
		return
	}

	app.LastWaitSeconds = finishedAt.Sub(app.PromptActiveStarted).Seconds()
	app.TotalWaitSeconds += app.LastWaitSeconds
	if app.MinWaitSeconds == 0 || app.LastWaitSeconds < app.MinWaitSeconds {
		app.MinWaitSeconds = app.LastWaitSeconds
	}
	if app.LastWaitSeconds > app.MaxWaitSeconds {
		app.MaxWaitSeconds = app.LastWaitSeconds
	}
	app.RecentPrompts = append([]PromptSummary{{
		Index:       app.PromptCount,
		PromptSecs:  app.PendingPromptSecs,
		WaitSecs:    app.LastWaitSeconds,
		Result:      result,
		FinishedAt:  finishedAt,
		Permissions: app.PromptPermissions,
	}}, app.RecentPrompts...)
	if len(app.RecentPrompts) > 10 {
		app.RecentPrompts = app.RecentPrompts[:10]
	}

	app.PromptActiveStarted = time.Time{}
	app.PromptPermissions = 0
	app.PendingPromptSecs = 0
	app.ActivePromptID = 0
}

func (s *Store) finishPromptLocked(app *App, result string, finishedAt time.Time) {
	if s.db == nil || app.ActivePromptID == 0 || app.PromptActiveStarted.IsZero() {
		return
	}
	waitSecs := finishedAt.Sub(app.PromptActiveStarted).Seconds()
	if err := s.db.FinishPrompt(app.ActivePromptID, finishedAt, app.PendingPromptSecs, waitSecs, app.PromptPermissions, result); err != nil {
		log.Printf("store prompt finish: %v", err)
	}
}

func (s *Store) finishRestartLocked(app *App, result, failureReason string) {
	if s.db == nil {
		return
	}
	healthyAt := time.Time{}
	if result == "success" {
		healthyAt = app.HealthyAt
	}
	if err := s.db.FinishRestart(app.ActiveRestartID, app.RestartFinishedAt, healthyAt, result, failureReason); err != nil {
		log.Printf("store restart finish: %v", err)
	}
	app.ActiveRestartID = 0
}

func (s *Store) ensureApp(appID string) *App {
	if app, ok := s.apps[appID]; ok {
		return app
	}

	app := &App{
		ID:                 appID,
		Name:               appID,
		CWD:                ".",
		Command:            []string{"sh", "-lc", "echo configure command for " + appID + "; sleep 3600"},
		HealthMode:         "url",
		HealthURL:          "http://localhost:3000",
		StartupTimeoutSecs: 60,
		SoundRepeatCount:   1,
		CodexEnvironment:   "host",
		RunnerEnvironment:  "host",
		Status:             "runner disconnected",
		LastEvent:          "created",
		LastEventAt:        time.Now(),
	}
	s.apps[appID] = app
	if s.db != nil {
		if err := s.db.UpsertApp(recordFromApp(app)); err != nil {
			log.Printf("store new app: %v", err)
		}
	}
	return app
}

func (s *Store) upsertApp(next *App) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if current, ok := s.apps[next.ID]; ok {
		next.Status = current.Status
		next.RunnerConnected = current.RunnerConnected
		next.RestartVersion = current.RestartVersion
		next.PromptCount = current.PromptCount
		next.PromptActiveStarted = current.PromptActiveStarted
		next.PromptPermissions = current.PromptPermissions
		next.PromptFinishedAt = current.PromptFinishedAt
		next.PendingPromptSecs = current.PendingPromptSecs
		next.TotalWaitSeconds = current.TotalWaitSeconds
		next.MinWaitSeconds = current.MinWaitSeconds
		next.MaxWaitSeconds = current.MaxWaitSeconds
		next.LastWaitSeconds = current.LastWaitSeconds
		next.LastRestartResult = current.LastRestartResult
		next.LastNotification = current.LastNotification
		next.LastEvent = "app_updated"
		next.LastEventAt = now
		next.RestartRequestedAt = current.RestartRequestedAt
		next.RestartStartedAt = current.RestartStartedAt
		next.RestartFinishedAt = current.RestartFinishedAt
		next.HealthyAt = current.HealthyAt
		next.FailureReason = current.FailureReason
		next.RecentPrompts = append([]PromptSummary(nil), current.RecentPrompts...)
		next.RecentEvents = append([]EventSummary(nil), current.RecentEvents...)
	} else {
		next.Status = "runner disconnected"
		next.LastEvent = "app_created"
		next.LastEventAt = now
	}
	prependEvent(next, next.LastEvent, "")
	if s.db != nil {
		if err := s.db.UpsertApp(recordFromApp(next)); err != nil {
			log.Printf("store app: %v", err)
		}
	}
	s.apps[next.ID] = next
	s.persistEventLocked(next, next.LastEvent, "", next.LastEventAt)
}

func (s *Store) deleteApp(appID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.apps, appID)
	if s.db != nil {
		if err := s.db.DeleteApp(appID); err != nil {
			log.Printf("delete app: %v", err)
		}
	}
}

func (s *Store) loadAppHistory(app *App) {
	if s.db == nil {
		return
	}
	stats, err := s.db.LoadPromptStats(app.ID)
	if err != nil {
		log.Printf("load prompt stats: %v", err)
	} else {
		app.PromptCount = stats.Count
		app.TotalWaitSeconds = stats.TotalWait
		app.MinWaitSeconds = stats.MinWait
		app.MaxWaitSeconds = stats.MaxWait
		app.LastWaitSeconds = stats.LastWait
		app.LastRestartResult = stats.LastResult
		for _, record := range stats.Recent {
			app.RecentPrompts = append(app.RecentPrompts, PromptSummary{
				Index:       record.Index,
				PromptSecs:  record.PromptSecs,
				WaitSecs:    record.WaitSecs,
				Result:      record.Result,
				FinishedAt:  record.FinishedAt,
				Permissions: record.Permissions,
			})
		}
	}

	events, err := s.db.LoadEvents(app.ID, 20)
	if err != nil {
		log.Printf("load events: %v", err)
		return
	}
	for _, record := range events {
		app.RecentEvents = append(app.RecentEvents, EventSummary{
			At:     record.At,
			Type:   record.Type,
			Detail: record.Detail,
		})
	}
	if len(events) > 0 {
		app.LastEvent = events[0].Type
		app.LastEventAt = events[0].At
	}
}

func prependEvent(app *App, eventType, detail string) {
	app.RecentEvents = append([]EventSummary{{At: time.Now(), Type: eventType, Detail: detail}}, app.RecentEvents...)
	if len(app.RecentEvents) > 20 {
		app.RecentEvents = app.RecentEvents[:20]
	}
}

func (s *Store) persistEventLocked(app *App, eventType, detail string, at time.Time) {
	if s.db == nil {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	if err := s.db.InsertEvent(app.ID, eventType, detail, at); err != nil {
		log.Printf("store event: %v", err)
	}
}

func cloneApp(app *App) *App {
	clone := *app
	clone.Command = append([]string(nil), app.Command...)
	clone.HealthCommand = append([]string(nil), app.HealthCommand...)
	clone.RecentPrompts = append([]PromptSummary(nil), app.RecentPrompts...)
	clone.RecentEvents = append([]EventSummary(nil), app.RecentEvents...)
	return &clone
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write json: %v", err)
	}
}

func (n *Notifier) SubscribeSounds() chan SoundEvent {
	events := make(chan SoundEvent, 16)
	n.mu.Lock()
	n.soundClients[events] = struct{}{}
	n.mu.Unlock()
	return events
}

func (n *Notifier) UnsubscribeSounds(events chan SoundEvent) {
	n.mu.Lock()
	delete(n.soundClients, events)
	close(events)
	n.mu.Unlock()
}

func (n *Notifier) Broadcast(event SoundEvent) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for events := range n.soundClients {
		select {
		case events <- event:
		default:
		}
	}
}

func (n *Notifier) SubscribeHealthChecks() chan HealthCheckEvent {
	events := make(chan HealthCheckEvent, 16)
	n.mu.Lock()
	n.healthClients[events] = struct{}{}
	n.mu.Unlock()
	return events
}

func (n *Notifier) UnsubscribeHealthChecks(events chan HealthCheckEvent) {
	n.mu.Lock()
	delete(n.healthClients, events)
	close(events)
	n.mu.Unlock()
}

func (n *Notifier) BroadcastHealthCheck(event HealthCheckEvent) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for events := range n.healthClients {
		select {
		case events <- event:
		default:
		}
	}
}

var soundSources = map[string]string{
	"success":    "https://actions.google.com/sounds/v1/cartoon/wood_plank_flicks.ogg",
	"failure":    "https://actions.google.com/sounds/v1/cartoon/cartoon_boing.ogg",
	"permission": "https://actions.google.com/sounds/v1/cartoon/woodpecker.ogg",
}

func cachedSoundPath(soundType, sourceURL string) (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(cacheRoot, "vibecode-helper", "sounds")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	path := filepath.Join(dir, soundType+".ogg")
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		return path, nil
	}

	response, err := http.Get(sourceURL)
	if err != nil {
		return "", fmt.Errorf("download sound: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download sound: %s", response.Status)
	}

	tempPath := path + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(file, response.Body); err != nil {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	return path, nil
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, POST")
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func publicBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func formatDuration(seconds float64) string {
	if seconds <= 0 {
		return "-"
	}
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	}
	minutes := int(seconds) / 60
	remaining := int(seconds) % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %02ds", minutes, remaining)
	}
	hours := minutes / 60
	minutes = minutes % 60
	return fmt.Sprintf("%dh %02dm", hours, minutes)
}

func averageDuration(total float64, count int) float64 {
	if count <= 0 {
		return 0
	}
	return total / float64(count)
}

func commandLine(command []string, sep string) string {
	if len(command) >= 3 && command[0] == "sh" && command[1] == "-lc" {
		return command[2]
	}
	return strings.Join(command, sep)
}

func appFromForm(r *http.Request, currentID string) *App {
	name := strings.TrimSpace(r.FormValue("name"))
	appID := currentID
	if appID == "" {
		appID = appIDFromName(name)
	}

	commandValue := strings.TrimSpace(r.FormValue("command"))
	command := []string{"sh", "-lc", commandValue}
	if commandValue == "" {
		command = []string{"sh", "-lc", "echo configure command for " + appID + "; sleep 3600"}
	}

	healthCommandValue := strings.TrimSpace(r.FormValue("health_command"))
	var healthCommand []string
	if healthCommandValue != "" {
		healthCommand = []string{"sh", "-lc", healthCommandValue}
	}
	healthMode := strings.TrimSpace(r.FormValue("health_mode"))
	if healthMode == "" {
		healthMode = "url"
	}

	timeout, _ := strconv.Atoi(r.FormValue("startup_timeout_seconds"))
	if timeout <= 0 {
		timeout = 60
	}

	repeats, _ := strconv.Atoi(r.FormValue("sound_repeat_count"))
	if repeats <= 0 {
		repeats = 1
	}

	if name == "" {
		name = appID
	}

	return &App{
		ID:                 appID,
		Name:               name,
		CWD:                valueOrDefault(r.FormValue("cwd"), "."),
		Command:            command,
		HealthMode:         healthMode,
		HealthURL:          strings.TrimSpace(r.FormValue("health_url")),
		HealthCommand:      healthCommand,
		StartupTimeoutSecs: timeout,
		SoundRepeatCount:   repeats,
		CodexEnvironment:   valueOrDefault(r.FormValue("codex_environment"), "host"),
		RunnerEnvironment:  valueOrDefault(r.FormValue("runner_environment"), "host"),
	}
}

func appFromRecord(record storage.AppRecord) *App {
	return &App{
		ID:                 record.ID,
		Name:               record.Name,
		CWD:                record.CWD,
		Command:            record.Command,
		HealthMode:         valueOrDefault(record.HealthMode, "url"),
		HealthURL:          record.HealthURL,
		HealthCommand:      record.HealthCommand,
		StartupTimeoutSecs: record.StartupTimeoutSecs,
		SoundRepeatCount:   repeatOrDefault(record.SoundRepeatCount),
		CodexEnvironment:   valueOrDefault(record.CodexEnvironment, "host"),
		RunnerEnvironment:  valueOrDefault(record.RunnerEnvironment, "host"),
		Status:             "runner disconnected",
		LastEvent:          "loaded",
		LastEventAt:        time.Now(),
	}
}

func recordFromApp(app *App) storage.AppRecord {
	return storage.AppRecord{
		ID:                 app.ID,
		Name:               app.Name,
		CWD:                app.CWD,
		Command:            app.Command,
		HealthMode:         app.HealthMode,
		HealthURL:          app.HealthURL,
		HealthCommand:      app.HealthCommand,
		StartupTimeoutSecs: app.StartupTimeoutSecs,
		SoundRepeatCount:   app.SoundRepeatCount,
		CodexEnvironment:   app.CodexEnvironment,
		RunnerEnvironment:  app.RunnerEnvironment,
	}
}

func appIDFromName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var builder strings.Builder
	lastDash := false
	for _, r := range name {
		isAlpha := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if isAlpha || isDigit {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	id := strings.Trim(builder.String(), "-")
	if id == "" {
		return "app"
	}
	return id
}

func valueOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func repeatOrDefault(repeat int) int {
	if repeat <= 0 {
		return 1
	}
	return repeat
}
