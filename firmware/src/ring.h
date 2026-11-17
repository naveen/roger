// A fixed-size sample queue for the reply audio. Replies arrive a little
// ahead of real time; this holds them so playback is smooth when Wi-Fi
// hiccups. Used from the main loop only, so it needs no locking.
#pragma once

#include <Arduino.h>

class Ring {
 public:
  // Prefer external RAM (PSRAM) when the board has it; otherwise use a
  // smaller buffer from the internal heap.
  bool begin(size_t psramSamples, size_t internalSamples) {
    buf_ = static_cast<int16_t*>(heap_caps_malloc(psramSamples * sizeof(int16_t), MALLOC_CAP_SPIRAM));
    cap_ = psramSamples;
    if (!buf_) {
      buf_ = static_cast<int16_t*>(malloc(internalSamples * sizeof(int16_t)));
      cap_ = internalSamples;
    }
    return buf_ != nullptr;
  }

  size_t size() const { return size_; }
  size_t space() const { return cap_ - size_; }
  void clear() { head_ = size_ = 0; }

  // Push up to n samples; returns how many fitted.
  size_t push(const int16_t* src, size_t n) {
    n = min(n, space());
    for (size_t i = 0; i < n; i++) buf_[(head_ + size_ + i) % cap_] = src[i];
    size_ += n;
    return n;
  }

  // Pop up to n samples; returns how many were available.
  size_t pop(int16_t* dst, size_t n) {
    n = min(n, size_);
    for (size_t i = 0; i < n; i++) dst[i] = buf_[(head_ + i) % cap_];
    head_ = (head_ + n) % cap_;
    size_ -= n;
    return n;
  }

 private:
  int16_t* buf_ = nullptr;
  size_t cap_ = 0, head_ = 0, size_ = 0;
};
