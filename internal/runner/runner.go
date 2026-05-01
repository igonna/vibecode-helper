package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Config struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	CWD                string   `json:"cwd"`
	Command            []string `json:"command"`
	HealthMode         string   `json:"health_mode"`
	HealthURL          string   `json:"health_url"`
	HealthCommand      []string `json:"health_command"`
	StartupTimeoutSecs int      `json:"startup_timeout_seconds"`
}

type nextAction struct {
	Action  string `json:"action"`
	Version int    `json:"version"`
}

type processHandle struct {
	cmd    *exec.Cmd
	done   chan struct{}
	output *tailBuffer
	mu     sync.Mutex
	err    error
}

type tailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

type apiClient struct {
	baseURL         string
	dockerContainer string
}

func Run(helperURL, appID string) error {
	api := newAPIClient(helperURL)
	config, err := fetchConfig(api, appID)
	if err != nil {
		return err
	}
	if len(config.Command) == 0 {
		return errors.New("configured command is empty")
	}

	fmt.Printf("\033[32m✅ Runner connected. Check web dashboard.\033[0m\n")

	_ = postEvent(api, appID, "runner_connected", "runner started", 0)
	handle, err := startCommand(api, appID, config)
	if err != nil {
		_ = postEvent(api, appID, "command_failed", err.Error(), 1)
		return err
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		stopCommand(handle)
		_ = postEvent(api, appID, "runner_disconnected", "runner stopped", 0)
		os.Exit(0)
	}()

	if ok, detail := waitForHealth(api, appID, config, handle); ok {
		_ = postEvent(api, appID, "health_ok", config.HealthURL, 0)
	} else {
		_ = postEvent(api, appID, "health_failed", detail, 1)
	}

	seenVersion := 0
	for {
		action, err := waitForAction(api, appID, seenVersion)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		seenVersion = action.Version
		if action.Action != "restart" {
			continue
		}

		handle = restartCommand(api, appID, config, handle)
	}
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

func waitForAction(api *apiClient, appID string, since int) (nextAction, error) {
	path := fmt.Sprintf(
		"/apps/%s/runner/next-action?since=%d",
		url.PathEscape(appID),
		since,
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

	stopCommand(handle)

	next, err := startCommand(api, appID, config)
	if err != nil {
		_ = postEvent(api, appID, "restart_failed", err.Error(), 1)
		return nil
	}

	_ = postEvent(api, appID, "restart_finished", "command restarted", 0)
	if ok, detail := waitForHealth(api, appID, config, next); ok {
		_ = postEvent(api, appID, "health_ok", config.HealthURL, 0)
	} else {
		_ = postEvent(api, appID, "health_failed", detail, 1)
	}
	return next
}

func startCommand(api *apiClient, appID string, config Config) (*processHandle, error) {
	cmd := exec.Command(config.Command[0], config.Command[1:]...)
	cmd.Dir = config.CWD
	cmd.Stdin = os.Stdin
	output := &tailBuffer{limit: 5000}
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	handle := &processHandle{cmd: cmd, done: make(chan struct{}), output: output}
	_ = postEvent(api, appID, "command_started", commandStartDetail(config), 0)
	go func() {
		err := cmd.Wait()
		handle.mu.Lock()
		handle.err = err
		handle.mu.Unlock()
		close(handle.done)
		exitCode := 0
		detail := "command exited"
		if err != nil {
			detail = err.Error()
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
		if outputText := output.String(); outputText != "" {
			detail += "; output: " + outputText
		}
		_ = postEvent(api, appID, "command_exited", detail, exitCode)
	}()

	return handle, nil
}

func stopCommand(handle *processHandle) {
	if handle == nil || handle.cmd == nil || handle.cmd.Process == nil {
		return
	}

	select {
	case <-handle.done:
		return
	default:
	}

	if err := handle.cmd.Process.Signal(os.Interrupt); err != nil {
		_ = handle.cmd.Process.Kill()
		return
	}

	select {
	case <-handle.done:
		return
	case <-time.After(15 * time.Second):
		_ = handle.cmd.Process.Kill()
		<-handle.done
	}
}

func waitForHealth(api *apiClient, appID string, config Config, handle *processHandle) (bool, string) {
	if config.HealthMode == "browser_url" {
		return waitForBrowserHealth(api, appID, config, handle)
	}
	if config.HealthMode == "command" {
		return waitForHealthCommand(config, handle)
	}
	if config.HealthURL == "" {
		return true, ""
	}

	timeout := time.Duration(config.StartupTimeoutSecs) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(deadline) {
		if exited, detail := commandExited(handle); exited {
			return false, "command exited before health check passed: " + detail
		}
		response, err := client.Get(config.HealthURL)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode < 500 {
				return true, ""
			}
		}
		time.Sleep(2 * time.Second)
	}
	return false, "startup timeout reached"
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
	for time.Now().Before(deadline) {
		if exited, detail := commandExited(handle); exited {
			return false, "command exited before health check passed: " + detail
		}
		path := fmt.Sprintf("/apps/%s/browser-health?timeout=3", url.PathEscape(appID))
		body, err := api.get(path, 8*time.Second)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		var result struct {
			OK     bool   `json:"ok"`
			Detail string `json:"detail"`
			Status int    `json:"status"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if result.OK {
			return true, ""
		}
		time.Sleep(2 * time.Second)
	}
	return false, "startup timeout reached"
}

func waitForHealthCommand(config Config, handle *processHandle) (bool, string) {
	if len(config.HealthCommand) == 0 {
		return true, ""
	}

	timeout := time.Duration(config.StartupTimeoutSecs) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if exited, detail := commandExited(handle); exited {
			return false, "command exited before health check passed: " + detail
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		cmd := exec.CommandContext(ctx, config.HealthCommand[0], config.HealthCommand[1:]...)
		cmd.Dir = config.CWD
		err := cmd.Run()
		cancel()
		if err == nil {
			return true, ""
		}
		time.Sleep(2 * time.Second)
	}
	return false, "startup timeout reached"
}

func commandExited(handle *processHandle) (bool, string) {
	if handle == nil {
		return false, ""
	}
	select {
	case <-handle.done:
		handle.mu.Lock()
		err := handle.err
		handle.mu.Unlock()
		if err == nil {
			return true, commandExitDetail(handle, "exit code 0")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return true, commandExitDetail(handle, fmt.Sprintf("exit code %d", exitErr.ExitCode()))
		}
		return true, commandExitDetail(handle, err.Error())
	default:
		return false, ""
	}
}

func commandStartDetail(config Config) string {
	return fmt.Sprintf("cwd: %s; command: %s", config.CWD, strings.Join(config.Command, " "))
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
	return len(p), nil
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
		"type":      eventType,
		"detail":    detail,
		"exit_code": exitCode,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return api.post(fmt.Sprintf("/apps/%s/runner/events", url.PathEscape(appID)), body, 5*time.Second)
}
