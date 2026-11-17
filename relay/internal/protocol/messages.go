// Package protocol defines the roger.v0 wire messages shared by the relay and
// the device simulator. The normative description lives in
// protocol/PROTOCOL.md at the repo root; keep the two in sync.
package protocol

// Subprotocol is the WebSocket subprotocol name devices must offer.
const Subprotocol = "roger.v0"

// Message types. Text frames carry one JSON object with a "type" field.
// Binary frames carry raw PCM16LE mono audio and have no envelope.
const (
	// Device -> relay.
	TypeHello   = "hello"
	TypePTTDown = "ptt.down"
	TypePTTUp   = "ptt.up"
	TypeCancel  = "cancel"

	// Relay -> device.
	TypeReady      = "ready"
	TypeThinking   = "thinking"
	TypeReplyStart = "reply.start"
	TypeReplyEnd   = "reply.end"
	TypeNotice     = "notice"
)

// Notice codes. A notice may be followed by a spoken reply with the same seq.
const (
	NoticeQuietHours       = "quiet_hours"
	NoticeAirtimeExhausted = "airtime_exhausted"
	NoticeNotHeard         = "not_heard"
	NoticeError            = "error"
)

// Codec names. v0 defines only raw PCM; "opus" is reserved.
const CodecPCM16 = "pcm16"

// Envelope is the superset of fields used by all text messages. Unused
// fields are omitted on the wire.
type Envelope struct {
	Type string `json:"type"`
	Seq  int    `json:"seq,omitempty"`

	// hello
	Proto string     `json:"proto,omitempty"`
	FW    string     `json:"fw,omitempty"`
	Board string     `json:"board,omitempty"`
	Mic   *AudioSpec `json:"mic,omitempty"`

	// ready
	Session string  `json:"session,omitempty"`
	Limits  *Limits `json:"limits,omitempty"`

	// reply.start
	Codec string `json:"codec,omitempty"`
	Rate  int    `json:"rate,omitempty"`

	// reply.end
	Cancelled bool `json:"cancelled,omitempty"`

	// notice
	Code string `json:"code,omitempty"`
	// Speech is true when a spoken reply with the same seq follows the
	// notice. When false the device plays its own built-in tone.
	Speech bool `json:"speech,omitempty"`
}

// AudioSpec describes the audio a device sends.
type AudioSpec struct {
	Codec string `json:"codec"`
	Rate  int    `json:"rate"`
}

// Limits are enforced by the relay and mirrored on the device.
type Limits struct {
	MaxPressMS int `json:"max_press_ms"`
	VolumeCap  int `json:"volume_cap"` // 0-100, set by the parent
}
