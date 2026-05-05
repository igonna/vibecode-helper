package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"vibecode-dev-notify/internal/protocol"
)

type Config struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	CWD                string   `json:"cwd"`
	CommandEnabled     bool     `json:"command_enabled"`
	Command            []string `json:"command"`
	HealthMode         string   `json:"health_mode"`
	HealthURL          string   `json:"health_url"`
	HealthCommand      []string `json:"health_command"`
	StartupTimeoutSecs int      `json:"startup_timeout_seconds"`
	RequiredRunner     int      `json:"required_runner_version"`
	RestartVersion     int      `json:"restart_version"`
}

type nextAction struct {
	Action  string `json:"action"`
	Version int    `json:"version"`
}

type processHandle struct {
	cmd      *exec.Cmd
	done     chan struct{}
	output   *tailBuffer
	mu       sync.Mutex
	err      error
	stopping bool
}

type tailBuffer struct {
	mu      sync.Mutex
	limit   int
	data    []byte
	version int64
}

type apiClient struct {
	baseURL         string
	dockerContainer string
}

const (
	commandInterruptGrace = 8 * time.Second
	commandTerminateGrace = 3 * time.Second
	runnerHeartbeatEvery  = 15 * time.Second
)

func Run(helperURL, appID, baseDir string) error {
	api := newAPIClient(helperURL)
	stopOlderRunners(appID)
	config := waitForConfig(api, appID)
	if config.RequiredRunner > protocol.RunnerVersion {
		if err := execFreshRunner(api, helperURL, appID, baseDir); err == nil {
			return nil
		}
		_ = postEvent(api, appID, "runner_update_required", "runner is older than helper", 1)
		return fmt.Errorf("runner update required")
	}
	if config.CommandEnabled && len(config.Command) == 0 {
		return errors.New("configured command is empty")
	}
	config = prepareConfig(config, baseDir)

	fmt.Printf("\033[32m✅ Runner connected. Check web dashboard.\033[0m\n")

	_ = postEvent(api, appID, "runner_connected", "runner started", 0)
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go runnerHeartbeat(api, appID, heartbeatDone)

	hooksChanged, err := installCodexHooks(api, appID, baseDir)
	if err != nil {
		_ = postEvent(api, appID, "codex_hooks_failed", err.Error(), 1)
	} else if hooksChanged {
		fmt.Printf("\033[36mCodex hooks are installed. Restart Codex once after first setup or hook changes.\033[0m\n")
		_ = postEvent(api, appID, "codex_hooks_installed", "Codex hooks changed; restart Codex once", 0)
	}
	var handle *processHandle
	if config.CommandEnabled {
		var err error
		handle, err = startCommand(api, appID, config)
		if err != nil {
			_ = postEvent(api, appID, "command_failed", err.Error(), 1)
			return err
		}
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGTSTP)
	go func() {
		sig := <-signals
		fmt.Printf("\nRunner stopping after %s. Dashboard will update shortly.\n", sig)
		stopped := make(chan struct{})
		go func() {
			_ = stopCommand(handle)
			close(stopped)
		}()
		select {
		case <-stopped:
		case second := <-signals:
			fmt.Printf("\nForce stopping after %s.\n", second)
			forceKillCommand(handle)
		}
		_ = postEvent(api, appID, "runner_disconnected", "runner stopped", 0)
		os.Exit(0)
	}()

	if config.CommandEnabled || config.HealthMode != "none" {
		if ok, detail := waitForHealth(api, appID, config, handle); ok {
			_ = postEvent(api, appID, "health_ok", detail, 0)
		} else {
			_ = postEvent(api, appID, "health_failed", detail, 1)
		}
	}

	latestConfig := waitForConfig(api, appID)
	seenVersion := latestConfig.RestartVersion
	connected := true
	for {
		action, err := waitForAction(api, appID, seenVersion)
		if err != nil {
			connected = false
			time.Sleep(2 * time.Second)
			continue
		}
		if !connected {
			_ = postEvent(api, appID, "runner_connected", "runner reconnected", 0)
			connected = true
		}
		seenVersion = action.Version
		if action.Action == "update_required" {
			if err := execFreshRunner(api, helperURL, appID, baseDir); err == nil {
				return nil
			}
			_ = postEvent(api, appID, "runner_update_required", "runner is older than helper", 1)
			continue
		}
		if action.Action == "stop" {
			handle = stopRunningCommand(api, appID, handle)
			continue
		}
		if action.Action != "restart" {
			continue
		}

		latestConfig, err := fetchConfig(api, appID)
		if err != nil {
			_ = postEvent(api, appID, "restart_failed", err.Error(), 1)
			continue
		}
		if latestConfig.RequiredRunner > protocol.RunnerVersion {
			if err := execFreshRunner(api, helperURL, appID, baseDir); err == nil {
				return nil
			}
			_ = postEvent(api, appID, "runner_update_required", "runner is older than helper", 1)
			continue
		}
		if latestConfig.CommandEnabled && len(latestConfig.Command) == 0 {
			_ = postEvent(api, appID, "restart_failed", "configured command is empty", 1)
			continue
		}
		config = prepareConfig(latestConfig, baseDir)
		handle = restartCommand(api, appID, config, handle)
	}
}

