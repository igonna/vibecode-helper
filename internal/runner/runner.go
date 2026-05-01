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
	cmd  *exec.Cmd
	done chan error
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

	fmt.Printf("Runner connected to %s for app %s\n", helperURL, config.Name)
	fmt.Printf("Working directory: %s\n", config.CWD)
	fmt.Printf("Command: %s\n", strings.Join(config.Command, " "))
	if config.HealthMode == "command" {
		fmt.Printf("Health command: %s\n", strings.Join(config.HealthCommand, " "))
	} else {
		fmt.Printf("Health URL: %s\n", config.HealthURL)
	}
	fmt.Println("Command output appears below.")

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

	if waitForHealth(api, appID, config) {
		_ = postEvent(api, appID, "health_ok", config.HealthURL, 0)
	} else {
		_ = postEvent(api, appID, "health_failed", "startup timeout reached", 1)
	}

	seenVersion := 0
	for {
		action, err := waitForAction(api, appID, seenVersion)
		if err != nil {
			fmt.Printf("Runner could not reach helper: %v\n", err)
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
	fmt.Println()
	fmt.Println("=== Restart requested ===")
	_ = postEvent(api, appID, "restart_started", "stopping command", 0)

	stopCommand(handle)

	next, err := startCommand(api, appID, config)
	if err != nil {
		fmt.Printf("Command failed to restart: %v\n", err)
		_ = postEvent(api, appID, "restart_failed", err.Error(), 1)
		return nil
	}

	_ = postEvent(api, appID, "restart_finished", "command restarted", 0)
	if waitForHealth(api, appID, config) {
		_ = postEvent(api, appID, "health_ok", config.HealthURL, 0)
	} else {
		_ = postEvent(api, appID, "health_failed", "startup timeout reached", 1)
	}
	return next
}

func startCommand(api *apiClient, appID string, config Config) (*processHandle, error) {
	fmt.Println()
	fmt.Println("=== Starting command ===")
	fmt.Println(strings.Join(config.Command, " "))

	cmd := exec.Command(config.Command[0], config.Command[1:]...)
	cmd.Dir = config.CWD
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	handle := &processHandle{cmd: cmd, done: make(chan error, 1)}
	_ = postEvent(api, appID, "command_started", strings.Join(config.Command, " "), 0)
	go func() {
		err := cmd.Wait()
		handle.done <- err
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

	fmt.Println()
	fmt.Println("=== Stopping command ===")
	if err := handle.cmd.Process.Signal(os.Interrupt); err != nil {
		_ = handle.cmd.Process.Kill()
		return
	}

	select {
	case <-handle.done:
		return
	case <-time.After(15 * time.Second):
		fmt.Println("Command did not stop after interrupt; killing it.")
		_ = handle.cmd.Process.Kill()
		<-handle.done
	}
}

func waitForHealth(api *apiClient, appID string, config Config) bool {
	if config.HealthMode == "browser_url" {
		return waitForBrowserHealth(api, appID, config)
	}
	if config.HealthMode == "command" {
		return waitForHealthCommand(config)
	}
	if config.HealthURL == "" {
		return true
	}

	timeout := time.Duration(config.StartupTimeoutSecs) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(deadline) {
		response, err := client.Get(config.HealthURL)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode < 500 {
				fmt.Printf("Health check: UP; %s (HTTP %d)\n", config.HealthURL, response.StatusCode)
				return true
			}
			fmt.Printf("Health check: DOWN; %s (HTTP %d)\n", config.HealthURL, response.StatusCode)
		} else {
			fmt.Printf("Health check: DOWN; %s (%v)\n", config.HealthURL, err)
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

func waitForBrowserHealth(api *apiClient, appID string, config Config) bool {
	if config.HealthURL == "" {
		return true
	}

	timeout := time.Duration(config.StartupTimeoutSecs) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		path := fmt.Sprintf("/apps/%s/browser-health?timeout=3", url.PathEscape(appID))
		body, err := api.get(path, 8*time.Second)
		if err != nil {
			fmt.Printf("Health check: DOWN; browser %s (%v)\n", config.HealthURL, err)
			time.Sleep(2 * time.Second)
			continue
		}

		var result struct {
			OK     bool   `json:"ok"`
			Detail string `json:"detail"`
			Status int    `json:"status"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Printf("Health check: DOWN; browser %s (%v)\n", config.HealthURL, err)
			time.Sleep(2 * time.Second)
			continue
		}
		if result.OK {
			fmt.Printf("Health check: UP; browser %s (%s)\n", config.HealthURL, result.Detail)
			return true
		}
		fmt.Printf("Health check: DOWN; browser %s (%s)\n", config.HealthURL, result.Detail)
		time.Sleep(2 * time.Second)
	}
	return false
}

func waitForHealthCommand(config Config) bool {
	if len(config.HealthCommand) == 0 {
		return true
	}

	timeout := time.Duration(config.StartupTimeoutSecs) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		cmd := exec.CommandContext(ctx, config.HealthCommand[0], config.HealthCommand[1:]...)
		cmd.Dir = config.CWD
		err := cmd.Run()
		cancel()
		if err == nil {
			fmt.Printf("Health check: UP; %s\n", strings.Join(config.HealthCommand, " "))
			return true
		}
		fmt.Printf("Health check: DOWN; %s (%v)\n", strings.Join(config.HealthCommand, " "), err)
		time.Sleep(2 * time.Second)
	}
	return false
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
