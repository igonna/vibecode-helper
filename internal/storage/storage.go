package storage

import (
	"database/sql"
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

type DB struct {
	sql *sql.DB
}

type AppRecord struct {
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
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ArchivedAt         time.Time
}

type PromptRecord struct {
	Index       int
	PromptSecs  float64
	WaitSecs    float64
	RestartSecs float64
	Result      string
	FinishedAt  time.Time
	Permissions int
}

type PromptStats struct {
	Count        int
	TotalWait    float64
	TotalPrompt  float64
	TotalRestart float64
	MinWait      float64
	MaxWait      float64
	LastWait     float64
	LastPrompt   float64
	LastRestart  float64
	LastResult   string
	Recent       []PromptRecord
}

type GlobalStats struct {
	Count         int
	ActiveWait    float64
	ActivePrompt  float64
	ActiveRestart float64
}

type EventRecord struct {
	At     time.Time
	Type   string
	Detail string
}

func DefaultPath() string {
	if value := os.Getenv("VIBECODE_HELPER_DB"); value != "" {
		return value
	}
	if info, err := os.Stat("/data"); err == nil && info.IsDir() {
		return "/data/helper.db"
	}
	if dataRoot, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dataRoot, "vibecode-helper", "helper.db")
	}
	return "helper.db"
}

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &DB{sql: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) init() error {
	if _, err := db.sql.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return err
	}

	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	_, err = db.sql.Exec(string(schema))
	if err != nil {
		return err
	}
	return db.ensureColumn("apps", "archived_at", "TEXT NOT NULL DEFAULT ''")
}

func (db *DB) ensureColumn(table, column, definition string) error {
	rows, err := db.sql.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.sql.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition)
	return err
}

