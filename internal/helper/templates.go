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
      color-scheme: light;
      --bg: #f6f7f9;
      --panel: #ffffff;
      --text: #17191c;
      --muted: #69707a;
      --line: #dfe3e8;
      --accent: #1f7a5a;
      --danger: #b42318;
      --warn: #a15c00;
      --shadow: 0 8px 24px rgba(21, 27, 38, .08);
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
      background: #fff;
    }
    .button {
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 10px 12px;
      background: #fff;
      color: var(--text);
      font: inherit;
      cursor: pointer;
      white-space: nowrap;
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
      grid-template-columns: minmax(180px, 1.2fr) minmax(160px, .9fr) repeat(4, minmax(92px, auto));
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
      background: #fafafa;
    }
    .status.healthy, .status.running { color: var(--accent); border-color: #b7dfcf; background: #f0faf6; }
    .status.failed { color: var(--danger); border-color: #f0b8b1; background: #fff3f1; }
    .status.restarting, .status.restart-requested, .status.checking-health, .status.permission-asked { color: var(--warn); border-color: #f2cf9d; background: #fff8ed; }
    .details {
      border-top: 1px solid var(--line);
      padding: 18px 16px 20px;
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 18px;
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
      margin: 8px 0 0;
      padding: 12px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #fbfbfc;
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
      background: #fff;
    }
    .form-grid .wide { grid-column: 1 / -1; }
    .event-list, .prompt-list {
      display: grid;
      gap: 8px;
    }
    .event, .prompt {
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 8px 10px;
      background: #fbfbfc;
    }
    .copy-state {
      color: var(--accent);
      min-height: 20px;
      margin-top: 8px;
    }
    @media (max-width: 840px) {
      .topbar { grid-template-columns: 1fr; }
      .app-card summary {
        grid-template-columns: 1fr 1fr;
      }
      .details { grid-template-columns: 1fr; }
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
            <label>Sound repeats <input name="sound_repeat_count" type="number" min="1" value="1"></label>
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
    {{range .}}
    <details class="app-card" data-app-card data-search-text="{{.Name}} {{.Status}} {{.HealthURL}}">
      <summary>
        <div>
          <div class="name">{{.Name}}</div>
          <div class="health">{{.HealthURL}}</div>
          {{if .LastNotification}}<div class="event-detail">{{.LastNotification}}</div>{{end}}
        </div>
        <div class="status {{statusClass .Status}}">{{.Status}}</div>
        <div class="metric"><span class="label">Prompts</span><strong>{{.PromptCount}}</strong></div>
        <div class="metric"><span class="label">Total wait</span><strong>{{duration .TotalWaitSeconds}}</strong></div>
        <div class="metric"><span class="label">Avg wait</span><strong>{{duration (avg .TotalWaitSeconds .PromptCount)}}</strong></div>
        <div class="metric"><span class="label">Last wait</span><strong>{{duration .LastWaitSeconds}}</strong></div>
      </summary>

      <div class="details">
        <section class="section">
          <h2>Overview</h2>
          <div class="row"><div class="label">Runner</div><div>{{if .RunnerConnected}}connected{{else}}disconnected{{end}}</div></div>
          <div class="row"><div class="label">Last result</div><div>{{if .LastRestartResult}}{{.LastRestartResult}}{{else}}-{{end}}</div></div>
          <div class="row"><div class="label">Last event</div><div>{{.LastEvent}} at {{shortTime .LastEventAt}}</div></div>
          <div class="row"><div class="label">Timeout</div><div>{{.StartupTimeoutSecs}}s</div></div>
          <div class="row"><div class="label">Min wait</div><div>{{duration .MinWaitSeconds}}</div></div>
          <div class="row"><div class="label">Max wait</div><div>{{duration .MaxWaitSeconds}}</div></div>
          <div class="row"><div class="label">Requested</div><div>{{shortTime .RestartRequestedAt}}</div></div>
          <div class="row"><div class="label">Healthy</div><div>{{shortTime .HealthyAt}}</div></div>
          {{if .FailureReason}}<div class="row"><div class="label">Failure</div><div>{{.FailureReason}}</div></div>{{end}}
        </section>

        <section class="section">
          <h2>Setup</h2>
          <div class="actions">
            <button class="button" type="button" data-copy-runner="{{.ID}}">Copy runner command</button>
            <button class="button" type="button" data-copy-hooks="{{.ID}}">Copy hooks</button>
            <form method="post" action="/apps/{{.ID}}/restart" hx-post="/apps/{{.ID}}/restart" hx-target="body" hx-swap="outerHTML"><button class="button" type="submit">Restart now</button></form>
            <button class="button" type="button" data-test-sound="{{.ID}}" data-sound-type="success">Test success</button>
            <button class="button" type="button" data-test-sound="{{.ID}}" data-sound-type="failure">Test failure</button>
            <button class="button" type="button" data-test-sound="{{.ID}}" data-sound-type="permission">Test permission</button>
            <form method="post" action="/apps/{{.ID}}/delete" hx-post="/apps/{{.ID}}/delete" hx-target="body" hx-swap="outerHTML"><button class="button" type="submit">Delete</button></form>
          </div>
          <pre>curl -fsSL /apps/{{.ID}}/runner.sh -o run.sh
chmod +x run.sh
./run.sh</pre>
          <div class="copy-state" data-copy-state></div>
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

        <section class="section">
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
              <label>Sound repeats <input name="sound_repeat_count" type="number" min="1" value="{{.SoundRepeatCount}}"></label>
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
    });

    document.querySelectorAll('[data-copy-runner]').forEach((button) => {
      button.addEventListener('click', async () => {
        const appID = button.dataset.copyRunner;
        const runnerURL = new URL('/apps/' + appID + '/runner.sh', window.location.href).toString();
        const text = 'mkdir -p ~/vibecode-runners/' + appID + '\n'
          + 'cd ~/vibecode-runners/' + appID + '\n'
          + 'curl -fsSL ' + runnerURL + ' -o run.sh\n'
          + 'chmod +x run.sh\n'
          + './run.sh';
        await navigator.clipboard.writeText(text);
        const state = button.closest('.section').querySelector('[data-copy-state]');
        if (state) state.textContent = 'Copied.';
      });
    });

    document.querySelectorAll('[data-copy-hooks]').forEach((button) => {
      button.addEventListener('click', async () => {
        const appID = button.dataset.copyHooks;
        const hooksURL = new URL('/apps/' + appID + '/hooks.json', window.location.href).toString();
        const response = await fetch(hooksURL);
        const text = JSON.stringify(await response.json(), null, 2);
        await navigator.clipboard.writeText(text);
        const state = button.closest('.section').querySelector('[data-copy-state]');
        if (state) state.textContent = 'Copied.';
      });
    });

    document.querySelectorAll('[data-test-sound]').forEach((button) => {
      button.addEventListener('click', async () => {
        await unlockSound();
        const form = new URLSearchParams();
        form.set('type', button.dataset.soundType);
        await fetch('/apps/' + button.dataset.testSound + '/test_sound', {
          method: 'POST',
          headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
          body: form
        });
      });
    });
  </script>
</body>
</html>`
