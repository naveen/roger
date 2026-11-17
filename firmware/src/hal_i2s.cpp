// HAL for the DIY build: a bare ESP32-S3 with an I2S MEMS microphone
// (INMP441) and an I2S amplifier (MAX98357A), using the ESP-IDF I2S driver.
#if defined(ROGER_BOARD_XIAO_DIY)

#include <driver/i2s_std.h>

#include "boards/xiao_diy.h"
#include "hal.h"

namespace {

i2s_chan_handle_t rx = nullptr; // microphone
i2s_chan_handle_t tx = nullptr; // amplifier
bool rxOn = false, txOn = false;
uint32_t txRate = 0;
uint32_t warmupLeft = 0; // samples still to silence after the mic starts
uint8_t outVolume = 70;  // 0-100
uint8_t knob = 100;      // 0-100
hal::Light current = hal::Light::Offline;

bool pttRaw() { return digitalRead(PIN_PTT) == (PTT_ACTIVE_HIGH ? HIGH : LOW); }

void led(bool on) { digitalWrite(PIN_LED, (on != (LED_ACTIVE_LOW != 0)) ? HIGH : LOW); }

i2s_std_gpio_config_t pins(int bclk, int ws, int dout, int din) {
  i2s_std_gpio_config_t g = {};
  g.mclk = I2S_GPIO_UNUSED;
  g.bclk = static_cast<gpio_num_t>(bclk);
  g.ws = static_cast<gpio_num_t>(ws);
  g.dout = dout < 0 ? I2S_GPIO_UNUSED : static_cast<gpio_num_t>(dout);
  g.din = din < 0 ? I2S_GPIO_UNUSED : static_cast<gpio_num_t>(din);
  return g;
}

} // namespace