func (db *DB) LoadApps() ([]AppRecord, error) {
	rows, err := db.sql.Query(`
		SELECT id, name, cwd, command_json, health_mode, health_url,
		       health_command_json, startup_timeout_seconds, sound_repeat_count,
		       codex_environment, runner_environment, created_at, updated_at, archived_at
		FROM apps
		WHERE archived_at = ''
		ORDER BY name COLLATE NOCASE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []AppRecord
	for rows.Next() {
		var record AppRecord
		var commandJSON string
		var healthCommandJSON string
		var createdAt string
		var updatedAt string
		var archivedAt string
		if err := rows.Scan(
			&record.ID,
			&record.Name,
			&record.CWD,
			&commandJSON,
			&record.HealthMode,
			&record.HealthURL,
			&healthCommandJSON,
			&record.StartupTimeoutSecs,
			&record.SoundRepeatCount,
			&record.CodexEnvironment,
			&record.RunnerEnvironment,
			&createdAt,
			&updatedAt,
			&archivedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(commandJSON), &record.Command)
		_ = json.Unmarshal([]byte(healthCommandJSON), &record.HealthCommand)
		record.CreatedAt = parseTime(createdAt)
		record.UpdatedAt = parseTime(updatedAt)
		record.ArchivedAt = parseTime(archivedAt)
		records = append(records, record)
	}
	return records, rows.Err()
}

func (db *DB) UpsertApp(record AppRecord) error {
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now

	commandJSON, err := json.Marshal(record.Command)
	if err != nil {
		return err
	}
	healthCommandJSON, err := json.Marshal(record.HealthCommand)
	if err != nil {
		return err
	}

	_, err = db.sql.Exec(`
		INSERT INTO apps (
			id, name, cwd, command_json, health_mode, health_url,
			health_command_json, startup_timeout_seconds, sound_repeat_count,
			codex_environment, runner_environment, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			cwd = excluded.cwd,
			command_json = excluded.command_json,
			health_mode = excluded.health_mode,
			health_url = excluded.health_url,
			health_command_json = excluded.health_command_json,
			startup_timeout_seconds = excluded.startup_timeout_seconds,
			sound_repeat_count = excluded.sound_repeat_count,
			codex_environment = excluded.codex_environment,
			runner_environment = excluded.runner_environment,
			updated_at = excluded.updated_at,
			archived_at = ''
	`, record.ID, record.Name, record.CWD, string(commandJSON), record.HealthMode, record.HealthURL,
		string(healthCommandJSON), record.StartupTimeoutSecs, record.SoundRepeatCount,
		record.CodexEnvironment, record.RunnerEnvironment, formatTime(record.CreatedAt), formatTime(record.UpdatedAt))
	return err
}

func (db *DB) DeleteApp(appID string) error {
	_, err := db.sql.Exec("UPDATE apps SET archived_at = ?, updated_at = ? WHERE id = ?", formatTime(time.Now().UTC()), formatTime(time.Now().UTC()), appID)
	return err
}

func (db *DB) RenameApp(oldID string, record AppRecord) error {
	if oldID == record.ID {
		return db.UpsertApp(record)
	}

	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now

	commandJSON, err := json.Marshal(record.Command)
	if err != nil {
		return err
	}
	healthCommandJSON, err := json.Marshal(record.HealthCommand)
	if err != nil {
		return err
	}

	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var createdAt string
	if err := tx.QueryRow("SELECT created_at FROM apps WHERE id = ?", oldID).Scan(&createdAt); err == nil && createdAt != "" {
		record.CreatedAt = parseTime(createdAt)
	}

	if _, err := tx.Exec(`
		INSERT INTO apps (
			id, name, cwd, command_json, health_mode, health_url,
			health_command_json, startup_timeout_seconds, sound_repeat_count,
			codex_environment, runner_environment, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.ID, record.Name, record.CWD, string(commandJSON), record.HealthMode, record.HealthURL,
		string(healthCommandJSON), record.StartupTimeoutSecs, record.SoundRepeatCount,
		record.CodexEnvironment, record.RunnerEnvironment, formatTime(record.CreatedAt), formatTime(record.UpdatedAt)); err != nil {
		return err
	}

	for _, table := range []string{"prompt_runs", "restart_runs", "events", "notifications"} {
		if _, err := tx.Exec("UPDATE "+table+" SET app_id = ? WHERE app_id = ?", record.ID, oldID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("DELETE FROM apps WHERE id = ?", oldID); err != nil {
		return err
	}
	return tx.Commit()
}

type timeInterval struct {
	start time.Time
	end   time.Time
}

func (db *DB) LoadGlobalStats() (GlobalStats, error) {
	return db.loadGlobalStatsSince(time.Time{})
}

func (db *DB) LoadTodayGlobalStats(now time.Time) (GlobalStats, error) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return db.loadGlobalStatsSince(start)
}

func (db *DB) loadGlobalStatsSince(since time.Time) (GlobalStats, error) {
	var stats GlobalStats
	query := `
		SELECT started_at,
		       COALESCE(finished_at, ''),
		       COALESCE(prompt_duration_seconds, 0)
		FROM prompt_runs
		WHERE finished_at IS NOT NULL
	`
	var args []any
	if !since.IsZero() {
		query += " AND finished_at >= ?"
		args = append(args, formatTime(since))
	}
	query += " ORDER BY started_at"

	rows, err := db.sql.Query(query, args...)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	var totalIntervals []timeInterval
	var promptIntervals []timeInterval
	var restartIntervals []timeInterval
	for rows.Next() {
		var startedAt string
		var finishedAt string
		var promptSecs float64
		if err := rows.Scan(&startedAt, &finishedAt, &promptSecs); err != nil {
			return stats, err
		}
		start := parseTime(startedAt)
		end := parseTime(finishedAt)
		if start.IsZero() || end.IsZero() || !end.After(start) {
			continue
		}
		if !since.IsZero() && start.Before(since) {
			start = since
			if !end.After(start) {
				continue
			}
		}
		stats.Count++
		promptEnd := start.Add(time.Duration(promptSecs * float64(time.Second)))
		if promptEnd.Before(start) {
			promptEnd = start
		}
		if promptEnd.After(end) {
			promptEnd = end
		}
		totalIntervals = append(totalIntervals, timeInterval{start: start, end: end})
		if promptEnd.After(start) {
			promptIntervals = append(promptIntervals, timeInterval{start: start, end: promptEnd})
		}
		if end.After(promptEnd) {
			restartIntervals = append(restartIntervals, timeInterval{start: promptEnd, end: end})
		}
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}

	stats.ActiveWait = mergedSeconds(totalIntervals)
	stats.ActivePrompt = mergedSeconds(promptIntervals)
	stats.ActiveRestart = mergedSeconds(restartIntervals)
	return stats, nil
}

func mergedSeconds(intervals []timeInterval) float64 {
	if len(intervals) == 0 {
		return 0
	}
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].start.Before(intervals[j].start)
	})

	current := intervals[0]
	var total float64
	for _, interval := range intervals[1:] {
		if interval.start.After(current.end) {
			total += current.end.Sub(current.start).Seconds()
			current = interval
			continue
		}
		if interval.end.After(current.end) {
			current.end = interval.end
		}
	}
	total += current.end.Sub(current.start).Seconds()
	return total
}