func runnerHeartbeat(api *apiClient, appID string, done <-chan struct{}) {
	ticker := time.NewTicker(runnerHeartbeatEvery)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			_ = postEvent(api, appID, "runner_heartbeat", "runner alive", 0)
		}
	}
}

func stopRunningCommand(api *apiClient, appID string, handle *processHandle) *processHandle {
	fmt.Println("Stop requested. Dashboard will show progress.")
	_ = postEvent(api, appID, "stop_started", "stopping command", 0)
	if err := stopCommand(handle); err != nil {
		_ = postEvent(api, appID, "stop_failed", err.Error(), 1)
		return handle
	}
	_ = postEvent(api, appID, "command_stopped", "command stopped", 0)
	return nil
}

func newAPIClient(helperURL string) *apiClient {
	trimmed := strings.TrimRight(helperURL, "/")
	if strings.HasPrefix(trimmed, "docker://") {
		return &apiClient{
			baseURL:         "http://127.0.0.1:8766",
			dockerContainer: strings.TrimPrefix(trimmed, "docker://"),
		}
	}
	return &apiClient{baseURL: trimmed}
}

func resolveWorkingDir(cwd, baseDir string) string {
	if cwd == "" {
		cwd = "."
	}
	if filepath.IsAbs(cwd) {
		return cwd
	}
	if baseDir == "" {
		if current, err := os.Getwd(); err == nil {
			baseDir = current
		}
	}
	if baseDir == "" {
		return cwd
	}
	return filepath.Clean(filepath.Join(baseDir, cwd))
}

func prepareConfig(config Config, baseDir string) Config {
	config.CWD = resolveWorkingDir(config.CWD, baseDir)
	return config
}

func waitForConfig(api *apiClient, appID string) Config {
	for {
		config, err := fetchConfig(api, appID)
		if err == nil {
			return config
		}
		time.Sleep(2 * time.Second)
	}
}

func execFreshRunner(api *apiClient, helperURL, appID, baseDir string) error {
	if api.dockerContainer == "" {
		return errors.New("automatic runner update requires docker transport")
	}

	tmpDir, err := os.MkdirTemp("", "vibecode-runner-*")
	if err != nil {
		return err
	}
	nextPath := filepath.Join(tmpDir, "vibecode-runner")
	copyCmd := exec.Command("docker", "cp", api.dockerContainer+":/usr/local/bin/vibecode-helper", nextPath)
	if output, err := copyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("copy fresh runner: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if err := os.Chmod(nextPath, 0o755); err != nil {
		return err
	}

	args := []string{nextPath, "runner", "--app", appID, "--helper-url", helperURL}
	if baseDir != "" {
		args = append(args, "--base-dir", baseDir)
	}
	return syscall.Exec(nextPath, args, os.Environ())
}

