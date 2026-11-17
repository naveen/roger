# Relay

The relay is the brain a Roger radio talks to. It checks the parent's rules, turns a press into text, asks a model for a reply, checks the reply, and speaks it back. It also writes the parent's daily digest.

It is one Go binary with one dependency (a WebSocket library). Run it on anything: a laptop, a Raspberry Pi, a small cloud server.

> **Status.** The test suite covers the protocol, rules, storage, digest and both provider clients against fake servers, and an end-to-end press works in mock mode. The live path (real speech and model providers) has not been run yet: it needs API keys. The Docker image has not been built yet either.

## Quick start, no keys

```
cd relay
make build
./bin/roger-relay token                 # prints a token and its hash
cp devices.example.json devices.json    # paste the hash in as token_sha256
make run-mock
```

In another terminal:

```
ROGER_TOKEN=rgr_... ./bin/roger-sim -out reply.wav
```

`roger-sim` plays the part of a radio: it sends a test tone as a press and saves what comes back. In mock mode that is a chirp followed by an echo of the press.

If the simulator prints `notice code=quiet_hours` instead of a reply, the rules are working: the example radio is quiet from 19:30 to 07:00 in its timezone. Change `quiet_hours` or `timezone` in `devices.json` to test in the evening.

## Running it for real

```
cp .env.example .env        # add your keys
docker compose up -d
```

or, without Docker:

```
set -a; . ./.env; set +a
./bin/roger-relay
```

You bring two providers:

| Stage | Default | Change with |
|---|---|---|
| Reply | Claude, through the Anthropic Messages API | `ROGER_LLM_MODEL`, `ANTHROPIC_BASE_URL` |
| Speech to text, text to speech | Any server with OpenAI-style `/audio/transcriptions` and `/audio/speech` endpoints | `ROGER_AUDIO_BASE_URL`, `ROGER_STT_MODEL`, `ROGER_TTS_MODEL`, `ROGER_TTS_VOICE` |

The audio endpoints are a common shape that several hosted services and self-hosted servers implement, so the base URL is configurable. Text to speech must support `response_format: "pcm"`. One caution about the defaults: OpenAI's documentation lists its `/audio/speech` models, including the default here, for shutdown on 6 January 2027. Plan to point `ROGER_AUDIO_BASE_URL` somewhere else before then, or add a provider (see below).

Before you put a relay in front of a child, read [`docs/SAFETY.md`](../docs/SAFETY.md). Each provider has its own terms for products used by children, and you are the one agreeing to them.

### Reaching the relay from outside your home

The relay speaks plain HTTP. On your home network a radio can use `ws://<machine>:8080` directly. For anything else, put a TLS reverse proxy in front and use `wss://`. With Caddy that is:

```
roger.example.com {
    reverse_proxy localhost:8080
}
```

Radios verify the certificate, so use a real one.

## Radios

Each radio is an entry in `devices.json`:

```json
{
  "id": "roger-01",
  "token_sha256": "<from roger-relay token>",
  "kid": { "name": "Sam", "age": 7 },
  "timezone": "America/Los_Angeles",
  "parent_email": "parent@example.com",
  "rules": { "...": "see parent/rules.schema.json" }
}
```

The relay stores only the hash of a token. `kid.name` should be a first name or nickname; the relay needs nothing else about a child, and the file should hold nothing else. A radio with no `rules` block gets conservative defaults (quiet from 19:30 to 07:00, 30 minutes a day, volume capped at 70%).

## Commands

| Command | What it does |
|---|---|
| `roger-relay` | Run the relay. |
| `roger-relay token` | Mint a device token and its hash. |
| `roger-relay digest -device ID [-date YYYY-MM-DD] [-out FILE] [-send]` | Render a day's digest, or email it. |
| `roger-relay erase -device ID` | Delete everything stored for a radio. |

## What is stored

Text only, under `ROGER_DATA_DIR`, one folder per radio:

| File | Contents | Kept for |
|---|---|---|
| `YYYY-MM-DD.jsonl` | Each press: what was heard, what Roger said, topic, flag, durations | `transcript_retention_days` (default 30) |
| `remembered.jsonl` | Things the kid asked Roger to remember, newest 50 | `remembered_retention_days` (default 90) |
| `state.json` | The date the last digest was sent | |

Audio is held in memory for the length of one press and is never written to disk. The log never contains what a child said or what Roger answered: only ids, durations and flags.

Files are created readable by the relay's user only.

## Layout

| Path | |
|---|---|
| `cmd/roger-relay` | The server and its subcommands |
| `cmd/roger-sim` | A radio in software |
| `internal/server` | The WebSocket endpoint and the per-press flow |
| `internal/pipeline` | Speech to text, model, text to speech; one interface each |
| `internal/persona` | The system prompt (`prompt.md`) and the reply schema |
| `internal/guard` | The deterministic check on what gets spoken |
| `internal/rules` | Quiet hours, air time, digest timing |
| `internal/store` | The files above |
| `internal/digest` | The daily email |

## Adding a provider

Implement one of the three small interfaces in `internal/pipeline/pipeline.go` (`STT`, `LLM`, `TTS`) and wire it up in `cmd/roger-relay/main.go`. `TTS.Speak` returns a stream of 16-bit mono PCM and its sample rate; the server handles pacing and framing.

## Tests

```
make test
```
