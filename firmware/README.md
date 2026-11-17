# Firmware

One codebase for both boards. It connects to Wi-Fi, holds a WebSocket open to a relay, sends microphone audio while the button is held, and plays what comes back.

> **Status: compiles, never run on hardware.** Both environments build cleanly with the pinned toolchain, including the TLS and over-the-air variants. No one has flashed a board yet. Expect to find bugs in the first hour, most likely in audio levels and timing.

## Build and flash

You need [PlatformIO](https://platformio.org/install/cli).

```
cd firmware
cp include/config.example.h include/config.h     # then edit it
pio run -e m5sticks3 -t upload                   # M5Stack StickS3
pio run -e xiao_diy  -t upload                   # DIY build
pio device monitor
```

`include/config.h` holds your Wi-Fi password and the radio's token. It is ignored by git. Get a token by running `roger-relay token` (see [`relay/README.md`](../relay/README.md)).

## First power-up

Run the relay in mock mode (`make run-mock` in `relay/`). It needs no API keys and answers every press with a chirp followed by your own voice played back. If you hear yourself, the microphone, speaker, Wi-Fi and protocol all work, and any remaining problem is in the providers.

| You hear | It means |
|---|---|
| Falling two-note tone on press | Not connected to the relay. Check Wi-Fi, the relay address and the token. |
| Short beep on release, then static | Normal. Roger is thinking. |
| Two short pips | The relay heard nothing it could use. Hold the button a little longer. |
| Low buzz | Something failed. Look at the relay's log. |

## How it is organised

| File | What it does |
|---|---|
| `src/main.cpp` | The state machine and the protocol. Start here. |
| `src/hal.h` | What the logic needs from a board: a button, a mic, a speaker, a light. |
| `src/hal_m5.cpp` | That interface for M5Stack boards, through M5Unified. |
| `src/hal_i2s.cpp` | That interface for a bare ESP32-S3 with I2S parts. |
| `src/sounds.cpp` | The static and the tones, synthesised on the device. |
| `src/ring.h` | The playback buffer. |
| `include/boards/` | One header per board: pins and options. |

The device is four states:

```
Idle --press--> Listening --release--> Waiting --reply.start--> Replying --> Idle
```

`Listening` is the only state in which the microphone runs. Pressing the button during `Waiting` or `Replying` interrupts Roger and starts a new press.

## Adding a board

1. Add `include/boards/yourboard.h` and a branch for it in `include/board.h`.
2. If it is an M5Stack board with a codec, `hal_m5.cpp` probably works as it is. If it has I2S parts, copy the pin block from `xiao_diy.h`.
3. Add an `[env:yourboard]` section to `platformio.ini`.
4. Open a pull request with a photo.

## Things this firmware deliberately does

- **Refuses unverified TLS.** If `ROGER_RELAY_TLS` is on and no CA certificate is given, it does not build. A kid's voice does not go over a connection anyone on the path could read.
- **Sends `ptt.down` before any audio**, and stops the microphone before sending `ptt.up`, so the relay can check that audio only ever arrives inside a press.
- **Caps the volume** at the parent's limit, which the relay sends on connect.
- **Limits a press to 30 seconds** even if a relay asks for more.
- **Never logs audio or text.** The serial log has state changes only.

## Known gaps

- Wi-Fi and the token are compiled in. There is no setup flow yet.
- No power management. The radio stays awake with Wi-Fi on, so battery life will be short (unmeasured; likely an hour or two on the StickS3's 250 mAh cell).
- Audio is uncompressed PCM. Fine on Wi-Fi, not viable on cellular.
- The StickS3 memory settings in `platformio.ini` (`qio_opi`) are from the module's datasheet family, not from a board on a bench. If PSRAM is not detected the firmware still runs with a smaller playback buffer.
- Over-the-air updates work on the local network only, and only if you set a password.

See [`docs/ROADMAP.md`](../docs/ROADMAP.md).

## Pinned versions

| | Version |
|---|---|
| Platform | pioarduino platform-espressif32 55.03.312 (Arduino core 3.3.x) |
| ArduinoJson | 7.4.3 |
| arduinoWebSockets | 2.7.2 |
| M5Unified, M5GFX | 0.3.0 (StickS3 only) |
