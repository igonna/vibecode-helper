package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const version = "0.0.0-dev"

var localRetentionCapBytes int64 = 100 * 1024 * 1024

type paths struct {
	home    string
	db      string
	authDir string
	lock    string
}

func main() {
	log.SetFlags(0)

	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("vibeengine %s\n", version)
		return
	}

	command := "status"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "status":
		statusCmd(os.Args[2:])
	case "hook":
		hookCmd(os.Args[2:])
	case "adapter":
		adapterCmd(os.Args[2:])
	case "hooks":
		hooksCmd(os.Args[2:])
	case "bootstrap":
		bootstrapCmd(os.Args[2:])
	case "check-in":
		checkInCmd(os.Args[2:])
	case "engine":
		engineCmd(os.Args[2:])
	case "command":
		commandCmd(os.Args[2:])
	case "sync":
		syncCmd(os.Args[2:])
	case "update":
		updateCmd(os.Args[2:])
	default:
		log.Fatalf("unknown command: %s", command)
	}
}

const releasePublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIIBojANBgkqhkiG9w0BAQEFAAOCAY8AMIIBigKCAYEAjmlhBOP6vkkBBZ8fyZJA
/0ypF40SIqOfMmbWx3Grgjffv2hbUai7hkQHcQVcMuIDmTAJ40R7z7yZH3aDRM90
CremSpi861Mh2Oxd+Kj+S/wzrp33ST32nAIVmIUn1Aeeq+lilEjyW0KAhXE7FfQ0
Tm9AVrMPPVUNIlWwXQ5DM+3a/ccr5yi+E7tNqrE9jHTAPuIe/ci7Na6N48xdd9ws
fbbVHjUu4vxDyGlBnDgtc0Vbja2+XZ0VyQ5relT+kYUyEETeuxIULUiQ8neLN8SF
UiBte4/kvlFRRjCgeb5+lgROcJ3zhv9vcOw4oTap22J+J066Zt5hS2Hb50jEtL+B
aRgiu2K4WitwEl0/XPyKQJwuXtPp1VFoQ0ClkF4qHeDU4nxNA4ljTvIW4lkdZgxt
8QZJ6M2WxVFdcbrOJhm6sFkaOBTm2hZvp72YHrAUTtiqGRygUTdqydnGdRZMZBZs
gkKVAqzL9ZBW18B1C5HwxDQMqkHQwA7Tfh4SpLh256T3AgMBAAE=
-----END PUBLIC KEY-----`

func hookCmd(args []string) {
	if len(args) == 0 {
		log.Fatal("hook event is required")
	}

	nativeCommand := args[0]
	eventType, ok := canonicalHookEvents[nativeCommand]
	if !ok {
		log.Fatalf("unknown hook event: %s", nativeCommand)
	}

	flags := flag.NewFlagSet("hook "+nativeCommand, flag.ExitOnError)
	project := flags.String("project", "", "project id or local project name")
	tool := flags.String("tool", "", "tool adapter")
	nativeEvent := flags.String("native-event", nativeCommand, "native tool event name")
	nativeID := flags.String("native-id", "", "native event id")
	sessionID := flags.String("session", "", "session id")
	promptID := flags.String("prompt", "", "prompt instance id")
	payload := flags.String("payload", "{}", "safe metadata JSON payload")
	adapterVersion := flags.String("adapter-version", version, "adapter version")
	_ = flags.Parse(args[1:])

	if *project == "" {
		log.Fatal("--project is required")
	}
	if *tool == "" {
		log.Fatal("--tool is required")
	}

	p, err := resolvePaths()
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := ensureLocalState(p); err != nil {
		log.Fatal(redact(err.Error()))
	}

	event, err := newHookEvent(hookEventInput{
		project:        *project,
		tool:           *tool,
		nativeEvent:    *nativeEvent,
		nativeID:       *nativeID,
		sessionID:      *sessionID,
		promptID:       *promptID,
		eventType:      eventType,
		payload:        *payload,
		adapterVersion: *adapterVersion,
	})
	if err != nil {
		log.Fatal(redact(err.Error()))
	}

	inserted, err := persistHookEvent(p, event)
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if nativeCommand == "session-start" {
		if err := startBackgroundEngineIfSafe(p); err != nil {
			fmt.Fprintf(os.Stderr, "vibeengine: %s\n", redact(err.Error()))
		}
	}
	if inserted {
		fmt.Printf("recorded %s\n", event.eventType)
	} else {
		fmt.Printf("duplicate %s ignored\n", event.eventType)
	}
}

var canonicalHookEvents = map[string]string{
	"session-start":    "session_started",
	"session-end":      "session_ended",
	"prompt-started":   "prompt_started",
	"prompt-finished":  "prompt_finished",
	"prompt-failed":    "prompt_failed",
	"permission-asked": "permission_requested",
	"tool-started":     "tool_started",
	"tool-finished":    "tool_finished",
}

type hookEventInput struct {
	project        string
	tool           string
	nativeEvent    string
	nativeID       string
	sessionID      string
	promptID       string
	eventType      string
	payload        string
	adapterVersion string
}

type hookEvent struct {
	localEventID   string
	project        string
	tool           string
	nativeEvent    string
	nativeID       string
	sessionID      string
	promptID       string
	eventType      string
	payload        string
	adapterVersion string
	createdAtUTC   string
}

type nativeHookEvent struct {
	eventName string
	sessionID string
	promptID  string
	nativeID  string
	payload   string
}

func adapterCmd(args []string) {
	if len(args) < 2 {
		log.Fatal("usage: vibeengine adapter <codex|claude-code> <native-event> --project <project>")
	}

	adapter := normalizeAdapterName(args[0])
	nativeEvent := args[1]
	if adapter == "" {
		log.Fatalf("unsupported adapter: %s", args[0])
	}

	flags := flag.NewFlagSet("adapter "+args[0]+" "+nativeEvent, flag.ExitOnError)
	project := flags.String("project", "", "project id or local project name")
	owner := flags.String("owner", "", "generated hook owner marker")
	adapterVersion := flags.String("adapter-version", version, "adapter version")
	_ = flags.Parse(args[2:])
	_ = owner

	if *project == "" {
		log.Fatal("--project is required")
	}

	stdin, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if len(strings.TrimSpace(string(stdin))) == 0 {
		stdin = []byte(`{}`)
	}

	mapped, err := mapNativeHookEvent(adapter, nativeEvent, stdin)
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	eventType, ok := canonicalHookEvents[mapped.eventName]
	if !ok {
		log.Fatalf("unsupported mapped event: %s", mapped.eventName)
	}

	p, err := resolvePaths()
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := ensureLocalState(p); err != nil {
		log.Fatal(redact(err.Error()))
	}

	event, err := newHookEvent(hookEventInput{
		project:        *project,
		tool:           adapter,
		nativeEvent:    nativeEvent,
		nativeID:       mapped.nativeID,
		sessionID:      mapped.sessionID,
		promptID:       mapped.promptID,
		eventType:      eventType,
		payload:        mapped.payload,
		adapterVersion: *adapterVersion,
	})
	if err != nil {
		log.Fatal(redact(err.Error()))
	}

	if _, err := persistHookEvent(p, event); err != nil {
		log.Fatal(redact(err.Error()))
	}
	if mapped.eventName == "session-start" {
		if err := startBackgroundEngineIfSafe(p); err != nil {
			fmt.Fprintf(os.Stderr, "vibeengine: %s\n", redact(err.Error()))
		}
	}
}

func newHookEvent(input hookEventInput) (hookEvent, error) {
	if !json.Valid([]byte(input.payload)) {
		return hookEvent{}, errors.New("payload must be valid JSON")
	}

	localEventID, err := randomID("evt")
	if err != nil {
		return hookEvent{}, err
	}
	sessionID := input.sessionID
	if sessionID == "" {
		sessionID, err = randomID("ses")
		if err != nil {
			return hookEvent{}, err
		}
	}
	promptID := input.promptID
	if promptID == "" && strings.HasPrefix(input.eventType, "prompt_") {
		promptID, err = randomID("prm")
		if err != nil {
			return hookEvent{}, err
		}
	}

	return hookEvent{
		localEventID:   localEventID,
		project:        input.project,
		tool:           input.tool,
		nativeEvent:    input.nativeEvent,
		nativeID:       input.nativeID,
		sessionID:      sessionID,
		promptID:       promptID,
		eventType:      input.eventType,
		payload:        input.payload,
		adapterVersion: input.adapterVersion,
		createdAtUTC:   nowUTC(),
	}, nil
}

func persistHookEvent(p paths, event hookEvent) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return false, err
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 1500;`); err != nil {
		return false, err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`insert or ignore into projects(project_id, account_id, name, state)
		 values(?, 'unknown', ?, 'active')`,
		event.project,
		event.project,
	); err != nil {
		return false, err
	}

	result, err := tx.ExecContext(
		ctx,
		`insert or ignore into events(
			local_event_id,
			local_sequence,
			project_id,
			session_id,
			prompt_instance_id,
			tool_adapter,
			native_event_name,
			native_event_id,
			adapter_version,
			type,
			payload_json,
			created_at_utc,
			delivery_state
		)
		values(
			?,
			(select coalesce(max(local_sequence), 0) + 1 from events),
			?,
			?,
			?,
			?,
			?,
			nullif(?, ''),
			?,
			?,
			?,
			?,
			'pending'
		)`,
		event.localEventID,
		event.project,
		event.sessionID,
		event.promptID,
		event.tool,
		event.nativeEvent,
		event.nativeID,
		event.adapterVersion,
		event.eventType,
		event.payload,
		event.createdAtUTC,
	)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if rowsAffected == 0 {
		return false, tx.Commit()
	}

	if event.eventType == "prompt_started" && event.promptID != "" {
		if _, err := tx.ExecContext(
			ctx,
			`insert or ignore into active_prompts(
				prompt_instance_id,
				project_id,
				session_id,
				started_event_id,
				started_at_utc,
				state
			) values(?, ?, ?, ?, ?, 'active')`,
			event.promptID,
			event.project,
			event.sessionID,
			event.localEventID,
			event.createdAtUTC,
		); err != nil {
			return false, err
		}
	}

	if (event.eventType == "prompt_finished" || event.eventType == "prompt_failed") && event.promptID != "" {
		if _, err := tx.ExecContext(
			ctx,
			`update active_prompts
			 set finished_event_id = ?, finished_at_utc = ?, state = 'finished'
			 where prompt_instance_id = ? and state = 'active'`,
			event.localEventID,
			event.createdAtUTC,
			event.promptID,
		); err != nil {
			return false, err
		}
	}

	return true, tx.Commit()
}