func (c *apiClient) get(path string, timeout time.Duration) ([]byte, error) {
	endpoint := c.baseURL + path
	if c.dockerContainer != "" {
		seconds := int(timeout.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		cmd := exec.Command("docker", "exec", c.dockerContainer, "wget", "-qO-", "-T", fmt.Sprint(seconds), endpoint)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("docker helper GET %s: %w: %s", path, err, strings.TrimSpace(string(output)))
		}
		return output, nil
	}

	client := &http.Client{Timeout: timeout}
	response, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func (c *apiClient) post(path string, payload []byte, timeout time.Duration) error {
	endpoint := c.baseURL + path
	if c.dockerContainer != "" {
		seconds := int(timeout.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		cmd := exec.Command("docker", "exec", c.dockerContainer, "wget", "-qO-", "-T", fmt.Sprint(seconds), "--header", "Content-Type: application/json", "--post-data", string(payload), endpoint)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("docker helper POST %s: %w: %s", path, err, strings.TrimSpace(string(output)))
		}
		return nil
	}

	response, err := http.Post(endpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("post %s: %s", path, response.Status)
	}
	return nil
}

func fetchConfig(api *apiClient, appID string) (Config, error) {
	var config Config
	body, err := api.get(fmt.Sprintf("/apps/%s/runner/config", url.PathEscape(appID)), 10*time.Second)
	if err != nil {
		return config, fmt.Errorf("fetch runner config: %w", err)
	}
	if err := json.Unmarshal(body, &config); err != nil {
		return config, fmt.Errorf("decode runner config: %w", err)
	}
	return config, nil
}

func fetchHooks(api *apiClient, appID string) (map[string]any, error) {
	body, err := api.get(fmt.Sprintf("/apps/%s/hooks.json", url.PathEscape(appID)), 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("fetch Codex hooks: %w", err)
	}
	var hooks map[string]any
	if err := json.Unmarshal(body, &hooks); err != nil {
		return nil, fmt.Errorf("decode Codex hooks: %w", err)
	}
	return hooks, nil
}

func installCodexHooks(api *apiClient, appID, baseDir string) (bool, error) {
	generatedHooks, err := fetchHooks(api, appID)
	if err != nil {
		return false, err
	}

	var installed []string
	var failures []string
	changed := false
	target := codexHookTarget(baseDir)
	targetChanged, err := installHooksAt(target, appID, generatedHooks)
	if err != nil {
		failures = append(failures, target+": "+err.Error())
	} else {
		installed = append(installed, target)
		changed = changed || targetChanged
	}

	for _, staleTarget := range staleCodexHookTargets(baseDir, target) {
		targetChanged, err := removeHooksAt(staleTarget, appID, generatedHooks)
		if err != nil {
			failures = append(failures, staleTarget+": "+err.Error())
			continue
		}
		changed = changed || targetChanged
	}
	if len(installed) == 0 {
		return false, fmt.Errorf("install Codex hooks: %s", strings.Join(failures, "; "))
	}
	return changed, nil
}

func codexHookTarget(baseDir string) string {
	projectDir := filepath.Join(projectRoot(baseDir), ".codex")
	if info, err := os.Stat(projectDir); err != nil || info.IsDir() {
		return projectDir
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".codex")
	}
	return projectDir
}

func staleCodexHookTargets(baseDir, selected string) []string {
	var targets []string
	projectDir := filepath.Join(projectRoot(baseDir), ".codex")
	if projectDir != selected {
		targets = append(targets, projectDir)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		homeDir := filepath.Join(home, ".codex")
		if homeDir != selected {
			targets = append(targets, homeDir)
		}
	}
	return targets
}

