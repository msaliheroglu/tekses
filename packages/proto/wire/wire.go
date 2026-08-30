// Package wire, Faz 0 WebSocket telinin JSON türlerini tanımlar.
//
// Şemanın gerçeği packages/proto/tekses/v1/*.proto dosyalarıdır; buradaki
// türler proto mesajlarını alan adlarıyla (snake_case) birebir izler.
// Faz 1'de ikili protobuf'a geçildiğinde bu paket üretilmiş koda devrolur.
package wire

import (
	"encoding/json"
	"fmt"
)

// ProtocolVersion, Faz 0 tel protokolü sürümüdür.
const ProtocolVersion = 1

// Mesaj türü ayırıcıları; envelope.proto'daki oneof alan adlarını izler.
const (
	TypeHello             = "hello"
	TypeWelcome           = "welcome"
	TypeClockSyncRequest  = "clock_sync_request"
	TypeClockSyncResponse = "clock_sync_response"
	TypeCueStart          = "cue_start"
	TypeIntervention      = "intervention"
)

// Envelope, tel üzerindeki tek çerçevedir: {"type": "...", "data": {...}}.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Hello — tekses.v1.Hello.
type Hello struct {
	ProtocolVersion uint32 `json:"protocol_version"`
	JoinCode        string `json:"join_code,omitempty"`
	ClientKind      string `json:"client_kind,omitempty"`
}

// Welcome — tekses.v1.Welcome.
type Welcome struct {
	ServerTimeMs    int64  `json:"server_time_ms"`
	ProtocolVersion uint32 `json:"protocol_version"`
	RoomID          string `json:"room_id"`
}

// ClockSyncRequest — tekses.v1.ClockSyncRequest.
type ClockSyncRequest struct {
	Seq          uint32 `json:"seq"`
	ClientMonoMs int64  `json:"client_mono_ms"`
}

// ClockSyncResponse — tekses.v1.ClockSyncResponse.
type ClockSyncResponse struct {
	Seq          uint32 `json:"seq"`
	ClientMonoMs int64  `json:"client_mono_ms"`
	ServerRecvMs int64  `json:"server_recv_ms"`
	ServerSendMs int64  `json:"server_send_ms"`
}

// CuePayload — tekses.v1.CuePayload.
type CuePayload struct {
	Color      string `json:"color"`
	Torch      bool   `json:"torch"`
	FlashHz    uint32 `json:"flash_hz"`
	DurationMs uint32 `json:"duration_ms"`
}

// CueStart — tekses.v1.CueStart.
type CueStart struct {
	RunID          string     `json:"run_id"`
	CueID          string     `json:"cue_id"`
	FireAtServerMs int64      `json:"fire_at_server_ms"`
	RepeatSeq      uint32     `json:"repeat_seq"`
	Payload        CuePayload `json:"payload"`
}

// Intervention — tekses.v1.Intervention.
type Intervention struct {
	RunID            string `json:"run_id"`
	Kind             string `json:"kind"` // HOLD | STOP | SKIP | BLACKOUT
	IssuedAtServerMs int64  `json:"issued_at_server_ms"`
}

// Geçerli müdahale türleri.
var InterventionKinds = map[string]bool{
	"HOLD":     true,
	"STOP":     true,
	"SKIP":     true,
	"BLACKOUT": true,
}

// Encode, bir mesajı tür ayırıcılı zarfa sarıp JSON'a çevirir.
func Encode(msgType string, msg any) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("wire: %s gövdesi kodlanamadı: %w", msgType, err)
	}
	return json.Marshal(Envelope{Type: msgType, Data: data})
}

// Decode, tel baytlarını zarfa çözer; gövde çağıran tarafça
// env.Data üzerinden türüne göre çözülür.
func Decode(raw []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Envelope{}, fmt.Errorf("wire: zarf çözülemedi: %w", err)
	}
	if env.Type == "" {
		return Envelope{}, fmt.Errorf("wire: zarfta type alanı yok")
	}
	return env, nil
}
