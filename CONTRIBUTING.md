# Contributing

Thanks for helping. Roger is small and early, so most contributions are welcome and the process is light.

## What helps most right now

1. **A bring-up report.** Flash a board, write down what happened. Open an issue titled "Bring-up: <board>", even if everything worked.
2. **A test print.** Print the front shell, fit the parts, post photos and the dimensions you had to change.
3. **Safety findings.** A question Roger handled badly, with the prompt change that fixes it.
4. **A new board.** See "Adding a board" in [`firmware/README.md`](firmware/README.md).

The longer list is in [`docs/ROADMAP.md`](docs/ROADMAP.md).

## Ground rules

These exist because the user is a child.

- **Never post what a real child said.** Not in issues, pull requests, logs, screenshots or test fixtures. Rephrase it in your own words, or make up an example.
- **Never add logging of speech content,** in either direction, to the relay or the firmware. Durations, ids and flags are fine.
- **Never store audio.** If a change needs audio to persist beyond one press, it needs a discussion first.
- **Changes to the persona prompt or the guard need a second reviewer** and a note on what you tested. See the table in [`docs/SAFETY.md`](docs/SAFETY.md).
- **Keep the radio thin.** Anything that decides what a child hears belongs in the relay.
- **Do not weaken the microphone rule.** The relay closes a connection that sends audio outside a press. That stays.

## Working on the relay

```
cd relay
make test          # go vet, then the tests with the race detector
make run-mock      # run with no keys
```

Go 1.24 or newer. Run `gofmt` before committing. New behaviour comes with a test; the server tests in `internal/server/server_test.go` show how to drive a whole press through a fake pipeline.

## Working on the firmware

```
cd firmware
cp include/config.example.h include/config.h
pio run                     # builds the default board
pio run -e xiao_diy
```

Both environments must build without warnings from this project's own sources. If you change a sound, change `design/sounds/generate.py` to match. If you change a pin, change `hardware/wiring/make_wiring.py` to match and regenerate the diagram.

## Working on the protocol

`protocol/PROTOCOL.md` is the contract. A change to it comes with the matching change to the relay, the simulator and the firmware in the same pull request. Additive changes that old peers can ignore stay in `roger.v0`. Anything else needs a new version name and a conversation.

## Working on the hardware

Edit the parameters in `hardware/enclosure/enclosure.py`, run it, and commit the regenerated `step/`, `stl/` and `preview.svg` together with the script. Say in the pull request whether you printed it.

## Pull requests

- One change per pull request, with a sentence on why.
- Say what you tested and on what. "Compiles" and "ran on a StickS3" are different claims; both are useful when labelled.
- Update the docs that your change makes untrue, especially the status tables.

## Licence

By contributing you agree that your contribution is licensed under the licence that covers the directory it lands in (see the table in the [README](README.md)).
