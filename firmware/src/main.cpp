// Roger firmware: a push-to-talk radio that speaks roger.v0 to a relay.
//
// The whole device is one small state machine:
//
//   Idle --press--> Listening --release--> Waiting --reply.start--> Replying --> Idle
//
// Listening is the only state in which the microphone runs. It starts when
// the button goes down and stops when it comes up; nothing else turns it on.
//
// See protocol/PROTOCOL.md for the wire format.

#include <Arduino.h>
#include <ArduinoJson.h>
#include <WebSocketsClient.h>
#include <WiFi.h>

#if __has_include("config.h")
#include "config.h"
#else
#error "Copy include/config.example.h to include/config.h and fill it in."
#endif

#if ROGER_RELAY_TLS && !defined(ROGER_RELAY_CA_PEM)
#error "ROGER_RELAY_TLS is on but ROGER_RELAY_CA_PEM is not set. Roger does not do unverified TLS."
#endif

#ifdef ROGER_OTA_PASSWORD
#include <ArduinoOTA.h>
#endif

#include "board.h"
#include "hal.h"
#include "ring.h"
#include "sounds.h"

namespace {

enum class State : uint8_t { Idle, Listening, Waiting, Replying };

constexpr uint32_t kWaitTimeoutMs = 35000;  // give up on a reply after this long
constexpr uint32_t kStallTimeoutMs = 8000;  // or if reply audio stops arriving
constexpr uint32_t kHardMaxPressMs = 30000; // never record longer, whatever the relay says
constexpr size_t kPlayChunk = 480;          // 20 ms at 24 kHz

WebSocketsClient ws;
Ring ring;

State state = State::Idle;
uint32_t stateSince = 0;
int seq = 0;

bool online = false;      // the relay has accepted this radio
bool needRelease = false; // ignore the button until it is let go
bool quietWait = false;   // waiting for a spoken notice: no static
bool replyEnded = false;
bool playing = false;
uint32_t replyRate = 24000;
uint32_t lastAudioMs = 0;

uint8_t volumeCap = 70; // parent's cap, sent by the relay
uint32_t maxPressMs = kHardMaxPressMs;

void enter(State s) {
  state = s;
  stateSince = millis();
  switch (s) {
    case State::Idle: break; // the loop shows Ready or Offline
    case State::Listening: hal::light(hal::Light::Listening); break;
    case State::Waiting: hal::light(hal::Light::Thinking); break;
    case State::Replying: hal::light(hal::Light::Speaking); break;
  }
}

void toIdle() {
  hal::micStop();
  hal::spkStop();
  ring.clear();
  needRelease = hal::pttDown();
  enter(State::Idle);
}

void sendEvent(const char* type, int s) {
  char msg[64];
  snprintf(msg, sizeof(msg), "{\"type\":\"%s\",\"seq\":%d}", type, s);
  ws.sendTXT(msg);
}

void sendHello() {
  char msg[200];
  snprintf(msg, sizeof(msg),
           "{\"type\":\"hello\",\"proto\":\"roger.v0\",\"fw\":\"%s\",\"board\":\"%s\","
           "\"mic\":{\"codec\":\"pcm16\",\"rate\":%d}}",
           ROGER_FW_VERSION, hal::boardName(), ROGER_MIC_RATE);
  ws.sendTXT(msg);
}

// Move up to one chunk from the buffer to the speaker.
void pump() {
  static int16_t chunk[kPlayChunk];
  const size_t n = ring.pop(chunk, kPlayChunk);
  if (n > 0) hal::spkWrite(chunk, n);
}

void buzz() {
  hal::light(hal::Light::Problem);
  sounds::problem();
}

void playNotice(const char* code) {
  if (!strcmp(code, "not_heard")) sounds::notHeard();
  else if (!strcmp(code, "error")) buzz();
  else sounds::offAir(); // quiet_hours, airtime_exhausted, anything newer
}

// ---- Button ---------------------------------------------------------------

void startPress() {
  hal::spkStop(); // also interrupts Roger mid-sentence
  ring.clear();
  if (!online) {
    sounds::offAir();
    hal::spkStop();
    needRelease = true;
    enter(State::Idle);
    return;
  }
  seq++;
  sendEvent("ptt.down", seq); // before any audio, so the relay is expecting it
  if (!hal::micStart(ROGER_MIC_RATE)) {
    sendEvent("ptt.up", seq);
    buzz();
    toIdle();
    needRelease = true;
    return;
  }
  enter(State::Listening);
}

void endPress() {
  hal::micStop(); // microphone off before anything else happens
  sendEvent("ptt.up", seq);
  needRelease = hal::pttDown(); // still held: the press hit the time limit
  sounds::rogerBeep();
  hal::spkStart(sounds::kRate);
  quietWait = false;
  enter(State::Waiting);
}

// ---- Relay messages ---------------------------------------------------------

void onText(const uint8_t* payload, size_t length) {
  JsonDocument doc;
  if (deserializeJson(doc, payload, length)) return;
  const char* type = doc["type"] | "";
  const int s = doc["seq"] | 0;

  if (!strcmp(type, "ready")) {
    online = true;
    volumeCap = constrain(doc["limits"]["volume_cap"] | 70, 0, 100);
    maxPressMs = min<uint32_t>(doc["limits"]["max_press_ms"] | kHardMaxPressMs, kHardMaxPressMs);
    return;
  }
  if (s != seq) return; // about a press we have already moved on from

  if (!strcmp(type, "reply.start")) {
    if (state != State::Waiting) return;
    replyRate = doc["rate"] | 24000;
    ring.clear();
    replyEnded = false;
    playing = false;
    lastAudioMs = millis();
    hal::spkStart(replyRate);
    enter(State::Replying);
  } else if (!strcmp(type, "reply.end")) {
    if (state == State::Replying) replyEnded = true;
  } else if (!strcmp(type, "notice")) {
    if (state == State::Listening) { // the relay refused this press
      hal::micStop();
      sendEvent("ptt.up", seq);
      needRelease = true;
    } else if (state != State::Waiting) {
      return;
    }
    if (doc["speech"] | false) { // Roger will say why; wait for it without static
      hal::spkStop();
      quietWait = true;
      enter(State::Waiting);
      return;
    }
    playNotice(doc["code"] | "");
    toIdle();
  }
  // "thinking" needs no action: the static is already playing.
}

void onAudio(const uint8_t* payload, size_t length) {
  if (state != State::Replying) return;
  lastAudioMs = millis();
  static int16_t aligned[256];
  for (size_t off = 0; off + 1 < length;) {
    const size_t n = min<size_t>((length - off) / 2, 256);
    memcpy(aligned, payload + off, n * 2);
    while (ring.space() < n) pump(); // buffer full: play some to make room
    ring.push(aligned, n);
    off += n * 2;
  }
}

void onWs(WStype_t type, uint8_t* payload, size_t length) {
  switch (type) {
    case WStype_CONNECTED:
      sendHello();
      break;
    case WStype_DISCONNECTED:
      online = false;
      if (state != State::Idle) {
        hal::micStop();
        sounds::offAir();
        toIdle();
        needRelease = true;
      }
      break;
    case WStype_TEXT:
      onText(payload, length);
      break;
    case WStype_BIN:
      onAudio(payload, length);
      break;
    default:
      break;
  }
}

} // namespace

