// DIY build: Seeed XIAO ESP32S3 + INMP441 + MAX98357A.
// This file is the single source of truth for the wiring; hardware/wiring
// documents the same map. Change both together.
#pragma once

#define ROGER_BOARD_NAME "xiao-diy"
#define ROGER_MIC_RATE 16000

// Push-to-talk button. XIAO pad D0.
#define PIN_PTT 1
// Standard wiring: button between D0 and GND, internal pull-up, pressed = LOW.
// Hard mic switch wiring (see hardware/README.md): the button feeds 3V3 to
// the microphone and to D0, pressed = HIGH. Set this to 1 for that wiring.
#define PTT_ACTIVE_HIGH 0
// The INMP441 outputs nothing useful while it starts up. With the hard mic
// switch the mic powers on at every press, so this much audio is silenced.
#define MIC_WARMUP_MS (PTT_ACTIVE_HIGH ? 260 : 20)

// MAX98357A amplifier (I2S out).
#define PIN_AMP_BCLK 2 // XIAO D1
#define PIN_AMP_LRC 3  // XIAO D2
#define PIN_AMP_DIN 4  // XIAO D3
#define PIN_AMP_SD 5   // XIAO D4, HIGH = amplifier on

// Volume potentiometer wiper. XIAO D5. Set to -1 if you leave the knob out.
#define PIN_VOLUME 6

// INMP441 microphone (I2S in). Tie its L/R pin to GND.
#define PIN_MIC_SCK 7 // XIAO D8
#define PIN_MIC_WS 8  // XIAO D9
#define PIN_MIC_SD 9  // XIAO D10

// Status LED: the XIAO's own orange LED, lit when the pin is LOW.
#define PIN_LED 21
#define LED_ACTIVE_LOW 1
