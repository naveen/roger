// The radio's own sounds: the static it plays while Roger thinks and the
// short tones that stand in for words when the relay cannot speak. They are
// synthesised here so no audio files have to be flashed.
// design/sounds has the same sounds as WAV files for reference.
#pragma once

#include <Arduino.h>

namespace sounds {

constexpr uint32_t kRate = 16000;

// Fill dst with radio static. Call repeatedly for continuous noise.
void staticFill(int16_t* dst, size_t n);

// Each of these starts the speaker at kRate, plays, and returns when done.
void rogerBeep(); // end of transmission: "your turn, Roger"
void overBeep();  // end of Roger's reply
void notHeard();  // two short pips: say again
void offAir();    // falling pair: not now (offline, quiet hours, out of air time)
void problem();   // low buzz: something broke

} // namespace sounds