func randomID(prefix string) (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(bytes[:]), nil
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func normalizeAdapterName(adapter string) string {
	switch adapter {
	case "codex":
		return "codex"
	case "claude", "claude-code", "claude_code":
		return "claude_code"
	default:
		return ""
	}
}

func mapNativeHookEvent(adapter, nativeEvent string, rawPayload []byte) (nativeHookEvent, error) {
	var payload map[string]any
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return nativeHookEvent{}, fmt.Errorf("native hook payload must be JSON: %w", err)
	}

	canonical, ok := nativeToCanonicalEvent(adapter, nativeEvent)
	if !ok {
		return nativeHookEvent{}, fmt.Errorf("unsupported %s hook event: %s", adapter, nativeEvent)
	}

	sessionID := stringField(payload, "session_id")
	promptID := stringField(payload, "turn_id")
	if promptID == "" {
		promptID = stringField(payload, "prompt_id")
	}
	toolUseID := stringField(payload, "tool_use_id")
	source := stringField(payload, "source")

	safePayload := map[string]any{
		"hook_event_name": nativeEvent,
	}
	for _, key := range []string{
		"session_id",
		"turn_id",
		"source",
		"tool_name",
		"tool_use_id",
		"permission_mode",
		"stop_hook_active",
		"trigger",
		"agent_type",
	} {
		if value, ok := payload[key]; ok && isSafeScalar(value) {
			safePayload[key] = value
		}
	}

	safeJSON, err := json.Marshal(safePayload)
	if err != nil {
		return nativeHookEvent{}, err
	}

	return nativeHookEvent{
		eventName: canonical,
		sessionID: sessionID,
		promptID:  promptID,
		nativeID:  nativeEventID(adapter, nativeEvent, sessionID, promptID, toolUseID, source),
		payload:   string(safeJSON),
	}, nil
}

func nativeToCanonicalEvent(adapter, nativeEvent string) (string, bool) {
	common := map[string]string{
		"SessionStart":      "session-start",
		"SessionEnd":        "session-end",
		"UserPromptSubmit":  "prompt-started",
		"Stop":              "prompt-finished",
		"StopFailure":       "prompt-failed",
		"PermissionRequest": "permission-asked",
		"PreToolUse":        "tool-started",
		"PostToolUse":       "tool-finished",
	}
	event, ok := common[nativeEvent]
	if !ok {
		return "", false
	}
	if adapter == "codex" && nativeEvent == "SessionEnd" {
		return "", false
	}
	return event, true
}

func nativeEventID(adapter, nativeEvent, sessionID, promptID, toolUseID, source string) string {
	parts := []string{adapter, nativeEvent}
	for _, part := range []string{sessionID, promptID, toolUseID, source} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ":")
}

func commandCmd(args []string) {
	if len(args) == 0 {
		log.Fatal("usage: vibeengine command <set|config|start|pause|resume|restart>")
	}

	p, err := resolvePaths()
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := ensureLocalState(p); err != nil {
		log.Fatal(redact(err.Error()))
	}

	switch args[0] {
	case "set":
		commandSetCmd(p, args[1:])
	case "config":
		commandConfigCmd(p, args[1:])
	case "start", "pause", "resume", "restart":
		commandActionCmd(p, args[0], args[1:])
	default:
		log.Fatalf("unknown command action: %s", args[0])
	}
}

func commandSetCmd(p paths, args []string) {
	flags := flag.NewFlagSet("command set", flag.ExitOnError)
	project := flags.String("project", "", "project id or local project name")
	_ = flags.Parse(args)
	if *project == "" {
		log.Fatal("--project is required")
	}
	commandText := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if commandText == "" {
		log.Fatal("command text is required after --")
	}
	hash := hashCommand(commandText)
	if err := storeApprovedCommand(p, *project, hash, commandText); err != nil {
		log.Fatal(redact(err.Error()))
	}
	fmt.Printf("Command automation configured for %s\n", *project)
}

func commandConfigCmd(p paths, args []string) {
	flags := flag.NewFlagSet("command config", flag.ExitOnError)
	project := flags.String("project", "", "project id or local project name")
	pauseDuringPrompt := flags.Bool("pause-during-prompt", false, "pause command while a prompt is active")
	noPauseDuringPrompt := flags.Bool("no-pause-during-prompt", false, "do not pause command while a prompt is active")
	autoPauseAfterReady := flags.Duration("auto-pause-after-ready", 0, "pause command after it has been ready for this duration")
	noAutoPause := flags.Bool("no-auto-pause-after-ready", false, "disable auto-pause after ready")
	healthCommand := flags.String("health-command", "", "local health command to run after command start")
	clearHealth := flags.Bool("clear-health-command", false, "disable health command")
	afterPrompt := flags.String("after-prompt", "", "command behavior after prompt finishes: resume, restart, or none")
	_ = flags.Parse(args)
	if *project == "" {
		log.Fatal("--project is required")
	}
	if err := updateCommandConfig(p, *project, commandConfigUpdate{
		pauseDuringPrompt:   *pauseDuringPrompt,
		noPauseDuringPrompt: *noPauseDuringPrompt,
		autoPauseAfterReady: *autoPauseAfterReady,
		noAutoPause:         *noAutoPause,
		healthCommand:       strings.TrimSpace(*healthCommand),
		clearHealth:         *clearHealth,
		afterPrompt:         strings.TrimSpace(*afterPrompt),
	}); err != nil {
		log.Fatal(redact(err.Error()))
	}
	fmt.Printf("Command automation updated for %s\n", *project)
}

func commandActionCmd(p paths, action string, args []string) {
	flags := flag.NewFlagSet("command "+action, flag.ExitOnError)
	project := flags.String("project", "", "project id or local project name")
	_ = flags.Parse(args)
	if *project == "" {
		log.Fatal("--project is required")
	}
	if err := enqueueLocalCommandAction(p, *project, action); err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := startBackgroundEngineIfSafe(p); err != nil {
		fmt.Fprintf(os.Stderr, "vibeengine: %s\n", redact(err.Error()))
	}
	fmt.Printf("Queued command %s for %s\n", action, *project)
}

type commandProposalPayload struct {
	Command                    string `json:"command"`
	PauseDuringPrompt          bool   `json:"pause_during_prompt"`
	AutoPauseAfterReadySeconds int    `json:"auto_pause_after_ready_seconds"`
	HealthCommand              string `json:"health_command"`
	AfterPrompt                string `json:"after_prompt"`
}

func syncCmd(args []string) {
	flags := flag.NewFlagSet("sync", flag.ExitOnError)
	approve := flags.String("approve", "", "proposal id to approve")
	reject := flags.String("reject", "", "proposal id to reject")
	confirmHash := flags.String("confirm-hash", "", "full command hash required for approval")
	_ = flags.Parse(args)

	p, err := resolvePaths()
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := ensureLocalState(p); err != nil {
		log.Fatal(redact(err.Error()))
	}

	if *approve != "" && *reject != "" {
		log.Fatal("use either --approve or --reject")
	}
	if *approve != "" {
		if err := approveCommandProposal(p, *approve, *confirmHash); err != nil {
			log.Fatal(redact(err.Error()))
		}
		fmt.Println("Proposal applied locally")
		return
	}
	if *reject != "" {
		if err := decideCommandProposal(p, *reject, "rejected"); err != nil {
			log.Fatal(redact(err.Error()))
		}
		fmt.Println("Proposal rejected locally")
		return
	}

	proposals, err := pendingCommandProposals(p)
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if len(proposals) == 0 {
		fmt.Println("No pending local approvals")
		return
	}
	for _, proposal := range proposals {
		payload, hash, err := parseCommandProposal(proposal.payloadJSON)
		if err != nil {
			log.Fatal(redact(err.Error()))
		}
		fmt.Printf("Proposal: %s\n", proposal.id)
		fmt.Printf("Project: %s\n", proposal.projectID)
		fmt.Printf("Command:\n%s\n", payload.Command)
		fmt.Printf("Command hash: %s\n", hash)
		fmt.Print("Type the full command hash to apply, or press Enter to skip: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(line) == hash {
			if err := approveCommandProposal(p, proposal.id, hash); err != nil {
				log.Fatal(redact(err.Error()))
			}
			fmt.Println("Proposal applied locally")
		}
	}
}

type localCommandProposal struct {
	id          string
	projectID   string
	payloadJSON string
}