func installHooksAt(codexDir, appID string, generatedHooks map[string]any) (bool, error) {
	if info, err := os.Stat(codexDir); err == nil && !info.IsDir() {
		return false, fmt.Errorf("path exists and is not a directory")
	}
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		return false, fmt.Errorf("create directory: %w", err)
	}
	configChanged, err := enableCodexHooks(filepath.Join(codexDir, "config.toml"))
	if err != nil {
		return false, err
	}
	hooksChanged, err := mergeHooksFile(filepath.Join(codexDir, "hooks.json"), appID, generatedHooks)
	if err != nil {
		return false, err
	}
	return configChanged || hooksChanged, nil
}

func projectRoot(baseDir string) string {
	if baseDir == "" {
		if current, err := os.Getwd(); err == nil {
			baseDir = current
		}
	}
	if baseDir == "" {
		return "."
	}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = baseDir
	output, err := cmd.Output()
	if err == nil {
		if root := strings.TrimSpace(string(output)); root != "" {
			return root
		}
	}
	return baseDir
}

func enableCodexHooks(path string) (bool, error) {
	body, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("read Codex config: %w", err)
	}
	if errors.Is(err, os.ErrNotExist) {
		return true, os.WriteFile(path, []byte("[features]\ncodex_hooks = true\n"), 0o644)
	}

	text := string(body)
	lines := strings.Split(text, "\n")
	inFeatures := false
	featuresSeen := false
	hookSeen := false
	insertAt := -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if inFeatures && insertAt == -1 {
				insertAt = index
			}
			inFeatures = trimmed == "[features]"
			if inFeatures {
				featuresSeen = true
			}
			continue
		}
		if inFeatures && strings.HasPrefix(trimmed, "codex_hooks") {
			lines[index] = "codex_hooks = true"
			hookSeen = true
		}
	}
	if !featuresSeen {
		if strings.TrimSpace(text) != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\n[features]\ncodex_hooks = true\n"
		if string(body) == text {
			return false, nil
		}
		return true, os.WriteFile(path, []byte(text), 0o644)
	}
	if !hookSeen {
		if insertAt == -1 {
			lines = append(lines, "codex_hooks = true")
		} else {
			lines = append(lines[:insertAt], append([]string{"codex_hooks = true"}, lines[insertAt:]...)...)
		}
	}
	next := strings.Join(lines, "\n")
	if !strings.HasSuffix(next, "\n") {
		next += "\n"
	}
	if string(body) == next {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(next), 0o644)
}

func mergeHooksFile(path, appID string, generated map[string]any) (bool, error) {
	existing := map[string]any{"hooks": map[string]any{}}
	body, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("read Codex hooks: %w", err)
	}
	if err == nil && strings.TrimSpace(string(body)) != "" {
		if err := json.Unmarshal(body, &existing); err != nil {
			return false, fmt.Errorf("existing .codex/hooks.json is not valid JSON: %w", err)
		}
	}

	existingHooks := objectMap(existing, "hooks")
	generatedHooks := objectMap(generated, "hooks")
	for eventName, generatedGroupsAny := range generatedHooks {
		existingGroups := removeGeneratedHookGroups(sliceValue(existingHooks[eventName]), hookCommandNeedles(generatedGroupsAny, appID))
		existingGroups = append(existingGroups, sliceValue(generatedGroupsAny)...)
		existingHooks[eventName] = existingGroups
	}
	existing["hooks"] = existingHooks

	output, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode Codex hooks: %w", err)
	}
	output = append(output, '\n')
	if err == nil && bytes.Equal(body, output) {
		return false, nil
	}
	return true, os.WriteFile(path, output, 0o644)
}

func removeHooksAt(codexDir, appID string, generated map[string]any) (bool, error) {
	info, err := os.Stat(codexDir)
	if errors.Is(err, os.ErrNotExist) || (err == nil && !info.IsDir()) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat Codex directory: %w", err)
	}
	return removeHooksFile(filepath.Join(codexDir, "hooks.json"), appID, generated)
}

