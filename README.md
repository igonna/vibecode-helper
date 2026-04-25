# Web Alive Notify

Monitors a web app and plays a browser sound when it recovers from `DOWN` to `UP`.
Other services can also request `success` or `failure` sounds.

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
- `SOUND_REPEAT_COUNT`: how many times the browser plays each requested sound. Default: `3`.
- `SUCCESS_SOUND_URL`: success sound URL. Default: Google wood plank flicks OGG.
- `FAILURE_SOUND_URL`: failure sound URL. Default: Google cartoon boing OGG.
- `APP_NETWORK`: external Docker network that contains the app service name. Default: `repo_default`.
- `BROWSER_PORT`: host port for the browser sound page. Default: `8765`.

Default `TARGET_URLS`:

```text
http://host.docker.internal:3000,http://platform-proxy:3000
```

Use `host.docker.internal` when the app is reachable from macOS as `localhost:3000`.
Use a Docker service name like `platform-proxy` when the monitor shares that service's Docker network.

By default, `web-monitor` joins both its own Compose network and the external
`repo_default` network, so `platform-proxy` can resolve even when this project is
started from another directory or terminal.

If port `8765` is already used, choose another browser port:

```bash
BROWSER_PORT=8766 docker compose up --build
```

Then open `http://localhost:8766`.

## Trigger Sounds

The monitor sends `success` automatically on `DOWN -> UP`.

Other services can request sounds with:

```bash
curl -X POST http://sound-notifier:8765/notify \
  -H 'Content-Type: application/json' \
  -d '{"event":"success"}'
```

```bash
curl -X POST http://sound-notifier:8765/notify \
  -H 'Content-Type: application/json' \
  -d '{"event":"failure"}'
```

From the host/browser port, use `http://localhost:8765/notify` instead.

`sound-notifier` is attached to both this Compose network and `APP_NETWORK`, so
containers attached to `APP_NETWORK` can use `http://sound-notifier:8765/notify`.
If a container is not attached to that network, Docker DNS will not resolve
`sound-notifier`.

To attach an existing caller container:

```bash
docker network connect repo_default <container-name>
```

To test from a temporary container on the right network:

```bash
docker run --rm --network repo_default curlimages/curl \
  -X POST http://sound-notifier:8765/notify \
  -H 'Content-Type: application/json' \
  -d '{"event":"failure"}'
```