func pendingCommandProposals(p paths) ([]localCommandProposal, error) {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(
		`select proposal_id, project_id, payload_json
		 from pending_approvals
		 where state = 'pending' and type = 'command_settings'
		 order by created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var proposals []localCommandProposal
	for rows.Next() {
		var proposal localCommandProposal
		if err := rows.Scan(&proposal.id, &proposal.projectID, &proposal.payloadJSON); err != nil {
			return nil, err
		}
		proposals = append(proposals, proposal)
	}
	return proposals, rows.Err()
}

func approveCommandProposal(p paths, proposalID, confirmHash string) error {
	proposal, err := loadCommandProposal(p, proposalID)
	if err != nil {
		return err
	}
	payload, hash, err := parseCommandProposal(proposal.payloadJSON)
	if err != nil {
		return err
	}
	if confirmHash != hash {
		return errors.New("full command hash confirmation is required")
	}
	if err := storeApprovedCommand(p, proposal.projectID, hash, payload.Command); err != nil {
		return err
	}
	update := commandConfigUpdate{
		pauseDuringPrompt:   payload.PauseDuringPrompt,
		noPauseDuringPrompt: !payload.PauseDuringPrompt,
		autoPauseAfterReady: time.Duration(payload.AutoPauseAfterReadySeconds) * time.Second,
		noAutoPause:         payload.AutoPauseAfterReadySeconds == 0,
		healthCommand:       payload.HealthCommand,
		clearHealth:         payload.HealthCommand == "",
		afterPrompt:         payload.AfterPrompt,
	}
	if err := updateCommandConfig(p, proposal.projectID, update); err != nil {
		return err
	}
	return decideCommandProposal(p, proposalID, "applied")
}

func loadCommandProposal(p paths, proposalID string) (localCommandProposal, error) {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return localCommandProposal{}, err
	}
	defer db.Close()
	var proposal localCommandProposal
	if err := db.QueryRow(
		`select proposal_id, project_id, payload_json
		 from pending_approvals
		 where proposal_id = ? and type = 'command_settings' and state = 'pending'`,
		proposalID,
	).Scan(&proposal.id, &proposal.projectID, &proposal.payloadJSON); err != nil {
		return localCommandProposal{}, err
	}
	return proposal, nil
}

func parseCommandProposal(raw string) (commandProposalPayload, string, error) {
	var payload commandProposalPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return commandProposalPayload{}, "", err
	}
	payload.Command = strings.TrimSpace(payload.Command)
	if payload.Command == "" {
		return commandProposalPayload{}, "", errors.New("proposal command is empty")
	}
	if payload.AfterPrompt == "" {
		payload.AfterPrompt = "resume"
	}
	return payload, hashCommand(payload.Command), nil
}

func decideCommandProposal(p paths, proposalID, state string) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(
		`update pending_approvals
		 set state = ?, decided_at = ?
		 where proposal_id = ? and state = 'pending'`,
		state,
		nowUTC(),
		proposalID,
	)
	return err
}

func hashCommand(commandText string) string {
	sum := sha256.Sum256([]byte(commandText))
	return hex.EncodeToString(sum[:])
}

func storeApprovedCommand(p paths, projectID, commandHash, commandText string) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	now := nowUTC()
	_, err = db.Exec(
		`insert into approved_commands(project_id, command_hash, command_preview, approved_at)
		 values(?, ?, ?, ?)
		 on conflict(project_id) do update set
			command_hash = excluded.command_hash,
			command_preview = excluded.command_preview,
			approved_at = excluded.approved_at`,
		projectID,
		commandHash,
		commandText,
		now,
	)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`insert into command_state(
			project_id, supervisor_state, command_hash, process_state,
			prompt_state, active_prompt_ids_json, last_transition_at
		) values(?, 'enabled', ?, 'stopped', 'idle', '[]', ?)
		on conflict(project_id) do update set
			supervisor_state = 'enabled',
			command_hash = excluded.command_hash,
			last_transition_at = excluded.last_transition_at`,
		projectID,
		commandHash,
		now,
	)
	return err
}

type commandConfigUpdate struct {
	pauseDuringPrompt   bool
	noPauseDuringPrompt bool
	autoPauseAfterReady time.Duration
	noAutoPause         bool
	healthCommand       string
	clearHealth         bool
	afterPrompt         string
}

func updateCommandConfig(p paths, projectID string, update commandConfigUpdate) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(
		`insert into command_state(
			project_id, supervisor_state, process_state, prompt_state,
			active_prompt_ids_json, last_transition_at
		) values(?, 'disabled', 'stopped', 'idle', '[]', ?)
		on conflict(project_id) do nothing`,
		projectID,
		nowUTC(),
	); err != nil {
		return err
	}
	if update.pauseDuringPrompt || update.noPauseDuringPrompt {
		value := 0
		if update.pauseDuringPrompt {
			value = 1
		}
		if _, err := db.Exec(`update command_state set pause_during_prompt = ? where project_id = ?`, value, projectID); err != nil {
			return err
		}
	}
	if update.autoPauseAfterReady > 0 || update.noAutoPause {
		seconds := int(update.autoPauseAfterReady.Seconds())
		if update.noAutoPause {
			seconds = 0
		}
		if _, err := db.Exec(`update command_state set auto_pause_after_ready = ? where project_id = ?`, seconds, projectID); err != nil {
			return err
		}
	}
	if update.healthCommand != "" {
		if err := setKV(p, "health_command:"+projectID, update.healthCommand); err != nil {
			return err
		}
		if _, err := db.Exec(`update command_state set health_mode = 'command' where project_id = ?`, projectID); err != nil {
			return err
		}
	}
	if update.clearHealth {
		if err := setKV(p, "health_command:"+projectID, ""); err != nil {
			return err
		}
		if _, err := db.Exec(`update command_state set health_mode = null, health_state = null where project_id = ?`, projectID); err != nil {
			return err
		}
	}
	if update.afterPrompt != "" {
		if update.afterPrompt != "resume" && update.afterPrompt != "restart" && update.afterPrompt != "none" {
			return errors.New("--after-prompt must be resume, restart, or none")
		}
		if err := setKV(p, "command_after_prompt:"+projectID, update.afterPrompt); err != nil {
			return err
		}
	}
	return nil
}

func enqueueLocalCommandAction(p paths, projectID, action string) error {
	actionID, err := randomID("act")
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(
		`insert into pending_actions(
			action_id, project_id, source, idempotency_key, type, state, created_at
		) values(?, ?, 'local', ?, ?, 'pending', ?)`,
		actionID,
		projectID,
		"local:"+projectID+":"+action+":"+actionID,
		"command_"+action,
		nowUTC(),
	)
	return err
}

func stringField(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func isSafeScalar(value any) bool {
	switch value.(type) {
	case string, bool, float64, nil:
		return true
	default:
		return false
	}
}

type stringListFlag []string

func (values *stringListFlag) String() string {
	return strings.Join(*values, ",")
}

func (values *stringListFlag) Set(value string) error {
	normalized := normalizeAdapterName(value)
	if normalized == "" {
		return fmt.Errorf("unsupported adapter: %s", value)
	}
	*values = append(*values, normalized)
	return nil
}

type hookConfigFile struct {
	Hooks map[string][]hookMatcherGroup `json:"hooks"`
}

type hookMatcherGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookHandler `json:"hooks"`
}

type hookHandler struct {
	Type           string `json:"type"`
	Command        string `json:"command"`
	CommandWindows string `json:"commandWindows,omitempty"`
	Timeout        int    `json:"timeout,omitempty"`
	StatusMessage  string `json:"statusMessage,omitempty"`
}

func hooksCmd(args []string) {
	if len(args) == 0 {
		log.Fatal("usage: vibeengine hooks install --repo <path> --project <project> --adapter <name>")
	}
	switch args[0] {
	case "install":
		installHooksCmd(args[1:])
	case "cleanup-legacy":
		cleanupLegacyHooksCmd(args[1:])
	default:
		log.Fatalf("unknown hooks command: %s", args[0])
	}
}

func installHooksCmd(args []string) {
	flags := flag.NewFlagSet("hooks install", flag.ExitOnError)
	repo := flags.String("repo", ".", "repository path")
	project := flags.String("project", "", "project id or local project name")
	var adapters stringListFlag
	flags.Var(&adapters, "adapter", "adapter to install; repeat for multiple adapters")
	_ = flags.Parse(args)

	if *project == "" {
		log.Fatal("--project is required")
	}
	if len(adapters) == 0 {
		log.Fatal("at least one --adapter is required")
	}

	if err := installHookAdapters(*repo, *project, adapters); err != nil {
		log.Fatal(redact(err.Error()))
	}
	fmt.Println("hooks installed")
}

func installHookAdapters(repo, project string, adapters []string) error {
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return err
	}
	if info, err := os.Stat(absRepo); err != nil {
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("repo is not a directory: %s", absRepo)
	}

	for _, adapter := range adapters {
		switch adapter {
		case "codex":
			if err := writeHookConfig(
				filepath.Join(absRepo, ".codex", "hooks.json"),
				adapter,
				codexGeneratedGroups(project),
			); err != nil {
				return err
			}
		case "claude_code":
			if err := writeHookConfig(
				filepath.Join(absRepo, ".claude", "settings.local.json"),
				adapter,
				claudeGeneratedGroups(project),
			); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported adapter: %s", adapter)
		}
	}
	return nil
}

func writeHookConfig(path, adapter string, generated map[string][]hookMatcherGroup) error {
	config, err := readHookConfig(path)
	if err != nil {
		return err
	}
	if config.Hooks == nil {
		config.Hooks = map[string][]hookMatcherGroup{}
	}

	for eventName, groups := range config.Hooks {
		config.Hooks[eventName] = removeOwnedHookHandlers(adapter, groups)
		if len(config.Hooks[eventName]) == 0 {
			delete(config.Hooks, eventName)
		}
	}
	for eventName, groups := range generated {
		config.Hooks[eventName] = append(config.Hooks[eventName], groups...)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	output = append(output, '\n')
	return os.WriteFile(path, output, 0o644)
}

func cleanupLegacyHooksCmd(args []string) {
	flags := flag.NewFlagSet("hooks cleanup-legacy", flag.ExitOnError)
	repo := flags.String("repo", ".", "repository path")
	_ = flags.Parse(args)
	if err := cleanupLegacyHookAdapters(*repo); err != nil {
		log.Fatal(redact(err.Error()))
	}
	fmt.Println("legacy hooks cleaned")
}

func cleanupLegacyHookAdapters(repo string) error {
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return err
	}
	for _, path := range []string{
		filepath.Join(absRepo, ".codex", "hooks.json"),
		filepath.Join(absRepo, ".claude", "settings.local.json"),
	} {
		if err := cleanupLegacyHookFile(path); err != nil {
			return err
		}
	}
	return nil
}

func cleanupLegacyHookFile(path string) error {
	config, err := readHookConfig(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	changed := false
	for eventName, groups := range config.Hooks {
		var kept []hookMatcherGroup
		for _, group := range groups {
			var handlers []hookHandler
			for _, handler := range group.Hooks {
				if isLegacyVibecodeHook(handler.Command) || isLegacyVibecodeHook(handler.CommandWindows) {
					changed = true
					continue
				}
				handlers = append(handlers, handler)
			}
			if len(handlers) > 0 {
				group.Hooks = handlers
				kept = append(kept, group)
			} else if len(group.Hooks) > 0 {
				changed = true
			}
		}
		if len(kept) == 0 {
			delete(config.Hooks, eventName)
		} else {
			config.Hooks[eventName] = kept
		}
	}
	if !changed {
		return nil
	}
	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	output = append(output, '\n')
	return os.WriteFile(path, output, 0o644)
}

func isLegacyVibecodeHook(command string) bool {
	return strings.Contains(command, "vibecode-dev-notify-helper-1") ||
		strings.Contains(command, "vibecode-helper") ||
		strings.Contains(command, "vibecode_helper_hook_root")
}

func readHookConfig(path string) (hookConfigFile, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return hookConfigFile{Hooks: map[string][]hookMatcherGroup{}}, nil
	}
	if err != nil {
		return hookConfigFile{}, err
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return hookConfigFile{Hooks: map[string][]hookMatcherGroup{}}, nil
	}

	var config hookConfigFile
	if err := json.Unmarshal(content, &config); err != nil {
		return hookConfigFile{}, err
	}
	if config.Hooks == nil {
		config.Hooks = map[string][]hookMatcherGroup{}
	}
	return config, nil
}

func removeOwnedHookHandlers(adapter string, groups []hookMatcherGroup) []hookMatcherGroup {
	var repaired []hookMatcherGroup
	for _, group := range groups {
		var kept []hookHandler
		for _, handler := range group.Hooks {
			if !isOwnedHookCommand(adapter, handler.Command) && !isOwnedHookCommand(adapter, handler.CommandWindows) {
				kept = append(kept, handler)
			}
		}
		if len(kept) > 0 {
			group.Hooks = kept
			repaired = append(repaired, group)
		}
	}
	return repaired
}

func isOwnedHookCommand(adapter, command string) bool {
	if !strings.Contains(command, "--owner vibeengine") {
		return false
	}
	aliases := []string{adapter}
	if adapter == "claude_code" {
		aliases = append(aliases, "claude-code", "claude")
	}
	for _, alias := range aliases {
		if strings.Contains(command, "vibeengine adapter "+alias+" ") ||
			strings.Contains(command, "vibeengine.exe adapter "+alias+" ") {
			return true
		}
	}
	return false
}

func codexGeneratedGroups(project string) map[string][]hookMatcherGroup {
	return generatedGroups("codex", project, []string{
		"SessionStart",
		"UserPromptSubmit",
		"Stop",
		"PermissionRequest",
		"PreToolUse",
		"PostToolUse",
	})
}

func claudeGeneratedGroups(project string) map[string][]hookMatcherGroup {
	return generatedGroups("claude-code", project, []string{
		"SessionStart",
		"SessionEnd",
		"UserPromptSubmit",
		"Stop",
		"StopFailure",
		"PermissionRequest",
		"PreToolUse",
		"PostToolUse",
	})
}

func generatedGroups(adapter, project string, events []string) map[string][]hookMatcherGroup {
	groups := map[string][]hookMatcherGroup{}
	for _, eventName := range events {
		groups[eventName] = []hookMatcherGroup{
			{
				Hooks: []hookHandler{
					{
						Type:           "command",
						Command:        generatedHookCommand(adapter, eventName, project, false),
						CommandWindows: generatedHookCommand(adapter, eventName, project, true),
						Timeout:        5,
						StatusMessage:  "Vibeengine",
					},
				},
			},
		}
	}
	return groups
}

func generatedHookCommand(adapter, eventName, project string, windows bool) string {
	binary := "vibeengine"
	if windows {
		binary = "vibeengine.exe"
	}
	return strings.Join([]string{
		binary,
		"adapter",
		adapter,
		eventName,
		"--project",
		shellQuote(project),
		"--owner",
		"vibeengine",
	}, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return (r < 'A' || r > 'Z') &&
			(r < 'a' || r > 'z') &&
			(r < '0' || r > '9') &&
			r != '_' && r != '-' && r != '.' && r != '/' && r != ':'
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

type bootstrapResponse struct {
	AccountID            string `json:"accountId"`
	ProjectID            string `json:"projectId"`
	ProjectName          string `json:"projectName"`
	UserID               string `json:"userId"`
	RefreshToken         string `json:"refreshToken"`
	AccessToken          string `json:"accessToken"`
	AccessTokenExpiresAt string `json:"accessTokenExpiresAt"`
	EngineAuthEpoch      int    `json:"engineAuthEpoch"`
}

type localCredentials struct {
	APIBaseURL           string `json:"api_base_url"`
	AccountID            string `json:"account_id"`
	UserID               string `json:"user_id"`
	RefreshToken         string `json:"refresh_token"`
	AccessToken          string `json:"access_token"`
	AccessTokenExpiresAt string `json:"access_token_expires_at"`
	EngineAuthEpoch      int    `json:"engine_auth_epoch"`
	UpdatedAtUTC         string `json:"updated_at_utc"`
}

func bootstrapCmd(args []string) {
	flags := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	api := flags.String("api", "", "hosted API base URL")
	token := flags.String("token", "", "one-time bootstrap token")
	repo := flags.String("repo", ".", "repository path")
	var adapters stringListFlag
	flags.Var(&adapters, "adapter", "adapter to install; repeat for multiple adapters")
	_ = flags.Parse(args)

	if *api == "" {
		log.Fatal("--api is required")
	}
	if *token == "" {
		log.Fatal("--token is required")
	}
	if len(adapters) == 0 {
		adapters = stringListFlag{"codex", "claude_code"}
	}

	p, err := resolvePaths()
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := ensureLocalState(p); err != nil {
		log.Fatal(redact(err.Error()))
	}

	response, err := exchangeBootstrapToken(*api, *token)
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := storeBootstrapResult(p, *api, *repo, adapters, response); err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := installHookAdapters(*repo, response.ProjectID, adapters); err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := startBackgroundEngineIfSafe(p); err != nil {
		fmt.Fprintf(os.Stderr, "vibeengine: %s\n", redact(err.Error()))
	}

	fmt.Printf("Installed Vibeengine for %s\n", response.ProjectName)
	fmt.Println("Hooks: installed")
	fmt.Println("Local capture: ok")
}

func exchangeBootstrapToken(api, token string) (bootstrapResponse, error) {
	baseURL := strings.TrimRight(api, "/")
	body, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		return bootstrapResponse{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		baseURL+"/api/engine/bootstrap",
		bytes.NewReader(body),
	)
	if err != nil {
		return bootstrapResponse{}, err
	}
	request.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	httpResponse, err := client.Do(request)
	if err != nil {
		return bootstrapResponse{}, err
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		return bootstrapResponse{}, err
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return bootstrapResponse{}, fmt.Errorf("bootstrap failed: %s", strings.TrimSpace(string(responseBody)))
	}

	var response bootstrapResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return bootstrapResponse{}, err
	}
	if response.AccountID == "" || response.ProjectID == "" || response.RefreshToken == "" {
		return bootstrapResponse{}, errors.New("bootstrap response is incomplete")
	}
	return response, nil
}

func storeBootstrapResult(p paths, api string, repo string, adapters []string, response bootstrapResponse) error {
	existing, err := readLocalCredentials(p)
	if err != nil {
		return err
	}
	if existing != nil && existing.AccountID != response.AccountID {
		return errors.New("this machine is already connected to another account; sign out or use a separate VIBEENGINE_HOME")
	}

	credentials := localCredentials{
		APIBaseURL:           strings.TrimRight(api, "/"),
		AccountID:            response.AccountID,
		UserID:               response.UserID,
		RefreshToken:         response.RefreshToken,
		AccessToken:          response.AccessToken,
		AccessTokenExpiresAt: response.AccessTokenExpiresAt,
		EngineAuthEpoch:      response.EngineAuthEpoch,
		UpdatedAtUTC:         nowUTC(),
	}
	credentialsJSON, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}
	credentialsJSON = append(credentialsJSON, '\n')
	if err := os.WriteFile(filepath.Join(p.authDir, "credentials.json"), credentialsJSON, 0o600); err != nil {
		return err
	}

	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(
		`insert into projects(
			project_id,
			account_id,
			name,
			repo_path,
			state,
			hooks_installed_at,
			last_status
		) values(?, ?, ?, ?, 'active', ?, 'Setup')
		on conflict(project_id) do update set
			account_id = excluded.account_id,
			name = excluded.name,
			repo_path = excluded.repo_path,
			state = 'active',
			hooks_installed_at = excluded.hooks_installed_at,
			last_status = excluded.last_status`,
		response.ProjectID,
		response.AccountID,
		response.ProjectName,
		absRepo,
		nowUTC(),
	)
	if err != nil {
		return err
	}

	adapterList, err := json.Marshal(adapters)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`insert into local_kv(key, value, updated_at)
		 values(?, ?, ?)
		 on conflict(key) do update set value = excluded.value, updated_at = excluded.updated_at`,
		"installed_adapters:"+response.ProjectID,
		string(adapterList),
		nowUTC(),
	)
	return err
}

func readLocalCredentials(p paths) (*localCredentials, error) {
	content, err := os.ReadFile(filepath.Join(p.authDir, "credentials.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var credentials localCredentials
	if err := json.Unmarshal(content, &credentials); err != nil {
		return nil, err
	}
	return &credentials, nil
}

type pendingEvent struct {
	LocalEventID     string          `json:"localEventId"`
	ProjectID        string          `json:"projectId"`
	SessionID        *string         `json:"sessionId,omitempty"`
	PromptInstanceID *string         `json:"promptInstanceId,omitempty"`
	ToolAdapter      string          `json:"toolAdapter"`
	NativeEventName  *string         `json:"nativeEventName,omitempty"`
	NativeEventID    *string         `json:"nativeEventId,omitempty"`
	Type             string          `json:"type"`
	PayloadJSON      json.RawMessage `json:"payloadJson,omitempty"`
	CreatedAtUTC     string          `json:"createdAtUtc"`
}

type checkInRequest struct {
	Events                    []pendingEvent         `json:"events"`
	ProjectStatuses           []projectStatusSummary `json:"projectStatuses"`
	LastAppliedRemoteActionID string                 `json:"lastAppliedRemoteActionId,omitempty"`
	ProposalResults           []localProposalResult  `json:"proposalResults,omitempty"`
}

type projectStatusSummary struct {
	ProjectID string `json:"projectId"`
	Status    string `json:"status"`
}

type localProposalResult struct {
	ProposalID string `json:"proposalId"`
	State      string `json:"state"`
}

type checkInResponse struct {
	AcceptedEventIDs []string `json:"acceptedEventIds"`
	Entitlement      struct {
		State string `json:"state"`
	} `json:"entitlement"`
	RemoteActions    []remoteActionInstruction    `json:"remoteActions"`
	CommandProposals []commandProposalInstruction `json:"commandProposals"`
}

type remoteActionInstruction struct {
	ID                string  `json:"id"`
	ProjectID         string  `json:"projectId"`
	Type              string  `json:"type"`
	TargetCommandHash *string `json:"targetCommandHash,omitempty"`
}

type commandProposalInstruction struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"projectId"`
	Type        string          `json:"type"`
	CommandHash *string         `json:"commandHash,omitempty"`
	PayloadJSON json.RawMessage `json:"payloadJson"`
}

type tokenRefreshResponse struct {
	AccessToken          string `json:"accessToken"`
	AccessTokenExpiresAt string `json:"accessTokenExpiresAt"`
	EngineAuthEpoch      int    `json:"engineAuthEpoch"`
}

type checkInResult struct {
	pending  int
	uploaded int
}

func checkInCmd(args []string) {
	flags := flag.NewFlagSet("check-in", flag.ExitOnError)
	limit := flags.Int("limit", 500, "maximum events to upload")
	_ = flags.Parse(args)

	p, err := resolvePaths()
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := ensureLocalState(p); err != nil {
		log.Fatal(redact(err.Error()))
	}

	result, err := checkInOnce(p, *limit)
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if result.pending == 0 {
		fmt.Println("No pending events")
		return
	}
	fmt.Printf("Uploaded %d event(s)\n", result.uploaded)
}

func checkInOnce(p paths, limit int) (checkInResult, error) {
	credentials, err := readLocalCredentials(p)
	if err != nil {
		return checkInResult{}, err
	}
	if credentials == nil || credentials.APIBaseURL == "" || credentials.RefreshToken == "" {
		return checkInResult{}, errors.New("engine is not connected; run the install command from the web app")
	}

	if err := refreshAccessTokenIfNeeded(p, credentials); err != nil {
		if isSignedOutError(err) {
			_ = setKV(p, "engine.state", "signed_out")
		}
		return checkInResult{}, err
	}

	events, err := loadPendingEvents(p, limit)
	if err != nil {
		return checkInResult{}, err
	}
	if len(events) == 0 {
		response, err := sendCheckIn(p, credentials, events)
		if err != nil {
			return checkInResult{}, err
		}
		if response.Entitlement.State != "" {
			_ = setKV(p, "entitlement.state", response.Entitlement.State)
		}
		if err := storeRemoteInstructions(p, response); err != nil {
			return checkInResult{}, err
		}
		_ = setKV(p, "engine.last_check_in_at", nowUTC())
		_ = setKV(p, "engine.last_error", "")
		return checkInResult{pending: 0}, nil
	}

	response, err := sendCheckIn(p, credentials, events)
	if err != nil {
		if isSignedOutError(err) {
			_ = setKV(p, "engine.state", "signed_out")
		}
		if isEntitlementInactiveError(err) {
			_ = setKV(p, "engine.state", "cloud_paused")
		}
		return checkInResult{}, err
	}
	if err := markEventsDelivered(p, response.AcceptedEventIDs); err != nil {
		return checkInResult{}, err
	}
	if response.Entitlement.State != "" {
		_ = setKV(p, "entitlement.state", response.Entitlement.State)
	}
	if err := storeRemoteInstructions(p, response); err != nil {
		return checkInResult{}, err
	}
	_ = setKV(p, "engine.last_check_in_at", nowUTC())
	_ = setKV(p, "engine.last_error", "")
	return checkInResult{pending: len(events), uploaded: len(response.AcceptedEventIDs)}, nil
}

func storeRemoteInstructions(p paths, response checkInResponse) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	for _, action := range response.RemoteActions {
		if !strings.HasPrefix(action.Type, "command_") {
			continue
		}
		targetHash := ""
		if action.TargetCommandHash != nil {
			targetHash = *action.TargetCommandHash
		}
		if _, err := db.Exec(
			`insert or ignore into pending_actions(
				action_id, project_id, source, idempotency_key, type,
				target_command_hash, state, created_at
			) values(?, ?, 'remote', ?, ?, nullif(?, ''), 'pending', ?)`,
			action.ID,
			action.ProjectID,
			"remote:"+action.ID,
			action.Type,
			targetHash,
			nowUTC(),
		); err != nil {
			return err
		}
	}
	for _, proposal := range response.CommandProposals {
		if proposal.Type != "command_settings" {
			continue
		}
		if _, err := db.Exec(
			`insert or ignore into pending_approvals(
				proposal_id, project_id, type, summary, payload_json, state, created_at
			) values(?, ?, ?, 'Command settings change', ?, 'pending', ?)`,
			proposal.ID,
			proposal.ProjectID,
			proposal.Type,
			string(proposal.PayloadJSON),
			nowUTC(),
		); err != nil {
			return err
		}
	}
	return nil
}

