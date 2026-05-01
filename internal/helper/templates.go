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
      padding: 32px 20px 16px;
      text-align: center;
    }
    h1 {
      margin: 0 0 18px;
      font-size: 28px;
      font-weight: 680;
      letter-spacing: 0;
    }
    .topbar {
      display: grid;
      grid-template-columns: 1fr auto;
      gap: 12px;
      align-items: center;
      max-width: 620px;
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
      max-width: 760px;
      margin: 12px auto 0;
      padding: 16px;
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
      text-align: left;
    }
    .search {
      width: 100%;
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 11px 13px;
      font: inherit;
      background: var(--panel-soft);
      color: var(--text);
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
      max-width: 620px;
      margin: 14px auto 0;
      display: flex;
      justify-content: center;
      align-items: center;
      gap: 10px;
      color: var(--muted);
    }
    .sound-status.enabled { color: var(--accent); }
    .sound-status.needs-action { color: var(--warn); }
    main {
      max-width: 1040px;
      margin: 0 auto;
      padding: 12px 20px 48px;
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
      grid-template-columns: minmax(180px, 1.2fr) auto minmax(160px, .9fr) repeat(4, minmax(92px, auto));
      gap: 14px;
      align-items: center;
      padding: 16px;
      cursor: pointer;
      list-style: none;
    }
    .app-card summary::-webkit-details-marker { display: none; }
    .name { font-weight: 660; font-size: 16px; }
    .health, .label, .event-detail { color: var(--muted); }
    .metric strong {
      display: block;
      font-size: 15px;
      font-weight: 650;
      color: var(--text);
    }
    .status {
      justify-self: start;
      border: 1px solid var(--line);
      border-radius: 999px;
      padding: 4px 9px;
      color: var(--muted);
      background: #211f1b;
    }
    .status.healthy, .status.running { color: var(--accent); border-color: #2f6d4c; background: #17251d; }
    .status.failed { color: var(--danger); border-color: #74352f; background: #2b1715; }
    .status.restarting, .status.restart-requested, .status.checking-health, .status.permission-asked { color: var(--warn); border-color: #72562b; background: #2b2112; }
    .details {
      border-top: 1px solid var(--line);
      padding: 16px;
      display: grid;
      grid-template-columns: minmax(280px, .9fr) minmax(360px, 1.1fr);
      gap: 16px;
      align-items: start;
    }
    .details-column {
      display: grid;
      gap: 16px;
      align-content: start;
    }
    .section h2 {
      margin: 0 0 10px;
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
      margin-top: 8px;
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
    .event-list, .prompt-list {
      display: grid;
      gap: 8px;
    }
    .event, .prompt {
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 8px 10px;
      background: #141311;
    }
    .setup-list {
      display: grid;
      gap: 10px;
    }
    .setup-item {
      display: grid;
      grid-template-columns: 1fr auto;
      gap: 12px;
      align-items: center;
      padding: 10px 10px 10px 12px;
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
      justify-self: start;
    }
    .restart-button {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      width: auto;
      padding: 0 10px;
      color: var(--text);
    }
    .danger-section {
      grid-column: 1 / -1;
      border-top: 1px solid var(--line);
      padding-top: 16px;
    }
    .danger-panel {
      display: grid;
      grid-template-columns: 1fr auto;
      gap: 12px;
      align-items: center;
      border: 1px solid #74352f;
      border-radius: 8px;
      background: #211211;
      padding: 12px;
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
      .app-card summary {
        grid-template-columns: 1fr auto;
      }
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
      <input class="search" type="search" placeholder="Search apps..." aria-label="Search apps" data-search>
      <details class="create">
        <summary class="button">+ Add app</summary>
        <form method="post" action="/apps" hx-post="/apps" hx-target="body" hx-swap="outerHTML">
          <div class="form-grid">
            <label>Name <input name="name" required placeholder="my-platform"></label>
            <label>Working dir <input name="cwd" value="."></label>
            <label class="wide">Command <input name="command" required placeholder="docker compose up --build"></label>
            <label>Health mode
              <select name="health_mode">
                <option value="browser_url">browser URL</option>
                <option value="url">runner URL</option>
                <option value="command">runner command</option>
              </select>
            </label>
            <label>Health URL <input name="health_url" value="http://localhost:3000"></label>
            <label class="wide">Health command <input name="health_command" placeholder="curl -fsS http://localhost:3000"></label>
            <label>Startup timeout <input name="startup_timeout_seconds" type="number" min="1" value="60"></label>
            <label>Codex runs
              <select name="codex_environment">
                <option value="host">on my machine</option>
                <option value="dev_container">inside a dev container</option>
              </select>
            </label>
            <label>Runner runs
              <select name="runner_environment">
                <option value="host">on my machine</option>
                <option value="dev_container">inside a dev container</option>
              </select>
            </label>
          </div>
          <div class="actions"><button class="button" type="submit">Create app</button></div>
        </form>
      </details>
    </div>
  </header>

  <main>
    {{range .Apps}}
    <details class="app-card" data-app-card data-app-id="{{.ID}}" data-search-text="{{.Name}} {{.Status}} {{.HealthURL}}">
      <summary>
        <div>
          <div class="name">{{.Name}}</div>
          <div class="health">{{.HealthURL}}</div>
          {{if .LastNotification}}<div class="event-detail">{{.LastNotification}}</div>{{end}}
        </div>
        <form class="restart-form" method="post" action="/apps/{{.ID}}/restart" hx-post="/apps/{{.ID}}/restart" hx-target="body" hx-swap="outerHTML" data-no-card-toggle>
          <button class="icon-button restart-button" type="submit" aria-label="Restart app" title="Restart app">{{rotateCcwIcon}}<span>Restart</span></button>
        </form>
        <div class="status {{statusClass .Status}}">{{.Status}}</div>
        <div class="metric"><span class="label">Prompts</span><strong>{{.PromptCount}}</strong></div>
        <div class="metric"><span class="label">Total wait</span><strong>{{duration .TotalWaitSeconds}}</strong></div>
        <div class="metric"><span class="label">Avg wait</span><strong>{{duration (avg .TotalWaitSeconds .PromptCount)}}</strong></div>
        <div class="metric"><span class="label">Last wait</span><strong>{{duration .LastWaitSeconds}}</strong></div>
      </summary>

      <div class="details">
        <div class="details-column">
          <section class="section section-config">
            <h2>Config</h2>
            <form method="post" action="/apps/{{.ID}}/update" hx-post="/apps/{{.ID}}/update" hx-target="body" hx-swap="outerHTML">
              <div class="form-grid">
                <label>Name <input name="name" value="{{.Name}}" required></label>
                <label>Working dir <input name="cwd" value="{{.CWD}}"></label>
                <label class="wide">Command <input name="command" value="{{command .Command " "}}" required></label>
                <label>Health mode
                  <select name="health_mode">
                    <option value="browser_url" {{if eq .HealthMode "browser_url"}}selected{{end}}>browser URL</option>
                    <option value="url" {{if eq .HealthMode "url"}}selected{{end}}>runner URL</option>
                    <option value="command" {{if eq .HealthMode "command"}}selected{{end}}>runner command</option>
                  </select>
                </label>
                <label>Health URL <input name="health_url" value="{{.HealthURL}}"></label>
                <label class="wide">Health command <input name="health_command" value="{{command .HealthCommand " "}}"></label>
                <label>Startup timeout <input name="startup_timeout_seconds" type="number" min="1" value="{{.StartupTimeoutSecs}}"></label>
                <label>Codex runs
                  <select name="codex_environment">
                    <option value="host" {{if eq .CodexEnvironment "host"}}selected{{end}}>on my machine</option>
                    <option value="dev_container" {{if eq .CodexEnvironment "dev_container"}}selected{{end}}>inside a dev container</option>
                  </select>
                </label>
                <label>Runner runs
                  <select name="runner_environment">
                    <option value="host" {{if eq .RunnerEnvironment "host"}}selected{{end}}>on my machine</option>
                    <option value="dev_container" {{if eq .RunnerEnvironment "dev_container"}}selected{{end}}>inside a dev container</option>
                  </select>
                </label>
              </div>
              <div class="actions"><button class="button" type="submit">Save config</button></div>
            </form>
          </section>

          <section class="section">
            <h2>Stats</h2>
            <div class="prompt-list">
              {{range .RecentPrompts}}
              <div class="prompt">#{{.Index}} wait {{duration .WaitSecs}} · prompt {{duration .PromptSecs}} · permissions {{.Permissions}} · {{.Result}} · {{shortTime .FinishedAt}}</div>
              {{else}}
              <div class="event-detail">No prompts recorded yet.</div>
              {{end}}
            </div>
          </section>

          <section class="section">
            <h2>Events</h2>
            <div class="event-list">
              {{range .RecentEvents}}
              <div class="event"><strong>{{shortTime .At}}</strong> {{.Type}} <span class="event-detail">{{.Detail}}</span></div>
              {{else}}
              <div class="event-detail">No events recorded yet.</div>
              {{end}}
            </div>
          </section>
        </div>

        <div class="details-column">
          <section class="section">
            <h2>Setup</h2>
            <div class="setup-list">
              <div class="setup-item">
                <div>
                  <div class="setup-title">1. Runner</div>
                  <div class="setup-detail">Copy and run in the terminal where your app should run.</div>
                </div>
                <button class="icon-button" type="button" aria-label="Copy runner command" title="Copy runner command" data-copy-code="[data-runner-code='{{.ID}}']">{{copyIcon}}</button>
                <pre class="copy-source" data-runner-code="{{.ID}}">{{runnerCommand $.BaseURL .ID}}</pre>
              </div>
              <div class="setup-item">
                <div>
                  <div class="setup-title">2. Codex hooks</div>
                  <div class="setup-detail">Copy into Codex config for prompt timing and restart signals.</div>
                </div>
                <button class="icon-button" type="button" aria-label="Copy hooks" title="Copy hooks" data-copy-code="[data-hooks-code='{{.ID}}']">{{copyIcon}}</button>
                <pre class="copy-source" data-hooks-code="{{.ID}}">{{hooksConfig $.BaseURL .ID}}</pre>
              </div>
            </div>
          </section>
        </div>

        <section class="section danger-section">
          <h2>Danger</h2>
          <div class="danger-panel">
            <div>
              <div class="danger-title">Delete app</div>
              <div class="danger-detail">Remove this app and its local helper history.</div>
            </div>
            <form method="post" action="/apps/{{.ID}}/delete" hx-post="/apps/{{.ID}}/delete" hx-target="body" hx-swap="outerHTML">
              <button class="button danger-button" type="submit">Delete</button>
            </form>
          </div>
        </section>
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
    soundStatus.innerHTML = '<button class="button" type="button" data-enable-sound>Enable sound required</button><span data-sound-label>Browser sound waiting for permission.</span>';
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
        setSoundStatus('enabled', 'Sound enabled.');
      } catch (error) {
        soundEnabled = false;
        setSoundStatus('blocked', 'Enable sound required.');
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
          setSoundStatus('enabled', 'Sound enabled.');
        } catch (error) {
          soundEnabled = false;
          setSoundStatus('blocked', 'Enable sound required.');
          break;
        }
      }
    }

    enableSoundButton.addEventListener('click', unlockSound);
    unlockSound();

    const events = new EventSource('/events');
    events.addEventListener('status', () => {
      if (!soundEnabled) setSoundStatus('blocked', 'Enable sound required.');
    });
    events.addEventListener('sound', (message) => {
      const event = JSON.parse(message.data);
      soundLabel.textContent = event.app_name + ': ' + event.type;
      playSound(event.type, event.repeat);
    });
    events.addEventListener('health_check', (message) => {
      const request = JSON.parse(message.data);
      runBrowserHealthCheck(request);
    });
    events.onerror = () => {
      if (!soundEnabled) setSoundStatus('blocked', 'Sound stream reconnecting. Enable sound required.');
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
          result.detail = String(secondError && secondError.message ? secondError.message : secondError);
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
    search?.addEventListener('input', () => {
      const query = search.value.trim().toLowerCase();
      document.querySelectorAll('[data-app-card]').forEach((card) => {
        const text = card.dataset.searchText.toLowerCase();
        card.hidden = query && !text.includes(query);
      });
      sessionStorage.setItem('vibecode-helper-search', search.value);
    });

    function rememberOpenCards() {
      const openIDs = Array.from(document.querySelectorAll('[data-app-card][open]')).map((card) => card.dataset.appId);
      sessionStorage.setItem('vibecode-helper-open-apps', JSON.stringify(openIDs));
      if (search) sessionStorage.setItem('vibecode-helper-search', search.value);
    }

    function restoreOpenCards() {
      const rawOpen = sessionStorage.getItem('vibecode-helper-open-apps') || '[]';
      let openIDs = [];
      try {
        openIDs = JSON.parse(rawOpen);
      } catch (error) {
        openIDs = [];
      }
      const openSet = new Set(openIDs);
      document.querySelectorAll('[data-app-card]').forEach((card) => {
        card.open = openSet.has(card.dataset.appId);
      });

      const savedSearch = sessionStorage.getItem('vibecode-helper-search');
      if (search && savedSearch !== null) {
        search.value = savedSearch;
        search.dispatchEvent(new Event('input'));
      }
    }

    document.body.addEventListener('htmx:beforeRequest', rememberOpenCards);
    document.body.addEventListener('htmx:afterSwap', restoreOpenCards);
    restoreOpenCards();

    document.querySelectorAll('[data-no-card-toggle]').forEach((control) => {
      control.addEventListener('click', (event) => event.stopPropagation());
      control.addEventListener('keydown', (event) => event.stopPropagation());
    });

    const copyIcon = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"></rect><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"></path></svg>';
    const checkIcon = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M20 6 9 17l-5-5"></path></svg>';

    function setCopiedIcon(button) {
      button.innerHTML = checkIcon;
      window.setTimeout(() => {
        button.innerHTML = copyIcon;
      }, 2000);
    }

    document.querySelectorAll('[data-copy-code]').forEach((button) => {
      button.addEventListener('click', async () => {
        const block = document.querySelector(button.dataset.copyCode);
        if (!block) return;
        await navigator.clipboard.writeText(block.textContent);
        setCopiedIcon(button);
      });
    });

  </script>
</body>
</html>`
