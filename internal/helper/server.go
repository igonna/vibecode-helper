package helper

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"vibecode-dev-notify/internal/protocol"
	"vibecode-dev-notify/internal/storage"
)

type App struct {
	ID                 string
	Name               string
	CWD                string
	CommandEnabled     bool
	Command            []string
	HealthMode         string
	HealthURL          string
	HealthCommand      []string
	StartupTimeoutSecs int
	CodexEnvironment   string
	RunnerEnvironment  string

	Status          string
	RunnerConnected bool
	LastRunnerSeen  time.Time
	RestartVersion  int
	PendingAction   string

	PromptCount         int
	PromptActiveStarted time.Time
	PromptPermissions   int
	PromptFinishedAt    time.Time
	PendingPromptSecs   float64
	TotalWaitSeconds    float64
	TotalPromptSeconds  float64
	TotalRestartSeconds float64
	TodayPromptCount    int
	TodayWaitSeconds    float64
	MinWaitSeconds      float64
	MaxWaitSeconds      float64
	LastWaitSeconds     float64
	LastPromptSeconds   float64
	LastRestartSeconds  float64
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

const (
	runnerStaleAfter       = 35 * time.Second
	restartPickupTimeout   = 75 * time.Second
	promptStartDedupeAfter = 5 * time.Second
	activePromptRestoreAge = 6 * time.Hour
	healthDisabledEventMsg = "health check disabled"
)

type DashboardData struct {
	Apps    []*App
	BaseURL string
	Global  storage.GlobalStats
	Today   storage.GlobalStats
}

type PromptSummary struct {
	Index       int
	PromptSecs  float64
	WaitSecs    float64
	RestartSecs float64
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
			"copyIcon": copyIcon,
			"depthClass": depthClass,
			"depthHint":  depthHint,
			"duration": formatDuration,
			"hooksConfig": func(base string, app *App) string {
				return hooksConfigJSON(base, app)
			},
			"hooksFeatureConfig": hooksFeatureConfig,
			"join":     strings.Join,
			"rotateCcwIcon": rotateCcwIcon,
			"runnerCommand": func(base string, app *App) string {
				return runnerCommand(base, app)
			},
			"friendlyProblem": friendlyProblem,
			"workSummary":     workSummary,
			"displayStatus":      displayStatus,
			"displayStatusClass": displayStatusClass,
			"statusClass":        statusClass,
			"shortTime": func(value time.Time) string {
				if value.IsZero() {
					return "-"
				}
				return value.Format("15:04")
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

func displayStatus(app *App) string {
	if app == nil {
		return ""
	}

	needsRunner := app.CommandEnabled || app.HealthMode != "none"
	if app.Status == "failed" {
		if app.LastNotification == "runner disconnected" || strings.Contains(strings.ToLower(app.FailureReason), "runner") {
			if needsRunner {
				return "runner disconnected"
			}
			return "not started"
		}
		return "needs attention"
	}
	if app.Status == "prompt active" || !app.PromptActiveStarted.IsZero() {
		return "prompt active"
	}
	if app.Status == "permission asked" {
		return "permission asked"
	}
	if app.Status == "runner update required" {
		return "runner update required"
	}
	if app.Status == "paused" || app.Status == "stopped" || app.Status == "stop requested" || app.Status == "stopping" {
		return "paused"
	}

	if app.Status == "restarting" || app.Status == "checking health" {
		if needsRunner && !app.RunnerConnected {
			return "runner disconnected"
		}
		return "app loading"
	}
	if needsRunner && !app.RunnerConnected {
		return "runner disconnected"
	}

	if app.Status == "ready" || app.Status == "healthy" || app.Status == "running" {
		return "app ready"
	}
	if app.RunnerConnected {
		return "runner connected"
	}
	if needsRunner {
		return "runner disconnected"
	}
	return "not started"
}

func displayStatusClass(app *App) string {
	return statusClass(displayStatus(app))
}

func statusClass(value string) string {
	return strings.ReplaceAll(strings.ToLower(value), " ", "-")
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
		HealthMode:         "none",
		HealthURL:          "http://localhost:3000",
		StartupTimeoutSecs: 60,
		CodexEnvironment:   "docker",
		RunnerEnvironment:  "docker",
		Status:             "not started",
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
	w.Header().Set("Cache-Control", "no-store")
	data := DashboardData{Apps: apps, BaseURL: publicBaseURL(r), Global: s.store.globalStats(), Today: s.store.todayGlobalStats()}
	if err := s.tmpl.Execute(w, data); err != nil {
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
	case "stop":
		s.handleStopApp(w, r, appID)
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
	if err := parseAnyForm(r); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	app := appFromForm(r, "", nil)
	if app.ID == "" {
		http.Error(w, "app name is required", http.StatusBadRequest)
		return
	}
	if err := s.store.upsertApp("", app); err != nil {
		http.Error(w, "could not save app", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleUpdateApp(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if err := parseAnyForm(r); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	current, ok := s.store.currentApp(appID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	app := appFromForm(r, appID, current)
	if err := s.store.upsertApp(appID, app); err != nil {
		http.Error(w, "could not save app", http.StatusInternalServerError)
		return
	}
	if r.Header.Get("X-Requested-With") == "vibecode-autosave" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func parseAnyForm(r *http.Request) error {
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		return r.ParseMultipartForm(1 << 20)
	}
	return r.ParseForm()
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

func (s *Server) handleStopApp(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	s.store.requestStop(appID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleHooks(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	app, ok := s.store.getApp(appID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, hooksConfig(publicBaseURL(r), app))
}

func hooksConfig(base string, app *App) map[string]any {
	return map[string]any{
		"hooks": map[string]any{
			"UserPromptSubmit": []any{map[string]any{"hooks": []any{hookCommand(base, app, "prompt_started", "Prompt started")}}},
			"PermissionRequest": []any{map[string]any{"hooks": []any{hookCommand(base, app, "permission_asked", "Permission asked")}}},
			"Stop": []any{map[string]any{
				"matcher": "*",
				"hooks":   []any{hookCommand(base, app, "prompt_finished", "Prompt finished")},
			}},
		},
	}
}

func hooksConfigJSON(base string, app *App) string {
	payload, err := json.MarshalIndent(hooksConfig(base, app), "", "  ")
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func hooksFeatureConfig() string {
	return "[features]\nhooks = true"
}

func runnerCommand(base string, app *App) string {
	helperURL := runnerHelperURL(base)
	return fmt.Sprintf("appdir=$(pwd) && tmpdir=$(mktemp -d) && trap 'rm -rf \"$tmpdir\"' EXIT && docker cp vibecode-dev-notify-helper-1:/usr/local/bin/vibecode-helper \"$tmpdir/vibecode-runner\" >/dev/null && chmod +x \"$tmpdir/vibecode-runner\" && (clear 2>/dev/null || printf '\\033c') && \"$tmpdir/vibecode-runner\" runner --app %s --helper-url %s --base-dir \"$appdir\"", shellQuote(app.ID), shellQuote(helperURL))
}

func runnerHelperURL(base string) string {
	return "docker://vibecode-dev-notify-helper-1"
}

func friendlyProblem(detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(detail, "cwd: ") && strings.Contains(detail, "; command: "):
		return "command: " + strings.TrimSpace(strings.SplitN(detail, "; command: ", 2)[1])
	case strings.Contains(detail, "no configuration file provided"):
		return "The command could not find a Docker Compose file. Run the runner from your app folder or use an explicit compose file path."
	case strings.HasPrefix(detail, "runner version "):
		return "Runner is old. Copy and run the Runner command again."
	case strings.Contains(detail, "browser helper page is not open or not connected"):
		return "Open the Vibecode Helper browser page and keep it open so it can check the app URL."
	case strings.Contains(detail, "startup timeout reached after"):
		return detail
	case strings.Contains(detail, "can't open file") && strings.Contains(detail, "No such file or directory"):
		return "The command could not find a file. Run the runner from your app folder or use an explicit file path."
	case strings.Contains(detail, "command exited before health check passed"):
		return "The app command stopped before the app became ready. Check the command and run the runner from your app folder."
	case strings.Contains(detail, "startup timeout reached"):
		return "The app did not become ready before the startup timeout."
	case strings.Contains(detail, "permission denied"):
		return "The command was blocked by permissions. Check the runner environment and Docker access."
	default:
		return detail
	}
}

func copyIcon() template.HTML {
	return template.HTML(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"></rect><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"></path></svg>`)
}

func rotateCcwIcon() template.HTML {
	return template.HTML(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"></path><path d="M3 3v5h5"></path></svg>`)
}

func hookCommand(base string, app *App, action, status string) map[string]any {
	endpoint := fmt.Sprintf("http://127.0.0.1:8766/apps/%s/%s", url.PathEscape(app.ID), action)
	command := fmt.Sprintf("docker exec vibecode-dev-notify-helper-1 wget -qO- -T 5 --header %s --post-data %s %s >/dev/null", shellQuote("Content-Type: application/json"), shellQuote("{}"), shellQuote(endpoint))
	return map[string]any{
		"type":          "command",
		"command":       command,
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
		"command_enabled":        app.CommandEnabled,
		"command":                app.Command,
		"health_mode":            app.HealthMode,
		"health_url":             app.HealthURL,
		"health_command":         app.HealthCommand,
		"startup_timeout_seconds": app.StartupTimeoutSecs,
		"required_runner_version": protocol.RunnerVersion,
		"restart_version":         app.RestartVersion,
	})
}

func (s *Server) handleRunnerNextAction(w http.ResponseWriter, r *http.Request, appID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	since, _ := strconv.Atoi(r.URL.Query().Get("since"))
	runnerVersion, _ := strconv.Atoi(r.URL.Query().Get("runner_version"))
	if runnerVersion < protocol.RunnerVersion {
		if !s.store.recordRunnerUpdateRequired(appID, runnerVersion) {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{"action": "update_required", "version": since})
		return
	}

	if !s.store.recordRunnerHeartbeat(appID) {
		http.NotFound(w, r)
		return
	}

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
			action := app.PendingAction
			if action == "" {
				action = "restart"
			}
			s.store.recordRunnerActionPickedUp(appID, action)
			writeJSON(w, map[string]any{"action": action, "version": app.RestartVersion})
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
		Type          string `json:"type"`
		Detail        string `json:"detail"`
		ExitCode      int    `json:"exit_code"`
		RunnerVersion int    `json:"runner_version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	if payload.RunnerVersion < protocol.RunnerVersion {
		s.store.recordRunnerUpdateRequired(appID, payload.RunnerVersion)
		w.WriteHeader(http.StatusAccepted)
		return
	}
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
	if s.notifier.HealthCheckClientCount() == 0 {
		writeJSON(w, HealthCheckResult{
			OK:     false,
			Detail: "browser helper page is not open or not connected",
		})
		return
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

	now := time.Now()
	apps := make([]*App, 0, len(s.apps))
	for _, app := range s.apps {
		s.markRunnerStaleIfNeededLocked(app, now)
		s.refreshTodayStatsLocked(app, now)
		apps = append(apps, cloneApp(app))
	}
	sort.Slice(apps, func(i, j int) bool {
		left := strings.ToLower(apps[i].Name)
		right := strings.ToLower(apps[j].Name)
		if left == right {
			return apps[i].ID < apps[j].ID
		}
		return left < right
	})
	return apps
}

func (s *Store) refreshTodayStatsLocked(app *App, now time.Time) {
	if s.db == nil {
		return
	}
	stats, err := s.db.LoadTodayPromptStats(app.ID, now)
	if err != nil {
		log.Printf("load today prompt stats: %v", err)
		return
	}
	app.TodayPromptCount = stats.Count
	app.TodayWaitSeconds = stats.TotalWait
}

func (s *Store) globalStats() storage.GlobalStats {
	if s.db == nil {
		return storage.GlobalStats{}
	}
	stats, err := s.db.LoadGlobalStats()
	if err != nil {
		log.Printf("load global stats: %v", err)
		return storage.GlobalStats{}
	}
	return stats
}

func (s *Store) todayGlobalStats() storage.GlobalStats {
	if s.db == nil {
		return storage.GlobalStats{}
	}
	stats, err := s.db.LoadTodayGlobalStats(time.Now())
	if err != nil {
		log.Printf("load today global stats: %v", err)
		return storage.GlobalStats{}
	}
	return stats
}

func (s *Store) getApp(appID string) (*App, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.apps[appID]
	if !ok {
		return nil, false
	}
	s.markRunnerStaleIfNeededLocked(app, time.Now())
	s.markRestartPickupTimedOutLocked(app, time.Now())
	return cloneApp(app), true
}

func (s *Store) currentApp(appID string) (*App, bool) {
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
	if !app.PromptActiveStarted.IsZero() && app.ActivePromptID != 0 && now.Sub(app.PromptActiveStarted) <= promptStartDedupeAfter {
		return
	}
	app.PromptActiveStarted = now
	app.PromptPermissions = 0
	app.PendingPromptSecs = 0
	app.Status = "prompt active"
	app.LastEvent = "prompt_started"
	app.LastEventAt = now
	if s.db != nil && app.ActiveRestartID == 0 {
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
	if !app.CommandEnabled && app.HealthMode == "none" {
		app.Status = "ready"
		app.LastRestartResult = "success"
		app.LastNotification = "prompt finished"
		app.HealthyAt = now
		app.FailureReason = ""
		app.LastEvent = "prompt_finished"
		app.LastEventAt = now
		prependEvent(app, "prompt_finished", "command and health check disabled")
		s.persistEventLocked(app, "prompt_finished", "command and health check disabled", now)
		s.finishPromptLocked(app, "success", now)
		finalizePrompt(app, "success", now)
		s.notifyLocked(app, "success")
		s.mu.Unlock()
		return
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
	if !app.CommandEnabled && app.HealthMode == "none" {
		app.Status = "ready"
		app.LastRestartResult = "success"
		app.LastNotification = "restart skipped"
		app.HealthyAt = now
		app.FailureReason = ""
		app.LastEvent = "restart_skipped"
		if detail == "restart requested" {
			app.LastEvent = "prompt_finished"
		}
		app.LastEventAt = now
		prependEvent(app, app.LastEvent, "command and health check disabled")
		s.persistEventLocked(app, app.LastEvent, "command and health check disabled", now)
		s.finishPromptLocked(app, "success", now)
		finalizePrompt(app, "success", now)
		s.notifyLocked(app, "success")
		return
	}
	if !s.runnerActiveLocked(app, now) {
		s.failRestartWithoutRunnerLocked(app, now)
		return
	}

	app.RestartVersion++
	app.PendingAction = "restart"
	app.Status = "restart requested"
	app.RestartRequestedAt = now
	app.LastEvent = "prompt_finished"
	if detail != "restart requested" {
		app.LastEvent = "restart_requested"
	}
	app.LastEventAt = now
	prependEvent(app, app.LastEvent, detail)
	s.persistEventLocked(app, app.LastEvent, detail, now)
	if s.db != nil && app.ActiveRestartID == 0 {
		restartID, err := s.db.InsertRestartRequested(app.ID, app.ActivePromptID, now)
		if err != nil {
			log.Printf("store restart request: %v", err)
		} else {
			app.ActiveRestartID = restartID
		}
	}
}

func (s *Store) requestStop(appID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app := s.ensureApp(appID)
	now := time.Now()
	if !s.runnerActiveLocked(app, now) {
		s.failRestartWithoutRunnerLocked(app, now)
		return
	}

	app.RestartVersion++
	app.PendingAction = "stop"
	app.Status = "stop requested"
	app.RestartRequestedAt = now
	app.LastEvent = "stop_requested"
	app.LastEventAt = now
	prependEvent(app, "stop_requested", "manual stop requested")
	s.persistEventLocked(app, "stop_requested", "manual stop requested", now)
}

func (s *Store) runnerActiveLocked(app *App, now time.Time) bool {
	s.markRunnerStaleIfNeededLocked(app, now)
	return app.RunnerConnected
}

func (s *Store) failRestartWithoutRunnerLocked(app *App, now time.Time) {
	detail := "Runner is not connected. Run the Runner command again."
	app.RunnerConnected = false
	app.Status = "failed"
	app.LastRestartResult = "failure"
	app.LastNotification = "runner disconnected"
	app.FailureReason = detail
	app.LastEvent = "restart_failed"
	app.LastEventAt = now

	if s.db != nil {
		restartID, err := s.db.InsertRestartRequested(app.ID, app.ActivePromptID, now)
		if err != nil {
			log.Printf("store restart request: %v", err)
		} else {
			app.ActiveRestartID = restartID
		}
	}

	prependEvent(app, "restart_failed", detail)
	s.persistEventLocked(app, "restart_failed", detail, now)
	shouldNotify := app.ActiveRestartID != 0 || !app.PromptActiveStarted.IsZero()
	s.finishPromptLocked(app, "failure", now)
	finalizePrompt(app, "failure", now)
	s.finishRestartLocked(app, "failure", detail)
	if shouldNotify {
		s.notifyLocked(app, "failure")
	}
}

func (s *Store) markRunnerStaleIfNeededLocked(app *App, now time.Time) {
	if !app.RunnerConnected || app.LastRunnerSeen.IsZero() || now.Sub(app.LastRunnerSeen) <= runnerStaleAfter {
		return
	}
	app.RunnerConnected = false
	if app.Status == "restart requested" || app.Status == "restarting" || app.Status == "checking health" || app.Status == "stop requested" || app.Status == "stopping" {
		s.failRestartWithoutRunnerLocked(app, now)
		return
	}
	if app.LastEvent != "runner_disconnected" {
		app.LastEvent = "runner_disconnected"
		app.LastEventAt = now
		prependEvent(app, "runner_disconnected", "runner heartbeat timed out")
		s.persistEventLocked(app, "runner_disconnected", "runner heartbeat timed out", now)
	}
	if app.Status == "running" {
		app.Status = "runner disconnected"
	}
}

func (s *Store) markRestartPickupTimedOutLocked(app *App, now time.Time) {
	if (app.Status != "restart requested" && app.Status != "stop requested") || app.RestartRequestedAt.IsZero() || now.Sub(app.RestartRequestedAt) <= restartPickupTimeout {
		return
	}

	detail := "Runner did not pick up the command. Run the Runner command again."
	app.RunnerConnected = false
	app.LastRunnerSeen = time.Time{}
	app.Status = "failed"
	app.LastRestartResult = "failure"
	app.LastNotification = "runner disconnected"
	app.FailureReason = detail
	app.LastEvent = "restart_failed"
	app.LastEventAt = now
	prependEvent(app, "restart_failed", detail)
	s.persistEventLocked(app, "restart_failed", detail, now)

	shouldNotify := app.ActiveRestartID != 0 || !app.PromptActiveStarted.IsZero()
	s.finishPromptLocked(app, "failure", now)
	finalizePrompt(app, "failure", now)
	s.finishRestartLocked(app, "failure", detail)
	if shouldNotify {
		s.notifyLocked(app, "failure")
	}
}

func (s *Store) recordRunnerEvent(appID, eventType, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app := s.ensureApp(appID)
	now := time.Now()
	if eventType == "runner_heartbeat" {
		app.RunnerConnected = true
		app.LastRunnerSeen = now
		return
	}

	app.LastEvent = eventType
	app.LastEventAt = now
	prependEvent(app, eventType, detail)

	switch eventType {
	case "runner_connected":
		app.RunnerConnected = true
		app.LastRunnerSeen = now
		app.FailureReason = ""
		if !app.CommandEnabled && app.HealthMode == "none" {
			app.Status = "ready"
			break
		}
		if app.Status == "" {
			app.Status = "not started"
		}
	case "runner_update_required":
		app.RunnerConnected = false
		app.LastRunnerSeen = time.Time{}
		app.Status = "runner update required"
		app.FailureReason = "Runner is old. Copy and run the Runner command again."
	case "runner_disconnected":
		app.RunnerConnected = false
		app.LastRunnerSeen = time.Time{}
		if app.Status == "running" {
			app.Status = "runner disconnected"
		}
	case "command_started":
		app.Status = "running"
	case "restart_started":
		app.Status = "restarting"
		app.RestartStartedAt = now
		app.RestartRequestedAt = time.Time{}
		app.FailureReason = ""
		if s.db != nil {
			if err := s.db.MarkRestartStarted(app.ActiveRestartID, now); err != nil {
				log.Printf("store restart start: %v", err)
			}
		}
	case "restart_finished":
		app.Status = "checking health"
		app.RestartFinishedAt = now
	case "stop_started":
		app.Status = "stopping"
		app.FailureReason = ""
	case "command_stopped":
		app.Status = "paused"
		app.LastRestartResult = ""
		app.LastNotification = "command stopped"
		app.FailureReason = ""
	case "stop_failed":
		app.Status = "failed"
		app.LastRestartResult = "failure"
		app.LastNotification = "stop failed"
		app.FailureReason = detail
	case "health_ok":
		if detail == healthDisabledEventMsg {
			app.Status = "running"
		} else {
			app.Status = "healthy"
		}
		app.LastRestartResult = "success"
		app.LastNotification = "restarted successfully"
		app.HealthyAt = now
		app.FailureReason = ""
		shouldNotify := app.ActiveRestartID != 0 || !app.PromptActiveStarted.IsZero()
		s.finishPromptLocked(app, "success", now)
		finalizePrompt(app, "success", now)
		s.finishRestartLocked(app, "success", "")
		if shouldNotify {
			s.notifyLocked(app, "success")
		}
	case "health_failed", "restart_failed":
		app.Status = "failed"
		app.LastRestartResult = "failure"
		app.LastNotification = "failed to become healthy"
		app.FailureReason = detail
		shouldNotify := app.ActiveRestartID != 0 || !app.PromptActiveStarted.IsZero()
		s.finishPromptLocked(app, "failure", now)
		finalizePrompt(app, "failure", now)
		s.finishRestartLocked(app, "failure", detail)
		if shouldNotify {
			s.notifyLocked(app, "failure")
		}
	case "command_exited":
		if app.Status == "restarting" || app.Status == "restart requested" || app.Status == "checking health" {
			break
		}
		app.Status = "failed"
		app.LastRestartResult = "failure"
		app.LastNotification = "command exited"
		app.FailureReason = detail
	case "command_finished":
		if app.Status == "restart requested" || app.Status == "restarting" {
			app.Status = "checking health"
		}
	case "command_failed":
		app.Status = "failed"
		app.LastRestartResult = "failure"
		app.LastNotification = "command failed"
		app.FailureReason = detail
		s.finishRestartLocked(app, "failure", detail)
	}
	s.persistEventLocked(app, eventType, detail, now)
}

func (s *Store) recordRunnerHeartbeat(appID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.apps[appID]
	if !ok {
		return false
	}
	app.RunnerConnected = true
	app.LastRunnerSeen = time.Now()
	if !app.CommandEnabled && app.HealthMode == "none" {
		app.Status = "ready"
		return true
	}
	if app.Status == "" {
		app.Status = "not started"
	}
	return true
}

func (s *Store) recordRunnerActionPickedUp(appID, action string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.apps[appID]
	if !ok {
		return
	}
	app.RunnerConnected = true
	app.LastRunnerSeen = time.Now()
	switch action {
	case "restart":
		if app.Status == "restart requested" {
			app.Status = "restarting"
		}
	case "stop":
		if app.Status == "stop requested" {
			app.Status = "stopping"
		}
	}
}

func (s *Store) recordRunnerUpdateRequired(appID string, runnerVersion int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.apps[appID]
	if !ok {
		return false
	}
	now := time.Now()
	app.RunnerConnected = false
	app.LastRunnerSeen = time.Time{}
	app.Status = "runner update required"
	app.FailureReason = "Runner is old. Copy and run the Runner command again."
	if app.LastEvent == "runner_update_required" {
		return true
	}
	app.LastEvent = "runner_update_required"
	app.LastEventAt = now
	detail := fmt.Sprintf("runner version %d, required %d", runnerVersion, protocol.RunnerVersion)
	prependEvent(app, "runner_update_required", detail)
	s.persistEventLocked(app, "runner_update_required", detail, now)
	return true
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
		Repeat:   1,
		SoundURL: "/sound/" + soundType,
	})
	if s.db != nil {
		if err := s.db.InsertNotification(app.ID, soundType, 1, time.Now()); err != nil {
			log.Printf("store notification: %v", err)
		}
	}
}

func finalizePrompt(app *App, result string, finishedAt time.Time) {
	if app.PromptActiveStarted.IsZero() {
		return
	}

	app.LastWaitSeconds = finishedAt.Sub(app.PromptActiveStarted).Seconds()
	app.LastPromptSeconds = app.PendingPromptSecs
	app.LastRestartSeconds = app.LastWaitSeconds - app.LastPromptSeconds
	if app.LastRestartSeconds < 0 {
		app.LastRestartSeconds = 0
	}
	app.TotalWaitSeconds += app.LastWaitSeconds
	app.TotalPromptSeconds += app.LastPromptSeconds
	app.TotalRestartSeconds += app.LastRestartSeconds
	if sameDay(time.Now(), finishedAt) {
		app.TodayPromptCount++
		app.TodayWaitSeconds += app.LastWaitSeconds
	}
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
		RestartSecs: app.LastRestartSeconds,
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

func sameDay(left, right time.Time) bool {
	left = left.In(time.Local)
	right = right.In(time.Local)
	return left.Year() == right.Year() && left.YearDay() == right.YearDay()
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
		CommandEnabled:     true,
		Command:            []string{"sh", "-lc", "echo configure command for " + appID + "; sleep 3600"},
		HealthMode:         "none",
		HealthURL:          "http://localhost:3000",
		StartupTimeoutSecs: 60,
		CodexEnvironment:   "docker",
		RunnerEnvironment:  "docker",
		Status:             "not started",
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

func (s *Store) upsertApp(currentID string, next *App) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if currentID == "" {
		currentID = next.ID
	}
	next.ID = s.uniqueAppIDLocked(next.ID, currentID)

	current, hadCurrent := s.apps[currentID]
	if !hadCurrent && currentID != next.ID {
		current, hadCurrent = s.apps[next.ID]
	}
	if hadCurrent {
		next.Status = current.Status
		next.RunnerConnected = current.RunnerConnected
		next.LastRunnerSeen = current.LastRunnerSeen
		next.RestartVersion = current.RestartVersion
		next.PendingAction = current.PendingAction
		next.PromptCount = current.PromptCount
		next.PromptActiveStarted = current.PromptActiveStarted
		next.PromptPermissions = current.PromptPermissions
		next.PromptFinishedAt = current.PromptFinishedAt
		next.PendingPromptSecs = current.PendingPromptSecs
		next.TotalWaitSeconds = current.TotalWaitSeconds
		next.TotalPromptSeconds = current.TotalPromptSeconds
		next.TotalRestartSeconds = current.TotalRestartSeconds
		next.TodayPromptCount = current.TodayPromptCount
		next.TodayWaitSeconds = current.TodayWaitSeconds
		next.MinWaitSeconds = current.MinWaitSeconds
		next.MaxWaitSeconds = current.MaxWaitSeconds
		next.LastWaitSeconds = current.LastWaitSeconds
		next.LastPromptSeconds = current.LastPromptSeconds
		next.LastRestartSeconds = current.LastRestartSeconds
		next.LastRestartResult = current.LastRestartResult
		next.LastNotification = current.LastNotification
		next.LastEvent = "app_updated"
		next.LastEventAt = now
		next.RestartRequestedAt = current.RestartRequestedAt
		next.RestartStartedAt = current.RestartStartedAt
		next.RestartFinishedAt = current.RestartFinishedAt
		next.HealthyAt = current.HealthyAt
		next.FailureReason = current.FailureReason
		next.ActivePromptID = current.ActivePromptID
		next.ActiveRestartID = current.ActiveRestartID
		next.RecentPrompts = append([]PromptSummary(nil), current.RecentPrompts...)
		next.RecentEvents = append([]EventSummary(nil), current.RecentEvents...)
		if currentID != next.ID {
			delete(s.apps, currentID)
		}
	} else {
		next.Status = "not started"
		next.LastEvent = "app_created"
		next.LastEventAt = now
	}
	prependEvent(next, next.LastEvent, "")
	if s.db != nil {
		if hadCurrent && currentID != next.ID {
			if err := s.db.RenameApp(currentID, recordFromApp(next)); err != nil {
				log.Printf("rename app: %v", err)
				return err
			}
		} else {
			if err := s.db.UpsertApp(recordFromApp(next)); err != nil {
				log.Printf("store app: %v", err)
				return err
			}
		}
	}
	s.apps[next.ID] = next
	s.persistEventLocked(next, next.LastEvent, "", next.LastEventAt)
	return nil
}

func (s *Store) uniqueAppIDLocked(baseID, currentID string) string {
	if baseID == "" {
		baseID = "app"
	}
	if app, ok := s.apps[baseID]; !ok || app.ID == currentID {
		return baseID
	}
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s-%d", baseID, index)
		if app, ok := s.apps[candidate]; !ok || app.ID == currentID {
			return candidate
		}
	}
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
		app.TotalPromptSeconds = stats.TotalPrompt
		app.TotalRestartSeconds = stats.TotalRestart
		app.MinWaitSeconds = stats.MinWait
		app.MaxWaitSeconds = stats.MaxWait
		app.LastWaitSeconds = stats.LastWait
		app.LastPromptSeconds = stats.LastPrompt
		app.LastRestartSeconds = stats.LastRestart
		app.LastRestartResult = stats.LastResult
		for _, record := range stats.Recent {
			app.RecentPrompts = append(app.RecentPrompts, PromptSummary{
				Index:       record.Index,
				PromptSecs:  record.PromptSecs,
				WaitSecs:    record.WaitSecs,
				RestartSecs: record.RestartSecs,
				Result:      record.Result,
				FinishedAt:  record.FinishedAt,
				Permissions: record.Permissions,
			})
		}
	}
	todayStats, err := s.db.LoadTodayPromptStats(app.ID, time.Now())
	if err != nil {
		log.Printf("load today prompt stats: %v", err)
	} else {
		app.TodayPromptCount = todayStats.Count
		app.TodayWaitSeconds = todayStats.TotalWait
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

	if prompt, ok, err := s.db.LoadActivePrompt(app.ID, time.Now().Add(-activePromptRestoreAge)); err != nil {
		log.Printf("load active prompt: %v", err)
	} else if ok {
		app.ActivePromptID = prompt.ID
		app.PromptActiveStarted = prompt.StartedAt
		app.PromptPermissions = prompt.Permissions
		app.Status = "prompt active"
	}

	if restart, ok, err := s.db.LoadActiveRestart(app.ID); err != nil {
		log.Printf("load active restart: %v", err)
	} else if ok {
		app.ActiveRestartID = restart.ID
		if app.ActivePromptID == 0 {
			app.ActivePromptID = restart.PromptID
		}
		app.RestartRequestedAt = restart.RequestedAt
		app.RestartStartedAt = restart.StartedAt
		app.RestartFinishedAt = restart.FinishedAt
		if !app.PromptActiveStarted.IsZero() && !restart.RequestedAt.IsZero() && app.PendingPromptSecs == 0 {
			app.PendingPromptSecs = restart.RequestedAt.Sub(app.PromptActiveStarted).Seconds()
			if app.PendingPromptSecs < 0 {
				app.PendingPromptSecs = 0
			}
		}
		switch {
		case !restart.FinishedAt.IsZero():
			app.Status = "checking health"
		case !restart.StartedAt.IsZero():
			app.Status = "restarting"
		default:
			app.Status = "restart requested"
		}
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
	w.Header().Set("Cache-Control", "no-store")
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

func (n *Notifier) HealthCheckClientCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.healthClients)
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

func delegationDepth(total float64, count int) float64 {
	if count <= 0 {
		return 0
	}
	return total / float64(count)
}

func depthClass(total float64, count int) string {
	depth := delegationDepth(total, count)
	if depth <= 0 {
		return "depth-empty"
	}
	if depth < 5*60 {
		return "depth-micro"
	}
	if depth < 15*60 {
		return "depth-shallow"
	}
	if depth < 30*60 {
		return "depth-solid"
	}
	return "depth-deep"
}

func depthHint(total float64, count int) string {
	depth := delegationDepth(total, count)
	if depth <= 0 {
		return "waiting for prompts"
	}
	if depth < 5*60 {
		return "batch more work"
	}
	if depth < 15*60 {
		return "give broader goals"
	}
	if depth < 30*60 {
		return "good delegation"
	}
	return "deep delegation"
}

func workSummary(stats storage.GlobalStats) string {
	if stats.Count == 0 || stats.Span <= 0 {
		return "No prompts yet"
	}
	if stats.EstimatedHumanTime <= 0 {
		return "Waiting for more prompt history"
	}

	activeShare := stats.EstimatedHumanTime / stats.Span * 100
	return fmt.Sprintf("You actively prompted about %.0f%% of this time", activeShare)
}

func commandLine(command []string, sep string) string {
	if len(command) >= 3 && command[0] == "sh" && command[1] == "-lc" {
		return command[2]
	}
	return strings.Join(command, sep)
}

func appFromForm(r *http.Request, currentID string, current *App) *App {
	name := strings.TrimSpace(r.FormValue("name"))
	appID := appIDFromName(name)
	if current != nil {
		name = current.Name
		appID = current.ID
	}
	if appID == "app" && name == "" && currentID != "" {
		appID = currentID
	}

	var command []string
	if values, ok := r.Form["command"]; ok {
		commandValue := ""
		if len(values) > 0 {
			commandValue = strings.TrimSpace(values[0])
		}
		if commandValue != "" {
			command = []string{"sh", "-lc", commandValue}
		}
	} else if current != nil {
		command = current.Command
	}
	commandEnabled := r.FormValue("command_enabled") == "1"

	healthMode := "none"
	if r.FormValue("health_enabled") == "1" {
		healthMode = "browser_url"
	}

	timeout, _ := strconv.Atoi(r.FormValue("startup_timeout_seconds"))
	if timeout <= 0 {
		if current != nil && current.StartupTimeoutSecs > 0 {
			timeout = current.StartupTimeoutSecs
		} else {
			timeout = 60
		}
	}

	healthURL := strings.TrimSpace(r.FormValue("health_url"))
	if _, ok := r.Form["health_url"]; !ok && current != nil {
		healthURL = current.HealthURL
	}

	if name == "" {
		name = appID
	}

	return &App{
		ID:                 appID,
		Name:               name,
		CWD:                ".",
		CommandEnabled:     commandEnabled,
		Command:            command,
		HealthMode:         healthMode,
		HealthURL:          healthURL,
		HealthCommand:      nil,
		StartupTimeoutSecs: timeout,
		CodexEnvironment:   "docker",
		RunnerEnvironment:  "docker",
	}
}

func appFromRecord(record storage.AppRecord) *App {
	healthMode := valueOrDefault(record.HealthMode, "none")
	if healthMode != "none" {
		healthMode = "browser_url"
	}
	return &App{
		ID:                 record.ID,
		Name:               record.Name,
		CWD:                ".",
		CommandEnabled:     record.CommandEnabled,
		Command:            record.Command,
		HealthMode:         healthMode,
		HealthURL:          record.HealthURL,
		HealthCommand:      nil,
		StartupTimeoutSecs: record.StartupTimeoutSecs,
		CodexEnvironment:   valueOrDefault(record.CodexEnvironment, "docker"),
		RunnerEnvironment:  valueOrDefault(record.RunnerEnvironment, "docker"),
		Status:             "not started",
		LastEvent:          "loaded",
		LastEventAt:        time.Now(),
	}
}

func recordFromApp(app *App) storage.AppRecord {
	return storage.AppRecord{
		ID:                 app.ID,
		Name:               app.Name,
		CWD:                ".",
		CommandEnabled:     app.CommandEnabled,
		Command:            app.Command,
		HealthMode:         app.HealthMode,
		HealthURL:          app.HealthURL,
		HealthCommand:      app.HealthCommand,
		StartupTimeoutSecs: app.StartupTimeoutSecs,
		SoundRepeatCount:   1,
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
