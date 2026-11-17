// Hardware abstraction: everything the radio logic needs from a board.
// One implementation per board family lives next to this file
// (hal_m5.cpp, hal_i2s.cpp); main.cpp never touches a pin.
#pragma once

#include <Arduino.h>

namespace hal {

enum class Light : uint8_t { Offline, Ready, Listening, Thinking, Speaking, Problem };

void begin();
void update(); // call once per loop: buttons, knob, LED animation

const char* boardName();
bool pttDown();
uint8_t userVolume();              // 0-100, what the kid has dialled in
void setOutputVolume(uint8_t pct); // 0-100, after the parent's cap
void light(Light l);
int batteryPercent(); // -1 when the board cannot tell

// Microphone. Capture runs only between micStart and micStop.
bool micStart(uint32_t rate);
size_t micRead(int16_t* dst, size_t samples); // blocks; returns samples read (0 on timeout)
void micStop();

// Speaker.
bool spkStart(uint32_t rate);
void spkWrite(const int16_t* src, size_t samples); // blocks until queued
void spkDrain();                                   // blocks until queued audio has played
void spkStop();

// Samples per micRead call. A whole number of these is always requested.
constexpr size_t kMicChunk = 320; // 20 ms at 16 kHz

} // namespace hal