void setup() {
  Serial.begin(115200);
  hal::begin();
  hal::light(hal::Light::Offline);
  // Two seconds of reply at 24 kHz when the board has PSRAM, 0.4 s otherwise.
  if (!ring.begin(48000, 9600)) Serial.println("roger: no memory for the reply buffer");

  WiFi.mode(WIFI_STA);
  WiFi.setAutoReconnect(true);
  WiFi.begin(ROGER_WIFI_SSID, ROGER_WIFI_PASS);

  ws.setExtraHeaders("Authorization: Bearer " ROGER_DEVICE_TOKEN);
#if ROGER_RELAY_TLS
  ws.beginSslWithCA(ROGER_RELAY_HOST, ROGER_RELAY_PORT, ROGER_RELAY_PATH, ROGER_RELAY_CA_PEM, "roger.v0");
#else
  ws.begin(ROGER_RELAY_HOST, ROGER_RELAY_PORT, ROGER_RELAY_PATH, "roger.v0");
#endif
  ws.onEvent(onWs);
  ws.setReconnectInterval(3000);
  ws.enableHeartbeat(15000, 3000, 2);

  Serial.printf("roger: firmware %s on %s\n", ROGER_FW_VERSION, hal::boardName());
}

void loop() {
  hal::update();
  hal::setOutputVolume(static_cast<uint16_t>(hal::userVolume()) * volumeCap / 100);

  if (WiFi.status() == WL_CONNECTED) {
    ws.loop();
#ifdef ROGER_OTA_PASSWORD
    static bool ota = false;
    if (!ota) {
      ArduinoOTA.setHostname("roger");
      ArduinoOTA.setPassword(ROGER_OTA_PASSWORD);
      ArduinoOTA.begin();
      ota = true;
    }
    if (state == State::Idle) ArduinoOTA.handle(); // never mid-conversation
#endif
  } else {
    online = false;
  }

  const bool down = hal::pttDown();
  if (!down) needRelease = false;
  const bool pressed = down && !needRelease;

  switch (state) {
    case State::Idle:
      hal::light(online ? hal::Light::Ready : hal::Light::Offline);
      if (pressed) startPress();
      else delay(5);
      break;

    case State::Listening: {
      static int16_t chunk[hal::kMicChunk];
      const size_t n = hal::micRead(chunk, hal::kMicChunk);
      if (n > 0) ws.sendBIN(reinterpret_cast<uint8_t*>(chunk), n * sizeof(int16_t));
      if (state != State::Listening) break; // the send can deliver a refusal
      if (!down || millis() - stateSince >= maxPressMs) endPress();
      break;
    }

    case State::Waiting:
      if (pressed) {
        startPress(); // changed their mind: ask something else
      } else if (millis() - stateSince > kWaitTimeoutMs) {
        sendEvent("cancel", seq);
        buzz();
        toIdle();
      } else if (!quietWait) {
        static int16_t noise[320];
        sounds::staticFill(noise, 320);
        hal::spkWrite(noise, 320); // blocks about 20 ms, which paces this loop
      } else {
        delay(5);
      }
      break;

    case State::Replying:
      if (pressed) {
        startPress(); // talking over Roger stops Roger
        break;
      }
      if (!playing && (ring.size() >= replyRate / 5 || replyEnded)) playing = true;
      if (playing && ring.size() > 0) {
        pump();
      } else if (replyEnded && ring.size() == 0) {
        hal::spkDrain();
        sounds::overBeep();
        toIdle();
      } else if (millis() - lastAudioMs > kStallTimeoutMs) {
        sendEvent("cancel", seq);
        buzz();
        toIdle();
      } else {
        playing = false; // ran dry: gather 200 ms again before resuming
        delay(2);
      }
      break;
  }
}