func removeHooksFile(path, appID string, generated map[string]any) (bool, error) {
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read Codex hooks: %w", err)
	}
	if strings.TrimSpace(string(body)) == "" {
		return false, nil
	}

	existing := map[string]any{}
	if err := json.Unmarshal(body, &existing); err != nil {
		return false, fmt.Errorf("existing .codex/hooks.json is not valid JSON: %w", err)
	}
	existingHooks, ok := existing["hooks"].(map[string]any)
	if !ok {
		return false, nil
	}

	changed := false
	generatedHooks := objectMap(generated, "hooks")
	for eventName, generatedGroupsAny := range generatedHooks {
		existingGroups := sliceValue(existingHooks[eventName])
		if len(existingGroups) == 0 {
			continue
		}
		keptGroups := removeGeneratedHookGroups(existingGroups, hookCommandNeedles(generatedGroupsAny, appID))
		if len(keptGroups) == len(existingGroups) {
			continue
		}
		changed = true
		if len(keptGroups) == 0 {
			delete(existingHooks, eventName)
		} else {
			existingHooks[eventName] = keptGroups
		}
	}
	if !changed {
		return false, nil
	}

	output, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode Codex hooks: %w", err)
	}
	output = append(output, '\n')
	return true, os.WriteFile(path, output, 0o644)
}

func objectMap(parent map[string]any, key string) map[string]any {
	if value, ok := parent[key].(map[string]any); ok {
		return value
	}
	next := map[string]any{}
	parent[key] = next
	return next
}

func sliceValue(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}
	return nil
}

func removeGeneratedHookGroups(groups []any, needles []string) []any {
	var kept []any
	for _, group := range groups {
		if !hookGroupContainsAny(group, needles) {
			kept = append(kept, group)
		}
	}
	return kept
}

func hookCommandNeedles(value any, appID string) []string {
	var needles []string
	prefix := "/apps/" + url.PathEscape(appID) + "/"
	collectHookCommandNeedles(value, prefix, &needles)
	return needles
}

func collectHookCommandNeedles(value any, prefix string, needles *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if key == "command" {
				if command, ok := item.(string); ok {
					*needles = append(*needles, commandEndpointNeedles(command, prefix)...)
				}
			}
			collectHookCommandNeedles(item, prefix, needles)
		}
	case []any:
		for _, item := range typed {
			collectHookCommandNeedles(item, prefix, needles)
		}
	}
}

func commandEndpointNeedles(command, prefix string) []string {
	var needles []string
	for {
		start := strings.Index(command, prefix)
		if start == -1 {
			return needles
		}
		rest := command[start:]
		end := len(rest)
		for index, char := range rest {
			if char == '\'' || char == '"' || char == ' ' || char == '\t' || char == '\n' || char == '\r' {
				end = index
				break
			}
		}
		needles = append(needles, rest[:end])
		command = rest[end:]
	}
}

func hookGroupContainsAny(value any, needles []string) bool {
	for _, needle := range needles {
		if needle != "" && hookGroupContains(value, needle) {
			return true
		}
	}
	return false
}

func hookGroupContains(value any, needle string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if key == "command" {
				if command, ok := item.(string); ok && strings.Contains(command, needle) {
					return true
				}
			}
			if hookGroupContains(item, needle) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if hookGroupContains(item, needle) {
				return true
			}
		}
	}
	return false
}

func waitForAction(api *apiClient, appID string, since int) (nextAction, error) {
	path := fmt.Sprintf(
		"/apps/%s/runner/next-action?since=%d&runner_version=%d",
		url.PathEscape(appID),
		since,
		protocol.RunnerVersion,
	)

	body, err := api.get(path, 40*time.Second)
	if err != nil {
		return nextAction{}, err
	}

	var action nextAction
	if err := json.Unmarshal(body, &action); err != nil {
		return nextAction{}, err
	}
	return action, nil
}

