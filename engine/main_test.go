package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestEnsureLocalStateIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	p := paths{
		home:    dir,
		db:      filepath.Join(dir, "state.db"),
		authDir: filepath.Join(dir, "auth"),
		lock:    filepath.Join(dir, "engine.lock"),
	}

	if err := ensureLocalState(p); err != nil {
		t.Fatalf("first init failed: %v", err)
	}
	if err := ensureLocalState(p); err != nil {
		t.Fatalf("second init failed: %v", err)
	}

	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var tableName string
	if err := db.QueryRow(`select name from sqlite_master where type='table' and name='events'`).Scan(&tableName); err != nil {
		t.Fatal(err)
	}
	if tableName != "events" {
		t.Fatalf("expected events table, got %q", tableName)
	}

	info, err := os.Stat(p.authDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("auth dir mode = %o, want 700", got)
	}
}

func TestResolvePathsUsesVibeengineHome(t *testing.T) {
	t.Setenv("VIBEENGINE_HOME", "/tmp/vibeengine-test")

	p, err := resolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	if p.home != "/tmp/vibeengine-test" {
		t.Fatalf("home = %q", p.home)
	}
}

func TestAcquireEngineLock(t *testing.T) {
	dir := t.TempDir()
	p := paths{
		home:    dir,
		db:      filepath.Join(dir, "state.db"),
		authDir: filepath.Join(dir, "auth"),
		lock:    filepath.Join(dir, "engine.lock"),
	}

	release, err := acquireEngineLock(p)
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	if _, err := acquireEngineLock(p); err == nil {
		t.Fatal("expected second lock attempt to fail")
	}
	if err := release(); err != nil {
		t.Fatalf("release lock: %v", err)
	}
	if release, err := acquireEngineLock(p); err != nil {
		t.Fatalf("reacquire lock: %v", err)
	} else if err := release(); err != nil {
		t.Fatalf("release second lock: %v", err)
	}
}

func TestPersistHookEventRecordsOfflineEvent(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}

	event, err := newHookEvent(hookEventInput{
		project:     "project-one",
		tool:        "codex",
		nativeEvent: "session-start",
		nativeID:    "native-session-1",
		eventType:   "session_started",
		payload:     `{"source":"test"}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	inserted, err := persistHookEvent(p, event)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted {
		t.Fatal("expected event insert")
	}

	db := openTestDB(t, p)
	defer db.Close()

	var sequence int
	var deliveryState string
	if err := db.QueryRow(
		`select local_sequence, delivery_state from events where local_event_id = ?`,
		event.localEventID,
	).Scan(&sequence, &deliveryState); err != nil {
		t.Fatal(err)
	}
	if sequence != 1 {
		t.Fatalf("local_sequence = %d, want 1", sequence)
	}
	if deliveryState != "pending" {
		t.Fatalf("delivery_state = %q, want pending", deliveryState)
	}
}

func TestPersistHookEventReturnsErrorWhenLocalDBUnavailable(t *testing.T) {
	dir := t.TempDir()
	p := paths{
		home:    dir,
		db:      filepath.Join(dir, "state.db"),
		authDir: filepath.Join(dir, "auth"),
		lock:    filepath.Join(dir, "engine.lock"),
	}
	if err := os.Mkdir(p.db, 0o700); err != nil {
		t.Fatal(err)
	}

	event, err := newHookEvent(hookEventInput{
		project:     "project-one",
		tool:        "codex",
		nativeEvent: "session-start",
		nativeID:    "native-session-1",
		eventType:   "session_started",
		payload:     `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := persistHookEvent(p, event); err == nil {
		t.Fatal("expected local DB write failure")
	}
}

func TestPersistHookEventDeduplicatesNativeEventID(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}

	input := hookEventInput{
		project:     "project-one",
		tool:        "codex",
		nativeEvent: "prompt-started",
		nativeID:    "native-prompt-start",
		sessionID:   "session-1",
		promptID:    "prompt-1",
		eventType:   "prompt_started",
		payload:     `{}`,
	}
	first, err := newHookEvent(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newHookEvent(input)
	if err != nil {
		t.Fatal(err)
	}

	inserted, err := persistHookEvent(p, first)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted {
		t.Fatal("expected first event insert")
	}
	inserted, err = persistHookEvent(p, second)
	if err != nil {
		t.Fatal(err)
	}
	if inserted {
		t.Fatal("expected duplicate native event to be ignored")
	}

	db := openTestDB(t, p)
	defer db.Close()

	var eventCount int
	var promptCount int
	if err := db.QueryRow(`select count(*) from events`).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`select count(*) from active_prompts`).Scan(&promptCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("event count = %d, want 1", eventCount)
	}
	if promptCount != 1 {
		t.Fatalf("active prompt count = %d, want 1", promptCount)
	}
}

