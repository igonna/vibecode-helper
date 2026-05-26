# Release Process

Vibeengine releases are signed before they are served by the hosted app.

## Signing Key

- Keep the private key outside the repository:
  `~/.config/vibeengine/release-signing-private.pem`
- Commit only the public key:
  `hosted/release-public/release-signing-public.pem`
- The repository `.gitignore` also ignores common accidental private-key paths.

## Build And Sign

From the repository root:

```sh
cd engine
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../hosted/releases/vibeengine-linux-amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ../hosted/releases/vibeengine-linux-arm64 .
chmod 700 ../hosted/releases/vibeengine-linux-*

cd ../hosted
npm run release:sign
```

The signing step creates:

- `vibeengine-<os>-<arch>`
- `vibeengine-<os>-<arch>.sha256`
- `vibeengine-<os>-<arch>.sig`

## Verification

Install scripts download the binary, checksum, signature, and public key. The
binary is installed only after checksum and RSA-SHA256 signature verification.

The engine update command uses the embedded public key:

```sh
vibeengine update --check
vibeengine update --apply
```

Updates write a new binary beside the current executable, move the current
binary to `<executable>.rollback`, and then swap the new binary into place. If
the final swap fails, the rollback file is restored.

## Legacy Hook Cleanup

After installing Vibeengine hooks, old generated prototype hooks can be removed:

```sh
vibeengine hooks cleanup-legacy --repo /path/to/repo
```

Only commands that clearly reference the old `vibecode-dev-notify` helper are
removed. Manual hooks are preserved.
