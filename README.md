# Web Alive Notify

Monitors a web app and plays a browser sound when it recovers from `DOWN` to `UP`.

## Run

```bash
docker compose up --build
```

Then open once:

```text
http://localhost:8765
```

If the page shows `Enable sound permission`, click it once and keep the tab
open. The page reconnects to Docker automatically if the notifier restarts.

Browsers usually block audible autoplay after a full browser restart. The page
remembers that sound was enabled and tries to restore it automatically, but your
browser may still require one click after restart.

## Configuration

Set variables inline when starting Compose:

```bash
SOUND_REPEAT_COUNT=3 TARGET_URLS=http://host.docker.internal:3000 docker compose up --build
```

Available variables:

- `TARGET_URLS`: comma-separated URLs to check. The monitor treats the app as UP if any URL responds with HTTP status below 500.
- `CHECK_INTERVAL_SECONDS`: check interval. Default: `3`.
- `REQUEST_TIMEOUT_SECONDS`: HTTP timeout. Default: `2`.
- `SOUND_REPEAT_COUNT`: how many times the browser plays the sound after recovery. Default: `3`.
- `SOUND_URL`: notification sound URL. Default: Google wood plank flicks OGG.
- `APP_NETWORK`: external Docker network that contains the app service name. Default: `repo_default`.

Default `TARGET_URLS`:

```text
http://host.docker.internal:3000,http://platform-proxy:3000
```

Use `host.docker.internal` when the app is reachable from macOS as `localhost:3000`.
Use a Docker service name like `platform-proxy` when the monitor shares that service's Docker network.

By default, `web-monitor` joins both its own Compose network and the external
`repo_default` network, so `platform-proxy` can resolve even when this project is
started from another directory or terminal.