func TestPromptFinishClosesActivePrompt(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}

	started, err := newHookEvent(hookEventInput{
		project:     "project-one",
		tool:        "codex",
		nativeEvent: "prompt-started",
		nativeID:    "native-start",
		sessionID:   "session-1",
		promptID:    "prompt-1",
		eventType:   "prompt_started",
		payload:     `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	finished, err := newHookEvent(hookEventInput{
		project:     "project-one",
		tool:        "codex",
		nativeEvent: "prompt-finished",
		nativeID:    "native-finish",
		sessionID:   "session-1",
		promptID:    "prompt-1",
		eventType:   "prompt_finished",
		payload:     `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	if inserted, err := persistHookEvent(p, started); err != nil {
		t.Fatal(err)
	} else if !inserted {
		t.Fatal("expected prompt-started insert")
	}
	if inserted, err := persistHookEvent(p, finished); err != nil {
		t.Fatal(err)
	} else if !inserted {
		t.Fatal("expected prompt-finished insert")
	}

	db := openTestDB(t, p)
	defer db.Close()

	var state string
	var finishedEventID string
	if err := db.QueryRow(
		`select state, finished_event_id from active_prompts where prompt_instance_id = ?`,
		"prompt-1",
	).Scan(&state, &finishedEventID); err != nil {
		t.Fatal(err)
	}
	if state != "finished" {
		t.Fatalf("prompt state = %q, want finished", state)
	}
	if finishedEventID != finished.localEventID {
		t.Fatalf("finished_event_id = %q, want %q", finishedEventID, finished.localEventID)
	}
}

func TestNewHookEventRejectsInvalidPayloadJSON(t *testing.T) {
	_, err := newHookEvent(hookEventInput{
		project:     "project-one",
		tool:        "codex",
		nativeEvent: "session-start",
		eventType:   "session_started",
		payload:     `{invalid`,
	})
	if err == nil {
		t.Fatal("expected invalid JSON payload to fail")
	}
}

func TestMapNativeHookEventDropsContentFields(t *testing.T) {
	mapped, err := mapNativeHookEvent("codex", "UserPromptSubmit", []byte(`{
		"session_id":"session-1",
		"turn_id":"turn-1",
		"hook_event_name":"UserPromptSubmit",
		"prompt":"secret prompt",
		"cwd":"/private/repo",
		"transcript_path":"/private/transcript.jsonl",
		"permission_mode":"default"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if mapped.eventName != "prompt-started" {
		t.Fatalf("eventName = %q, want prompt-started", mapped.eventName)
	}
	if mapped.sessionID != "session-1" {
		t.Fatalf("sessionID = %q, want session-1", mapped.sessionID)
	}
	if mapped.promptID != "turn-1" {
		t.Fatalf("promptID = %q, want turn-1", mapped.promptID)
	}
	for _, forbidden := range []string{"secret prompt", "/private/repo", "transcript"} {
		if strings.Contains(mapped.payload, forbidden) {
			t.Fatalf("safe payload leaked %q: %s", forbidden, mapped.payload)
		}
	}
}

func TestMapClaudeStopProducesPromptFinished(t *testing.T) {
	mapped, err := mapNativeHookEvent("claude_code", "Stop", []byte(`{
		"session_id":"session-1",
		"turn_id":"turn-1",
		"last_assistant_message":"private response"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if mapped.eventName != "prompt-finished" {
		t.Fatalf("eventName = %q, want prompt-finished", mapped.eventName)
	}
	if strings.Contains(mapped.payload, "private response") {
		t.Fatalf("safe payload leaked assistant response: %s", mapped.payload)
	}
}

func TestInstallHookAdaptersPreservesManualHooksAndRepairsOwnedHooks(t *testing.T) {
	repo := t.TempDir()
	configPath := filepath.Join(repo, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `{
		"hooks": {
			"Stop": [
				{
					"hooks": [
						{"type":"command","command":"echo manual"},
						{"type":"command","command":"vibeengine adapter codex Stop --project old --owner vibeengine"}
					]
				}
			]
		}
	}`
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installHookAdapters(repo, "project-one", []string{"codex"}); err != nil {
		t.Fatal(err)
	}

	var config hookConfigFile
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &config); err != nil {
		t.Fatal(err)
	}

	var manualFound bool
	var oldOwnedFound bool
	var newOwnedFound bool
	for _, groups := range config.Hooks {
		for _, group := range groups {
			for _, hook := range group.Hooks {
				if hook.Command == "echo manual" {
					manualFound = true
				}
				if strings.Contains(hook.Command, "--project old") {
					oldOwnedFound = true
				}
				if strings.Contains(hook.Command, "vibeengine adapter codex") &&
					strings.Contains(hook.Command, "--project project-one") &&
					strings.Contains(hook.Command, "--owner vibeengine") {
					newOwnedFound = true
				}
			}
		}
	}
	if !manualFound {
		t.Fatal("manual hook was not preserved")
	}
	if oldOwnedFound {
		t.Fatal("old owned hook was not repaired")
	}
	if !newOwnedFound {
		t.Fatal("new owned hook was not generated")
	}
}

func TestInstallHookAdaptersSupportsMultipleAdapters(t *testing.T) {
	repo := t.TempDir()
	if err := installHookAdapters(repo, "project-one", []string{"codex", "claude_code"}); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		filepath.Join(repo, ".codex", "hooks.json"),
		filepath.Join(repo, ".claude", "settings.local.json"),
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(content) {
			t.Fatalf("%s is not valid JSON", path)
		}
	}
}

func TestCleanupLegacyHooksPreservesManualHooks(t *testing.T) {
	repo := t.TempDir()
	hookPath := filepath.Join(repo, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		t.Fatal(err)
	}
	config := hookConfigFile{
		Hooks: map[string][]hookMatcherGroup{
			"UserPromptSubmit": {
				{
					Hooks: []hookHandler{
						{Type: "command", Command: "echo manual"},
						{Type: "command", Command: "docker exec vibecode-dev-notify-helper-1 wget http://localhost"},
					},
				},
			},
		},
	}
	body, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hookPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cleanupLegacyHookAdapters(repo); err != nil {
		t.Fatal(err)
	}
	cleaned, err := readHookConfig(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	handlers := cleaned.Hooks["UserPromptSubmit"][0].Hooks
	if len(handlers) != 1 || handlers[0].Command != "echo manual" {
		t.Fatalf("unexpected handlers after cleanup: %#v", handlers)
	}
}

func TestExchangeBootstrapTokenAndStoreResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/engine/bootstrap" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		_ = json.NewEncoder(response).Encode(bootstrapResponse{
			AccountID:            "account-1",
			ProjectID:            "project-1",
			ProjectName:          "ProjectOne",
			UserID:               "user-1",
			RefreshToken:         "refresh-token",
			AccessToken:          "access-token",
			AccessTokenExpiresAt: "2026-05-26T00:00:00Z",
			EngineAuthEpoch:      3,
		})
	}))
	defer server.Close()

	result, err := exchangeBootstrapToken(server.URL, "bootstrap-token")
	if err != nil {
		t.Fatal(err)
	}
	if result.ProjectID != "project-1" {
		t.Fatalf("ProjectID = %q, want project-1", result.ProjectID)
	}

	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	if err := storeBootstrapResult(p, server.URL, repo, []string{"codex"}, result); err != nil {
		t.Fatal(err)
	}
	if err := installHookAdapters(repo, result.ProjectID, []string{"codex"}); err != nil {
		t.Fatal(err)
	}

	credentials, err := readLocalCredentials(p)
	if err != nil {
		t.Fatal(err)
	}
	if credentials == nil || credentials.AccountID != "account-1" {
		t.Fatalf("credentials = %#v, want account-1", credentials)
	}

	db := openTestDB(t, p)
	defer db.Close()
	var repoPath string
	if err := db.QueryRow(`select repo_path from projects where project_id = ?`, "project-1").Scan(&repoPath); err != nil {
		t.Fatal(err)
	}
	if repoPath != repo {
		t.Fatalf("repo_path = %q, want %q", repoPath, repo)
	}
}

