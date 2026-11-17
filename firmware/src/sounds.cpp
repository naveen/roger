#include "sounds.h"

#include "hal.h"

namespace {

uint32_t rng = 0x2545F491u;
int32_t lp = 0, lp2 = 0;

inline int32_t noise() {
  rng ^= rng << 13;
  rng ^= rng >> 17;
  rng ^= rng << 5;
  return static_cast<int32_t>(rng & 0xFFFF) - 32768;
}

// A sine burst with 5 ms fades, written straight to the speaker.
void beep(float freq, uint16_t ms, float gain = 0.25f) {
  static int16_t buf[160];
  const uint32_t total = sounds::kRate * ms / 1000;
  const uint32_t fade = sounds::kRate / 200;
  const float step = 2.0f * PI * freq / sounds::kRate;
  for (uint32_t i = 0; i < total;) {
    size_t n = 0;
    for (; n < 160 && i < total; n++, i++) {
      float env = 1.0f;
      if (i < fade) env = static_cast<float>(i) / fade;
      else if (total - i < fade) env = static_cast<float>(total - i) / fade;
      buf[n] = static_cast<int16_t>(sinf(step * i) * gain * env * 32767.0f);
    }
    hal::spkWrite(buf, n);
  }
}

void gap(uint16_t ms) {
  static const int16_t zeros[160] = {};
  for (uint32_t left = sounds::kRate * ms / 1000; left > 0;) {
    const size_t n = min<uint32_t>(left, 160);
    hal::spkWrite(zeros, n);
    left -= n;
  }
}

} // namespace

namespace sounds {

void staticFill(int16_t* dst, size_t n) {
  // White noise, band-passed to the "small radio speaker" range, with a slow
  // wobble so it sounds like a signal coming and going rather than a hiss.
  static uint32_t t = 0;
  for (size_t i = 0; i < n; i++, t++) {
    const int32_t x = noise();
    lp += (x - lp) * 45 / 100;  // low-pass, about 2 kHz
    lp2 += (lp - lp2) * 6 / 100; // slower low-pass, about 150 Hz
    const int32_t band = lp - lp2;
    const float wobble = 0.75f + 0.25f * sinf(t * (2.0f * PI * 1.7f / kRate));
    dst[i] = static_cast<int16_t>(band * 0.28f * wobble);
  }
}

void rogerBeep() {
  hal::spkStart(kRate);
  beep(1047, 70);
  hal::spkDrain();
}

void overBeep() {
  hal::spkStart(kRate);
  int16_t buf[160];
  for (int i = 0; i < 6; i++) { // a 60 ms squelch tail
    staticFill(buf, 160);
    hal::spkWrite(buf, 160);
  }
  beep(784, 60);
  hal::spkDrain();
}

void notHeard() {
  hal::spkStart(kRate);
  beep(880, 70);
  gap(70);
  beep(880, 70);
  hal::spkDrain();
}

void offAir() {
  hal::spkStart(kRate);
  beep(523, 140);
  gap(40);
  beep(392, 220);
  hal::spkDrain();
}

void problem() {
  hal::spkStart(kRate);
  beep(196, 320, 0.3f);
  hal::spkDrain();
}

} // namespace sounds
