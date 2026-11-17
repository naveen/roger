// Roger firmware settings.
//
// Copy this file to include/config.h and fill it in. config.h is ignored by
// git: it holds your Wi-Fi password and the radio's token, so never commit it.
#pragma once

// --- Wi-Fi (2.4 GHz) --------------------------------------------------------
#define ROGER_WIFI_SSID "your-network"
#define ROGER_WIFI_PASS "your-password"

// --- Relay ------------------------------------------------------------------
// The machine running roger-relay. For a relay on your home network this is
// its LAN address and plain ws:// (ROGER_RELAY_TLS 0).
#define ROGER_RELAY_HOST "192.168.1.20"
#define ROGER_RELAY_PORT 8080
#define ROGER_RELAY_PATH "/v1/radio"

// Set to 1 for a relay reached over the internet (wss://). TLS is always
// verified: you must also provide the PEM of the root CA that signed the
// relay's certificate. The firmware refuses to build with TLS on and no CA,
// because unverified TLS would let anyone on the network path listen in.
#define ROGER_RELAY_TLS 0
// #define ROGER_RELAY_CA_PEM R"PEM(
// -----BEGIN CERTIFICATE-----
// ...
// -----END CERTIFICATE-----
// )PEM"

// --- This radio's token -------------------------------------------------------
// From `roger-relay token`. The relay stores only its hash.
#define ROGER_DEVICE_TOKEN "rgr_replace_me"

// --- Optional: over-the-air updates on your home network ----------------------
// Uncomment to accept firmware uploads over Wi-Fi (`pio run -t upload
// --upload-port roger.local`). Choose a real password; anyone on the network
// who knows it can replace the firmware.
// #define ROGER_OTA_PASSWORD "change-me"