func TestRunEngineLoopOnceUploadsPendingEvents(t *testing.T) {
	var uploaded int
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/engine/check-in":
			if request.Header.Get("authorization") != "Bearer access-token" {
				t.Fatalf("missing bearer token: %s", request.Header.Get("authorization"))
			}
			var body checkInRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			uploaded = len(body.Events)
			var accepted []string
			for _, event := range body.Events {
				accepted = append(accepted, event.LocalEventID)
			}
			_ = json.NewEncoder(response).Encode(checkInResponse{AcceptedEventIDs: accepted})
		case "/api/health":
			_ = json.NewEncoder(response).Encode(map[string]bool{"ok": true})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	credentials := localCredentials{
		APIBaseURL:           server.URL,
		AccountID:            "account-1",
		UserID:               "user-1",
		RefreshToken:         "refresh-token",
		AccessToken:          "access-token",
		AccessTokenExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		EngineAuthEpoch:      1,
		UpdatedAtUTC:         nowUTC(),
	}
	output, err := json.Marshal(credentials)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.authDir, "credentials.json"), output, 0o600); err != nil {
		t.Fatal(err)
	}
	event, err := newHookEvent(hookEventInput{
		project:     "project-one",
		tool:        "codex",
		nativeEvent: "prompt-started",
		nativeID:    "native-start",
		eventType:   "prompt_started",
		payload:     `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if inserted, err := persistHookEvent(p, event); err != nil {
		t.Fatal(err)
	} else if !inserted {
		t.Fatal("expected hook event insert")
	}

	if err := runEngineLoop(p, true, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if uploaded != 1 {
		t.Fatalf("uploaded = %d, want 1", uploaded)
	}

	db := openTestDB(t, p)
	defer db.Close()
	var deliveryState string
	if err := db.QueryRow(`select delivery_state from events where local_event_id = ?`, event.localEventID).Scan(&deliveryState); err != nil {
		t.Fatal(err)
	}
	if deliveryState != "delivered" {
		t.Fatalf("delivery_state = %q, want delivered", deliveryState)
	}
}

func TestCheckInOutageKeepsPendingEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		t.Fatalf("server should be closed before check-in")
	}))
	apiURL := server.URL
	server.Close()

	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	writeTestCredentials(t, p, apiURL, "access-token")
	event := persistTestEvent(t, p, "project-one", "prompt_started", "native-start")

	if _, err := checkInOnce(p, 500); err == nil {
		t.Fatal("expected outage error")
	}

	assertEventDeliveryState(t, p, event.localEventID, "pending")
}

func TestCheckInEntitlementInactiveKeepsLocalCapture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/engine/check-in":
			response.WriteHeader(402)
			_, _ = response.Write([]byte(`{"error":{"code":"entitlement_inactive","message":"inactive"}}`))
		case "/api/health":
			_ = json.NewEncoder(response).Encode(map[string]bool{"ok": true})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	writeTestCredentials(t, p, server.URL, "access-token")
	event := persistTestEvent(t, p, "project-one", "prompt_started", "native-start")

	if _, err := checkInOnce(p, 500); !isEntitlementInactiveError(err) {
		t.Fatalf("expected entitlement inactive error, got %v", err)
	}

	assertEventDeliveryState(t, p, event.localEventID, "pending")
	state, err := getKV(p, "engine.state")
	if err != nil {
		t.Fatal(err)
	}
	if state != "cloud_paused" {
		t.Fatalf("engine.state = %q, want cloud_paused", state)
	}
}

func TestStatusHelpersSummarizeLocalState(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	event, err := newHookEvent(hookEventInput{
		project:     "project-one",
		tool:        "codex",
		nativeEvent: "permission-asked",
		nativeID:    "permission-1",
		eventType:   "permission_requested",
		payload:     `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := persistHookEvent(p, event); err != nil {
		t.Fatal(err)
	}

	count, oldest, err := pendingEventStats(p)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || oldest == "" {
		t.Fatalf("pending stats = %d, %q; want one pending event", count, oldest)
	}
	mappings, err := localProjectMappings(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(mappings) != 1 || !strings.Contains(mappings[0], "project-one") {
		t.Fatalf("unexpected mappings: %#v", mappings)
	}
}

func TestCommandSetStoresApprovedCommandAndState(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	commandText := longRunningTestCommand()
	hash := hashCommand(commandText)

	if err := storeApprovedCommand(p, "project-one", hash, commandText); err != nil {
		t.Fatal(err)
	}

	db := openTestDB(t, p)
	defer db.Close()
	var storedHash string
	var supervisorState string
	if err := db.QueryRow(
		`select approved_commands.command_hash, command_state.supervisor_state
		 from approved_commands
		 join command_state on command_state.project_id = approved_commands.project_id
		 where approved_commands.project_id = 'project-one'`,
	).Scan(&storedHash, &supervisorState); err != nil {
		t.Fatal(err)
	}
	if storedHash != hash {
		t.Fatalf("command hash = %q, want %q", storedHash, hash)
	}
	if supervisorState != "enabled" {
		t.Fatalf("supervisor state = %q, want enabled", supervisorState)
	}
}

func TestCommandSupervisorPausesDuringPromptAndResumesOnFinish(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	commandText := longRunningTestCommand()
	if err := storeApprovedCommand(p, "project-one", hashCommand(commandText), commandText); err != nil {
		t.Fatal(err)
	}
	if err := updateCommandConfig(p, "project-one", commandConfigUpdate{pauseDuringPrompt: true}); err != nil {
		t.Fatal(err)
	}
	if err := startCommandProcess(p, "project-one", "test"); err != nil {
		t.Fatal(err)
	}
	defer stopCommandProcess(p, "project-one", "test_cleanup")

	started, err := newHookEvent(hookEventInput{
		project:     "project-one",
		tool:        "codex",
		nativeEvent: "prompt-started",
		nativeID:    "native-start",
		sessionID:   "session-1",
		promptID:    "prompt-1",
		eventType:   "prompt_started",
		payload:     `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := persistHookEvent(p, started); err != nil {
		t.Fatal(err)
	}
	if err := runCommandSupervisorOnce(p); err != nil {
		t.Fatal(err)
	}
	assertCommandProcessState(t, p, "project-one", "paused", "prompt")

	finished, err := newHookEvent(hookEventInput{
		project:     "project-one",
		tool:        "codex",
		nativeEvent: "prompt-finished",
		nativeID:    "native-finish",
		sessionID:   "session-1",
		promptID:    "prompt-1",
		eventType:   "prompt_finished",
		payload:     `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := persistHookEvent(p, finished); err != nil {
		t.Fatal(err)
	}
	if err := runCommandSupervisorOnce(p); err != nil {
		t.Fatal(err)
	}
	assertCommandProcessState(t, p, "project-one", "running", "")
}

func TestCommandSupervisorIgnoresStalePromptFinish(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	commandText := longRunningTestCommand()
	if err := storeApprovedCommand(p, "project-one", hashCommand(commandText), commandText); err != nil {
		t.Fatal(err)
	}
	if err := storePromptCommandState(p, "project-one", []string{"prompt-real"}, "prompt"); err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t, p)
	if _, err := db.Exec(`update command_state set process_state = 'paused' where project_id = 'project-one'`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	finished, err := newHookEvent(hookEventInput{
		project:     "project-one",
		tool:        "codex",
		nativeEvent: "prompt-finished",
		nativeID:    "native-finish",
		sessionID:   "session-1",
		promptID:    "prompt-stale",
		eventType:   "prompt_finished",
		payload:     `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := persistHookEvent(p, finished); err != nil {
		t.Fatal(err)
	}
	if err := runCommandSupervisorOnce(p); err != nil {
		t.Fatal(err)
	}
	assertCommandProcessState(t, p, "project-one", "paused", "prompt")
}

func TestCommandSupervisorAutoPausesAfterReady(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	commandText := longRunningTestCommand()
	if err := storeApprovedCommand(p, "project-one", hashCommand(commandText), commandText); err != nil {
		t.Fatal(err)
	}
	if err := updateCommandConfig(p, "project-one", commandConfigUpdate{autoPauseAfterReady: time.Second}); err != nil {
		t.Fatal(err)
	}
	if err := startCommandProcess(p, "project-one", "test"); err != nil {
		t.Fatal(err)
	}
	defer stopCommandProcess(p, "project-one", "test_cleanup")

	db := openTestDB(t, p)
	if _, err := db.Exec(
		`update command_state set ready_since = ? where project_id = 'project-one'`,
		time.Now().Add(-2*time.Second).UTC().Format(time.RFC3339Nano),
	); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	if err := enforceCommandTimers(p); err != nil {
		t.Fatal(err)
	}
	assertCommandProcessState(t, p, "project-one", "paused", "auto_pause")
}

func TestCommandHealthRejectsStaleRunID(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	commandText := longRunningTestCommand()
	if err := storeApprovedCommand(p, "project-one", hashCommand(commandText), commandText); err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t, p)
	if _, err := db.Exec(`update command_state set run_id = 'run_current' where project_id = 'project-one'`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	if err := setCommandHealth(p, "project-one", "run_old", "ready", ""); err == nil {
		t.Fatal("expected stale health result to be rejected")
	}
}

func TestStoreRemoteInstructionsQueuesActionAndApproval(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	response := checkInResponse{
		RemoteActions: []remoteActionInstruction{
			{ID: "remote-action-1", ProjectID: "project-one", Type: "command_pause"},
		},
		CommandProposals: []commandProposalInstruction{
			{
				ID:          "proposal-1",
				ProjectID:   "project-one",
				Type:        "command_settings",
				PayloadJSON: json.RawMessage(`{"command":"sleep 30","after_prompt":"resume"}`),
			},
		},
	}
	if err := storeRemoteInstructions(p, response); err != nil {
		t.Fatal(err)
	}
	if err := storeRemoteInstructions(p, response); err != nil {
		t.Fatal(err)
	}

	db := openTestDB(t, p)
	defer db.Close()
	var actionCount int
	var approvalCount int
	if err := db.QueryRow(`select count(*) from pending_actions where action_id = 'remote-action-1'`).Scan(&actionCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`select count(*) from pending_approvals where proposal_id = 'proposal-1'`).Scan(&approvalCount); err != nil {
		t.Fatal(err)
	}
	if actionCount != 1 || approvalCount != 1 {
		t.Fatalf("queued action/proposal counts = %d/%d, want 1/1", actionCount, approvalCount)
	}
}

func TestApproveCommandProposalRequiresFullHash(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	commandText := longRunningTestCommand()
	payload := fmt.Sprintf(`{"command":%q,"pause_during_prompt":true,"auto_pause_after_ready_seconds":5,"after_prompt":"restart"}`, commandText)
	db := openTestDB(t, p)
	if _, err := db.Exec(
		`insert into pending_approvals(proposal_id, project_id, type, payload_json, state, created_at)
		 values('proposal-1', 'project-one', 'command_settings', ?, 'pending', ?)`,
		payload,
		nowUTC(),
	); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	if err := approveCommandProposal(p, "proposal-1", "wrong"); err == nil {
		t.Fatal("expected full hash confirmation to be required")
	}
	if err := approveCommandProposal(p, "proposal-1", hashCommand(commandText)); err != nil {
		t.Fatal(err)
	}
	state, err := loadCommandState(p, "project-one")
	if err != nil {
		t.Fatal(err)
	}
	if state == nil || !state.pauseDuringPrompt || state.autoPauseSeconds != 5 {
		t.Fatalf("unexpected command state after approval: %#v", state)
	}
}

func TestVerifySignedArtifactRejectsWrongSignature(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER}))
	body := []byte("release-body")
	digest := sha256.Sum256(body)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySignedArtifact(body, signature, hex.EncodeToString(digest[:]), publicKeyPEM); err != nil {
		t.Fatal(err)
	}
	if err := verifySignedArtifact([]byte("tampered"), signature, hex.EncodeToString(digest[:]), publicKeyPEM); err == nil {
		t.Fatal("expected tampered artifact to fail verification")
	}
}

func TestApplySignedUpdateRollsBackOnSwapFailure(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	executable := filepath.Join(dir, "vibeengine")
	if err := os.WriteFile(executable, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(dir, "vibeengine.rollback")
	if err := os.Mkdir(blocker, 0o700); err != nil {
		t.Fatal(err)
	}
	body := []byte("new")
	sum := sha256.Sum256(body)
	if err := applySignedUpdateToPath(p, executable, body, hex.EncodeToString(sum[:])); err == nil {
		t.Fatal("expected rollback path conflict to fail update")
	}
	current, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != "old" {
		t.Fatalf("executable = %q, want old", string(current))
	}
}

func TestApplySignedUpdateSwapsBinaryAndRecordsRollback(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(t.TempDir(), "vibeengine")
	if err := os.WriteFile(executable, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	body := []byte("new")
	sum := sha256.Sum256(body)
	if err := applySignedUpdateToPath(p, executable, body, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != "new" {
		t.Fatalf("executable = %q, want new", string(current))
	}
	rollback, err := os.ReadFile(executable + ".rollback")
	if err != nil {
		t.Fatal(err)
	}
	if string(rollback) != "old" {
		t.Fatalf("rollback = %q, want old", string(rollback))
	}
	if status := readUpdateStatus(p); !strings.Contains(status, "updated") {
		t.Fatalf("update status = %q, want updated", status)
	}
}

func TestEnforceLocalRetentionDropsOldDeliveredEvents(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t, p)
	if _, err := db.Exec(
		`insert into events(
			local_event_id, local_sequence, project_id, type, payload_json,
			created_at_utc, delivery_state, delivered_at
		) values('old-delivered', 1, 'project-one', 'prompt_started', '{}', ?, 'delivered', ?)`,
		time.Now().Add(-10*24*time.Hour).UTC().Format(time.RFC3339Nano),
		time.Now().Add(-8*24*time.Hour).UTC().Format(time.RFC3339Nano),
	); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	if err := enforceLocalRetention(p); err != nil {
		t.Fatal(err)
	}
	db = openTestDB(t, p)
	defer db.Close()
	var count int
	if err := db.QueryRow(`select count(*) from events where local_event_id = 'old-delivered'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("old delivered event count = %d, want 0", count)
	}
}

func TestEnforceLocalRetentionDropsOldUndeliveredOnlyOverCap(t *testing.T) {
	previousCap := localRetentionCapBytes
	localRetentionCapBytes = 1
	defer func() { localRetentionCapBytes = previousCap }()

	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t, p)
	if _, err := db.Exec(
		`insert into events(
			local_event_id, local_sequence, project_id, type, payload_json,
			created_at_utc, delivery_state
		) values
			('old-pending', 1, 'project-one', 'prompt_started', '{}', ?, 'pending'),
			('fresh-pending', 2, 'project-one', 'prompt_started', '{}', ?, 'pending')`,
		time.Now().Add(-31*24*time.Hour).UTC().Format(time.RFC3339Nano),
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	if err := enforceLocalRetention(p); err != nil {
		t.Fatal(err)
	}
	db = openTestDB(t, p)
	defer db.Close()
	var oldCount int
	var freshCount int
	if err := db.QueryRow(`select count(*) from events where local_event_id = 'old-pending'`).Scan(&oldCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`select count(*) from events where local_event_id = 'fresh-pending'`).Scan(&freshCount); err != nil {
		t.Fatal(err)
	}
	if oldCount != 0 || freshCount != 1 {
		t.Fatalf("pending counts old/fresh = %d/%d, want 0/1", oldCount, freshCount)
	}
}

func TestRedactHidesSecretWords(t *testing.T) {
	message := "token=abc password=def authorization=ghi cookie=jkl secret=mno"
	redacted := redact(message)
	for _, forbidden := range []string{"token", "password", "authorization", "cookie", "secret"} {
		if strings.Contains(strings.ToLower(redacted), forbidden) {
			t.Fatalf("redacted message still contains %q: %s", forbidden, redacted)
		}
	}
}

func TestStatusOutputGoldenCleanInstall(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}

	var output strings.Builder
	if err := writeStatus(&output, p, false); err != nil {
		t.Fatal(err)
	}

	expected := strings.Join([]string{
		"Vibeengine local status",
		"Engine: not running",
		"Local capture: ok",
		"Queued events: 0",
		"Local DB: <size>",
		"Retention: ok",
		"Cloud: not configured",
		"Update: not checked",
		"Subscription: unknown",
		"Projects: 0 mapped",
		"Adapters: none installed",
		"Command automation: unavailable",
		"",
	}, "\n")
	if got := normalizeStatusOutput(output.String()); got != expected {
		t.Fatalf("status output mismatch\n--- got ---\n%s--- want ---\n%s", got, expected)
	}
}

func TestStatusOutputGoldenAttentionState(t *testing.T) {
	p := testPaths(t)
	if err := ensureLocalState(p); err != nil {
		t.Fatal(err)
	}
	event, err := newHookEvent(hookEventInput{
		project:     "project-one",
		tool:        "codex",
		nativeEvent: "permission-asked",
		nativeID:    "permission-1",
		eventType:   "permission_requested",
		payload:     `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := persistHookEvent(p, event); err != nil {
		t.Fatal(err)
	}
	if err := setKV(p, "entitlement.state", "trialing"); err != nil {
		t.Fatal(err)
	}
	if err := setKV(p, "engine.state", "signed_out"); err != nil {
		t.Fatal(err)
	}
	if err := setKV(p, "engine.last_error", "engine_signed_out"); err != nil {
		t.Fatal(err)
	}
	if err := setKV(p, "installed_adapters:test", `["codex","claude_code"]`); err != nil {
		t.Fatal(err)
	}

	var output strings.Builder
	if err := writeStatus(&output, p, false); err != nil {
		t.Fatal(err)
	}

	expected := strings.Join([]string{
		"Vibeengine local status",
		"Engine: signed_out",
		"Local capture: attention",
		"Queued events: 1",
		"Oldest pending: <timestamp>",
		"Local DB: <size>",
		"Retention: ok",
		"Cloud: not configured",
		"Update: not checked",
		"Subscription: trialing",
		"Last error: engine_signed_out",
		"Action: open the web app, copy the install command, and run it in this repo again.",
		"Projects: 1 mapped",
		"  - project-one (active)",
		"Adapters: claude_code, codex",
		"Command automation: unavailable",
		"",
	}, "\n")
	if got := normalizeStatusOutput(output.String()); got != expected {
		t.Fatalf("status output mismatch\n--- got ---\n%s--- want ---\n%s", got, expected)
	}
}

func TestCloudReachabilityExplainsNetworkRestrictedEnvironments(t *testing.T) {
	dnsMessage := cloudReachability("https://vibeengine-status-smoke.invalid")
	for _, want := range []string{"cannot resolve API host", "proxy", "container network"} {
		if !strings.Contains(dnsMessage, want) {
			t.Fatalf("DNS diagnostic %q does not contain %q", dnsMessage, want)
		}
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedURL := "http://" + listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	reachabilityMessage := cloudReachability(closedURL)
	for _, want := range []string{"cannot reach API", "firewall", "container network"} {
		if !strings.Contains(reachabilityMessage, want) {
			t.Fatalf("reachability diagnostic %q does not contain %q", reachabilityMessage, want)
		}
	}
}

func normalizeStatusOutput(output string) string {
	lines := strings.Split(output, "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "Local DB: ") {
			lines[index] = "Local DB: <size>"
		}
		if strings.HasPrefix(line, "Oldest pending: ") {
			lines[index] = "Oldest pending: <timestamp>"
		}
	}
	return strings.Join(lines, "\n")
}

func testPaths(t *testing.T) paths {
	t.Helper()

	dir := t.TempDir()
	return paths{
		home:    dir,
		db:      filepath.Join(dir, "state.db"),
		authDir: filepath.Join(dir, "auth"),
		lock:    filepath.Join(dir, "engine.lock"),
	}
}

func openTestDB(t *testing.T, p paths) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func writeTestCredentials(t *testing.T, p paths, apiURL string, accessToken string) {
	t.Helper()

	credentials := localCredentials{
		APIBaseURL:           apiURL,
		AccountID:            "account-1",
		UserID:               "user-1",
		RefreshToken:         "refresh-token",
		AccessToken:          accessToken,
		AccessTokenExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		EngineAuthEpoch:      1,
		UpdatedAtUTC:         nowUTC(),
	}
	output, err := json.Marshal(credentials)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.authDir, "credentials.json"), output, 0o600); err != nil {
		t.Fatal(err)
	}
}

func persistTestEvent(t *testing.T, p paths, projectID, eventType, nativeID string) hookEvent {
	t.Helper()

	event, err := newHookEvent(hookEventInput{
		project:     projectID,
		tool:        "codex",
		nativeEvent: eventType,
		nativeID:    nativeID,
		eventType:   eventType,
		payload:     `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if inserted, err := persistHookEvent(p, event); err != nil {
		t.Fatal(err)
	} else if !inserted {
		t.Fatal("expected hook event insert")
	}
	return event
}

func assertEventDeliveryState(t *testing.T, p paths, eventID, want string) {
	t.Helper()

	db := openTestDB(t, p)
	defer db.Close()
	var deliveryState string
	if err := db.QueryRow(`select delivery_state from events where local_event_id = ?`, eventID).Scan(&deliveryState); err != nil {
		t.Fatal(err)
	}
	if deliveryState != want {
		t.Fatalf("delivery_state = %q, want %q", deliveryState, want)
	}
}

func longRunningTestCommand() string {
	if runtime.GOOS == "windows" {
		return "ping -n 30 127.0.0.1 > NUL"
	}
	return "sleep 30"
}

func assertCommandProcessState(t *testing.T, p paths, projectID, wantState, wantPausedBy string) {
	t.Helper()

	db := openTestDB(t, p)
	defer db.Close()
	var processState string
	var pausedBy sql.NullString
	if err := db.QueryRow(
		`select process_state, paused_by from command_state where project_id = ?`,
		projectID,
	).Scan(&processState, &pausedBy); err != nil {
		t.Fatal(err)
	}
	if processState != wantState {
		t.Fatalf("process_state = %q, want %q", processState, wantState)
	}
	if pausedBy.String != wantPausedBy {
		t.Fatalf("paused_by = %q, want %q", pausedBy.String, wantPausedBy)
	}
}
