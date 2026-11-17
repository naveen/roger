// HAL for M5Stack boards with a built-in codec, via M5Unified.
// Written for the StickS3; other M5 boards with a mic and speaker should
// need only a new board header.
#if defined(ROGER_BOARD_M5STICKS3)

#include <M5Unified.h>
#include <Preferences.h>

#include "boards/m5sticks3.h"
#include "hal.h"

namespace {

// M5Unified records into caller-owned buffers and reports each one back
// through a callback. Two buffers used alternately give gap-free capture.
int16_t micBuf[2][hal::kMicChunk];
QueueHandle_t micDone = nullptr;
uint32_t micRate = ROGER_MIC_RATE;
bool micOn = false;

// playRaw queues at most two buffers per channel and reads them in place, so
// three buffers used in rotation are always safe to refill.
constexpr size_t kSpkChunk = 1024;
constexpr uint8_t kChannel = 0;
int16_t spkBuf[3][kSpkChunk];
uint8_t spkNext = 0;
uint32_t spkRate = 16000;
bool spkOn = false;

Preferences prefs;
uint8_t volLevel = 3; // 1..4
hal::Light shown = hal::Light::Problem;
bool drawn = false;

void micReleased(void*, void* data, size_t) {
  // Runs on the capture task: hand the filled buffer to the main loop.
  xQueueSend(micDone, &data, 0);
}

} // namespace

namespace hal {

void begin() {
  auto cfg = M5.config();
  cfg.internal_mic = true;
  cfg.internal_spk = true;
  M5.begin(cfg);
  M5.Display.setBrightness(40); // nothing to stare at: a dim status dot only
  M5.Display.fillScreen(TFT_BLACK);

  micDone = xQueueCreate(4, sizeof(void*));
  M5.Mic.setBufferReleaseCallback(nullptr, micReleased);

  prefs.begin("roger", false);
  volLevel = constrain(prefs.getUChar("vol", 3), 1, 4);
}

void update() {
  M5.update();
  if (M5.BtnB.wasClicked()) {
    volLevel = volLevel % 4 + 1;
    prefs.putUChar("vol", volLevel);
  }
}

const char* boardName() { return ROGER_BOARD_NAME; }

bool pttDown() { return M5.BtnA.isPressed(); }

uint8_t userVolume() { return volLevel * 25; }

void setOutputVolume(uint8_t pct) { M5.Speaker.setVolume((uint16_t)min<uint8_t>(pct, 100) * 255 / 100); }

void light(Light l) {
  if (drawn && l == shown) return;
  shown = l;
  drawn = true;
  uint32_t c = TFT_DARKGREY;
  switch (l) {
    case Light::Offline: c = 0x404040u; break;
    case Light::Ready: c = 0x4FE0A0u; break;     // mint
    case Light::Listening: c = 0xF2622Eu; break; // orange
    case Light::Thinking: c = 0xFFC83Du; break;  // yellow
    case Light::Speaking: c = 0xF4EBDDu; break;  // cream
    case Light::Problem: c = 0xC0392Bu; break;
  }
  M5.Display.fillScreen(TFT_BLACK);
  M5.Display.fillCircle(M5.Display.width() / 2, M5.Display.height() / 2, 14, M5.Display.color888(c >> 16, (c >> 8) & 0xFF, c & 0xFF));
}

int batteryPercent() { return M5.Power.getBatteryLevel(); }

bool micStart(uint32_t rate) {
  // The StickS3's codec does one direction at a time.
  spkStop();
  micRate = rate;
  if (!M5.Mic.begin()) return false;
  xQueueReset(micDone);
  micOn = true;
  return M5.Mic.record(micBuf[0], kMicChunk, micRate) && M5.Mic.record(micBuf[1], kMicChunk, micRate);
}

size_t micRead(int16_t* dst, size_t samples) {
  if (!micOn || samples != kMicChunk) return 0;
  void* filled = nullptr;
  if (xQueueReceive(micDone, &filled, pdMS_TO_TICKS(250)) != pdTRUE) return 0;
  memcpy(dst, filled, kMicChunk * sizeof(int16_t));
  M5.Mic.record(static_cast<int16_t*>(filled), kMicChunk, micRate); // hand it straight back
  return kMicChunk;
}

void micStop() {
  if (!micOn) return;
  micOn = false;
  M5.Mic.end(); // drops queued requests; capture is fully off from here
}

bool spkStart(uint32_t rate) {
  micStop();
  spkRate = rate;
  if (spkOn) return true;
  spkOn = M5.Speaker.begin();
  return spkOn;
}

void spkWrite(const int16_t* src, size_t samples) {
  if (!spkOn) return;
  while (samples > 0) {
    size_t n = min(samples, kSpkChunk);
    memcpy(spkBuf[spkNext], src, n * sizeof(int16_t));
    while (M5.Speaker.isPlaying(kChannel) >= 2) delay(1); // wait for a free queue slot
    M5.Speaker.playRaw(spkBuf[spkNext], n, spkRate, false, 1, kChannel, false);
    spkNext = (spkNext + 1) % 3;
    src += n;
    samples -= n;
  }
}

void spkDrain() {
  if (!spkOn) return;
  uint32_t t0 = millis();
  while (M5.Speaker.isPlaying(kChannel) && millis() - t0 < 3000) delay(2);
  delay(40); // let the last DMA buffers leave the codec
}

void spkStop() {
  if (!spkOn) return;
  M5.Speaker.stop(kChannel);
  M5.Speaker.end();
  spkOn = false;
}

} // namespace hal

#endif // ROGER_BOARD_M5STICKS3
