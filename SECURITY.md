# Security

Roger carries children's voices. Please treat a security problem here with that in mind.

## Reporting

Do not open a public issue for a vulnerability. Use GitHub's private reporting instead: on this repository, go to **Security**, then **Report a vulnerability**.

Include what you found, how to reproduce it, and what an attacker could do with it. If the report involves anything a real child said, leave that out.

## What counts

Especially:

- Any way to make a radio or relay capture or forward audio outside a button press.
- Any way to read another radio's stored text, digest or token.
- Bypassing a parent's rules from the device side.
- Getting the relay to speak attacker-chosen audio or text to a child.
- Authentication or TLS verification weaknesses.

## Known limits

These are documented design gaps in v0, not undisclosed vulnerabilities:

- Stored text is not encrypted at rest.
- The device token and Wi-Fi password are stored unencrypted in the radio's flash.
- Plain `ws://` is supported for relays on a home network.
- Over-the-air updates, when enabled, are protected by a shared password only.

See [`docs/PRIVACY.md`](docs/PRIVACY.md) and [`docs/ROADMAP.md`](docs/ROADMAP.md).
