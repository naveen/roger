# Roadmap

Now, next, later. Items move up when the thing before them has taught us something.

## Now: find out if it works at all

The question for this phase is whether a kid still picks it up on day seven. Nothing below needs custom hardware.

- [ ] **Bring up a StickS3.** Flash it, run the relay in mock mode, hear your own voice come back. Fix what breaks. Record what you changed.
- [ ] **Run the live pipeline.** Real keys, real questions. Record release-to-first-audio latency from the relay's log.
- [ ] **Go through the safety table** in [`SAFETY.md`](SAFETY.md) by hand and fix the prompt where it fails.
- [ ] **Tune the sounds** on a real speaker: static level, beep lengths, whether the pause feels like a radio.
- [ ] **Print the front shell** and test-fit the button, speaker and pot.
- [ ] **Five to ten radios in real homes** for two weeks. Watch presses per day and day-seven use, read the digests, talk to the parents.

## Next: make it something you could hand to a friend

- [ ] **Setup without a compiler.** A pairing flow for Wi-Fi and the token (the spoken-code idea in the wireframes), so nobody edits a header file.
- [ ] **Power management.** Sleep between presses, wake on the button, and a measured battery life.
- [ ] **An independent moderation pass** on the transcript and the reply, plus a written test set that runs on every prompt change.
- [ ] **Faster turns.** Streaming transcription during the press; start speaking on the first sentence.
- [ ] **A parent web page** in place of the JSON file: the six wireframe screens.
- [ ] **Opus** on the wire.
- [ ] **Over-the-air updates** from the relay, signed.
- [ ] **A real datastore and accounts**, if a hosted relay is on the table.
- [ ] **Expert review** of the persona prompt and the digest by people who work with children.

## Later: make it a product

- [ ] **A custom board**, with a load switch on the microphone so "off" is a fact about the circuit.
- [ ] **A sealed, drop-tested enclosure** that earns "built for outside".
- [ ] **Cellular**, following [`CELLULAR.md`](CELLULAR.md).
- [ ] **Certification and compliance**: radio approvals, toy safety, battery shipping, children's privacy.
- [ ] **Friend channels.** Parent-approved kid-to-kid radio. The concept page says "not at launch". It is the hardest thing on this list to do safely.
- [ ] **More languages.**

## Not planned

- An always-listening mode or wake word.
- A screen.
- Advertising, or any use of a child's words beyond answering them and summarising for their parent.