func restartCommand(api *apiClient, appID string, config Config, handle *processHandle) *processHandle {
	fmt.Println("Restart requested. Dashboard will show progress.")
	_ = postEvent(api, appID, "restart_started", "stopping command", 0)

	if err := stopCommand(handle); err != nil {
		_ = postEvent(api, appID, "restart_failed", err.Error(), 1)
		return nil
	}

	var next *processHandle
	if config.CommandEnabled {
		var err error
		next, err = startCommand(api, appID, config)
		if err != nil {
			_ = postEvent(api, appID, "restart_failed", err.Error(), 1)
			return nil
		}
	}

	_ = postEvent(api, appID, "restart_finished", "command restarted", 0)
	if ok, detail := waitForHealth(api, appID, config, next); ok {
		_ = postEvent(api, appID, "health_ok", detail, 0)
	} else {
		_ = postEvent(api, appID, "health_failed", detail, 1)
	}
	return next
}

func startCommand(api *apiClient, appID string, config Config) (*processHandle, error) {
	displayCommand := commandLine(config.Command)
	fmt.Printf("\n\033[36mRunning command:\033[0m %s\n", displayCommand)
	fmt.Printf("\033[36mFrom:\033[0m %s\n\n", config.CWD)

	cmd := exec.Command(config.Command[0], config.Command[1:]...)
	cmd.Dir = config.CWD
	cmd.Env = commandEnv()
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	output := &tailBuffer{limit: 5000}
	writer := io.MultiWriter(os.Stdout, output)
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	handle := &processHandle{cmd: cmd, done: make(chan struct{}), output: output}
	_ = postEvent(api, appID, "command_started", commandStartDetail(config), 0)
	go func() {
		err := cmd.Wait()
		handle.mu.Lock()
		handle.err = err
		stopping := handle.stopping
		handle.mu.Unlock()
		close(handle.done)
		if stopping {
			return
		}
		if err == nil {
			_ = postEvent(api, appID, "command_finished", commandExitDetail(handle, "command finished"), 0)
			return
		}

		exitCode := 1
		detail := err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		if outputText := output.String(); outputText != "" {
			detail += "; output: " + outputText
		}
		_ = postEvent(api, appID, "command_exited", detail, exitCode)
	}()

	return handle, nil
}

func commandLine(command []string) string {
	parts := make([]string, 0, len(command))
	for _, part := range command {
		parts = append(parts, shellArg(part))
	}
	return strings.Join(parts, " ")
}

func shellArg(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(char rune) bool {
		return !(char >= 'A' && char <= 'Z') &&
			!(char >= 'a' && char <= 'z') &&
			!(char >= '0' && char <= '9') &&
			!strings.ContainsRune("_+-./:=,@%", char)
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func commandEnv() []string {
	env := os.Environ()
	env = setEnvDefault(env, "TERM", "xterm-256color")
	env = setEnvDefault(env, "FORCE_COLOR", "1")
	env = setEnvDefault(env, "CLICOLOR_FORCE", "1")
	env = setEnvDefault(env, "COMPOSE_ANSI", "always")
	env = setEnvDefault(env, "COMPOSE_PROGRESS", "plain")
	env = setEnvDefault(env, "BUILDKIT_PROGRESS", "plain")
	return env
}

func setEnvDefault(env []string, key, value string) []string {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return env
		}
	}
	return append(env, prefix+value)
}

func stopOlderRunners(appID string) {
	currentPID := os.Getpid()
	currentPgid, _ := syscall.Getpgid(currentPID)
	runners := findRunnerProcesses(appID, currentPID)
	if len(runners) == 0 {
		return
	}

	fmt.Printf("\033[33mStopping %d old runner(s) for %s.\033[0m\n", len(runners), appID)
	processes := processTable()
	for _, pid := range runners {
		stopRunnerProcess(pid, processes, currentPID, currentPgid)
	}
}