func refreshAccessTokenIfNeeded(p paths, credentials *localCredentials) error {
	expiresAt, err := time.Parse(time.RFC3339, credentials.AccessTokenExpiresAt)
	if err == nil && time.Until(expiresAt) > time.Minute && credentials.AccessToken != "" {
		return nil
	}

	body, err := json.Marshal(map[string]string{"refreshToken": credentials.RefreshToken})
	if err != nil {
		return err
	}
	responseBody, err := postJSON(credentials.APIBaseURL+"/api/engine/token", "", body)
	if err != nil {
		return err
	}
	var refreshed tokenRefreshResponse
	if err := json.Unmarshal(responseBody, &refreshed); err != nil {
		return err
	}
	credentials.AccessToken = refreshed.AccessToken
	credentials.AccessTokenExpiresAt = refreshed.AccessTokenExpiresAt
	credentials.EngineAuthEpoch = refreshed.EngineAuthEpoch
	credentials.UpdatedAtUTC = nowUTC()

	output, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}
	output = append(output, '\n')
	return os.WriteFile(filepath.Join(p.authDir, "credentials.json"), output, 0o600)
}

func loadPendingEvents(p paths, limit int) ([]pendingEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(
		`select local_event_id, project_id, session_id, prompt_instance_id,
			tool_adapter, native_event_name, native_event_id, type, payload_json,
			created_at_utc
		 from events
		 where delivery_state = 'pending'
		 order by local_sequence
		 limit ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []pendingEvent
	for rows.Next() {
		var event pendingEvent
		var payload string
		if err := rows.Scan(
			&event.LocalEventID,
			&event.ProjectID,
			&event.SessionID,
			&event.PromptInstanceID,
			&event.ToolAdapter,
			&event.NativeEventName,
			&event.NativeEventID,
			&event.Type,
			&payload,
			&event.CreatedAtUTC,
		); err != nil {
			return nil, err
		}
		if payload == "" {
			payload = "{}"
		}
		event.PayloadJSON = json.RawMessage(payload)
		events = append(events, event)
	}
	return events, rows.Err()
}

func sendCheckIn(p paths, credentials *localCredentials, events []pendingEvent) (checkInResponse, error) {
	statusByProject := map[string]string{}
	for _, event := range events {
		if status := localStatusFromEvent(event.Type); status != "" {
			statusByProject[event.ProjectID] = status
		}
	}
	statuses := make([]projectStatusSummary, 0, len(statusByProject))
	for projectID, status := range statusByProject {
		statuses = append(statuses, projectStatusSummary{ProjectID: projectID, Status: status})
	}

	lastApplied, _ := getKV(p, "last_applied_remote_action_id")
	proposalResults, err := localProposalResults(p)
	if err != nil {
		return checkInResponse{}, err
	}
	body, err := json.Marshal(checkInRequest{
		Events:                    events,
		ProjectStatuses:           statuses,
		LastAppliedRemoteActionID: lastApplied,
		ProposalResults:           proposalResults,
	})
	if err != nil {
		return checkInResponse{}, err
	}
	responseBody, err := postJSON(credentials.APIBaseURL+"/api/engine/check-in", credentials.AccessToken, body)
	if err != nil {
		return checkInResponse{}, err
	}
	var response checkInResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return checkInResponse{}, err
	}
	return response, nil
}

func localProposalResults(p paths) ([]localProposalResult, error) {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(
		`select proposal_id, state
		 from pending_approvals
		 where state in ('applied', 'rejected', 'failed')
		 order by decided_at desc
		 limit 100`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []localProposalResult
	for rows.Next() {
		var result localProposalResult
		if err := rows.Scan(&result.ProposalID, &result.State); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func postJSON(url, bearer string, body []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("content-type", "application/json")
	if bearer != "" {
		request.Header.Set("authorization", "Bearer "+bearer)
	}

	httpResponse, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed: %s", strings.TrimSpace(string(responseBody)))
	}
	return responseBody, nil
}

func markEventsDelivered(p paths, eventIDs []string) error {
	if len(eventIDs) == 0 {
		return nil
	}
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, eventID := range eventIDs {
		if _, err := tx.Exec(
			`update events
			 set delivery_state = 'delivered', delivered_at = ?
			 where local_event_id = ?`,
			nowUTC(),
			eventID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func engineCmd(args []string) {
	flags := flag.NewFlagSet("engine", flag.ExitOnError)
	once := flags.Bool("once", false, "run one upload cycle and exit")
	interval := flags.Duration("interval", 5*time.Second, "upload loop interval")
	_ = flags.Parse(args)

	p, err := resolvePaths()
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := ensureLocalState(p); err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := runEngineLoop(p, *once, *interval); err != nil {
		log.Fatal(redact(err.Error()))
	}
}

func runEngineLoop(p paths, once bool, interval time.Duration) error {
	release, err := acquireEngineLock(p)
	if err != nil {
		return errors.New("background engine is already running")
	}
	defer release()

	if interval <= 0 {
		interval = 5 * time.Second
	}
	if err := setKV(p, "engine.pid", fmt.Sprintf("%d", os.Getpid())); err != nil {
		return err
	}
	if err := setKV(p, "engine.state", "running"); err != nil {
		return err
	}
	defer setKV(p, "engine.state", "stopped")

	backoff := interval
	for {
		if !once {
			if err := checkForUpdatesIfDue(p); err != nil {
				_ = setUpdateState(p, "check_failed:"+redact(err.Error()), "")
			}
		}
		if err := runCommandSupervisorOnce(p); err != nil {
			_ = setKV(p, "command_supervisor.last_error", err.Error())
		}
		if err := enforceLocalRetention(p); err != nil {
			_ = setKV(p, "retention.last_error", err.Error())
		}
		_, err := checkInOnce(p, 500)
		if err != nil {
			_ = setKV(p, "engine.last_error", err.Error())
			if isSignedOutError(err) {
				return nil
			}
			if isEntitlementInactiveError(err) {
				time.Sleep(time.Minute)
				if once {
					return nil
				}
				continue
			}
			time.Sleep(backoff)
			if backoff < time.Minute {
				backoff *= 2
			}
		} else {
			backoff = interval
			if once {
				return nil
			}
			time.Sleep(interval)
		}
		if once {
			return nil
		}
	}
}

type commandState struct {
	projectID           string
	supervisorState     string
	commandHash         string
	processState        string
	promptState         string
	activePromptIDsJSON string
	pausedBy            string
	readySince          string
	pauseDuringPrompt   bool
	autoPauseSeconds    int
	healthMode          string
	healthState         string
	restartAttempts     int
	nextRestartAfter    string
}

func runCommandSupervisorOnce(p paths) error {
	if err := applyPendingCommandActions(p); err != nil {
		return err
	}
	if err := processCommandPromptEvents(p); err != nil {
		return err
	}
	return enforceCommandTimers(p)
}

func applyPendingCommandActions(p paths) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(
		`select action_id, project_id, source, type
		 from pending_actions
		 where state = 'pending' and type like 'command_%'
		 order by created_at
		 limit 25`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type action struct {
		id        string
		projectID string
		source    string
		action    string
	}
	var actions []action
	for rows.Next() {
		var item action
		var actionType string
		if err := rows.Scan(&item.id, &item.projectID, &item.source, &actionType); err != nil {
			return err
		}
		item.action = strings.TrimPrefix(actionType, "command_")
		actions = append(actions, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range actions {
		message := "applied"
		err := applyCommandAction(p, item.projectID, item.action)
		state := "applied"
		if err != nil {
			state = "failed"
			message = redact(err.Error())
		}
		if _, updateErr := db.Exec(
			`update pending_actions
			 set state = ?, applied_at = ?, result_message = ?
			 where action_id = ? and state = 'pending'`,
			state,
			nowUTC(),
			message,
			item.id,
		); updateErr != nil {
			return updateErr
		}
		if item.source == "remote" && state == "applied" {
			if err := setKV(p, "last_applied_remote_action_id", item.id); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyCommandAction(p paths, projectID, action string) error {
	state, err := loadCommandState(p, projectID)
	if err != nil {
		return err
	}
	if state == nil || state.supervisorState != "enabled" {
		return errors.New("command automation is not enabled")
	}
	if err := stopCommandIfProjectDeleted(p, projectID); err != nil {
		return err
	}

	switch action {
	case "start", "resume":
		return startCommandProcess(p, projectID, "local")
	case "pause":
		return stopCommandProcess(p, projectID, "local")
	case "restart":
		if err := registerRestartAttempt(p, projectID, state); err != nil {
			return err
		}
		_ = stopCommandProcess(p, projectID, "restart")
		return startCommandProcess(p, projectID, "restart")
	default:
		return fmt.Errorf("unsupported command action: %s", action)
	}
}

func processCommandPromptEvents(p paths) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(
		`select local_event_id, project_id, prompt_instance_id, type
		 from events
		 where supervisor_processed_at is null
		   and type in ('prompt_started', 'prompt_finished', 'prompt_failed')
		 order by local_sequence
		 limit 100`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type event struct {
		id        string
		projectID string
		promptID  sql.NullString
		eventType string
	}
	var events []event
	for rows.Next() {
		var item event
		if err := rows.Scan(&item.id, &item.projectID, &item.promptID, &item.eventType); err != nil {
			return err
		}
		events = append(events, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, event := range events {
		if err := applyPromptEventToCommand(p, event.projectID, event.promptID.String, event.eventType); err != nil {
			_ = setCommandLastError(p, event.projectID, err.Error())
		}
		if _, err := db.Exec(
			`update events set supervisor_processed_at = ? where local_event_id = ?`,
			nowUTC(),
			event.id,
		); err != nil {
			return err
		}
	}
	return nil
}

func applyPromptEventToCommand(p paths, projectID, promptID, eventType string) error {
	state, err := loadCommandState(p, projectID)
	if err != nil {
		return err
	}
	if state == nil || state.supervisorState != "enabled" {
		return nil
	}
	if err := stopCommandIfProjectDeleted(p, projectID); err != nil {
		return err
	}
	active := decodePromptIDs(state.activePromptIDsJSON)
	changed := false

	switch eventType {
	case "prompt_started":
		if promptID != "" && !containsString(active, promptID) {
			active = append(active, promptID)
			changed = true
		}
		if state.pauseDuringPrompt && state.processState == "running" {
			if err := stopCommandProcess(p, projectID, "prompt"); err != nil {
				return err
			}
			state.pausedBy = "prompt"
			state.processState = "paused"
			changed = true
		}
	case "prompt_finished", "prompt_failed":
		if promptID == "" || !containsString(active, promptID) {
			return nil
		}
		active = removeString(active, promptID)
		changed = true
		if len(active) == 0 {
			afterPrompt, _ := getKV(p, "command_after_prompt:"+projectID)
			if afterPrompt == "" {
				afterPrompt = "resume"
			}
			if afterPrompt == "restart" {
				_ = stopCommandProcess(p, projectID, "prompt_finished")
				if err := startCommandProcess(p, projectID, "prompt_finished"); err != nil {
					return err
				}
				state.pausedBy = ""
				state.processState = "running"
			} else if afterPrompt == "resume" && state.pausedBy == "prompt" {
				if err := startCommandProcess(p, projectID, "prompt_finished"); err != nil {
					return err
				}
				state.pausedBy = ""
				state.processState = "running"
			}
		}
	}
	if changed {
		return storePromptCommandState(p, projectID, active, state.pausedBy)
	}
	return nil
}

func enforceCommandTimers(p paths) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.Query(
		`select project_id
		 from command_state
		 where supervisor_state = 'enabled'
		   and process_state = 'running'
		   and auto_pause_after_ready > 0`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	var projects []string
	for rows.Next() {
		var projectID string
		if err := rows.Scan(&projectID); err != nil {
			return err
		}
		projects = append(projects, projectID)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, projectID := range projects {
		state, err := loadCommandState(p, projectID)
		if err != nil {
			return err
		}
		if state == nil || state.readySince == "" {
			continue
		}
		readySince, err := time.Parse(time.RFC3339Nano, state.readySince)
		if err != nil {
			continue
		}
		if time.Since(readySince) >= time.Duration(state.autoPauseSeconds)*time.Second {
			if err := stopCommandProcess(p, projectID, "auto_pause"); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadCommandState(p paths, projectID string) (*commandState, error) {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var state commandState
	var commandHash, processState, promptState, activeIDs, pausedBy, readySince, healthMode, healthState, nextRestartAfter sql.NullString
	var pauseDuringPrompt int
	if err := db.QueryRow(
		`select project_id, supervisor_state, command_hash, process_state,
			prompt_state, active_prompt_ids_json, paused_by, ready_since,
			pause_during_prompt, auto_pause_after_ready, health_mode, health_state,
			restart_attempts, next_restart_after
		 from command_state
		 where project_id = ?`,
		projectID,
	).Scan(
		&state.projectID,
		&state.supervisorState,
		&commandHash,
		&processState,
		&promptState,
		&activeIDs,
		&pausedBy,
		&readySince,
		&pauseDuringPrompt,
		&state.autoPauseSeconds,
		&healthMode,
		&healthState,
		&state.restartAttempts,
		&nextRestartAfter,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	state.commandHash = commandHash.String
	state.processState = processState.String
	state.promptState = promptState.String
	state.activePromptIDsJSON = activeIDs.String
	state.pausedBy = pausedBy.String
	state.readySince = readySince.String
	state.pauseDuringPrompt = pauseDuringPrompt == 1
	state.healthMode = healthMode.String
	state.healthState = healthState.String
	state.nextRestartAfter = nextRestartAfter.String
	return &state, nil
}

func startCommandProcess(p paths, projectID, reason string) error {
	state, err := loadCommandState(p, projectID)
	if err != nil {
		return err
	}
	if state == nil || state.commandHash == "" {
		return errors.New("no approved command configured")
	}
	if pid, _ := commandPID(p, projectID); pid > 0 && processAlive(pid) {
		return updateCommandRuntime(p, projectID, "running", "", "already running", "", "")
	}
	commandText, repoPath, err := approvedCommand(p, projectID, state.commandHash)
	if err != nil {
		return err
	}
	if commandText == "" {
		return errors.New("approved command is missing")
	}
	runID, err := randomID("run")
	if err != nil {
		return err
	}
	logFile, err := commandLogFile(p, projectID)
	if err != nil {
		return err
	}
	defer logFile.Close()

	command := shellCommand(commandText)
	if repoPath != "" {
		command.Dir = repoPath
	} else {
		command.Dir = p.home
	}
	command.Stdout = logFile
	command.Stderr = logFile
	command.Env = os.Environ()
	if err := command.Start(); err != nil {
		return err
	}
	if err := command.Process.Release(); err != nil {
		return err
	}
	if err := setKV(p, "command_pid:"+projectID, fmt.Sprintf("%d", command.Process.Pid)); err != nil {
		return err
	}
	if err := updateCommandRuntime(p, projectID, "running", runID, reason, "", ""); err != nil {
		return err
	}
	return runCommandHealthCheck(p, projectID, runID, command.Dir)
}

func stopCommandProcess(p paths, projectID, reason string) error {
	if pid, _ := commandPID(p, projectID); pid > 0 && processAlive(pid) {
		process, err := os.FindProcess(pid)
		if err == nil {
			_ = process.Kill()
		}
	}
	processState := "paused"
	if reason == "restart" {
		processState = "stopped"
	}
	return updateCommandRuntime(p, projectID, processState, "", reason, reason, "")
}

func approvedCommand(p paths, projectID, commandHash string) (string, string, error) {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return "", "", err
	}
	defer db.Close()
	var commandText string
	var repoPath sql.NullString
	if err := db.QueryRow(
		`select approved_commands.command_preview, projects.repo_path
		 from approved_commands
		 left join projects on projects.project_id = approved_commands.project_id
		 where approved_commands.project_id = ? and approved_commands.command_hash = ?`,
		projectID,
		commandHash,
	).Scan(&commandText, &repoPath); err != nil {
		return "", "", err
	}
	return commandText, repoPath.String, nil
}

func commandPID(p paths, projectID string) (int, error) {
	pidText, err := getKV(p, "command_pid:"+projectID)
	if err != nil || pidText == "" {
		return 0, err
	}
	var pid int
	if _, err := fmt.Sscanf(pidText, "%d", &pid); err != nil {
		return 0, err
	}
	return pid, nil
}

func updateCommandRuntime(p paths, projectID, processState, runID, reason, pausedBy, lastError string) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	readySince := sql.NullString{}
	if processState == "running" {
		readySince.Valid = true
		readySince.String = nowUTC()
	}
	_, err = db.Exec(
		`update command_state
		 set process_state = ?,
		     run_id = coalesce(nullif(?, ''), run_id),
		     paused_by = nullif(?, ''),
		     ready_since = ?,
		     last_error = nullif(?, ''),
		     last_transition_at = ?
		 where project_id = ?`,
		processState,
		runID,
		pausedBy,
		readySince,
		lastError,
		nowUTC(),
		projectID,
	)
	_ = reason
	return err
}

func runCommandHealthCheck(p paths, projectID, runID, dir string) error {
	healthCommand, _ := getKV(p, "health_command:"+projectID)
	healthCommand = strings.TrimSpace(healthCommand)
	if healthCommand == "" {
		return setCommandHealth(p, projectID, runID, "ready", "")
	}
	command := shellCommand(healthCommand)
	command.Dir = dir
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command = shellCommandContext(ctx, healthCommand)
	command.Dir = dir
	if err := command.Run(); err != nil {
		return setCommandHealth(p, projectID, runID, "failed", "health command failed")
	}
	return setCommandHealth(p, projectID, runID, "ready", "")
}

func setCommandHealth(p paths, projectID, runID, healthState, lastError string) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.Exec(
		`update command_state
		 set health_state = ?, last_error = nullif(?, ''), ready_since = ?
		 where project_id = ? and run_id = ?`,
		healthState,
		lastError,
		nowUTC(),
		projectID,
		runID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("stale health result ignored")
	}
	return nil
}

func setCommandLastError(p paths, projectID, message string) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`update command_state set last_error = ? where project_id = ?`, redact(message), projectID)
	return err
}

func shellCommand(commandText string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", commandText)
	}
	return exec.Command("sh", "-c", commandText)
}

func shellCommandContext(ctx context.Context, commandText string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", commandText)
	}
	return exec.CommandContext(ctx, "sh", "-c", commandText)
}

func commandLogFile(p paths, projectID string) (*os.File, error) {
	dir := filepath.Join(p.home, "command-logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, safeFileName(projectID)+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

func safeFileName(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('_')
		}
	}
	if builder.Len() == 0 {
		return "project"
	}
	return builder.String()
}

func registerRestartAttempt(p paths, projectID string, state *commandState) error {
	if state.nextRestartAfter != "" {
		next, err := time.Parse(time.RFC3339Nano, state.nextRestartAfter)
		if err == nil && time.Now().Before(next) {
			return errors.New("restart backoff is active")
		}
	}
	attempts := state.restartAttempts + 1
	var next any = nil
	if attempts >= 5 {
		next = time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano)
		attempts = 0
	}
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(
		`update command_state
		 set restart_attempts = ?, next_restart_after = ?
		 where project_id = ?`,
		attempts,
		next,
		projectID,
	)
	return err
}

func stopCommandIfProjectDeleted(p paths, projectID string) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	var state string
	if err := db.QueryRow(`select state from projects where project_id = ?`, projectID).Scan(&state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if state != "deleted" {
		return nil
	}
	_ = stopCommandProcess(p, projectID, "project_deleted")
	_, err = db.Exec(
		`update command_state
		 set supervisor_state = 'disabled', process_state = 'stopped', last_transition_at = ?
		 where project_id = ?`,
		nowUTC(),
		projectID,
	)
	return err
}

func decodePromptIDs(raw string) []string {
	if raw == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return values
}

func storePromptCommandState(p paths, projectID string, active []string, pausedBy string) error {
	raw, err := json.Marshal(active)
	if err != nil {
		return err
	}
	promptState := "idle"
	if len(active) > 0 {
		promptState = "active"
	}
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(
		`update command_state
		 set prompt_state = ?,
		     active_prompt_ids_json = ?,
		     paused_by = nullif(?, ''),
		     last_prompt_event_at = ?,
		     last_transition_at = ?
		 where project_id = ?`,
		promptState,
		string(raw),
		pausedBy,
		nowUTC(),
		nowUTC(),
		projectID,
	)
	return err
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	var filtered []string
	for _, value := range values {
		if value != target {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func startBackgroundEngineIfSafe(p paths) error {
	if engineRunning(p) {
		return nil
	}
	credentials, err := readLocalCredentials(p)
	if err != nil {
		return err
	}
	if credentials == nil || credentials.APIBaseURL == "" || credentials.RefreshToken == "" {
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	logPath := filepath.Join(p.home, "engine.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()

	process, err := os.StartProcess(
		executable,
		[]string{executable, "engine", "--interval", "5s"},
		&os.ProcAttr{
			Dir:   p.home,
			Files: []*os.File{os.Stdin, logFile, logFile},
			Env:   os.Environ(),
		},
	)
	if err != nil {
		return err
	}
	_ = process.Release()
	_ = setKV(p, "engine.pid", fmt.Sprintf("%d", process.Pid))
	_ = setKV(p, "engine.state", "starting")
	return nil
}

func engineRunning(p paths) bool {
	pidText, err := getKV(p, "engine.pid")
	if err != nil || pidText == "" {
		return false
	}
	var pid int
	if _, err := fmt.Sscanf(pidText, "%d", &pid); err != nil || pid <= 0 {
		return false
	}
	return processAlive(pid)
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return process.Signal(os.Signal(nil)) == nil
}

func isSignedOutError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "engine_signed_out")
}

func isEntitlementInactiveError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "entitlement_inactive")
}

func localStatusFromEvent(eventType string) string {
	switch eventType {
	case "prompt_started":
		return "Prompting"
	case "prompt_finished":
		return "Ready"
	case "prompt_failed":
		return "Attention"
	case "permission_requested":
		return "Attention"
	default:
		return ""
	}
}

func updateCmd(args []string) {
	flags := flag.NewFlagSet("update", flag.ExitOnError)
	checkOnly := flags.Bool("check", false, "check whether a signed update is available")
	apply := flags.Bool("apply", false, "download, verify, and install a signed update")
	_ = flags.Parse(args)
	if !*checkOnly && !*apply {
		*checkOnly = true
	}

	p, err := resolvePaths()
	if err != nil {
		log.Fatal(redact(err.Error()))
	}
	if err := ensureLocalState(p); err != nil {
		log.Fatal(redact(err.Error()))
	}

	result, err := checkSignedUpdate(p)
	if err != nil {
		_ = setUpdateState(p, "check_failed:"+redact(err.Error()), "")
		log.Fatal(redact(err.Error()))
	}
	if !result.available {
		_ = setUpdateState(p, "current", "")
		fmt.Println("Vibeengine is current")
		return
	}
	if *checkOnly && !*apply {
		_ = setUpdateState(p, "update_available", "")
		fmt.Printf("Signed update available: %s\n", result.artifact)
		return
	}
	if err := applySignedUpdate(p, result); err != nil {
		_ = setUpdateState(p, "apply_failed:"+redact(err.Error()), "")
		log.Fatal(redact(err.Error()))
	}
	fmt.Println("Signed update installed")
}

type signedUpdate struct {
	available bool
	artifact  string
	body      []byte
	sha256    string
}

func checkForUpdatesIfDue(p paths) error {
	lastCheck, _ := getKV(p, "update.last_check_at")
	if lastCheck != "" {
		parsed, err := time.Parse(time.RFC3339Nano, lastCheck)
		if err == nil && time.Since(parsed) < 12*time.Hour {
			return nil
		}
	}
	result, err := checkSignedUpdate(p)
	if err != nil {
		return err
	}
	if result.available {
		return setUpdateState(p, "update_available", "")
	}
	return setUpdateState(p, "current", "")
}

func checkSignedUpdate(p paths) (signedUpdate, error) {
	credentials, err := readLocalCredentials(p)
	if err != nil {
		return signedUpdate{}, err
	}
	if credentials == nil || credentials.APIBaseURL == "" {
		return signedUpdate{}, errors.New("engine is not connected")
	}
	artifact, err := runtimeArtifactName()
	if err != nil {
		return signedUpdate{}, err
	}
	body, signature, expectedHash, err := downloadSignedArtifact(credentials.APIBaseURL, artifact)
	if err != nil {
		return signedUpdate{}, err
	}
	if err := verifySignedArtifact(body, signature, expectedHash, releasePublicKeyPEM); err != nil {
		return signedUpdate{}, err
	}
	currentHash, err := currentExecutableHash()
	if err != nil {
		return signedUpdate{}, err
	}
	update := signedUpdate{
		available: currentHash != expectedHash,
		artifact:  artifact,
		body:      body,
		sha256:    expectedHash,
	}
	return update, nil
}

func downloadSignedArtifact(apiBaseURL, artifact string) ([]byte, []byte, string, error) {
	body, err := getURL(strings.TrimRight(apiBaseURL, "/") + "/releases/" + artifact)
	if err != nil {
		return nil, nil, "", err
	}
	signature, err := getURL(strings.TrimRight(apiBaseURL, "/") + "/releases/" + artifact + ".sig")
	if err != nil {
		return nil, nil, "", err
	}
	checksumBody, err := getURL(strings.TrimRight(apiBaseURL, "/") + "/releases/" + artifact + ".sha256")
	if err != nil {
		return nil, nil, "", err
	}
	fields := strings.Fields(string(checksumBody))
	if len(fields) == 0 {
		return nil, nil, "", errors.New("release checksum is empty")
	}
	return body, signature, strings.ToLower(fields[0]), nil
}

func verifySignedArtifact(body, signature []byte, expectedHash, publicKeyPEM string) error {
	sum := sha256.Sum256(body)
	actualHash := hex.EncodeToString(sum[:])
	if actualHash != strings.ToLower(expectedHash) {
		return errors.New("release checksum verification failed")
	}
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return errors.New("release public key is invalid")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return errors.New("release public key is not RSA")
	}
	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, sum[:], signature)
}

func currentExecutableHash() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	body, err := os.ReadFile(executable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func applySignedUpdate(p paths, update signedUpdate) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return applySignedUpdateToPath(p, executable, update.body, update.sha256)
}

func applySignedUpdateToPath(p paths, executable string, body []byte, expectedHash string) error {
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != expectedHash {
		return errors.New("release body changed before install")
	}
	info, err := os.Stat(executable)
	if err != nil {
		return err
	}
	rollbackPath := executable + ".rollback"
	tmpPath := executable + ".new"
	if err := os.WriteFile(tmpPath, body, info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Rename(executable, rollbackPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, executable); err != nil {
		_ = os.Rename(rollbackPath, executable)
		_ = os.Remove(tmpPath)
		return err
	}
	return setUpdateState(p, "updated", rollbackPath)
}

func runtimeArtifactName() (string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if goos != "linux" && goos != "darwin" && goos != "windows" {
		return "", fmt.Errorf("unsupported update OS: %s", goos)
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", fmt.Errorf("unsupported update architecture: %s", goarch)
	}
	name := "vibeengine-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name, nil
}

func setUpdateState(p paths, result string, rollbackPath string) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(
		`delete from update_state`,
	)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`insert into update_state(current_version, channel, last_check_at, last_result, previous_version, rollback_path)
		 values(?, 'stable', ?, ?, ?, nullif(?, ''))`,
		version,
		nowUTC(),
		result,
		version,
		rollbackPath,
	)
	if err == nil {
		_ = setKV(p, "update.last_check_at", nowUTC())
	}
	return err
}

func readUpdateStatus(p paths) string {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return "unknown"
	}
	defer db.Close()
	var result sql.NullString
	var checked sql.NullString
	if err := db.QueryRow(`select last_result, last_check_at from update_state limit 1`).Scan(&result, &checked); err != nil {
		return "not checked"
	}
	if result.String == "" {
		return "not checked"
	}
	if checked.String != "" {
		return result.String + " at " + checked.String
	}
	return result.String
}

func enforceLocalRetention(p paths) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	deliveredBefore := time.Now().Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(
		`delete from events
		 where delivery_state = 'delivered'
		   and coalesce(delivered_at, created_at_utc) < ?`,
		deliveredBefore,
	); err != nil {
		return err
	}
	size, err := localDBSize(p)
	if err != nil {
		return err
	}
	if size <= localRetentionCapBytes {
		return nil
	}
	undeliveredBefore := time.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)
	_, err = db.Exec(
		`delete from events
		 where delivery_state != 'delivered'
		   and created_at_utc < ?`,
		undeliveredBefore,
	)
	return err
}

func getURL(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("download failed: HTTP %d", response.StatusCode)
	}
	return body, nil
}

func statusCmd(args []string) {
	flags := flag.NewFlagSet("status", flag.ExitOnError)
	verbose := flags.Bool("verbose", false, "show implementation details")
	noRepair := flags.Bool("no-repair", false, "do not start background engine")
	_ = flags.Parse(args)

	p, err := resolvePaths()
	if err != nil {
		log.Fatal(redact(err.Error()))
	}

	if err := ensureLocalState(p); err != nil {
		log.Fatal(redact(err.Error()))
	}
	if !*noRepair {
		if err := startBackgroundEngineIfSafe(p); err != nil {
			fmt.Fprintf(os.Stderr, "vibeengine: %s\n", redact(err.Error()))
		}
	}

	if err := writeStatus(os.Stdout, p, *verbose); err != nil {
		log.Fatal(redact(err.Error()))
	}
}

func writeStatus(w io.Writer, p paths, verbose bool) error {
	credentials, err := readLocalCredentials(p)
	if err != nil {
		return err
	}
	pendingCount, oldestPending, err := pendingEventStats(p)
	if err != nil {
		return err
	}
	dbSize, err := localDBSize(p)
	if err != nil {
		return err
	}
	projectMappings, err := localProjectMappings(p)
	if err != nil {
		return err
	}
	adapters, err := installedAdapters(p)
	if err != nil {
		return err
	}
	commandSummary, err := commandAutomationSummary(p)
	if err != nil {
		return err
	}
	lastError, _ := getKV(p, "engine.last_error")
	lastCheckIn, _ := getKV(p, "engine.last_check_in_at")
	cloud := "not configured"
	if credentials != nil && credentials.APIBaseURL != "" {
		cloud = cloudReachability(credentials.APIBaseURL)
	}
	engineState := "not running"
	if engineRunning(p) {
		engineState = "running"
	} else if state, _ := getKV(p, "engine.state"); state != "" && state != "stopped" {
		engineState = state
	}
	retention := "ok"
	if dbSize > 80*1024*1024 {
		retention = "warning: local queue is approaching the 100 MB cap"
	}
	capture := "ok"
	if pendingCount > 0 {
		capture = fmt.Sprintf("ok, %d queued", pendingCount)
	}
	if lastError != "" {
		capture = "attention"
	}

	fmt.Fprintln(w, "Vibeengine local status")
	fmt.Fprintf(w, "Engine: %s\n", engineState)
	fmt.Fprintf(w, "Local capture: %s\n", capture)
	fmt.Fprintf(w, "Queued events: %d\n", pendingCount)
	if oldestPending != "" {
		fmt.Fprintf(w, "Oldest pending: %s\n", oldestPending)
	}
	fmt.Fprintf(w, "Local DB: %s\n", formatBytes(dbSize))
	fmt.Fprintf(w, "Retention: %s\n", retention)
	fmt.Fprintf(w, "Cloud: %s\n", cloud)
	fmt.Fprintf(w, "Update: %s\n", readUpdateStatus(p))
	entitlement, _ := getKV(p, "entitlement.state")
	if entitlement == "" {
		entitlement = "unknown"
	}
	fmt.Fprintf(w, "Subscription: %s\n", entitlement)
	if lastCheckIn != "" {
		fmt.Fprintf(w, "Last check-in: %s\n", lastCheckIn)
	}
	if lastError != "" {
		fmt.Fprintf(w, "Last error: %s\n", redact(lastError))
		if isSignedOutError(errors.New(lastError)) {
			fmt.Fprintln(w, "Action: open the web app, copy the install command, and run it in this repo again.")
		}
	}
	fmt.Fprintf(w, "Projects: %d mapped\n", len(projectMappings))
	for _, mapping := range projectMappings {
		fmt.Fprintf(w, "  - %s\n", mapping)
	}
	if len(adapters) > 0 {
		fmt.Fprintf(w, "Adapters: %s\n", strings.Join(adapters, ", "))
	} else {
		fmt.Fprintln(w, "Adapters: none installed")
	}
	fmt.Fprintf(w, "Command automation: %s\n", commandSummary)
	if verbose {
		fmt.Fprintf(w, "State home: %s\n", redactPath(p.home))
	}
	return nil
}

func pendingEventStats(p paths) (int, string, error) {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return 0, "", err
	}
	defer db.Close()
	var count int
	var oldest sql.NullString
	if err := db.QueryRow(
		`select count(*), min(created_at_utc) from events where delivery_state = 'pending'`,
	).Scan(&count, &oldest); err != nil {
		return 0, "", err
	}
	if !oldest.Valid {
		return count, "", nil
	}
	return count, oldest.String, nil
}

func localProjectMappings(p paths) ([]string, error) {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`select name, state from projects order by name limit 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var mappings []string
	for rows.Next() {
		var name, state string
		if err := rows.Scan(&name, &state); err != nil {
			return nil, err
		}
		mappings = append(mappings, fmt.Sprintf("%s (%s)", name, state))
	}
	return mappings, rows.Err()
}

func commandAutomationSummary(p paths) (string, error) {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return "", err
	}
	defer db.Close()
	var configured int
	var running int
	if err := db.QueryRow(
		`select
			coalesce(sum(case when supervisor_state = 'enabled' then 1 else 0 end), 0),
			coalesce(sum(case when supervisor_state = 'enabled' and process_state = 'running' then 1 else 0 end), 0)
		 from command_state`,
	).Scan(&configured, &running); err != nil {
		return "", err
	}
	if configured == 0 {
		return "unavailable", nil
	}
	if running > 0 {
		return fmt.Sprintf("%d configured, %d running", configured, running), nil
	}
	return fmt.Sprintf("%d configured", configured), nil
}

