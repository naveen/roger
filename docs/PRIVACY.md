# Privacy

What Roger collects about a child, where it goes, how long it stays, and how to delete it. This describes the code in this repository. It is not legal advice and it is not a privacy policy for any service.

## Principles

1. **Listen only while pressed.** Enforced on the device and checked by the relay.
2. **Keep no audio.** A press is held in memory until it has been transcribed, then zeroed.
3. **Keep little text, briefly.** Enough for the parent's digest, deleted on a schedule.
4. **Know almost nothing about the kid.** A first name or nickname, and an age.
5. **Summarise for parents; do not surveil.** The digest is topics and flags, not a transcript or a live feed.
6. **Nothing is sold, and there are no ads.**

## Data map

| Data | Where it is | How long | Who else sees it |
|---|---|---|---|
| A child's voice, one press at a time | Relay memory | Until transcribed (seconds) | The speech-to-text provider |
| What the child said, as text | Relay disk, `YYYY-MM-DD.jsonl` | `transcript_retention_days`, default 30 | The model provider |
| What Roger said, as text | Same file | Same | The model provider; the text-to-speech provider |
| Roger's voice | Relay memory, then the radio's playback buffer | Seconds | |
| Things the kid asked Roger to remember | Relay disk, `remembered.jsonl`, newest 50 | `remembered_retention_days`, default 90 | The model provider, as part of the prompt |
| Kid's first name and age | `devices.json` | Until you remove it | The model provider, as part of the prompt |
| Parent's email | `devices.json` | Until you remove it | Your SMTP provider |
| Press counts and durations | Relay log and day files | Log: your log retention. Files: as above | |
| Wi-Fi password, device token | The radio's flash | Until reflashed | |

Things the relay does not collect: location, contacts, device identifiers beyond the id you assign, photos, anything from a microphone outside a press.

The relay's log is written to contain no speech in either direction: ids, durations and flags only.

## The part that is not in your hands

The relay itself keeps no audio. But to answer a question, each press is sent to a speech-to-text provider, and the resulting text to a model provider and a text-to-speech provider. **What those companies keep, and for how long, is set by their terms and your account settings, not by this code.**

Before a child uses a relay you run:

- Read each provider's data retention terms for API use.
- Turn on zero-retention or no-training options where they are offered and you qualify.
- Or run speech-to-text and text-to-speech on your own hardware. The relay only needs two HTTP endpoints for audio, so a local server works. Only the model call then leaves your network.

## Deleting

| To delete | Do |
|---|---|
| Everything for one radio | `roger-relay erase -device ID` |
| One day | Delete that day's `.jsonl` file |
| One remembered note | Remove its line from `remembered.jsonl` |
| Automatically | Set the retention days in the rules. The relay prunes once a day and when a radio connects. |

There is no export command yet; the files are plain JSON lines and can be copied as they are.

Stored text is protected by file permissions only. It is not encrypted at rest. Use an encrypted disk for the data directory.

## Children's privacy law

This matters most if you run a relay for anyone outside your own household.

In the United States, COPPA covers online services directed at children under 13. The FTC's amended rule took effect in 2025 with a compliance date of 22 April 2026. Points that bear directly on a product like this:

- **A recording of a child's voice is personal information**, and a voiceprint is a biometric identifier.
- Operators need **verifiable parental consent** before collecting personal information from a child.
- Operators need a **written data retention policy** and may not keep children's data indefinitely.
- The privacy notice must say how audio files are used and deleted, and name the third parties that receive children's data.
- Operators need a **written security programme**.

How the design here lines up, as a starting point and nothing more: audio is transcribed and discarded; retention is finite by default and cannot be set to "forever"; the third parties are the providers you configure; and deletion is one command.

What is missing for anyone operating a service: a consent flow, a privacy notice, a security programme, and a contract review of each provider. Other countries have their own rules (the UK's Children's Code and GDPR-K in the EU, among others).

A parent running a relay at home for their own child is in a very different position from a company offering Roger to families. If you are, or might become, the second, talk to a lawyer who works on children's privacy before the first child outside your family uses it.

## Honest gaps

- The standard DIY wiring leaves the microphone powered between presses. "Off" is enforced in firmware and by the relay, not by a switch. The hard-switch wiring in [`hardware/README.md`](../hardware/README.md) addresses this and is untested.
- A radio on plain `ws://` sends audio unencrypted on the local network. That is for a relay at home only.
- The device token and Wi-Fi password are stored unencrypted in the radio's flash.
- The digest travels by ordinary email.
