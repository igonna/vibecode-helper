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
	ID                  string
	Name                string
	CWD                 string
	CommandEnabled      bool
	PauseOnPromptStart  bool
	AutoPauseAfterReady bool
	Command             []string
	HealthMode          string
	HealthURL           string
	HealthCommand       []string
	StartupTimeoutSecs  int
	SoundRepeatCount    int
	CodexEnvironment    string
	RunnerEnvironment   string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ArchivedAt          time.Time
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

type ActivePrompt struct {
	ID          int64
	StartedAt   time.Time
	Permissions int
}

type ActiveRestart struct {
	ID          int64
	PromptID    int64
	RequestedAt time.Time
	StartedAt   time.Time
	FinishedAt  time.Time
}

type GlobalStats struct {
	Count                 int
	ActiveWait            float64
	FreeTime              float64
	EstimatedHumanTime    float64
	MedianPromptingGap    float64
	MedianPromptingSample int
	Span                  float64
	FirstStart            time.Time
}

type EventRecord struct {
	At     time.Time
	Type   string
	Detail string
}

const staleRestoredPromptSeconds = 3600

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
	if err := db.ensureColumn("apps", "archived_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := db.ensureColumn("apps", "command_enabled", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := db.ensureColumn("apps", "pause_on_prompt_start", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return db.ensureColumn("apps", "auto_pause_after_ready", "INTEGER NOT NULL DEFAULT 0")
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
		SELECT id, name, cwd, command_enabled, pause_on_prompt_start, auto_pause_after_ready, command_json, health_mode, health_url,
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
			&record.CommandEnabled,
			&record.PauseOnPromptStart,
			&record.AutoPauseAfterReady,
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
			id, name, cwd, command_enabled, pause_on_prompt_start, auto_pause_after_ready, command_json, health_mode, health_url,
			health_command_json, startup_timeout_seconds, sound_repeat_count,
			codex_environment, runner_environment, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			cwd = excluded.cwd,
			command_enabled = excluded.command_enabled,
			pause_on_prompt_start = excluded.pause_on_prompt_start,
			auto_pause_after_ready = excluded.auto_pause_after_ready,
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
	`, record.ID, record.Name, record.CWD, record.CommandEnabled, record.PauseOnPromptStart, record.AutoPauseAfterReady, string(commandJSON), record.HealthMode, record.HealthURL,
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
			id, name, cwd, command_enabled, pause_on_prompt_start, auto_pause_after_ready, command_json, health_mode, health_url,
			health_command_json, startup_timeout_seconds, sound_repeat_count,
			codex_environment, runner_environment, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.ID, record.Name, record.CWD, record.CommandEnabled, record.PauseOnPromptStart, record.AutoPauseAfterReady, string(commandJSON), record.HealthMode, record.HealthURL,
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
	return db.loadGlobalStatsSince(time.Time{}, time.Now())
}

func (db *DB) LoadTodayGlobalStats(now time.Time) (GlobalStats, error) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return db.loadGlobalStatsSince(start, now)
}

func (db *DB) loadGlobalStatsSince(since, until time.Time) (GlobalStats, error) {
	var stats GlobalStats
	query := `
		SELECT started_at,
		       COALESCE(finished_at, '')
		FROM prompt_runs
		WHERE finished_at IS NOT NULL
		  AND NOT (COALESCE(prompt_duration_seconds, 0) = 0 AND COALESCE(user_wait_seconds, 0) > ?)
	`
	var args []any = []any{staleRestoredPromptSeconds}
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
	var firstStart time.Time
	var lastEnd time.Time
	for rows.Next() {
		var startedAt string
		var finishedAt string
		if err := rows.Scan(&startedAt, &finishedAt); err != nil {
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
		if firstStart.IsZero() || start.Before(firstStart) {
			firstStart = start
		}
		if end.After(lastEnd) {
			lastEnd = end
		}
		stats.Count++
		totalIntervals = append(totalIntervals, timeInterval{start: start, end: end})
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}

	stats.ActiveWait = mergedSeconds(totalIntervals)
	stats.MedianPromptingGap, stats.EstimatedHumanTime, stats.MedianPromptingSample = promptingGapStats(totalIntervals)
	if !firstStart.IsZero() && lastEnd.After(firstStart) {
		spanEnd := lastEnd
		if until.After(spanEnd) {
			spanEnd = until
		}
		stats.Span = spanEnd.Sub(firstStart).Seconds()
		stats.FirstStart = firstStart
		stats.FreeTime = stats.Span - stats.EstimatedHumanTime
		if stats.FreeTime < 0 {
			stats.FreeTime = 0
		}
	}
	return stats, nil
}

func mergedSeconds(intervals []timeInterval) float64 {
	var total float64
	for _, interval := range mergeIntervals(intervals) {
		total += interval.end.Sub(interval.start).Seconds()
	}
	return total
}

func mergeIntervals(intervals []timeInterval) []timeInterval {
	if len(intervals) == 0 {
		return nil
	}
	ordered := append([]timeInterval(nil), intervals...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].start.Before(ordered[j].start)
	})

	merged := []timeInterval{ordered[0]}
	for _, interval := range ordered[1:] {
		last := &merged[len(merged)-1]
		if interval.start.After(last.end) {
			merged = append(merged, interval)
			continue
		}
		if interval.end.After(last.end) {
			last.end = interval.end
		}
	}
	return merged
}

func promptingGapStats(intervals []timeInterval) (float64, float64, int) {
	merged := mergeIntervals(intervals)
	if len(merged) < 2 {
		return 0, 0, 0
	}
	gaps := make([]float64, 0, len(merged)-1)
	previous := merged[0]
	for _, interval := range merged[1:] {
		if interval.start.After(previous.end) {
			gaps = append(gaps, interval.start.Sub(previous.end).Seconds())
		}
		previous = interval
	}
	if len(gaps) == 0 {
		return 0, 0, 0
	}
	sort.Float64s(gaps)
	mid := len(gaps) / 2
	var median float64
	if len(gaps)%2 == 1 {
		median = gaps[mid]
	} else {
		median = (gaps[mid-1] + gaps[mid]) / 2
	}

	capSeconds := median * 3
	maxCapSeconds := (15 * time.Minute).Seconds()
	if capSeconds > maxCapSeconds {
		capSeconds = maxCapSeconds
	}

	var estimated float64
	for _, gap := range gaps {
		if gap > capSeconds {
			estimated += capSeconds
			continue
		}
		estimated += gap
	}
	return median, estimated, len(gaps)
}

func (db *DB) LoadPromptStats(appID string) (PromptStats, error) {
	return db.loadPromptStats(appID, time.Time{})
}

func (db *DB) LoadTodayPromptStats(appID string, now time.Time) (PromptStats, error) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return db.loadPromptStatsForWindow(appID, start, now)
}

func (db *DB) loadPromptStats(appID string, since time.Time) (PromptStats, error) {
	var stats PromptStats
	where := `
		app_id = ?
		AND finished_at IS NOT NULL
		AND NOT (COALESCE(prompt_duration_seconds, 0) = 0 AND COALESCE(user_wait_seconds, 0) > ?)
	`
	args := []any{appID, staleRestoredPromptSeconds}
	if !since.IsZero() {
		where += " AND started_at >= ?"
		args = append(args, formatTime(since))
	}
	err := db.sql.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(user_wait_seconds), 0),
		       COALESCE(SUM(prompt_duration_seconds), 0),
		       COALESCE(SUM(MAX(user_wait_seconds - prompt_duration_seconds, 0)), 0),
		       COALESCE(MIN(user_wait_seconds), 0),
		       COALESCE(MAX(user_wait_seconds), 0)
		FROM prompt_runs
		WHERE `+where, args...).Scan(&stats.Count, &stats.TotalWait, &stats.TotalPrompt, &stats.TotalRestart, &stats.MinWait, &stats.MaxWait)
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
		WHERE `+where+`
		ORDER BY finished_at DESC
		LIMIT 10
	`, args...)
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

func (db *DB) loadPromptStatsForWindow(appID string, since, until time.Time) (PromptStats, error) {
	var stats PromptStats
	rows, err := db.sql.Query(`
		SELECT prompt_index,
		       started_at,
		       finished_at,
		       COALESCE(prompt_duration_seconds, 0),
		       COALESCE(restart_result, ''),
		       permission_count
		FROM prompt_runs
		WHERE app_id = ?
		  AND finished_at IS NOT NULL
		  AND finished_at >= ?
		  AND NOT (COALESCE(prompt_duration_seconds, 0) = 0 AND COALESCE(user_wait_seconds, 0) > ?)
		ORDER BY finished_at DESC
	`, appID, formatTime(since), staleRestoredPromptSeconds)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	var minWait float64
	for rows.Next() {
		var record PromptRecord
		var startedAt, finishedAt string
		if err := rows.Scan(&record.Index, &startedAt, &finishedAt, &record.PromptSecs, &record.Result, &record.Permissions); err != nil {
			return stats, err
		}
		start := parseTime(startedAt)
		end := parseTime(finishedAt)
		if start.IsZero() || end.IsZero() || !end.After(start) {
			continue
		}
		if start.Before(since) {
			start = since
		}
		if end.After(until) {
			end = until
		}
		if !end.After(start) {
			continue
		}
		record.FinishedAt = parseTime(finishedAt)
		record.WaitSecs = end.Sub(start).Seconds()
		record.RestartSecs = record.WaitSecs - record.PromptSecs
		if record.RestartSecs < 0 {
			record.RestartSecs = 0
		}
		stats.Count++
		stats.TotalWait += record.WaitSecs
		stats.TotalPrompt += record.PromptSecs
		stats.TotalRestart += record.RestartSecs
		if minWait == 0 || record.WaitSecs < minWait {
			minWait = record.WaitSecs
		}
		if record.WaitSecs > stats.MaxWait {
			stats.MaxWait = record.WaitSecs
		}
		if stats.LastWait == 0 {
			stats.LastWait = record.WaitSecs
			stats.LastPrompt = record.PromptSecs
			stats.LastRestart = record.RestartSecs
			stats.LastResult = record.Result
		}
		if len(stats.Recent) < 10 {
			stats.Recent = append(stats.Recent, record)
		}
	}
	stats.MinWait = minWait
	return stats, rows.Err()
}

func (db *DB) LoadActivePrompt(appID string, notBefore time.Time) (ActivePrompt, bool, error) {
	var record ActivePrompt
	var startedAt string
	err := db.sql.QueryRow(`
		SELECT id, started_at, permission_count
		FROM prompt_runs
		WHERE app_id = ? AND finished_at IS NULL AND started_at >= ?
		ORDER BY id DESC
		LIMIT 1
	`, appID, formatTime(notBefore)).Scan(&record.ID, &startedAt, &record.Permissions)
	if err == sql.ErrNoRows {
		return record, false, nil
	}
	if err != nil {
		return record, false, err
	}
	record.StartedAt = parseTime(startedAt)
	return record, true, nil
}

func (db *DB) LoadActiveRestart(appID string) (ActiveRestart, bool, error) {
	var record ActiveRestart
	var promptID sql.NullInt64
	var requestedAt, startedAt, finishedAt sql.NullString
	err := db.sql.QueryRow(`
		SELECT id, prompt_run_id, requested_at, started_at, finished_at
		FROM restart_runs
		WHERE app_id = ? AND result IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, appID).Scan(&record.ID, &promptID, &requestedAt, &startedAt, &finishedAt)
	if err == sql.ErrNoRows {
		return record, false, nil
	}
	if err != nil {
		return record, false, err
	}
	if promptID.Valid {
		record.PromptID = promptID.Int64
	}
	if requestedAt.Valid {
		record.RequestedAt = parseTime(requestedAt.String)
	}
	if startedAt.Valid {
		record.StartedAt = parseTime(startedAt.String)
	}
	if finishedAt.Valid {
		record.FinishedAt = parseTime(finishedAt.String)
	}
	return record, true, nil
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
