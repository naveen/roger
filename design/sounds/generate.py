"""Writes Roger's built-in sounds as WAV files, for listening and for design work.

The radio does not play these files. It synthesises the same sounds itself in
firmware/src/sounds.cpp, and this script mirrors that code step for step so
what you hear here is what the radio makes. Change both together.

    python generate.py

No dependencies beyond the Python standard library.
"""

import math
import os
import struct
import wave

RATE = 16000


class Static:
    """The band-passed noise from sounds.cpp: xorshift noise, two one-pole filters, a slow wobble."""

    def __init__(self):
        self.rng = 0x2545F491
        self.lp = 0
        self.lp2 = 0
        self.t = 0

    def _noise(self):
        r = self.rng
        r ^= (r << 13) & 0xFFFFFFFF
        r ^= r >> 17
        r ^= (r << 5) & 0xFFFFFFFF
        self.rng = r
        return (r & 0xFFFF) - 32768

    def fill(self, n):
        out = []
        for _ in range(n):
            x = self._noise()
            self.lp += int((x - self.lp) * 45 / 100)
            self.lp2 += int((self.lp - self.lp2) * 6 / 100)
            band = self.lp - self.lp2
            wobble = 0.75 + 0.25 * math.sin(self.t * (2 * math.pi * 1.7 / RATE))
            out.append(int(band * 0.28 * wobble))
            self.t += 1
        return out


def beep(freq, ms, gain=0.25):
    total = RATE * ms // 1000
    fade = RATE // 200
    step = 2 * math.pi * freq / RATE
    out = []
    for i in range(total):
        env = 1.0
        if i < fade:
            env = i / fade
        elif total - i < fade:
            env = (total - i) / fade
        out.append(int(math.sin(step * i) * gain * env * 32767))
    return out


def gap(ms):
    return [0] * (RATE * ms // 1000)


def write(name, samples):
    path = os.path.join(os.path.dirname(os.path.abspath(__file__)), name)
    with wave.open(path, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(RATE)
        w.writeframes(b"".join(struct.pack("<h", max(-32768, min(32767, s))) for s in samples))
    print(f"{name:18s} {len(samples) / RATE:5.2f} s")


if __name__ == "__main__":
    write("static.wav", Static().fill(RATE * 3))
    write("roger_beep.wav", beep(1047, 70))
    write("over_beep.wav", Static().fill(160 * 6) + beep(784, 60))
    write("not_heard.wav", beep(880, 70) + gap(70) + beep(880, 70))
    write("off_air.wav", beep(523, 140) + gap(40) + beep(392, 220))
    write("problem.wav", beep(196, 320, 0.3))
    # What a whole turn sounds like from the kid's side, minus Roger's voice.
    write("turn_demo.wav", beep(1047, 70) + Static().fill(RATE * 2) + gap(1200) + Static().fill(160 * 6) + beep(784, 60))
