package wire

import (
	"encoding/json"
	"testing"
)

func TestBinaryRoundTrip(t *testing.T) {
	cases := []struct {
		msgType string
		msg     any
	}{
		{TypeHello, Hello{ProtocolVersion: ProtocolVersionBinary, JoinCode: "ABC234", ClientKind: "loadgen"}},
		{TypeWelcome, Welcome{ServerTimeMs: 1789816791704, ProtocolVersion: ProtocolVersionBinary, RoomID: "room_1"}},
		{TypeClockSyncRequest, ClockSyncRequest{Seq: 7, ClientMonoMs: 123456}},
		{TypeClockSyncResponse, ClockSyncResponse{Seq: 7, ClientMonoMs: 123456, ServerRecvMs: 200, ServerSendMs: 201}},
		{TypeCueStart, CueStart{
			RunID: "c3a4f80219eb0db9", CueID: "program", FireAtServerMs: 1789816792298, RepeatSeq: 2,
			Payload: CuePayload{Color: "#FF2A2A", Torch: true, FlashHz: 2, DurationMs: 4000},
		}},
		{TypeIntervention, Intervention{RunID: "r1", Kind: "BLACKOUT", IssuedAtServerMs: 42}},
	}
	for _, tc := range cases {
		data, err := EncodeBinary(tc.msgType, tc.msg)
		if err != nil {
			t.Fatalf("%s kodlanamadı: %v", tc.msgType, err)
		}
		gotType, gotMsg, err := DecodeBinary(data)
		if err != nil {
			t.Fatalf("%s çözülemedi: %v", tc.msgType, err)
		}
		if gotType != tc.msgType {
			t.Fatalf("tür = %s, beklenen %s", gotType, tc.msgType)
		}
		// Karşılaştırma için JSON gösterimine indirgenir (yapılar düz veridir).
		want, _ := json.Marshal(tc.msg)
		got, _ := json.Marshal(gotMsg)
		if string(want) != string(got) {
			t.Fatalf("%s tur kaybı:\nistenen %s\nalınan  %s", tc.msgType, want, got)
		}
	}
}

// Karar hedefi: kue çerçevesi ikili telde ~40 bayt mertebesinde ve JSON'dan
// bariz küçük olmalı.
func TestBinaryCueSize(t *testing.T) {
	cue := CueStart{
		RunID: "c3a4f80219eb0db9", CueID: "program", FireAtServerMs: 1789816792298, RepeatSeq: 1,
		Payload: CuePayload{Color: "#FF2A2A", Torch: true, FlashHz: 2, DurationMs: 4000},
	}
	bin, err := EncodeBinary(TypeCueStart, cue)
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := Encode(TypeCueStart, cue)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("kue çerçevesi: ikili %d bayt, JSON %d bayt", len(bin), len(jsonData))
	if len(bin) >= 100 {
		t.Fatalf("ikili kue çerçevesi %d bayt; 100 baytın altında olmalı", len(bin))
	}
	if len(bin)*2 >= len(jsonData) {
		t.Fatalf("ikili (%d B) JSON'un (%d B) yarısından küçük değil", len(bin), len(jsonData))
	}
}

func TestDecodeBinaryRejectsGarbage(t *testing.T) {
	if _, _, err := DecodeBinary([]byte{0xff, 0x00, 0x13, 0x37}); err == nil {
		t.Fatal("bozuk baytlar kabul edildi")
	}
	if _, _, err := DecodeBinary(nil); err == nil {
		t.Fatal("boş zarf kabul edildi")
	}
}