func stopRunnerProcess(pid int, processes map[int]int, currentPID int, currentPgid int) {
	for _, child := range descendantsOf(pid, processes) {
		if child == currentPID {
			continue
		}
		stopProcessGroup(child, currentPgid)
	}
	_ = syscall.Kill(pid, syscall.SIGCONT)
	_ = syscall.Kill(pid, syscall.SIGTERM)
	time.Sleep(300 * time.Millisecond)
	if processExists(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func stopProcessGroup(pid int, currentPgid int) {
	pgid, err := syscall.Getpgid(pid)
	if err != nil || pgid <= 0 || pgid == currentPgid {
		_ = syscall.Kill(pid, syscall.SIGTERM)
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGCONT)
	_ = syscall.Kill(-pgid, syscall.SIGINT)
	time.Sleep(300 * time.Millisecond)
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	time.Sleep(300 * time.Millisecond)
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}

func findRunnerProcesses(appID string, currentPID int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var pids []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == currentPID {
			continue
		}
		args := procCmdline(pid)
		if isRunnerForApp(args, appID) {
			pids = append(pids, pid)
		}
	}
	return pids
}

func isRunnerForApp(args []string, appID string) bool {
	hasRunnerCommand := false
	for index, arg := range args {
		if index > 0 && arg == "runner" {
			hasRunnerCommand = true
		}
		if arg == "--app" && index+1 < len(args) && args[index+1] == appID {
			return hasRunnerCommand
		}
		if strings.HasPrefix(arg, "--app=") && strings.TrimPrefix(arg, "--app=") == appID {
			return hasRunnerCommand
		}
	}
	return false
}

func procCmdline(pid int) []string {
	body, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil || len(body) == 0 {
		return nil
	}
	parts := bytes.Split(bytes.TrimRight(body, "\x00"), []byte{0})
	args := make([]string, 0, len(parts))
	for _, part := range parts {
		args = append(args, string(part))
	}
	return args
}

func processTable() map[int]int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	table := map[int]int{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		if ppid, ok := procParent(pid); ok {
			table[pid] = ppid
		}
	}
	return table
}

func procParent(pid int) (int, bool) {
	body, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0, false
	}
	text := string(body)
	end := strings.LastIndex(text, ")")
	if end == -1 || end+2 >= len(text) {
		return 0, false
	}
	fields := strings.Fields(text[end+2:])
	if len(fields) < 2 {
		return 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	return ppid, err == nil
}

func descendantsOf(pid int, processes map[int]int) []int {
	var descendants []int
	queue := []int{pid}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		for child, ppid := range processes {
			if ppid != parent {
				continue
			}
			descendants = append(descendants, child)
			queue = append(queue, child)
		}
	}
	for left, right := 0, len(descendants)-1; left < right; left, right = left+1, right-1 {
		descendants[left], descendants[right] = descendants[right], descendants[left]
	}
	return descendants
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func stopCommand(handle *processHandle) error {
	if handle == nil || handle.cmd == nil || handle.cmd.Process == nil {
		return nil
	}

	select {
	case <-handle.done:
		return nil
	default:
	}
	handle.mu.Lock()
	handle.stopping = true
	handle.mu.Unlock()

	pgid, err := syscall.Getpgid(handle.cmd.Process.Pid)
	if err != nil {
		pgid = handle.cmd.Process.Pid
	}
	if err := syscall.Kill(-pgid, syscall.SIGINT); err != nil {
		_ = handle.cmd.Process.Signal(os.Interrupt)
	}

	select {
	case <-handle.done:
		return nil
	case <-time.After(commandInterruptGrace):
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		select {
		case <-handle.done:
			return nil
		case <-time.After(commandTerminateGrace):
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			select {
			case <-handle.done:
				return nil
			case <-time.After(2 * time.Second):
				return errors.New("old command did not stop")
			}
		}
	}
}

func forceKillCommand(handle *processHandle) {
	if handle == nil || handle.cmd == nil || handle.cmd.Process == nil {
		return
	}
	handle.mu.Lock()
	handle.stopping = true
	handle.mu.Unlock()
	pgid, err := syscall.Getpgid(handle.cmd.Process.Pid)
	if err != nil {
		pgid = handle.cmd.Process.Pid
	}
	_ = syscall.Kill(-pgid, syscall.SIGCONT)
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	_ = handle.cmd.Process.Kill()
}

