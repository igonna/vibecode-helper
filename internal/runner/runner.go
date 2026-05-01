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
	SoundRepeatCount   int      `json:"sound_repeat_count"`
}

type nextAction struct {
	Action  string `json:"action"`
	Version int    `json:"version"`
}

type processHandle struct {
	cmd  *exec.Cmd
	done chan error
}

func Run(helperURL, appID string) error {
	baseURL := strings.TrimRight(helperURL, "/")
	config, err := fetchConfig(baseURL, appID)
	if err != nil {
		return err
	}
	if len(config.Command) == 0 {
		return errors.New("configured command is empty")
	}

	fmt.Printf("Runner connected to %s for app %s\n", baseURL, config.Name)
	fmt.Printf("Working directory: %s\n", config.CWD)
	fmt.Printf("Command: %s\n", strings.Join(config.Command, " "))
	if config.HealthMode == "command" {
		fmt.Printf("Health command: %s\n", strings.Join(config.HealthCommand, " "))
	} else {
		fmt.Printf("Health URL: %s\n", config.HealthURL)
	}
	fmt.Println("Command output appears below.")

	_ = postEvent(baseURL, appID, "runner_connected", "runner started", 0)
	handle, err := startCommand(baseURL, appID, config)
	if err != nil {
		_ = postEvent(baseURL, appID, "command_failed", err.Error(), 1)
		return err
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		stopCommand(handle)
		_ = postEvent(baseURL, appID, "runner_disconnected", "runner stopped", 0)
		os.Exit(0)
	}()

	if waitForHealth(baseURL, appID, config) {
		_ = postEvent(baseURL, appID, "health_ok", config.HealthURL, 0)
	} else {
		_ = postEvent(baseURL, appID, "health_failed", "startup timeout reached", 1)
	}

	seenVersion := 0
	for {
		action, err := waitForAction(baseURL, appID, seenVersion)
		if err != nil {
			fmt.Printf("Runner could not reach helper: %v\n", err)
			time.Sleep(2 * time.Second)
			continue
		}
		seenVersion = action.Version
		if action.Action != "restart" {
			continue
		}

		handle = restartCommand(baseURL, appID, config, handle)
	}
}

func fetchConfig(baseURL, appID string) (Config, error) {
	var config Config
	endpoint := fmt.Sprintf("%s/apps/%s/runner/config", baseURL, url.PathEscape(appID))
	response, err := http.Get(endpoint)
	if err != nil {
		return config, fmt.Errorf("fetch runner config: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return config, fmt.Errorf("fetch runner config: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(response.Body).Decode(&config); err != nil {
		return config, fmt.Errorf("decode runner config: %w", err)
	}
	return config, nil
}

func waitForAction(baseURL, appID string, since int) (nextAction, error) {
	endpoint := fmt.Sprintf(
		"%s/apps/%s/runner/next-action?since=%d",
		baseURL,
		url.PathEscape(appID),
		since,
	)

	client := &http.Client{Timeout: 35 * time.Second}
	response, err := client.Get(endpoint)
	if err != nil {
		return nextAction{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nextAction{}, fmt.Errorf("next action: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	var action nextAction
	if err := json.NewDecoder(response.Body).Decode(&action); err != nil {
		return nextAction{}, err
	}
	return action, nil
}

func restartCommand(baseURL, appID string, config Config, handle *processHandle) *processHandle {
	fmt.Println()
	fmt.Println("=== Restart requested ===")
	_ = postEvent(baseURL, appID, "restart_started", "stopping command", 0)

	stopCommand(handle)

	next, err := startCommand(baseURL, appID, config)
	if err != nil {
		fmt.Printf("Command failed to restart: %v\n", err)
		_ = postEvent(baseURL, appID, "restart_failed", err.Error(), 1)
		return nil
	}

	_ = postEvent(baseURL, appID, "restart_finished", "command restarted", 0)
	if waitForHealth(baseURL, appID, config) {
		_ = postEvent(baseURL, appID, "health_ok", config.HealthURL, 0)
	} else {
		_ = postEvent(baseURL, appID, "health_failed", "startup timeout reached", 1)
	}
	return next
}

func startCommand(baseURL, appID string, config Config) (*processHandle, error) {
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
	_ = postEvent(baseURL, appID, "command_started", strings.Join(config.Command, " "), 0)
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
		_ = postEvent(baseURL, appID, "command_exited", detail, exitCode)
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

func waitForHealth(baseURL, appID string, config Config) bool {
	if config.HealthMode == "browser_url" {
		return waitForBrowserHealth(baseURL, appID, config)
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

func waitForBrowserHealth(baseURL, appID string, config Config) bool {
	if config.HealthURL == "" {
		return true
	}

	timeout := time.Duration(config.StartupTimeoutSecs) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 8 * time.Second}
	for time.Now().Before(deadline) {
		endpoint := fmt.Sprintf("%s/apps/%s/browser-health?timeout=3", baseURL, url.PathEscape(appID))
		response, err := client.Get(endpoint)
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
		decodeErr := json.NewDecoder(response.Body).Decode(&result)
		_ = response.Body.Close()
		if decodeErr != nil {
			fmt.Printf("Health check: DOWN; browser %s (%v)\n", config.HealthURL, decodeErr)
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

func postEvent(baseURL, appID, eventType, detail string, exitCode int) error {
	payload := map[string]any{
		"type":      eventType,
		"detail":    detail,
		"exit_code": exitCode,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/apps/%s/runner/events", baseURL, url.PathEscape(appID))
	response, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("post event %s: %s", eventType, response.Status)
	}
	return nil
}
