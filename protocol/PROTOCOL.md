# roger.v0 wire protocol

This is the contract between a Roger radio and a relay. A radio that follows it works with any relay that follows it, whoever built either one.

**Status:** v0, draft. Expect breaking changes until v1. The reference implementations are `firmware/src/main.cpp` (radio) and `relay/internal/server` (relay). The relay's tests exercise every rule marked **must** below.

## Design in one paragraph

The radio is thin. It knows how to hold a WebSocket open, send microphone audio while a button is held, and play audio it is sent. It holds no prompt, no model key and no rules. Everything that decides what a child hears lives in the relay. Turns are half-duplex and delimited by the button, so the protocol needs no wake word, no voice-activity detection and no echo cancellation.

## Transport

| | |
|---|---|
| Transport | WebSocket (RFC 6455) |
| Path | `/v1/radio` |
| Subprotocol | `roger.v0`, sent in `Sec-WebSocket-Protocol`. The relay closes the connection if it is not negotiated. |
| Authentication | `Authorization: Bearer <device token>` on the upgrade request. A missing or wrong token gets HTTP 401 and no upgrade. |
| Encryption | `wss://` for any relay reached over the internet. Radios must verify the certificate. Plain `ws://` is for a relay on the same home network only. |
| Text frames | One JSON object each, with a `type` field. |
| Binary frames | Raw audio. No envelope. |

A relay accepts one connection per device token. A new connection replaces the old one.

Receivers must ignore unknown message types and unknown fields. That is how v0 radios and relays of different ages keep working together.

## Audio

v0 defines one codec, `pcm16`: signed 16-bit little-endian, mono, no header.

| Direction | Rate | Framing |
|---|---|---|
| Radio to relay | Declared by the radio in `hello` (the reference radios use 16000 Hz). 8000 to 48000 is accepted. | Any frame size up to 64 KiB. Frames must hold a whole number of samples. |
| Relay to radio | Declared per reply in `reply.start`. Radios must support 16000 and 24000 Hz. | The reference relay sends 40 ms frames. |

The relay sends reply audio slightly ahead of real time (the reference relay stays about 600 ms ahead). A radio should buffer about a second and start playing once it holds about 200 ms. A radio with less memory can stop reading from the socket while its buffer is full; TCP flow control then slows the relay down.

`opus` is reserved as a codec name for a later version. Raw PCM at 16 kHz is 256 kbit/s, which is fine on Wi-Fi and too much for a cellular radio; Opus is the plan for that.

## Messages

### Radio to relay

| `type` | Fields | Meaning |
|---|---|---|
| `hello` | `proto`, `fw`, `board`, `mic: {codec, rate}` | First message, within 5 seconds of connecting. |
| `ptt.down` | `seq` | The button went down. Audio follows. |
| *(binary)* | | Microphone audio for the current press. |
| `ptt.up` | `seq` | The button came up. No more audio for this press. |
| `cancel` | `seq` | Stop working on this press and stop sending its reply. |

`seq` is a number the radio picks for each press, increasing by one. The relay echoes it on everything about that press, so a radio can discard messages about a press it has moved on from.

### Relay to radio

| `type` | Fields | Meaning |
|---|---|---|
| `ready` | `session`, `limits: {max_press_ms, volume_cap}` | Reply to `hello`. The radio may now accept presses. |
| `thinking` | `seq` | The press was received and is being answered. |
| `reply.start` | `seq`, `codec`, `rate` | Reply audio follows. |
| *(binary)* | | Reply audio. |
| `reply.end` | `seq`, `cancelled` | The reply is complete. `cancelled` is true if it was cut short. |
| `notice` | `seq`, `code`, `speech` | The press will not be answered normally. See below. |

`limits.volume_cap` is 0 to 100 and is set by the parent. The radio must not play louder than this fraction of its maximum.

### Notices

| `code` | When |
|---|---|
| `quiet_hours` | The press arrived during the parent's quiet hours. |
| `airtime_exhausted` | Today's air time is used up. |
| `not_heard` | The press was too short, or nothing intelligible was in it. |
| `error` | Something failed in the relay or a provider. |

If `speech` is true, a spoken explanation follows as a normal `reply.start` … `reply.end` with the same `seq`. If `speech` is false, the radio plays its own built-in tone for that code. Relays rate-limit spoken notices, so a radio must handle both. A radio must treat an unknown code like `quiet_hours`: play the "not now" tone.

## A normal press

```
radio                                   relay
  | -- ptt.down {seq:7} ----------------> |
  | -- binary (mic audio) --------------> |
  | -- binary --------------------------> |
  | -- ptt.up {seq:7} ------------------> |
  | <------------- thinking {seq:7} ----- |   radio plays static
  | <-- reply.start {seq:7, rate:24000} - |   static stops
  | <------------------------- binary --- |
  | <------------------------- binary --- |
  | <----------- reply.end {seq:7} ------ |   radio plays its "over" tone
```

## Rules

**The microphone rule.** A radio must capture audio only between `ptt.down` and `ptt.up`, and must send `ptt.down` before the first audio frame of a press. A relay **must** close the connection (status 1008) if it receives audio when no press is open. This makes "listens only while pressed" something the relay checks, in addition to something the radio promises.

**Refused presses.** If the relay answers `ptt.down` with a notice, it drops any audio for that press without processing it. The radio should stop its microphone and send `ptt.up` as soon as it sees the notice.

**Press limits.** A relay **must** stop keeping audio after `limits.max_press_ms`. A press shorter than 300 ms gets `not_heard` without being processed.

**Interrupting.** A new `ptt.down` cancels whatever the relay was doing for the previous press. If a reply was in progress the relay sends `reply.end` with `cancelled: true` for the old `seq`, and it **must** send nothing else for the old `seq` afterwards.

**One thing at a time.** The relay never sends audio outside a `reply.start` … `reply.end` pair, and never interleaves two replies.

**Keep-alive.** Both sides should send WebSocket pings. The reference radio pings every 15 seconds and reconnects after two missed pongs.

## What the relay does with a press

This is not part of the wire contract, but a radio builder should know it:

1. Check the parent's rules (quiet hours, air time). Refuse with a notice if either says no.
2. Transcribe the audio to text. The audio is then discarded; it is never written to disk.
3. Ask the model for a reply, as structured fields: what to say, a topic for the parent's summary, and a flag.
4. Run the reply through a deterministic guard.
5. Turn the reply into speech and stream it back.
6. Store the text of the turn for the parent's daily digest, subject to the retention rule.

See `docs/SAFETY.md` and `docs/PRIVACY.md` for the detail.

## Versioning

The subprotocol name carries the version. A breaking change means a new name (`roger.v1`), and a relay may offer several. Additions that old peers can ignore (new message types, new fields, new notice codes) do not change the name.

## Try it without a radio

`roger-sim` in `relay/cmd/roger-sim` implements the radio side in about 150 lines of Go and prints the message timeline:

```
$ ROGER_PROVIDER=mock ./bin/roger-relay &
$ ./bin/roger-sim -token rgr_...
   0.00s  ready           max_press=30000ms volume_cap=70
   0.00s  ptt.down        sending 1.5s of audio
   1.53s  ptt.up
   1.53s  thinking        (the radio plays static now)
   1.53s  reply.start     pcm16 @ 16000 Hz, 0.00s after ptt.up
   2.81s  reply.end       1.9s of audio
saved reply.wav
```
