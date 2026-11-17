// Package audio has the small PCM helpers the relay needs. The relay never
// writes audio to disk; these work on in-memory buffers only.
package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"time"
)

// EncodeWAV wraps PCM16LE mono samples in a WAV container.
func EncodeWAV(pcm []byte, rate int) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+len(pcm)))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(&b, binary.LittleEndian, uint16(1)) // mono
	binary.Write(&b, binary.LittleEndian, uint32(rate))
	binary.Write(&b, binary.LittleEndian, uint32(rate*2))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(len(pcm)))
	b.Write(pcm)
	return b.Bytes()
}

// DecodeWAV reads a PCM16 mono WAV and returns its samples and rate.
func DecodeWAV(r io.Reader) (pcm []byte, rate int, err error) {
	var hdr [12]byte
	if _, err = io.ReadFull(r, hdr[:]); err != nil {
		return nil, 0, err
	}
	if string(hdr[0:4]) != "RIFF" || string(hdr[8:12]) != "WAVE" {
		return nil, 0, errors.New("not a WAV file")
	}
	for {
		var ch [8]byte
		if _, err = io.ReadFull(r, ch[:]); err != nil {
			return nil, 0, errors.New("WAV has no data chunk")
		}
		size := binary.LittleEndian.Uint32(ch[4:8])
		body := make([]byte, size)
		switch string(ch[0:4]) {
		case "fmt ":
			if _, err = io.ReadFull(r, body); err != nil {
				return nil, 0, err
			}
			if len(body) < 16 {
				return nil, 0, errors.New("short fmt chunk")
			}
			format := binary.LittleEndian.Uint16(body[0:2])
			channels := binary.LittleEndian.Uint16(body[2:4])
			bits := binary.LittleEndian.Uint16(body[14:16])
			if format != 1 || channels != 1 || bits != 16 {
				return nil, 0, errors.New("WAV must be PCM, mono, 16-bit")
			}
			rate = int(binary.LittleEndian.Uint32(body[4:8]))
		case "data":
			n, _ := io.ReadFull(r, body)
			if rate == 0 {
				return nil, 0, errors.New("WAV data before fmt")
			}
			return body[:n&^1], rate, nil
		default:
			if _, err = io.ReadFull(r, body); err != nil {
				return nil, 0, err
			}
		}
		if size%2 == 1 { // chunks are word-aligned
			io.CopyN(io.Discard, r, 1)
		}
	}
}

// Duration of n bytes of PCM16 mono at rate.
func Duration(n, rate int) time.Duration {
	if rate <= 0 {
		return 0
	}
	return time.Duration(n/2) * time.Second / time.Duration(rate)
}

// Tone returns a sine burst with a short fade at each end, as PCM16LE mono.
func Tone(freq float64, d time.Duration, rate int, gain float64) []byte {
	n := int(d.Seconds() * float64(rate))
	out := make([]byte, n*2)
	fade := rate / 100 // 10 ms
	for i := 0; i < n; i++ {
		env := 1.0
		if i < fade {
			env = float64(i) / float64(fade)
		} else if n-i < fade {
			env = float64(n-i) / float64(fade)
		}
		v := math.Sin(2*math.Pi*freq*float64(i)/float64(rate)) * gain * env
		binary.LittleEndian.PutUint16(out[i*2:], uint16(int16(v*32767)))
	}
	return out
}

// RMS returns the root-mean-square level of PCM16LE samples, 0..1.
func RMS(pcm []byte) float64 {
	n := len(pcm) / 2
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		s := float64(int16(binary.LittleEndian.Uint16(pcm[i*2:]))) / 32768
		sum += s * s
	}
	return math.Sqrt(sum / float64(n))
}