func installedAdapters(p paths) ([]string, error) {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`select value from local_kv where key like 'installed_adapters:%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var values []string
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			continue
		}
		for _, value := range values {
			seen[value] = true
		}
	}
	var values []string
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values, rows.Err()
}

func localDBSize(p paths) (int64, error) {
	info, err := os.Stat(p.db)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func cloudReachability(apiBaseURL string) string {
	parsed, err := url.Parse(apiBaseURL)
	if err != nil || parsed.Hostname() == "" {
		return "invalid API URL"
	}
	if _, err := net.LookupHost(parsed.Hostname()); err != nil {
		return "cannot resolve API host; check DNS, proxy, VPN, or container network"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(apiBaseURL, "/")+"/api/health", nil)
	if err != nil {
		return "invalid health request"
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		return "cannot reach API; check proxy, firewall, VPN, or container network"
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Sprintf("API returned HTTP %d", response.StatusCode)
	}
	if parsed.Scheme != "https" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" {
		return "reachable, but HTTPS is not enabled"
	}
	return "reachable"
}

func formatBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func redactPath(path string) string {
	if path == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func resolvePaths() (paths, error) {
	home := os.Getenv("VIBEENGINE_HOME")
	if home == "" {
		var base string
		switch runtime.GOOS {
		case "windows":
			base = os.Getenv("LOCALAPPDATA")
			if base == "" {
				return paths{}, errors.New("LOCALAPPDATA is not set")
			}
		case "darwin":
			userHome, err := os.UserHomeDir()
			if err != nil {
				return paths{}, err
			}
			base = filepath.Join(userHome, "Library", "Application Support")
		default:
			base = os.Getenv("XDG_DATA_HOME")
			if base == "" {
				userHome, err := os.UserHomeDir()
				if err != nil {
					return paths{}, err
				}
				base = filepath.Join(userHome, ".local", "share")
			}
		}
		home = filepath.Join(base, "vibeengine")
	}

	return paths{
		home:    home,
		db:      filepath.Join(home, "state.db"),
		authDir: filepath.Join(home, "auth"),
		lock:    filepath.Join(home, "engine.lock"),
	}, nil
}

func ensureLocalState(p paths) error {
	if err := os.MkdirAll(p.home, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(p.authDir, 0o700); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		return err
	}
	if _, err := db.Exec(`PRAGMA synchronous = NORMAL;`); err != nil {
		return err
	}

	for _, statement := range localMigrations {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}

	if err := os.Chmod(p.home, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(p.authDir, 0o700); err != nil {
		return err
	}
	return nil
}

func acquireEngineLock(p paths) (func() error, error) {
	file, err := os.OpenFile(p.lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}

	if _, err := fmt.Fprintf(file, "%d\n", os.Getpid()); err != nil {
		_ = file.Close()
		_ = os.Remove(p.lock)
		return nil, err
	}

	return func() error {
		closeErr := file.Close()
		removeErr := os.Remove(p.lock)
		if closeErr != nil {
			return closeErr
		}
		return removeErr
	}, nil
}

func setKV(p paths, key string, value string) error {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(
		`insert into local_kv(key, value, updated_at)
		 values(?, ?, ?)
		 on conflict(key) do update set value = excluded.value, updated_at = excluded.updated_at`,
		key,
		value,
		nowUTC(),
	)
	return err
}

func getKV(p paths, key string) (string, error) {
	db, err := sql.Open("sqlite", p.db)
	if err != nil {
		return "", err
	}
	defer db.Close()
	var value string
	if err := db.QueryRow(`select value from local_kv where key = ?`, key).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

var localMigrations = []string{
	`create table if not exists local_kv (
		key text primary key,
		value text not null,
		updated_at text not null
	);`,
	`create table if not exists projects (
		project_id text primary key,
		account_id text not null,
		name text not null,
		repo_path text,
		state text not null,
		hooks_installed_at text,
		last_cloud_seen_at text,
		last_status text
	);`,
	`create table if not exists events (
		local_event_id text primary key,
		local_sequence integer,
		project_id text not null,
		session_id text,
		prompt_instance_id text,
		tool_adapter text,
		native_event_name text,
		native_event_id text,
		adapter_version text,
		type text not null,
		payload_json text,
		created_at_utc text not null,
		delivery_state text not null,
		delivery_attempts integer not null default 0,
		delivered_at text,
		last_error text,
		supervisor_processed_at text
	);`,
	`create table if not exists pending_actions (
		action_id text primary key,
		project_id text not null,
		source text not null,
		idempotency_key text not null,
		type text not null,
		target_command_hash text,
		payload_json text,
		state text not null,
		attempt_count integer not null default 0,
		lease_until text,
		created_at text not null,
		applied_at text,
		result_message text
	);`,
	`create table if not exists active_prompts (
		prompt_instance_id text primary key,
		project_id text not null,
		session_id text,
		started_event_id text,
		finished_event_id text,
		started_at_utc text not null,
		finished_at_utc text,
		stale_at_utc text,
		state text not null,
		pause_requested integer not null default 0
	);`,
	`create table if not exists pending_approvals (
		proposal_id text primary key,
		project_id text not null,
		type text not null,
		summary text,
		payload_json text,
		state text not null,
		created_at text not null,
		decided_at text
	);`,
	`create table if not exists approved_commands (
		project_id text primary key,
		command_hash text not null,
		command_preview text,
		approved_at text not null
	);`,
	`create table if not exists command_state (
		project_id text primary key,
		supervisor_state text not null,
		command_hash text,
		run_id text,
		process_state text,
		prompt_state text,
		active_prompt_ids_json text,
		last_prompt_event_at text,
		paused_by text,
		ready_since text,
		pause_during_prompt integer not null default 0,
		auto_pause_after_ready integer not null default 0,
		health_mode text,
		health_state text,
		restart_attempts integer not null default 0,
		next_restart_after text,
		last_error text,
		last_transition_at text
	);`,
	`create table if not exists update_state (
		current_version text,
		channel text,
		last_check_at text,
		last_result text,
		previous_version text,
		rollback_path text
	);`,
	`create index if not exists events_project_sequence_idx on events(project_id, local_sequence);`,
	`create unique index if not exists events_project_type_native_idx
		on events(project_id, type, native_event_id)
		where native_event_id is not null;`,
}

func redact(message string) string {
	replacements := []string{"token", "secret", "password", "authorization", "cookie"}
	redacted := message
	for _, word := range replacements {
		redacted = strings.ReplaceAll(redacted, word, "[redacted]")
		redacted = strings.ReplaceAll(redacted, strings.ToUpper(word), "[redacted]")
	}
	return redacted
}