namespace hal {

void begin() {
  pinMode(PIN_PTT, PTT_ACTIVE_HIGH ? INPUT_PULLDOWN : INPUT_PULLUP);
  pinMode(PIN_LED, OUTPUT);
  led(false);
  pinMode(PIN_AMP_SD, OUTPUT);
  digitalWrite(PIN_AMP_SD, LOW); // amplifier off until there is something to say
#if PIN_VOLUME >= 0
  analogReadResolution(12);
#endif
}

void update() {
#if PIN_VOLUME >= 0
  // Light smoothing so a noisy pot does not make the volume flutter.
  static uint32_t smooth = 0;
  smooth = (smooth * 7 + analogRead(PIN_VOLUME)) / 8;
  knob = map(smooth, 0, 4095, 0, 100);
#endif
  const uint32_t t = millis();
  switch (current) {
    case Light::Offline: led(t % 2000 < 100); break;
    case Light::Ready: led(false); break;
    case Light::Listening: led(true); break;
    case Light::Thinking: led(t % 300 < 150); break;
    case Light::Speaking: led(t % 1000 < 800); break;
    case Light::Problem: led(t % 200 < 100); break;
  }
}

const char* boardName() { return ROGER_BOARD_NAME; }

bool pttDown() {
  // Debounce: report a change only after it has held for 15 ms.
  static bool stable = false, last = false;
  static uint32_t since = 0;
  const bool raw = pttRaw();
  if (raw != last) {
    last = raw;
    since = millis();
  } else if (raw != stable && millis() - since >= 15) {
    stable = raw;
  }
  return stable;
}

uint8_t userVolume() { return knob; }

void setOutputVolume(uint8_t pct) { outVolume = min<uint8_t>(pct, 100); }

void light(Light l) { current = l; }

int batteryPercent() { return -1; }

bool micStart(uint32_t rate) {
  if (!rx) {
    i2s_chan_config_t cc = I2S_CHANNEL_DEFAULT_CONFIG(I2S_NUM_0, I2S_ROLE_MASTER);
    if (i2s_new_channel(&cc, nullptr, &rx) != ESP_OK) return false;
    i2s_std_config_t sc = {};
    sc.clk_cfg = I2S_STD_CLK_DEFAULT_CONFIG(rate);
    // The INMP441 sends 24-bit samples in 32-bit slots.
    sc.slot_cfg = I2S_STD_PHILIPS_SLOT_DEFAULT_CONFIG(I2S_DATA_BIT_WIDTH_32BIT, I2S_SLOT_MODE_MONO);
    sc.slot_cfg.slot_mask = I2S_STD_SLOT_LEFT; // L/R pin tied to GND
    sc.gpio_cfg = pins(PIN_MIC_SCK, PIN_MIC_WS, -1, PIN_MIC_SD);
    if (i2s_channel_init_std_mode(rx, &sc) != ESP_OK) return false;
  }
  if (!rxOn && i2s_channel_enable(rx) != ESP_OK) return false;
  rxOn = true;
  warmupLeft = rate * MIC_WARMUP_MS / 1000;
  return true;
}

size_t micRead(int16_t* dst, size_t samples) {
  if (!rxOn) return 0;
  static int32_t raw[kMicChunk];
  samples = min(samples, kMicChunk);
  size_t got = 0;
  if (i2s_channel_read(rx, raw, samples * sizeof(int32_t), &got, pdMS_TO_TICKS(250)) != ESP_OK) return 0;
  const size_t n = got / sizeof(int32_t);
  for (size_t i = 0; i < n; i++) {
    // Keep the top bits with a little gain; the INMP441 is a quiet mic.
    dst[i] = static_cast<int16_t>(constrain(raw[i] >> 14, -32768, 32767));
    if (warmupLeft > 0) {
      dst[i] = 0;
      warmupLeft--;
    }
  }
  return n;
}

void micStop() {
  if (!rxOn) return;
  i2s_channel_disable(rx); // clocks stop: the microphone is no longer being read
  rxOn = false;
}

bool spkStart(uint32_t rate) {
  if (!tx) {
    i2s_chan_config_t cc = I2S_CHANNEL_DEFAULT_CONFIG(I2S_NUM_1, I2S_ROLE_MASTER);
    cc.auto_clear = true; // play silence, not a stuck buffer, on underrun
    if (i2s_new_channel(&cc, &tx, nullptr) != ESP_OK) return false;
    i2s_std_config_t sc = {};
    sc.clk_cfg = I2S_STD_CLK_DEFAULT_CONFIG(rate);
    sc.slot_cfg = I2S_STD_PHILIPS_SLOT_DEFAULT_CONFIG(I2S_DATA_BIT_WIDTH_16BIT, I2S_SLOT_MODE_MONO);
    sc.gpio_cfg = pins(PIN_AMP_BCLK, PIN_AMP_LRC, PIN_AMP_DIN, -1);
    if (i2s_channel_init_std_mode(tx, &sc) != ESP_OK) return false;
    txRate = rate;
  } else if (rate != txRate) {
    if (txOn) {
      i2s_channel_disable(tx);
      txOn = false;
    }
    i2s_std_clk_config_t clk = I2S_STD_CLK_DEFAULT_CONFIG(rate);
    if (i2s_channel_reconfig_std_clock(tx, &clk) != ESP_OK) return false;
    txRate = rate;
  }
  if (!txOn && i2s_channel_enable(tx) != ESP_OK) return false;
  txOn = true;
  digitalWrite(PIN_AMP_SD, HIGH);
  return true;
}

void spkWrite(const int16_t* src, size_t samples) {
  if (!txOn) return;
  // The MAX98357A has no volume control, so scale the samples.
  static int16_t scaled[256];
  const int32_t gain = static_cast<int32_t>(outVolume) * 256 / 100;
  while (samples > 0) {
    const size_t n = min<size_t>(samples, 256);
    for (size_t i = 0; i < n; i++) scaled[i] = static_cast<int16_t>((src[i] * gain) >> 8);
    size_t written = 0;
    i2s_channel_write(tx, scaled, n * sizeof(int16_t), &written, portMAX_DELAY);
    src += n;
    samples -= n;
  }
}

void spkDrain() {
  if (!txOn) return;
  // Push silence through so the last real samples leave the DMA buffers.
  static const int16_t zeros[256] = {};
  for (int i = 0; i < 8; i++) {
    size_t written = 0;
    i2s_channel_write(tx, zeros, sizeof(zeros), &written, portMAX_DELAY);
  }
}

void spkStop() {
  if (!txOn) return;
  digitalWrite(PIN_AMP_SD, LOW);
  i2s_channel_disable(tx);
  txOn = false;
}

} // namespace hal

#endif // ROGER_BOARD_XIAO_DIY