func (db *DB) LoadPromptStats(appID string) (PromptStats, error) {
	var stats PromptStats
	err := db.sql.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(user_wait_seconds), 0),
		       COALESCE(SUM(prompt_duration_seconds), 0),
		       COALESCE(SUM(MAX(user_wait_seconds - prompt_duration_seconds, 0)), 0),
		       COALESCE(MIN(user_wait_seconds), 0),
		       COALESCE(MAX(user_wait_seconds), 0)
		FROM prompt_runs
		WHERE app_id = ? AND finished_at IS NOT NULL
	`, appID).Scan(&stats.Count, &stats.TotalWait, &stats.TotalPrompt, &stats.TotalRestart, &stats.MinWait, &stats.MaxWait)
	if err != nil {
		return stats, err
	}

	rows, err := db.sql.Query(`
		SELECT prompt_index,
		       COALESCE(prompt_duration_seconds, 0),
		       COALESCE(user_wait_seconds, 0),
		       COALESCE(restart_result, ''),
		       COALESCE(finished_at, ''),
		       permission_count
		FROM prompt_runs
		WHERE app_id = ? AND finished_at IS NOT NULL
		ORDER BY finished_at DESC
		LIMIT 10
	`, appID)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var record PromptRecord
		var finishedAt string
		if err := rows.Scan(
			&record.Index,
			&record.PromptSecs,
			&record.WaitSecs,
			&record.Result,
			&finishedAt,
			&record.Permissions,
		); err != nil {
			return stats, err
		}
		record.FinishedAt = parseTime(finishedAt)
		record.RestartSecs = record.WaitSecs - record.PromptSecs
		if record.RestartSecs < 0 {
			record.RestartSecs = 0
		}
		if stats.LastWait == 0 {
			stats.LastWait = record.WaitSecs
			stats.LastPrompt = record.PromptSecs
			stats.LastRestart = record.RestartSecs
			stats.LastResult = record.Result
		}
		stats.Recent = append(stats.Recent, record)
	}
	return stats, rows.Err()
}

func (db *DB) LoadEvents(appID string, limit int) ([]EventRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.sql.Query(`
		SELECT type, detail, created_at
		FROM events
		WHERE app_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, appID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []EventRecord
	for rows.Next() {
		var record EventRecord
		var createdAt string
		if err := rows.Scan(&record.Type, &record.Detail, &createdAt); err != nil {
			return nil, err
		}
		record.At = parseTime(createdAt)
		records = append(records, record)
	}
	return records, rows.Err()
}

func (db *DB) InsertEvent(appID, eventType, detail string, at time.Time) error {
	_, err := db.sql.Exec(
		"INSERT INTO events (app_id, type, detail, created_at) VALUES (?, ?, ?, ?)",
		appID, eventType, detail, formatTime(at),
	)
	return err
}

func (db *DB) InsertPromptStarted(appID string, promptIndex int, startedAt time.Time) (int64, error) {
	result, err := db.sql.Exec(
		"INSERT INTO prompt_runs (app_id, prompt_index, started_at) VALUES (?, ?, ?)",
		appID, promptIndex, formatTime(startedAt),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) FinishPrompt(id int64, finishedAt time.Time, promptSecs, waitSecs float64, permissions int, result string) error {
	if id == 0 {
		return nil
	}
	_, err := db.sql.Exec(`
		UPDATE prompt_runs
		SET finished_at = ?,
		    prompt_duration_seconds = ?,
		    user_wait_seconds = ?,
		    permission_count = ?,
		    restart_result = ?,
		    health_result = ?
		WHERE id = ?
	`, formatTime(finishedAt), promptSecs, waitSecs, permissions, result, result, id)
	return err
}

func (db *DB) InsertRestartRequested(appID string, promptRunID int64, requestedAt time.Time) (int64, error) {
	var promptArg any
	if promptRunID != 0 {
		promptArg = promptRunID
	}
	result, err := db.sql.Exec(
		"INSERT INTO restart_runs (app_id, prompt_run_id, requested_at) VALUES (?, ?, ?)",
		appID, promptArg, formatTime(requestedAt),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) MarkRestartStarted(id int64, startedAt time.Time) error {
	if id == 0 {
		return nil
	}
	_, err := db.sql.Exec("UPDATE restart_runs SET started_at = ? WHERE id = ?", formatTime(startedAt), id)
	return err
}

func (db *DB) FinishRestart(id int64, finishedAt, healthyAt time.Time, result, failureReason string) error {
	if id == 0 {
		return nil
	}
	healthyValue := any(nil)
	if !healthyAt.IsZero() {
		healthyValue = formatTime(healthyAt)
	}
	_, err := db.sql.Exec(`
		UPDATE restart_runs
		SET finished_at = ?, healthy_at = ?, result = ?, failure_reason = ?
		WHERE id = ?
	`, formatTime(finishedAt), healthyValue, result, failureReason, id)
	return err
}

func (db *DB) InsertNotification(appID, notificationType string, repeat int, at time.Time) error {
	_, err := db.sql.Exec(
		"INSERT INTO notifications (app_id, type, repeat_count, created_at) VALUES (?, ?, ?, ?)",
		appID, notificationType, repeat, formatTime(at),
	)
	return err
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