func waitForHealth(api *apiClient, appID string, config Config, handle *processHandle) (bool, string) {
	if config.HealthMode == "none" {
		return true, "health check disabled"
	}
	if config.HealthMode == "browser_url" {
		return waitForBrowserHealth(api, appID, config, handle)
	}
	if config.HealthURL == "" {
		return true, ""
	}
	return waitForBrowserHealth(api, appID, config, handle)
}

func waitForBrowserHealth(api *apiClient, appID string, config Config, handle *processHandle) (bool, string) {
	if config.HealthURL == "" {
		return true, ""
	}

	timeout := time.Duration(config.StartupTimeoutSecs) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	deadline := time.Now().Add(timeout)
	lastDetail := "no browser result yet"
	lastOutputVersion := outputVersion(handle)
	for time.Now().Before(deadline) {
		if exited, failed, detail := commandFinished(handle); exited && failed {
			return false, "command exited before health check passed: " + detail
		}
		currentOutputVersion := outputVersion(handle)
		if currentOutputVersion != lastOutputVersion {
			lastOutputVersion = currentOutputVersion
			deadline = time.Now().Add(timeout)
		}
		path := fmt.Sprintf("/apps/%s/browser-health?timeout=3", url.PathEscape(appID))
		body, err := api.get(path, 8*time.Second)
		if err != nil {
			lastDetail = err.Error()
			time.Sleep(2 * time.Second)
			continue
		}

		var result struct {
			OK     bool   `json:"ok"`
			Detail string `json:"detail"`
			Status int    `json:"status"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			lastDetail = "could not read browser health result: " + err.Error()
			time.Sleep(2 * time.Second)
			continue
		}
		if result.OK {
			return true, ""
		}
		lastDetail = result.Detail
		if result.Status > 0 {
			lastDetail = fmt.Sprintf("%s (HTTP %d)", lastDetail, result.Status)
		}
		time.Sleep(2 * time.Second)
	}
	return false, fmt.Sprintf("startup timeout reached after %s checking %s from browser; last result: %s", timeout, config.HealthURL, lastDetail)
}

func outputVersion(handle *processHandle) int64 {
	if handle == nil || handle.output == nil {
		return 0
	}
	return handle.output.Version()
}

func commandFinished(handle *processHandle) (bool, bool, string) {
	if handle == nil {
		return false, false, ""
	}
	select {
	case <-handle.done:
		handle.mu.Lock()
		err := handle.err
		handle.mu.Unlock()
		if err == nil {
			return true, false, commandExitDetail(handle, "exit code 0")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return true, true, commandExitDetail(handle, fmt.Sprintf("exit code %d", exitErr.ExitCode()))
		}
		return true, true, commandExitDetail(handle, err.Error())
	default:
		return false, false, ""
	}
}

func commandStartDetail(config Config) string {
	return fmt.Sprintf("command: %s", commandLine(config.Command))
}

func commandExitDetail(handle *processHandle, detail string) string {
	if handle == nil || handle.output == nil {
		return detail
	}
	if output := handle.output.String(); output != "" {
		return detail + "; output: " + output
	}
	return detail
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if b.limit > 0 && len(b.data) > b.limit {
		b.data = append([]byte(nil), b.data[len(b.data)-b.limit:]...)
	}
	b.version++
	return len(p), nil
}

func (b *tailBuffer) Version() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.version
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	text := strings.TrimSpace(string(b.data))
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 1000 {
		text = "..." + text[len(text)-1000:]
	}
	return text
}

func postEvent(api *apiClient, appID, eventType, detail string, exitCode int) error {
	payload := map[string]any{
		"type":           eventType,
		"detail":         detail,
		"exit_code":      exitCode,
		"runner_version": protocol.RunnerVersion,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/apps/%s/runner/events", url.PathEscape(appID))
	if eventType == "runner_heartbeat" || eventType == "runner_disconnected" {
		return api.post(path, body, 5*time.Second)
	}

	deadline := time.Now().Add(45 * time.Second)
	var lastErr error
	for {
		lastErr = api.post(path, body, 5*time.Second)
		if lastErr == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return lastErr
		}
		time.Sleep(1 * time.Second)
	}
}
