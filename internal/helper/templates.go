package helper

const dashboardTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Vibecode Helper</title>
  <script src="https://unpkg.com/htmx.org@2.0.4" defer></script>
  <style>
    :root {
      color-scheme: dark;
      --bg: #10100f;
      --panel: #1a1917;
      --panel-soft: #201f1c;
      --text: #f4f1e8;
      --muted: #aaa397;
      --line: #34312c;
      --accent: #58c98d;
      --danger: #ff7b6e;
      --warn: #e4b15f;
      --shadow: 0 14px 34px rgba(0, 0, 0, .34);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--bg);
      color: var(--text);
      font: 14px/1.45 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    header {
      max-width: 1040px;
      margin: 0 auto;
      padding: 30px 20px 14px;
      text-align: center;
    }
    h1 {
      margin: 0 0 16px;
      font-size: 26px;
      font-weight: 680;
      letter-spacing: 0;
    }
    .topbar {
      display: grid;
      grid-template-columns: 1fr auto;
      gap: 10px;
      align-items: center;
      max-width: 700px;
      margin: 0 auto;
    }
    .create summary {
      list-style: none;
    }
    .create summary::-webkit-details-marker { display: none; }
    .create summary:focus,
    .create summary:focus-visible,
    .app-card summary:focus,
    .app-card summary:focus-visible,
    .section:focus,
    .details:focus {
      outline: none;
    }
    .create form {
      position: absolute;
      z-index: 4;
      right: 20px;
      left: 20px;
      max-width: 620px;
      margin: 12px auto 0;
      padding: 16px;
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
      text-align: left;
    }
    .create-title {
      margin: 0 0 12px;
      font-size: 15px;
      font-weight: 650;
      color: var(--text);
    }
    .create .actions {
      justify-content: flex-end;
    }
    .search-wrap {
      position: relative;
      display: grid;
      align-items: center;
    }
    .search-icon {
      position: absolute;
      left: 14px;
      width: 16px;
      height: 16px;
      color: var(--muted);
      pointer-events: none;
    }
    .search {
      width: 100%;
      border: 1px solid var(--line);
      border-radius: 8px;
      height: 42px;
      padding: 0 42px;
      font: inherit;
      background: var(--panel-soft);
      color: var(--text);
    }
    .search::placeholder { color: #7f786d; }
    .search::-webkit-search-cancel-button { display: none; }
    .search-clear {
      position: absolute;
      right: 6px;
      display: none;
      place-items: center;
      width: 30px;
      height: 30px;
      border: 0;
      border-radius: 7px;
      background: transparent;
      color: var(--muted);
      cursor: pointer;
    }
    .search-clear:hover {
      background: #282620;
      color: var(--text);
    }
    .search-clear svg {
      width: 15px;
      height: 15px;
    }
    .search-wrap.has-value .search-clear { display: grid; }
    .search:focus,
    .search:focus-visible {
      outline: none;
      border-color: var(--line);
      box-shadow: none;
    }
    .button {
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 10px 12px;
      background: var(--panel-soft);
      color: var(--text);
      font: inherit;
      cursor: pointer;
      white-space: nowrap;
    }
    .button:hover { border-color: #5a554d; background: #282620; }
    .add-button {
      height: 42px;
      display: inline-flex;
      align-items: center;
      padding: 0 13px;
    }
    .button:focus,
    .button:focus-visible,
    .icon-button:focus,
    .icon-button:focus-visible,
    .form-grid input:focus,
    .form-grid input:focus-visible,
    .form-grid select:focus,
    .form-grid select:focus-visible {
      outline: none;
      border-color: #5a554d;
      box-shadow: 0 0 0 2px rgba(88, 201, 141, .12);
    }
    .sound-status {
      max-width: 700px;
      margin: 12px auto 0;
      display: inline-flex;
      justify-content: center;
      align-items: center;
      gap: 7px;
      min-height: 30px;
      padding: 0 10px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel-soft);
      color: var(--muted);
      font-size: 12px;
      line-height: 1;
    }
    .sound-dot {
      width: 6px;
      height: 6px;
      border-radius: 999px;
      background: var(--warn);
      box-shadow: 0 0 0 3px rgba(228, 177, 95, .12);
    }
    .sound-status.enabled { color: var(--muted); }
    .sound-status.enabled .sound-dot {
      background: var(--accent);
      box-shadow: 0 0 0 3px rgba(88, 201, 141, .12);
    }
    .sound-status.needs-action { color: var(--warn); }
    .sound-status .button {
      height: 24px;
      padding: 0 8px;
      font-size: 12px;
      border-radius: 6px;
    }
    main {
      max-width: 1040px;
      margin: 0 auto;
      padding: 12px 20px 48px;
    }
    .global-stats {
      margin: 2px 0 12px;
      padding: 10px 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
      box-shadow: var(--shadow);
    }
    .global-row {
      display: grid;
      grid-template-columns: 190px minmax(0, 1fr);
      gap: 18px;
      align-items: center;
      min-height: 46px;
    }
    .global-row + .global-row {
      border-top: 1px solid var(--line);
      margin-top: 8px;
      padding-top: 8px;
    }
    .global-title {
      font-size: 14px;
      font-weight: 650;
      color: var(--text);
    }
    .global-subtitle {
      color: var(--muted);
      font-size: 11px;
    }
    .global-insight {
      margin-top: 2px;
      color: var(--accent);
      font-size: 11px;
      line-height: 1.25;
    }
    .app-card {
      margin: 12px 0;
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
      overflow: visible;
    }
    .app-card summary {
      display: grid;
      grid-template-columns: 240px 140px minmax(320px, 1fr);
      gap: 14px;
      align-items: center;
      padding: 14px 16px;
      cursor: pointer;
      list-style: none;
    }
    .app-card summary::-webkit-details-marker { display: none; }
    .card-identity {
      min-width: 0;
      display: grid;
      gap: 7px;
    }
    .name {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-weight: 660;
      font-size: 16px;
    }
    .label, .event-detail { color: var(--muted); }
    .card-statuses {
      min-width: 0;
      display: grid;
      gap: 3px;
      align-items: start;
    }
    .metric-strip {
      display: grid;
      grid-template-columns: repeat(5, minmax(0, 1fr));
      align-items: center;
      gap: 0;
      min-width: 360px;
    }
    .metric-strip .metric {
      min-width: 0;
      padding: 0 12px;
      border-left: 1px solid var(--line);
    }
    .metric-strip .metric:first-child {
      border-left: 0;
    }
    .metric-spacer {
      min-height: 1px;
    }
    .problem-box {
      border: 1px solid #74352f;
      border-radius: 8px;
      padding: 12px;
      background: #211211;
      min-width: 0;
    }
    .problem-title {
      color: var(--danger);
      font-weight: 650;
      margin-bottom: 4px;
    }
    .problem-detail {
      color: var(--muted);
      font-size: 13px;
      max-height: 150px;
      overflow: auto;
      overflow-wrap: anywhere;
      white-space: pre-wrap;
    }
    .metric strong {
      display: block;
      font-size: 14px;
      font-weight: 650;
      color: var(--text);
      line-height: 1.2;
      font-variant-numeric: tabular-nums;
    }
    .metric .label {
      display: block;
      font-size: 10px;
      line-height: 1.2;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .metric .subvalue {
      display: block;
      margin-top: 1px;
      color: var(--muted);
      font-size: 10px;
      line-height: 1.1;
      white-space: nowrap;
    }
    .metric.depth-micro strong,
    .metric.depth-micro .subvalue,
    .metric.depth-shallow strong,
    .metric.depth-shallow .subvalue {
      color: var(--warn);
    }
    .metric.depth-solid strong,
    .metric.depth-solid .subvalue,
    .metric.depth-deep strong,
    .metric.depth-deep .subvalue {
      color: var(--accent);
    }
    .card-metrics {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
    .card-metrics .metric-primary strong {
      font-size: 16px;
    }
    .status {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      font-size: 12px;
      line-height: 1.2;
      white-space: nowrap;
      color: var(--muted);
    }
    .status.healthy, .status.running, .status.ready, .status.app-ready, .status.runner-ready, .status.runner-connected { color: var(--accent); }
    .status.runner-disconnected { color: var(--muted); }
    .status.failed, .status.needs-attention { color: var(--danger); }
    .status.restarting, .status.restart-requested, .status.checking-health, .status.app-loading, .status.applying-config, .status.pausing-for-prompt, .status.permission-asked, .status.prompt-active, .status.runner-update-required, .status.stop-requested, .status.stopping { color: var(--warn); }
    .status.paused, .status.paused-for-prompt, .status.stopped, .status.not-started, .status.command-off { color: var(--muted); }
    .status.prompt-active {
      display: inline-flex;
      align-items: center;
      gap: 7px;
    }
    .status.prompt-active::before {
      content: "";
      width: 6px;
      height: 6px;
      border-radius: 999px;
      background: var(--warn);
      animation: pulse 1s ease-in-out infinite;
    }
    @keyframes pulse {
      0%, 100% { opacity: .35; transform: scale(.75); }
      50% { opacity: 1; transform: scale(1); }
    }
    .details {
      border-top: 1px solid var(--line);
      padding: 18px;
      display: grid;
      grid-template-columns: minmax(340px, 1fr) minmax(320px, .82fr);
      gap: 18px;
      align-items: start;
    }
    .details-column {
      display: grid;
      gap: 14px;
      align-content: start;
    }
    .section h2 {
      margin: 0 0 8px;
      font-size: 13px;
      text-transform: uppercase;
      letter-spacing: .04em;
      color: var(--muted);
    }
    .row {
      display: grid;
      grid-template-columns: 120px minmax(0, 1fr);
      gap: 10px;
      padding: 4px 0;
    }
    code, pre {
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size: 12px;
    }
    pre {
      margin: 0;
      padding: 12px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #141311;
      overflow: auto;
      white-space: pre-wrap;
    }
    .actions {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      margin-top: 10px;
    }
    .form-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 10px;
    }
    .form-grid label {
      position: relative;
      display: grid;
      gap: 4px;
      color: var(--muted);
    }
    .form-grid input, .form-grid select {
      width: 100%;
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 9px 76px 9px 10px;
      font: inherit;
      color: var(--text);
      background: #141311;
    }
    .form-grid .wide { grid-column: 1 / -1; }
    .autosave-badge {
      position: absolute;
      right: 9px;
      bottom: 8px;
      display: inline-flex;
      align-items: center;
      gap: 4px;
      min-height: 22px;
      padding: 0 7px;
      border-radius: 6px;
      background: #1a1917;
      color: var(--accent);
      font-size: 11px;
      line-height: 1;
      opacity: 0;
      pointer-events: none;
      transform: translateY(2px);
      transition: opacity .16s ease, transform .16s ease;
    }
    .autosave-badge svg {
      width: 13px;
      height: 13px;
    }
    .autosave-badge.visible {
      opacity: 1;
      transform: translateY(0);
    }
    .autosave-badge.saving {
      color: var(--muted);
    }
    .autosave-badge.error {
      color: var(--danger);
    }
    .section-config .form-grid {
      grid-template-columns: 1fr;
    }
    .check-row {
      position: relative;
      display: flex;
      align-items: center;
      gap: 10px;
      color: var(--text);
      cursor: pointer;
    }
    .check-row input {
      width: 18px;
      height: 18px;
      accent-color: var(--accent);
    }
    .check-toggle {
      position: relative;
      display: flex;
      align-items: center;
      gap: 10px;
      width: 100%;
      min-height: 24px;
      padding: 0 76px 0 0;
      border: 0;
      background: transparent;
      color: var(--text);
      font: inherit;
      text-align: left;
      cursor: pointer;
    }
    .check-toggle-box {
      display: inline-grid;
      place-items: center;
      flex: 0 0 auto;
      width: 18px;
      height: 18px;
      border: 1px solid #57514a;
      border-radius: 4px;
      color: #0f0e0c;
      transition: background .12s ease, border-color .12s ease, color .12s ease;
    }
    .check-toggle-box::after {
      content: "";
      width: 8px;
      height: 5px;
      margin-top: -2px;
      border-left: 2px solid currentColor;
      border-bottom: 2px solid currentColor;
      opacity: 0;
      transform: rotate(-45deg);
    }
    .check-toggle.is-on .check-toggle-box {
      border-color: var(--accent);
      background: var(--accent);
      color: #10100e;
    }
    .check-toggle.is-on .check-toggle-box::after {
      opacity: 1;
    }
    .config-groups {
      display: grid;
      gap: 12px;
    }
    .config-group {
      display: grid;
      gap: 10px;
      padding: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #141311;
    }
    .config-group-title {
      color: var(--muted);
      font-size: 12px;
      font-weight: 650;
      text-transform: uppercase;
      letter-spacing: .04em;
    }
    .config-subgroup {
      display: grid;
      gap: 10px;
      margin-top: 2px;
      padding-top: 12px;
      border-top: 1px solid var(--line);
      transition: opacity .16s ease;
    }
    .config-subgroup.is-inactive {
      opacity: .56;
    }
    .config-title-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
    }
    .dependency-pill {
      color: var(--muted);
      font-size: 11px;
      font-weight: 600;
      text-transform: none;
      letter-spacing: 0;
      white-space: nowrap;
    }
    .setup-list {
      display: grid;
      gap: 10px;
    }
    .setup-item {
      display: grid;
      grid-template-columns: 1fr auto;
      gap: 14px;
      align-items: center;
      min-height: 74px;
      padding: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #141311;
    }
    .setup-title {
      font-weight: 650;
      color: var(--text);
    }
    .setup-detail {
      color: var(--muted);
      font-size: 12px;
      max-width: 46ch;
    }
    .copy-source {
      display: none;
    }
    .icon-button {
      display: inline-grid;
      place-items: center;
      width: 32px;
      height: 32px;
      border: 1px solid #4a453f;
      border-radius: 8px;
      background: var(--panel-soft);
      color: var(--text);
      cursor: pointer;
    }
    .icon-button:hover {
      border-color: var(--line);
      background: #282620;
      color: var(--text);
    }
    .icon-button svg {
      width: 16px;
      height: 16px;
    }
    .restart-form {
      margin: 0;
      justify-self: end;
    }
    .card-actions {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      justify-self: start;
    }
    .command-split {
      position: relative;
      display: inline-flex;
      align-items: stretch;
      min-width: 0;
    }
    .command-split-primary {
      border-top-right-radius: 0;
      border-bottom-right-radius: 0;
    }
    .command-menu-trigger {
      width: 28px;
      height: 28px;
      border-top-left-radius: 0;
      border-bottom-left-radius: 0;
      border-left: 0;
    }
    .command-menu-trigger svg {
      width: 14px;
      height: 14px;
    }
    .command-menu {
      position: absolute;
      top: calc(100% + 6px);
      right: 0;
      z-index: 30;
      display: none;
      min-width: 180px;
      padding: 4px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #141311;
      box-shadow: var(--shadow);
    }
    .command-split.is-open .command-menu {
      display: grid;
      gap: 2px;
    }
    .command-menu .restart-form {
      justify-self: stretch;
    }
    .command-menu-button {
      justify-content: flex-start;
      width: 100%;
      height: 30px;
      border: 0;
      background: transparent;
    }
    .command-menu-button:hover {
      background: #282620;
    }
    .restart-button {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      width: auto;
      height: 28px;
      padding: 0 9px;
      font-size: 12px;
      color: var(--text);
    }
    .pause-button {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      height: 28px;
      padding: 0 9px;
      width: auto;
      font-size: 12px;
    }
    .danger-panel {
      display: grid;
      grid-template-columns: 1fr auto;
      gap: 14px;
      align-items: center;
      border: 1px solid #74352f;
      border-radius: 8px;
      background: #211211;
      min-height: 74px;
      padding: 14px;
    }
    .danger-title {
      color: var(--danger);
      font-weight: 650;
    }
    .danger-detail {
      color: var(--muted);
      font-size: 12px;
    }
    .danger-button {
      border-color: #74352f;
      background: #2b1715;
      color: var(--danger);
    }
    .danger-button:hover {
      border-color: #9c453b;
      background: #351c19;
      color: var(--danger);
    }
    @media (max-width: 840px) {
      .topbar { grid-template-columns: 1fr; }
      .global-row,
      .app-card summary {
        grid-template-columns: 1fr;
        gap: 12px;
      }
      .metric-strip {
        display: flex;
        min-width: 0;
        justify-content: start;
        flex-wrap: wrap;
        gap: 8px 0;
      }
      .metric-strip .metric {
        border-left: 0;
        padding-left: 0;
      }
      .card-actions { justify-self: start; }
      .details { grid-template-columns: 1fr; }
      .details-column { gap: 14px; }
      .form-grid { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body hx-boost="true" hx-target="body" hx-swap="outerHTML">
  <header>
    <h1>Vibecode Helper</h1>
    <div class="topbar">
      <div class="search-wrap" data-search-wrap>
        <svg class="search-icon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m21 21-4.34-4.34"></path><circle cx="11" cy="11" r="8"></circle></svg>
        <input class="search" type="search" placeholder="Search apps" aria-label="Search apps" data-search>
        <button class="search-clear" type="button" aria-label="Clear search" title="Clear search" data-clear-search>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M18 6 6 18"></path><path d="m6 6 12 12"></path></svg>
        </button>
      </div>
      <details class="create">
        <summary class="button add-button">Add app</summary>
        <form method="post" action="/apps" hx-post="/apps" hx-select="main" hx-target="main" hx-swap="outerHTML">
          <div class="create-title">Add app</div>
          <div class="config-groups">
            <div class="config-group">
              <div class="config-group-title">Command</div>
              <div class="form-grid">
                <label>Name <input name="name" required placeholder="my-platform"></label>
                <button class="check-toggle wide is-on" type="button" role="switch" aria-checked="true" data-command-toggle><span class="check-toggle-box" aria-hidden="true"></span><span>Run shell command</span></button>
                <input type="hidden" name="command_enabled" value="1" data-command-value>
                <label class="wide">Shell <input name="command" placeholder="docker compose up --build"></label>
                <button class="check-toggle wide" type="button" role="switch" aria-checked="false" data-pause-prompt-toggle><span class="check-toggle-box" aria-hidden="true"></span><span>Pause during prompt</span></button>
                <input type="hidden" name="pause_on_prompt_start" value="0" data-pause-prompt-value>
                <button class="check-toggle wide" type="button" role="switch" aria-checked="false" data-auto-pause-toggle><span class="check-toggle-box" aria-hidden="true"></span><span>Pause after 5 min ready</span></button>
                <input type="hidden" name="auto_pause_after_ready" value="0" data-auto-pause-value>
              </div>
              <div class="config-subgroup" data-health-dependent>
                <div class="config-title-row">
                  <div class="config-group-title">Health check</div>
                  <div class="dependency-pill" data-health-state>After command</div>
                </div>
                <div class="form-grid">
                  <button class="check-toggle wide" type="button" role="switch" aria-checked="false" data-health-toggle><span class="check-toggle-box" aria-hidden="true"></span><span>Check URL</span></button>
                  <input type="hidden" name="health_enabled" value="0" data-health-value>
                  <label>URL <input name="health_url" placeholder="http://localhost:3000"></label>
                  <label>Timeout <input name="startup_timeout_seconds" type="number" min="1" value="60"></label>
                </div>
              </div>
            </div>
          </div>
          <div class="actions">
            <button class="button" type="submit">Create app</button>
            <button class="button" type="button" data-cancel-create>Cancel</button>
          </div>
        </form>
      </details>
    </div>
  </header>

  <main>
    <section class="global-stats">
      <div class="global-row">
        <div>
          <div class="global-title">Today</div>
          <div class="global-subtitle">Since midnight{{if .Today.Count}} · first prompt {{shortTime .Today.FirstStart}} · {{duration .Today.Span}} span{{end}}</div>
          <div class="global-insight">{{workSummary .Today}}</div>
        </div>
        <div class="metric-strip">
          <div class="metric metric-primary"><span class="label">Human prompting</span><strong>{{duration .Today.EstimatedHumanTime}}</strong><span class="subvalue">typical {{duration .Today.MedianPromptingGap}}</span></div>
          <div class="metric"><span class="label">AI working</span><strong>{{duration .Today.ActiveWait}}</strong></div>
          <div class="metric"><span class="label">Free time</span><strong>{{duration .Today.FreeTime}}</strong></div>
          <div class="metric {{depthClass .Today.ActiveWait .Today.Count}}"><span class="label">Delegation depth</span><strong>{{duration (avg .Today.ActiveWait .Today.Count)}}</strong><span class="subvalue">{{depthHint .Today.ActiveWait .Today.Count}}</span></div>
          <div class="metric"><span class="label">Prompts</span><strong>{{.Today.Count}}</strong></div>
        </div>
      </div>
      <div class="global-row">
        <div>
          <div class="global-title">All time</div>
          <div class="global-subtitle">Saved history</div>
        </div>
        <div class="metric-strip">
          <div class="metric metric-primary"><span class="label">Human prompting</span><strong>{{duration .Global.EstimatedHumanTime}}</strong><span class="subvalue">typical {{duration .Global.MedianPromptingGap}}</span></div>
          <div class="metric"><span class="label">AI working</span><strong>{{duration .Global.ActiveWait}}</strong></div>
          <div class="metric-spacer" aria-hidden="true"></div>
          <div class="metric-spacer" aria-hidden="true"></div>
          <div class="metric"><span class="label">Prompts</span><strong>{{.Global.Count}}</strong></div>
        </div>
      </div>
    </section>

    {{range .Apps}}
    <details class="app-card" data-app-card data-app-id="{{.ID}}" data-search-text="{{.Name}} {{displayStatus .}} {{.HealthURL}}">
      <summary>
        <div class="card-identity">
          <div class="name">{{.Name}}</div>
          <div class="card-actions" data-no-card-toggle>
            {{if needsRunner .}}
            <div class="command-split" data-command-split>
              {{if eq .Status "paused"}}
                <form class="restart-form" method="post" action="/apps/{{.ID}}/restart" hx-post="/apps/{{.ID}}/restart" hx-select="main" hx-target="main" hx-swap="outerHTML">
                  <button class="icon-button restart-button command-split-primary" type="submit" aria-label="Resume command" title="Resume command">{{playIcon}}<span>Resume command</span></button>
                </form>
              {{else}}
                <form class="restart-form" method="post" action="/apps/{{.ID}}/restart" hx-post="/apps/{{.ID}}/restart" hx-select="main" hx-target="main" hx-swap="outerHTML">
                  <button class="icon-button restart-button command-split-primary" type="submit" aria-label="Restart command" title="Restart command">{{rotateCcwIcon}}<span>Restart command</span></button>
                </form>
              {{end}}
                <button class="icon-button command-menu-trigger" type="button" aria-label="More command actions" title="More command actions" aria-expanded="false" data-command-menu-trigger>{{chevronDownIcon}}</button>
                <div class="command-menu" data-command-menu>
              {{if eq .Status "paused"}}
                  <form class="restart-form" method="post" action="/apps/{{.ID}}/restart" hx-post="/apps/{{.ID}}/restart" hx-select="main" hx-target="main" hx-swap="outerHTML">
                    <button class="icon-button restart-button command-menu-button" type="submit" aria-label="Restart command" title="Restart command">{{rotateCcwIcon}}<span>Restart command</span></button>
                  </form>
              {{else}}
                  <form class="restart-form" method="post" action="/apps/{{.ID}}/stop" hx-post="/apps/{{.ID}}/stop" hx-select="main" hx-target="main" hx-swap="outerHTML">
                    <button class="icon-button pause-button command-menu-button" type="submit" aria-label="Pause command" title="Pause command">{{pauseIcon}}<span>Pause command</span></button>
                  </form>
              {{end}}
                </div>
            </div>
            {{end}}
          </div>
        </div>
        <div class="card-statuses">
          <div class="status {{displayStatusClass .}}" data-card-status>{{displayStatus .}}</div>
        </div>
        <div class="card-metrics metric-strip">
          <div class="metric metric-primary"><span class="label">AI today</span><strong>{{duration .TodayWaitSeconds}}</strong></div>
          <div class="metric {{depthClass .TodayWaitSeconds .TodayPromptCount}}"><span class="label">Depth today</span><strong>{{duration (avg .TodayWaitSeconds .TodayPromptCount)}}</strong><span class="subvalue">{{depthHint .TodayWaitSeconds .TodayPromptCount}}</span></div>
          <div class="metric"><span class="label">Prompts today</span><strong>{{.TodayPromptCount}}</strong></div>
        </div>
      </summary>

      <div class="details">
        <div class="details-column">
          {{if .FailureReason}}
          <section class="section">
            <div class="problem-box">
              <div class="problem-title">Config needs attention</div>
              <div class="problem-detail">{{friendlyProblem .FailureReason}}</div>
            </div>
          </section>
          {{end}}
          <section class="section section-config">
            <h2>Config</h2>
            <form method="post" action="/apps/{{.ID}}/update" hx-boost="false" data-autosave-form>
              <div class="config-groups">
                <div class="config-group">
                  <div class="config-group-title">Command</div>
                  <div class="form-grid">
                    <button class="check-toggle wide {{if .CommandEnabled}}is-on{{end}}" type="button" role="switch" aria-checked="{{if .CommandEnabled}}true{{else}}false{{end}}" data-command-toggle><span class="check-toggle-box" aria-hidden="true"></span><span>Run shell command</span><span class="autosave-badge" data-save-badge></span></button>
                    <input type="hidden" name="command_enabled" value="{{if .CommandEnabled}}1{{else}}0{{end}}" data-command-value>
                    <label class="wide">Shell <input name="command" value="{{command .Command " "}}" placeholder="docker compose up --build"><span class="autosave-badge" data-save-badge></span></label>
                    <button class="check-toggle wide {{if .PauseOnPromptStart}}is-on{{end}}" type="button" role="switch" aria-checked="{{if .PauseOnPromptStart}}true{{else}}false{{end}}" data-pause-prompt-toggle><span class="check-toggle-box" aria-hidden="true"></span><span>Pause during prompt</span><span class="autosave-badge" data-save-badge></span></button>
                    <input type="hidden" name="pause_on_prompt_start" value="{{if .PauseOnPromptStart}}1{{else}}0{{end}}" data-pause-prompt-value>
                    <button class="check-toggle wide {{if .AutoPauseAfterReady}}is-on{{end}}" type="button" role="switch" aria-checked="{{if .AutoPauseAfterReady}}true{{else}}false{{end}}" data-auto-pause-toggle><span class="check-toggle-box" aria-hidden="true"></span><span>Pause after 5 min ready</span><span class="autosave-badge" data-save-badge></span></button>
                    <input type="hidden" name="auto_pause_after_ready" value="{{if .AutoPauseAfterReady}}1{{else}}0{{end}}" data-auto-pause-value>
                  </div>
                  <div class="config-subgroup {{if not .CommandEnabled}}is-inactive{{end}}" data-health-dependent>
                    <div class="config-title-row">
                      <div class="config-group-title">Health check</div>
                      <div class="dependency-pill" data-health-state>{{if .CommandEnabled}}After command{{else}}Command off{{end}}</div>
                    </div>
                    <div class="form-grid">
                      <button class="check-toggle wide {{if ne .HealthMode "none"}}is-on{{end}}" type="button" role="switch" aria-checked="{{if ne .HealthMode "none"}}true{{else}}false{{end}}" data-health-toggle><span class="check-toggle-box" aria-hidden="true"></span><span>Check URL</span><span class="autosave-badge" data-save-badge></span></button>
                      <input type="hidden" name="health_enabled" value="{{if ne .HealthMode "none"}}1{{else}}0{{end}}" data-health-value>
                      <label>URL <input name="health_url" value="{{.HealthURL}}"><span class="autosave-badge" data-save-badge></span></label>
                      <label>Timeout <input name="startup_timeout_seconds" type="number" min="1" value="{{.StartupTimeoutSecs}}"><span class="autosave-badge" data-save-badge></span></label>
                    </div>
                  </div>
                </div>
              </div>
            </form>
          </section>
        </div>

        <div class="details-column">
          <section class="section">
            <h2>Setup</h2>
            <div class="setup-list">
              <div class="setup-item">
                <div>
                  <div class="setup-title">Runner command</div>
                  <div class="setup-detail">Run from your app folder and keep it open. Config changes apply automatically.</div>
                </div>
                <button class="icon-button" type="button" aria-label="Copy runner command" title="Copy runner command" data-copy-code="[data-runner-code='{{.ID}}']">{{copyIcon}}</button>
                <pre class="copy-source" data-runner-code="{{.ID}}">{{runnerCommand $.BaseURL .}}</pre>
              </div>
            </div>
          </section>

          <section class="section danger-section">
            <h2>Danger</h2>
            <div class="danger-panel">
              <div>
                <div class="danger-title">Delete app</div>
                <div class="danger-detail">Remove this app and its local helper history.</div>
              </div>
              <form method="post" action="/apps/{{.ID}}/delete" hx-post="/apps/{{.ID}}/delete" hx-select="main" hx-target="main" hx-swap="outerHTML" hx-confirm="Delete {{.Name}}?">
                <button class="button danger-button" type="submit">Delete</button>
              </form>
            </div>
          </section>
        </div>
      </div>
    </details>
    {{end}}
  </main>

  <audio data-audio="success" preload="auto" src="/sound/success"></audio>
  <audio data-audio="failure" preload="auto" src="/sound/failure"></audio>
  <audio data-audio="permission" preload="auto" src="/sound/permission"></audio>

	  <script>
	    function cookieValue(name) {
	      return document.cookie.split('; ').find((item) => item.startsWith(name + '='))?.split('=').slice(1).join('=') || '';
	    }
	    const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
	    if (browserTimezone && decodeURIComponent(cookieValue('vibecode_timezone')) !== browserTimezone) {
	      document.cookie = 'vibecode_timezone=' + encodeURIComponent(browserTimezone) + '; path=/; max-age=31536000; SameSite=Lax';
	      window.location.reload();
	    }

	    let soundEnabled = false;
    const soundStatus = document.createElement('div');
    soundStatus.className = 'sound-status needs-action';
    soundStatus.innerHTML = '<span class="sound-dot" aria-hidden="true"></span><span data-sound-label>Sound needs permission</span><button class="button" type="button" data-enable-sound>Enable sound</button>';
    document.querySelector('header')?.appendChild(soundStatus);

    const soundLabel = soundStatus.querySelector('[data-sound-label]');
    const enableSoundButton = soundStatus.querySelector('[data-enable-sound]');
    const audioByType = new Map(Array.from(document.querySelectorAll('[data-audio]')).map((audio) => [audio.dataset.audio, audio]));

    function setSoundStatus(state, text) {
      soundStatus.classList.toggle('enabled', state === 'enabled');
      soundStatus.classList.toggle('needs-action', state !== 'enabled');
      soundLabel.textContent = text;
      enableSoundButton.hidden = state === 'enabled';
    }

    async function unlockSound() {
      try {
        const audio = audioByType.get('success');
        audio.volume = 0.01;
        await audio.play();
        audio.pause();
        audio.currentTime = 0;
        audio.volume = 1;
        soundEnabled = true;
        setSoundStatus('enabled', 'Sound enabled');
      } catch (error) {
        soundEnabled = false;
        setSoundStatus('blocked', 'Sound needs permission');
      }
    }

    async function playSound(type, repeat) {
      const audio = audioByType.get(type);
      if (!audio) return;
      const count = Math.max(1, Number(repeat || 1));
      for (let index = 0; index < count; index++) {
        try {
          audio.pause();
          audio.currentTime = 0;
          await audio.play();
          await new Promise((resolve) => {
            audio.onended = resolve;
            setTimeout(resolve, 2500);
          });
          soundEnabled = true;
          setSoundStatus('enabled', 'Sound enabled');
        } catch (error) {
          soundEnabled = false;
          setSoundStatus('blocked', 'Sound needs permission');
          break;
        }
      }
    }

    enableSoundButton.addEventListener('click', unlockSound);
    unlockSound();

    const events = new EventSource('/events');
    events.addEventListener('status', () => {
      if (!soundEnabled) setSoundStatus('blocked', 'Sound needs permission');
    });
    events.addEventListener('sound', (message) => {
      const event = JSON.parse(message.data);
      playSound(event.type, event.repeat);
    });
    events.addEventListener('health_check', (message) => {
      const request = JSON.parse(message.data);
      runBrowserHealthCheck(request);
    });
    events.onerror = () => {
      if (!soundEnabled) setSoundStatus('blocked', 'Sound needs permission');
    };

    async function runBrowserHealthCheck(request) {
      const result = {
        request_id: request.request_id,
        ok: false,
        detail: '',
        status: 0
      };
      const timeout = Math.max(1, Number(request.timeout_seconds || 3)) * 1000;
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), timeout);
      try {
        const response = await fetch(request.url, { cache: 'no-store', signal: controller.signal });
        result.status = response.status;
        result.ok = response.status < 500;
        result.detail = 'browser HTTP ' + response.status;
      } catch (firstError) {
        try {
          await fetch(request.url, { cache: 'no-store', mode: 'no-cors', signal: controller.signal });
          result.ok = true;
          result.detail = 'browser reached URL without readable status';
        } catch (secondError) {
          result.ok = false;
          if (secondError && secondError.name === 'AbortError') {
            result.detail = 'browser request timed out after ' + (timeout / 1000) + 's';
          } else {
            result.detail = 'browser could not reach URL: ' + String(secondError && secondError.message ? secondError.message : secondError);
          }
        }
      } finally {
        clearTimeout(timer);
      }

      await fetch('/browser-health-result', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(result)
      });
    }

    const search = document.querySelector('[data-search]');
    const searchWrap = document.querySelector('[data-search-wrap]');
    const clearSearch = document.querySelector('[data-clear-search]');
    function updateSearchUI() {
      searchWrap?.classList.toggle('has-value', Boolean(search?.value));
    }
    search?.addEventListener('input', () => {
      const query = search.value.trim().toLowerCase();
      document.querySelectorAll('[data-app-card]').forEach((card) => {
        const text = card.dataset.searchText.toLowerCase();
        card.hidden = query && !text.includes(query);
      });
      updateSearchUI();
      sessionStorage.setItem('vibecode-helper-search', search.value);
    });
    clearSearch?.addEventListener('click', () => {
      if (!search) return;
      search.value = '';
      search.dispatchEvent(new Event('input'));
      search.focus();
    });

    function rememberOpenCards() {
      const openIDs = Array.from(document.querySelectorAll('[data-app-card][open]')).map((card) => card.dataset.appId);
      sessionStorage.setItem('vibecode-helper-open-apps', JSON.stringify(openIDs));
      if (search) sessionStorage.setItem('vibecode-helper-search', search.value);
    }

    function restoreDashboardState(restoreCards) {
      const rawOpen = sessionStorage.getItem('vibecode-helper-open-apps') || '[]';
      let openIDs = [];
      try {
        openIDs = JSON.parse(rawOpen);
      } catch (error) {
        openIDs = [];
      }
      if (restoreCards) {
        const openSet = new Set(openIDs);
        document.querySelectorAll('[data-app-card]').forEach((card) => {
          card.open = openSet.has(card.dataset.appId);
        });
      }

      const savedSearch = sessionStorage.getItem('vibecode-helper-search');
      if (search && savedSearch !== null) {
        search.value = savedSearch;
        search.dispatchEvent(new Event('input'));
      }
    }

    document.body.addEventListener('htmx:beforeRequest', rememberOpenCards);
    document.body.addEventListener('htmx:afterRequest', (event) => {
      const form = event.target && event.target.closest ? event.target.closest('.create form') : null;
      if (!form || !event.detail.successful) return;
      form.reset();
      syncCommandDependencies(form);
      const create = form.closest('.create');
      if (create) create.open = false;
    });
    document.body.addEventListener('htmx:afterSwap', () => {
      restoreDashboardState(true);
      bindNoCardToggleControls();
      bindCommandSplitMenus();
      bindCommandDependencies();
      bindAutosaveForms();
    });
    restoreDashboardState(false);

    document.addEventListener('click', (event) => {
      const button = event.target.closest('[data-cancel-create]');
      if (!button) return;
      const create = button.closest('.create');
      if (create) create.open = false;
    });

    function bindNoCardToggleControls() {
      document.querySelectorAll('[data-no-card-toggle]:not([data-bound-no-toggle])').forEach((control) => {
        control.dataset.boundNoToggle = '1';
        control.addEventListener('click', (event) => event.stopPropagation());
        control.addEventListener('keydown', (event) => event.stopPropagation());
      });
    }
    bindNoCardToggleControls();

    function closeCommandSplitMenus(except) {
      document.querySelectorAll('[data-command-split].is-open').forEach((split) => {
        if (split === except) return;
        split.classList.remove('is-open');
        split.querySelector('[data-command-menu-trigger]')?.setAttribute('aria-expanded', 'false');
      });
    }

    function bindCommandSplitMenus() {
      document.querySelectorAll('[data-command-menu-trigger]:not([data-command-menu-bound])').forEach((trigger) => {
        trigger.dataset.commandMenuBound = '1';
        trigger.addEventListener('click', (event) => {
          event.preventDefault();
          event.stopPropagation();
          const split = trigger.closest('[data-command-split]');
          const open = !split?.classList.contains('is-open');
          closeCommandSplitMenus(open ? split : null);
          if (!split) return;
          split.classList.toggle('is-open', open);
          trigger.setAttribute('aria-expanded', open ? 'true' : 'false');
        });
      });
    }
    bindCommandSplitMenus();

    document.addEventListener('click', (event) => {
      if (event.target.closest('[data-command-split]')) return;
      closeCommandSplitMenus();
    });
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') closeCommandSplitMenus();
    });

    let formInteractionUntil = 0;
    function pauseRefreshForForms() {
      formInteractionUntil = Date.now() + 4000;
    }
    document.addEventListener('pointerdown', (event) => {
      if (event.target.closest('form')) pauseRefreshForForms();
    }, true);
    document.addEventListener('input', (event) => {
      if (event.target.closest('form')) pauseRefreshForForms();
    }, true);
    document.addEventListener('change', (event) => {
      if (event.target.closest('form')) pauseRefreshForForms();
    }, true);
    document.addEventListener('focusin', (event) => {
      if (event.target.closest('form')) pauseRefreshForForms();
    }, true);

    function userIsEditing() {
      const active = document.activeElement;
      if (Date.now() < formInteractionUntil) return true;
      if (!active) return false;
      if (active.closest('.create[open]')) return true;
      return Boolean(active.closest('form'));
    }

    function captureScrollState() {
      const panels = {};
      document.querySelectorAll('[data-scroll-key]').forEach((element) => {
        panels[element.dataset.scrollKey] = {
          top: element.scrollTop,
          left: element.scrollLeft
        };
      });
      return {
        x: window.scrollX,
        y: window.scrollY,
        panels
      };
    }

    function restoreScrollState(state) {
      if (!state) return;
      Object.entries(state.panels || {}).forEach(([key, position]) => {
        const element = document.querySelector('[data-scroll-key="' + CSS.escape(key) + '"]');
        if (!element) return;
        element.scrollTop = position.top || 0;
        element.scrollLeft = position.left || 0;
      });
      window.scrollTo(state.x || 0, state.y || 0);
    }

    let dashboardRefreshInFlight = false;
    let lastDashboardRefreshAt = 0;
    async function refreshDashboard(options) {
      const force = options && options.force === true;
      if (dashboardRefreshInFlight || document.hidden || (!force && userIsEditing())) return;
      dashboardRefreshInFlight = true;
      lastDashboardRefreshAt = Date.now();
      rememberOpenCards();
      const scrollState = captureScrollState();
      try {
        const response = await fetch(window.location.href, { cache: 'no-store' });
        if (!response.ok) return;
        const html = await response.text();
        const doc = new DOMParser().parseFromString(html, 'text/html');
        const nextMain = doc.querySelector('main');
        const main = document.querySelector('main');
        if (!nextMain || !main) return;
        main.replaceWith(nextMain);
        if (window.htmx) window.htmx.process(nextMain);
        restoreDashboardState(true);
        bindNoCardToggleControls();
        bindCommandDependencies();
        bindAutosaveForms();
        restoreScrollState(scrollState);
      } catch (error) {
        // Keep the dashboard stable if a refresh races with a container restart.
      } finally {
        dashboardRefreshInFlight = false;
      }
    }
    function refreshDashboardSoon() {
      if (Date.now() - lastDashboardRefreshAt < 3000) return;
      refreshDashboard();
    }
    window.setInterval(refreshDashboard, 60000);
    window.addEventListener('focus', refreshDashboardSoon);
    document.addEventListener('visibilitychange', () => {
      if (!document.hidden) refreshDashboardSoon();
    });

    const copyIcon = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"></rect><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"></path></svg>';
    const checkIcon = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 6 9 17l-5-5"></path></svg>';

    const autosaveTextDelayMs = 1400;
    const autosaveStates = new WeakMap();

    function isToggleValue(field) {
      return field?.dataset?.commandValue !== undefined || field?.dataset?.pausePromptValue !== undefined || field?.dataset?.autoPauseValue !== undefined || field?.dataset?.healthValue !== undefined;
    }

    function isInstantAutosaveField(field) {
      return isToggleValue(field) || field.type === 'checkbox' || field.type === 'radio' || field.tagName === 'SELECT';
    }

    function autosaveSnapshot(form) {
      const values = [];
      form.querySelectorAll('input, textarea, select').forEach((field) => {
        if (!field.name || field.disabled) return;
        if (field.type === 'checkbox' || field.type === 'radio') {
          values.push([field.name, field.type, field.checked ? (field.value || '1') : '0']);
          return;
        }
        values.push([field.name, field.type, field.value]);
      });
      return JSON.stringify(values);
    }

    function autosaveState(form) {
      let state = autosaveStates.get(form);
      if (!state) {
        state = {
          dirty: false,
          inFlight: false,
          lastField: null,
          lastSaved: autosaveSnapshot(form),
          timer: 0
        };
        autosaveStates.set(form, state);
      }
      return state;
    }

    function saveBadgeForField(field) {
      if (field?.dataset?.commandValue !== undefined) {
        return field.closest('form')?.querySelector('[data-command-toggle] [data-save-badge]');
      }
      if (field?.dataset?.pausePromptValue !== undefined) {
        return field.closest('form')?.querySelector('[data-pause-prompt-toggle] [data-save-badge]');
      }
      if (field?.dataset?.autoPauseValue !== undefined) {
        return field.closest('form')?.querySelector('[data-auto-pause-toggle] [data-save-badge]');
      }
      if (field?.dataset?.healthValue !== undefined) {
        return field.closest('form')?.querySelector('[data-health-toggle] [data-save-badge]');
      }
      return field.closest('label')?.querySelector('[data-save-badge]') ||
        field.closest('[data-autosave-form]')?.querySelector('[data-save-badge]');
    }

    function saveBadgeForForm(form, field) {
      if (field && field.form === form) return saveBadgeForField(field);
      return form.querySelector('[data-save-badge]');
    }

    function setSaveBadge(badge, state) {
      if (!badge) return;
      badge.classList.remove('saving', 'error');
      badge.classList.add('visible');
      if (state === 'saving') {
        badge.classList.add('saving');
        badge.textContent = 'Saving';
      } else if (state === 'error') {
        badge.classList.add('error');
        badge.textContent = 'Not saved';
      } else {
        badge.innerHTML = checkIcon + '<span>Saved</span>';
      }
      window.clearTimeout(badge._hideTimer);
      badge._hideTimer = window.setTimeout(() => {
        badge.classList.remove('visible', 'saving', 'error');
      }, state === 'saved' ? 1800 : 2600);
    }

    async function autosaveForm(form, field) {
      if (!form || !form.isConnected) return;
      const state = autosaveState(form);
      if (state.inFlight) {
        state.dirty = true;
        if (field) state.lastField = field;
        return;
      }

      const snapshot = autosaveSnapshot(form);
      if (snapshot === state.lastSaved) return;

      const badge = saveBadgeForForm(form, field || state.lastField);
      setSaveBadge(badge, 'saving');
      pauseRefreshForForms();
      state.inFlight = true;
      state.dirty = false;
      let saveLatestAfterThis = false;

      try {
        const response = await fetch(form.action, {
          method: 'POST',
          body: new FormData(form),
          cache: 'no-store',
          headers: { 'X-Requested-With': 'vibecode-autosave' }
        });
        if (!response.ok) throw new Error('save failed');
        state.lastSaved = snapshot;
        saveLatestAfterThis = form.isConnected && autosaveSnapshot(form) !== snapshot;
        if (!saveLatestAfterThis) {
          setSaveBadge(badge, 'saved');
          if (isToggleValue(field || state.lastField)) {
            window.setTimeout(() => refreshDashboard({ force: true }), 250);
          }
        }
      } catch (error) {
        setSaveBadge(badge, 'error');
      } finally {
        state.inFlight = false;
        state.dirty = false;
        if (saveLatestAfterThis) {
          scheduleAutosave(form, state.lastField || field, 0);
        }
      }
    }

    function scheduleAutosave(form, field, delay) {
      const state = autosaveState(form);
      if (field) state.lastField = field;
      window.clearTimeout(state.timer);
      state.timer = window.setTimeout(() => autosaveForm(form, state.lastField), delay);
    }

    function syncCommandDependencies(form) {
      const commandValue = form.querySelector('[data-command-value]');
      const commandToggle = form.querySelector('[data-command-toggle]');
      if (!commandValue || !commandToggle) return;
      const commandOn = commandValue.value === '1';
      commandToggle.classList.toggle('is-on', commandOn);
      commandToggle.setAttribute('aria-checked', commandOn ? 'true' : 'false');
      const pausePromptValue = form.querySelector('[data-pause-prompt-value]');
      const pausePromptToggle = form.querySelector('[data-pause-prompt-toggle]');
      if (pausePromptValue && pausePromptToggle) {
        const pauseOn = pausePromptValue.value === '1';
        pausePromptToggle.classList.toggle('is-on', pauseOn);
        pausePromptToggle.setAttribute('aria-checked', pauseOn ? 'true' : 'false');
      }
      const autoPauseValue = form.querySelector('[data-auto-pause-value]');
      const autoPauseToggle = form.querySelector('[data-auto-pause-toggle]');
      if (autoPauseValue && autoPauseToggle) {
        const autoPauseOn = autoPauseValue.value === '1';
        autoPauseToggle.classList.toggle('is-on', autoPauseOn);
        autoPauseToggle.setAttribute('aria-checked', autoPauseOn ? 'true' : 'false');
      }
      const healthValue = form.querySelector('[data-health-value]');
      const healthToggle = form.querySelector('[data-health-toggle]');
      if (healthValue && healthToggle) {
        const healthOn = healthValue.value === '1';
        healthToggle.classList.toggle('is-on', healthOn);
        healthToggle.setAttribute('aria-checked', healthOn ? 'true' : 'false');
      }
      form.querySelectorAll('[data-health-dependent]').forEach((group) => {
        group.classList.toggle('is-inactive', !commandOn);
        group.querySelectorAll('[data-health-state]').forEach((state) => {
          state.textContent = commandOn ? 'After command' : 'Command off';
        });
      });
    }

    function updateCommandStatusPreview(form) {
      const card = form.closest('[data-app-card]');
      const status = card?.querySelector('[data-card-status]');
      const commandOn = form.querySelector('[data-command-value]')?.value === '1';
      if (!status) return;
      status.className = 'status ' + (commandOn ? 'applying-config' : 'command-off');
      status.textContent = commandOn ? 'applying config' : 'command off';
    }

    function bindCommandDependencies() {
      document.querySelectorAll('form:not([data-command-deps-bound])').forEach((form) => {
        const commandToggle = form.querySelector('[data-command-toggle]');
        const commandValue = form.querySelector('[data-command-value]');
        const pausePromptToggle = form.querySelector('[data-pause-prompt-toggle]');
        const pausePromptValue = form.querySelector('[data-pause-prompt-value]');
        const autoPauseToggle = form.querySelector('[data-auto-pause-toggle]');
        const autoPauseValue = form.querySelector('[data-auto-pause-value]');
        const healthToggle = form.querySelector('[data-health-toggle]');
        const healthValue = form.querySelector('[data-health-value]');
        if (!commandToggle || !commandValue || !form.querySelector('[data-health-dependent]')) return;
        form.dataset.commandDepsBound = '1';
        syncCommandDependencies(form);
        commandToggle.addEventListener('click', () => {
          commandValue.value = commandValue.value === '1' ? '0' : '1';
          syncCommandDependencies(form);
          updateCommandStatusPreview(form);
          commandValue.dispatchEvent(new Event('change', { bubbles: true }));
        });
        if (pausePromptToggle && pausePromptValue) {
          pausePromptToggle.addEventListener('click', () => {
            pausePromptValue.value = pausePromptValue.value === '1' ? '0' : '1';
            syncCommandDependencies(form);
            pausePromptValue.dispatchEvent(new Event('change', { bubbles: true }));
          });
        }
        if (autoPauseToggle && autoPauseValue) {
          autoPauseToggle.addEventListener('click', () => {
            autoPauseValue.value = autoPauseValue.value === '1' ? '0' : '1';
            syncCommandDependencies(form);
            autoPauseValue.dispatchEvent(new Event('change', { bubbles: true }));
          });
        }
        if (healthToggle && healthValue) {
          healthToggle.addEventListener('click', () => {
            healthValue.value = healthValue.value === '1' ? '0' : '1';
            syncCommandDependencies(form);
            healthValue.dispatchEvent(new Event('change', { bubbles: true }));
          });
        }
      });
    }

    function bindAutosaveForms() {
      document.querySelectorAll('[data-autosave-form]:not([data-autosave-bound])').forEach((form) => {
        form.dataset.autosaveBound = '1';
        autosaveState(form).lastSaved = autosaveSnapshot(form);
        form.addEventListener('submit', (event) => {
          event.preventDefault();
          const field = document.activeElement?.closest?.('input, textarea, select');
          scheduleAutosave(form, field && field.form === form ? field : null, 0);
        });
        form.addEventListener('input', (event) => {
          const field = event.target.closest('input, textarea, select');
          if (!field || !field.name) return;
          if (isInstantAutosaveField(field)) return;
          scheduleAutosave(form, field, autosaveTextDelayMs);
        });
        form.addEventListener('change', (event) => {
          const field = event.target.closest('input, textarea, select');
          if (!field || !field.name) return;
          scheduleAutosave(form, field, isInstantAutosaveField(field) ? 0 : 250);
        });
        form.addEventListener('focusout', (event) => {
          const field = event.target.closest('input, textarea, select');
          if (!field || !field.name || isInstantAutosaveField(field)) return;
          scheduleAutosave(form, field, 0);
        });
      });
    }
    bindCommandDependencies();
    bindAutosaveForms();

    function setCopiedIcon(button) {
      button.innerHTML = checkIcon;
      window.setTimeout(() => {
        button.innerHTML = copyIcon;
      }, 2000);
    }

    async function copyText(text) {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
        return;
      }
      const input = document.createElement('textarea');
      input.value = text;
      input.setAttribute('readonly', '');
      input.style.position = 'fixed';
      input.style.opacity = '0';
      document.body.appendChild(input);
      input.select();
      document.execCommand('copy');
      input.remove();
    }

    document.addEventListener('click', async (event) => {
      const button = event.target.closest('[data-copy-code]');
      if (!button) return;
      const block = document.querySelector(button.dataset.copyCode);
      if (!block) return;
      try {
        await copyText(block.textContent);
        setCopiedIcon(button);
      } catch (error) {
        console.error('copy failed', error);
      }
    });

  </script>
</body>
</html>`
