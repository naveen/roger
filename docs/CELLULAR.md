# Cellular

Roger v0 is Wi-Fi only. That covers a house and usually a garden. This note is about what it would take to work at the park, and where a connectivity platform such as Gigs fits.

Nothing here is built. It is a plan and a list of open questions.

## The framework: two layers

"Add cellular" is two separate problems that are easy to blur.

| Layer | Question | Who solves it |
|---|---|---|
| Hardware | Can the device physically get on a network? Modem, antenna, power, certification. | You, with a module vendor |
| Commercial | Who is the subscriber, what is the plan, who bills, who pays telecom taxes, who answers support? | A connectivity platform, or an IoT SIM provider |

Gigs and its peers work on the second layer. No one can take the first off your hands.

## Hardware layer

| Topic | What it means for Roger |
|---|---|
| Modem class | LTE Cat 1 bis is the comfortable choice for voice-rate audio with a single antenna. LTE-M can carry the bitrate but adds latency and has patchier coverage in some countries. NB-IoT is too slow. |
| Power | Modems draw current in short, large bursts. The battery, regulator and layout all have to be designed for that; the StickS3's 250 mAh cell is not enough. |
| Antenna | Needs real space and a ground plane. It will shape the enclosure. The decorative antenna on the concept could become a real one. |
| Certification | A device with a cellular radio needs regulatory approval (FCC in the US) and usually carrier approval. Pre-certified modules reduce the work but do not remove it. This is the long pole. |
| SIM | A screenless device cannot scan a QR code. It needs a physical SIM or a soldered eSIM with a profile loaded at the factory or over the air. |

For a prototype, an off-the-shelf ESP32-S3 board with a Cat 1 modem already on it avoids all of the above except the SIM. Several exist (LilyGo and Waveshare both sell ESP32-S3 boards with SIMCom Cat 1 modems; the Walter Cat 1 bis module was in crowdfunding at the time of writing, with delivery estimated for late 2026). None has been tested with this firmware.

## The bitrate problem

The v0 protocol sends uncompressed audio, which is fine on Wi-Fi and far too heavy for a data plan.

| | Up (kid talking) | Down (Roger talking) | 20 minutes of air time a day, per month |
|---|---|---|---|
| PCM, as today | 256 kbit/s | 384 kbit/s | About 1.4 GB |
| Opus at 16 kbit/s | 16 kbit/s | 16 kbit/s | About 75 MB, before overhead |

(Assumes the 20 minutes splits evenly between talking and listening.)

So Opus is a prerequisite, not an optimisation. The protocol reserves the codec name; the work is an encoder on the device, a decoder and encoder in the relay, and a negotiation in `hello`.

At roughly 100 MB a month, Roger is an IoT-sized data user, not a phone-sized one.

## Commercial layer: where Gigs fits

What Gigs describes on its site (checked November 2026):

- Aimed at hardware companies selling phones, tablets and wearables that want to offer their own plans.
- One API across carriers. AT&T in the US and Vodafone in the UK are named.
- Hosted checkout, recurring billing, retries, invoicing, and telecom taxes handled.
- Physical SIMs and eSIMs.
- No stated minimum commitments or onboarding fees.
- API model: a user, a plan and a subscription, which provisions a SIM. eSIM installation is by install link, QR code or activation code.
- Access is by arrangement: Gigs sets up your organisation and plans.

What that means for Roger:

**It fits the business model more than the prototype.** Roger has a recurring cost per kid (model and speech usage) whether or not it has cellular. A monthly subscription that bundles the AI and the connectivity is the natural shape for the product, and billing, taxes and support for that subscription are exactly what a platform like this takes on.

**It is unclear whether it fits the device.** The published flow assumes something with a screen that can install an eSIM profile. Questions to put to them before assuming anything:

1. Do you support data-only plans on a device that is not a phone, tablet or watch?
2. Can SIMs or eSIM profiles be provisioned in bulk before the device ships, with no user-side install step?
3. Which modem modules have your carriers already approved, and what is the approval path for a new device?
4. What does a roughly 100 MB a month plan cost at low volume?
5. Is there an age or use restriction on plans attached to a children's device?

**For a prototype, it is the wrong tool.** A handful of test devices need a data SIM that works today, which is what IoT SIM providers sell (Hologram, 1NCE, Soracom and others). No commercial agreement, no plan design.

## Suggested sequence

1. **Now.** Wi-Fi only. Learn whether kids use it.
2. **Spike, in parallel.** One Cat 1 dev board, one IoT SIM, the existing firmware with PCM. Measure latency and dropouts on a walk. This costs a weekend and answers whether the feel survives a cellular link.
3. **If the spike is encouraging.** Add Opus. Talk to Gigs and one IoT SIM provider with the five questions above and a working unit in hand.
4. **Custom board.** Choose the modem from the answer to question 3. Budget for certification.

A cheaper interim worth knowing about: a radio can join a parent's phone hotspot. That gets a Wi-Fi radio to the park with no new hardware, at the cost of some setup friction.
