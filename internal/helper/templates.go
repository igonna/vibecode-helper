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
      overflow: hidden;
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
    .status.healthy, .status.running, .status.ready, .status.runner-ready, .status.runner-connected { color: var(--accent); }
    .status.runner-disconnected { color: var(--muted); }
    .status.failed { color: var(--danger); }
    .status.restarting, .status.restart-requested, .status.checking-health, .status.permission-asked, .status.prompt-active, .status.runner-update-required, .status.stop-requested, .status.stopping { color: var(--warn); }
    .status.paused, .status.stopped, .status.not-started { color: var(--muted); }
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
      display: grid;
      gap: 4px;
      color: var(--muted);
    }
    .form-grid input, .form-grid select {
      width: 100%;
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 9px 10px;
      font: inherit;
      color: var(--text);
      background: #141311;
    }
    .form-grid .wide { grid-column: 1 / -1; }
    .section-config .form-grid {
      grid-template-columns: 1fr;
    }
    .check-row {
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
    .command-fields,
    .health-url-fields {
      display: none;
      gap: 10px;
    }
    [data-command-run]:has(input[type="checkbox"]:checked) .command-fields {
      display: grid;
    }
    [data-health-check]:has(input[type="checkbox"]:checked) .health-url-fields {
      display: grid;
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
    .stop-button {
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
              <div class="config-group-title">App</div>
              <div class="form-grid">
                <label>Name <input name="name" required placeholder="my-platform"></label>
                <div data-command-run class="wide">
                  <label class="check-row"><input type="checkbox" name="command_enabled" value="1" checked> <span>Run command after prompt finished</span></label>
                  <div class="command-fields">
                    <label class="wide">Command <input name="command" placeholder="docker compose up --build"></label>
                  </div>
                </div>
              </div>
            </div>
            <div class="config-group">
              <div class="config-group-title">Health check</div>
              <div class="form-grid">
                <div data-health-check>
                  <label class="check-row"><input type="checkbox" name="health_enabled" value="1"> <span>Check browser URL before success</span></label>
                  <div class="health-url-fields">
                    <label>URL <input name="health_url" value="http://localhost:3000"></label>
                    <label>Timeout <input name="startup_timeout_seconds" type="number" min="1" value="60"></label>
                  </div>
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
          <div class="global-subtitle">Since midnight{{if .Today.Count}} · first prompt {{shortTime .Today.FirstStart}}{{end}}</div>
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
    <details class="app-card" data-app-card data-app-id="{{.ID}}" data-search-text="{{.Name}} {{.Status}} {{.HealthURL}}">
      <summary>
        <div class="card-identity">
          <div class="name">{{.Name}}</div>
          <div class="card-actions" data-no-card-toggle>
            {{if not .CommandEnabled}}
            <form class="restart-form" method="post" action="/apps/{{.ID}}/restart" hx-post="/apps/{{.ID}}/restart" hx-select="main" hx-target="main" hx-swap="outerHTML">
              <button class="icon-button restart-button" type="submit" aria-label="Restart app" title="Restart app">{{rotateCcwIcon}}<span>Restart</span></button>
            </form>
            {{else}}
            {{if eq .Status "paused"}}
            <form class="restart-form" method="post" action="/apps/{{.ID}}/restart" hx-post="/apps/{{.ID}}/restart" hx-select="main" hx-target="main" hx-swap="outerHTML">
              <button class="icon-button restart-button" type="submit" aria-label="Resume app command" title="Resume app command">{{rotateCcwIcon}}<span>Resume</span></button>
            </form>
            {{else}}
            <form class="restart-form" method="post" action="/apps/{{.ID}}/stop" hx-post="/apps/{{.ID}}/stop" hx-select="main" hx-target="main" hx-swap="outerHTML">
              <button class="icon-button stop-button" type="submit" aria-label="Stop app command" title="Stop app command">Stop</button>
            </form>
            <form class="restart-form" method="post" action="/apps/{{.ID}}/restart" hx-post="/apps/{{.ID}}/restart" hx-select="main" hx-target="main" hx-swap="outerHTML">
              <button class="icon-button restart-button" type="submit" aria-label="Restart app" title="Restart app">{{rotateCcwIcon}}<span>Restart</span></button>
            </form>
            {{end}}
            {{end}}
          </div>
        </div>
        <div class="card-statuses">
          {{if eq .Status "runner update required"}}
          <div class="status runner-update-required">runner update required</div>
          {{else if .RunnerConnected}}
          <div class="status runner-connected">runner connected</div>
          {{else if or .CommandEnabled (ne .HealthMode "none")}}
          <div class="status runner-disconnected">runner disconnected</div>
          {{end}}
          <div class="status {{statusClass .Status}}">{{.Status}}</div>
        </div>
        <div class="card-metrics metric-strip">
          <div class="metric metric-primary"><span class="label">AI working</span><strong>{{duration .TotalWaitSeconds}}</strong></div>
          <div class="metric {{depthClass .TotalWaitSeconds .PromptCount}}"><span class="label">Delegation depth</span><strong>{{duration (avg .TotalWaitSeconds .PromptCount)}}</strong><span class="subvalue">{{depthHint .TotalWaitSeconds .PromptCount}}</span></div>
          <div class="metric"><span class="label">Prompts</span><strong>{{.PromptCount}}</strong></div>
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
            <form method="post" action="/apps/{{.ID}}/update" hx-post="/apps/{{.ID}}/update" hx-select="main" hx-target="main" hx-swap="outerHTML">
              <div class="config-groups">
                <div class="config-group">
                  <div class="config-group-title">App</div>
                  <div class="form-grid">
                    <label>Name <input name="name" value="{{.Name}}" required></label>
                    <div data-command-run class="wide">
                      <label class="check-row"><input type="checkbox" name="command_enabled" value="1" {{if .CommandEnabled}}checked{{end}}> <span>Run command after prompt finished</span></label>
                      <div class="command-fields">
                        <label class="wide">Command <input name="command" value="{{command .Command " "}}" placeholder="docker compose up --build"></label>
                      </div>
                    </div>
                  </div>
                </div>

                <div class="config-group">
                  <div class="config-group-title">Health check</div>
                  <div class="form-grid">
                    <div data-health-check>
                      <label class="check-row"><input type="checkbox" name="health_enabled" value="1" {{if ne .HealthMode "none"}}checked{{end}}> <span>Check browser URL before success</span></label>
                      <div class="health-url-fields">
                        <label>URL <input name="health_url" value="{{.HealthURL}}"></label>
                        <label>Timeout <input name="startup_timeout_seconds" type="number" min="1" value="{{.StartupTimeoutSecs}}"></label>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
              <div class="actions"><button class="button" type="submit">Save config</button></div>
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
                  <div class="setup-detail">Run from your app folder. It installs one Codex hook set and removes duplicate generated hooks. Restart Codex once after first setup or hook changes.</div>
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
      const create = form.closest('.create');
      if (create) create.open = false;
    });
    document.body.addEventListener('htmx:afterSwap', () => {
      restoreDashboardState(true);
      bindNoCardToggleControls();
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

    function captureFormState() {
      const state = [];
      document.querySelectorAll('form').forEach((form, formIndex) => {
        const action = form.getAttribute('action') || '';
        form.querySelectorAll('input, textarea, select').forEach((field, fieldIndex) => {
          if (!field.name) return;
          state.push({
            key: action + '::' + field.name + '::' + fieldIndex + '::' + formIndex,
            name: field.name,
            action,
            formIndex,
            fieldIndex,
            type: field.type,
            checked: field.checked,
            value: field.value
          });
        });
      });
      return state;
    }

    function restoreFormState(state) {
      if (!state || !state.length) return;
      state.forEach((item) => {
        const forms = Array.from(document.querySelectorAll('form')).filter((form) => (form.getAttribute('action') || '') === item.action);
        const form = forms[item.formIndex] || forms[0];
        if (!form) return;
        const fields = Array.from(form.querySelectorAll('input, textarea, select')).filter((field) => field.name === item.name);
        const field = fields[item.fieldIndex] || fields[0];
        if (!field) return;
        if (field.type === 'checkbox' || field.type === 'radio') {
          field.checked = item.checked;
        } else {
          field.value = item.value;
        }
      });
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
    async function refreshDashboard() {
      if (dashboardRefreshInFlight || document.hidden || userIsEditing()) return;
      dashboardRefreshInFlight = true;
      rememberOpenCards();
      const scrollState = captureScrollState();
      const formState = captureFormState();
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
        restoreFormState(formState);
        restoreScrollState(scrollState);
      } catch (error) {
        // Keep the dashboard stable if a refresh races with a container restart.
      } finally {
        dashboardRefreshInFlight = false;
      }
    }
    window.setInterval(refreshDashboard, 2500);

    const copyIcon = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"></rect><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"></path></svg>';
    const checkIcon = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 6 9 17l-5-5"></path></svg>';

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
