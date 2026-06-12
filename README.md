# Ye Old Wizard API (yowapi)

This repo contains the api used by Ye Old Wizard.

The local backend stack started here is:

- `yowapi` - API/game server
- `nats` - JetStream message broker used for move requests
- `mongodb` - local database

The King worker is deployed separately from the `yowking` repo.

## Requirements

- Docker with Docker Compose
- Go
- Task
- `curl`
- `openssl`

Install Task if needed:

```bash
npm install --global @go-task/cli
```

## Environment

Create the deploy env file:

```bash
cp deploy/env.example deploy/.env
```

Edit `deploy/.env` and set real values:

```bash
NATS_TOKEN=replace_with_long_random_token
NATS_URL=nats://nats:4222
JWT_KEY=replace_with_long_random_jwt_signing_key
ADMIN_USER=your_lichess_username
LICHESS_TOKEN=replace_with_lichess_api_token
```

`ADMIN_USER` and `LICHESS_TOKEN` are optional for some local workflows, but `NATS_TOKEN`, `NATS_URL`, and `JWT_KEY` should be set.

Compose reads `deploy/.env` automatically when run from the `deploy/` directory. The compose file passes only the specific variables each container needs.

## Build The API Image

From the repo root:

```bash
task build:dist
task build:image
```

`task build:dist` creates `dist/`, builds the API binary, downloads `personalities.json`, and creates local cert files under `dist/certs`.

`task build:image` builds:

```text
zen:5000/yowapi:latest
```

## Start The Local Services

From the repo root:

```bash
./deploy/init.sh
task up
```

`deploy/init.sh` is idempotent. It creates the Docker resources required by the compose file:

- Docker networks: `yow`, `mongonet`, `proxynet`
- Docker volume: `mongodb`

This runs:

```bash
cd deploy
docker compose -f compose.yaml up -d
```

The compose file treats those networks and the MongoDB volume as external resources. Do not remove the `mongodb` volume unless you intentionally want to delete local database data.

## Stop The Local Services

```bash
task down
```

This stops and removes the API, NATS, and MongoDB containers for this compose project. It does not delete the `mongodb` volume unless you run Compose with `-v`.

## Useful Dev Commands

List tasks:

```bash
task
```

Run tests:

```bash
task test
```

Run the API directly from `dist/`:

```bash
task build:dist
task run
```

Clean local build artifacts and image/container:

```bash
task clean:all
```

## Deploy The Worker

After the services in this repo are running, deploy the King worker from:

https://github.com/thinktt/yowking

The worker must be started after this stack because it connects to the `yow` Docker network and uses NATS for move requests.
