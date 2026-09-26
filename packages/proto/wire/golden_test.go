package wire

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Altın çerçeveler: iki gerçekleme (Go ve Dart) aynı mesajlar için AYNI
// baytları üretmelidir. Dosya testdata/golden_frames.txt; Dart testi de aynı
// dosyayı okuyup kendi kodeğini doğrular (apps/participant/test/wire_test.dart)
// — tel sapması böyle yakalanır, beacon'daki WAV çapraz doğrulamasının eşleniği.
//
// Örneklerde varsayılan (sıfır) değerli alan YOKTUR: proto3 sıfır değeri tele
// yazmaz ve iki taraf da alanları numara sırasıyla yazar; bu yüzden bayt-bayt
// eşitlik beklemek güvenlidir. Şema değişince: go test ./packages/proto/wire
// -run TestGoldenFrames -update
var goldenCases = []struct {
	msgType string
	msg     any
}{
	{TypeHello, Hello{ProtocolVersion: 2, JoinCode: "ABC234", ClientKind: "flutter"}},
	{TypeWelcome, Welcome{ServerTimeMs: 1789816791704, ProtocolVersion: 2, RoomID: "room_1"}},
	{TypeClockSyncRequest, ClockSyncRequest{Seq: 7, ClientMonoMs: 123456}},
	{TypeClockSyncResponse, ClockSyncResponse{Seq: 7, ClientMonoMs: 123456, ServerRecvMs: 1789816791800, ServerSendMs: 1789816791801}},
	{TypeCueStart, CueStart{
		RunID: "c3a4f80219eb0db9", CueID: "program", FireAtServerMs: 1789816792298, RepeatSeq: 2,
		Payload: CuePayload{Color: "#FF2A2A", Torch: true, FlashHz: 2, DurationMs: 4000},
	}},
	{TypeIntervention, Intervention{RunID: "r1", Kind: "BLACKOUT", IssuedAtServerMs: 42}},
	{TypeShowActivated, ShowActivated{RoomID: "room_1", ShowVersionID: "sv_42"}},
}

const goldenPath = "testdata/golden_frames.txt"

var update = flag.Bool("update", false, "altın çerçeve dosyasını yeniden üret")

func TestGoldenFrames(t *testing.T) {
	if *update {
		var b strings.Builder
		b.WriteString("# tür<TAB>hex — üretici: go test ./packages/proto/wire -run TestGoldenFrames -update\n")
		for _, tc := range goldenCases {
			data, err := EncodeBinary(tc.msgType, tc.msg)
			if err != nil {
				t.Fatalf("%s kodlanamadı: %v", tc.msgType, err)
			}
			fmt.Fprintf(&b, "%s\t%s\n", tc.msgType, hex.EncodeToString(data))
		}
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("altın dosya okunamadı (önce -update ile üretin): %v", err)
	}
	lines := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			t.Fatalf("bozuk altın satır: %q", line)
		}
		lines[parts[0]] = parts[1]
	}
	if len(lines) != len(goldenCases) {
		t.Fatalf("altın dosyada %d satır var, %d örnek bekleniyor — -update ile tazeleyin", len(lines), len(goldenCases))
	}
	for _, tc := range goldenCases {
		wantHex, ok := lines[tc.msgType]
		if !ok {
			t.Fatalf("altın dosyada %s yok — -update ile tazeleyin", tc.msgType)
		}
		// Kodlama baytları birebir tutmalı (Dart tarafı da aynı hex'e karşı test eder).
		data, err := EncodeBinary(tc.msgType, tc.msg)
		if err != nil {
			t.Fatalf("%s kodlanamadı: %v", tc.msgType, err)
		}
		if got := hex.EncodeToString(data); got != wantHex {
			t.Errorf("%s kodlaması altından saptı:\nistenen %s\nalınan  %s", tc.msgType, wantHex, got)
		}
		// Ve altın baytlar aynı mesaja geri çözülmeli.
		wantBytes, err := hex.DecodeString(wantHex)
		if err != nil {
			t.Fatalf("%s altın hex bozuk: %v", tc.msgType, err)
		}
		gotType, gotMsg, err := DecodeBinary(wantBytes)
		if err != nil {
			t.Fatalf("%s altın baytları çözülemedi: %v", tc.msgType, err)
		}
		if gotType != tc.msgType {
			t.Fatalf("tür = %s, beklenen %s", gotType, tc.msgType)
		}
		want, _ := json.Marshal(tc.msg)
		got, _ := json.Marshal(gotMsg)
		if string(want) != string(got) {
			t.Errorf("%s altın çözümü sapıyor:\nistenen %s\nalınan  %s", tc.msgType, want, got)
		}
	}
}
