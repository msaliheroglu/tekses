package wire

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	teksesv1 "github.com/msaliheroglu/tekses/packages/proto/gen/go/tekses/v1"
)

// ProtocolVersionBinary, ikili protobuf telinin sürümüdür (v2). v1 istemciler
// JSON metin çerçeveleriyle konuşmayı sürdürür; gateway ikisini aynı anda
// destekler ve istemcinin hello çerçevesinin biçimi kodeki belirler.
const ProtocolVersionBinary = 2

var kindToProto = map[string]teksesv1.InterventionKind{
	"HOLD":     teksesv1.InterventionKind_INTERVENTION_KIND_HOLD,
	"STOP":     teksesv1.InterventionKind_INTERVENTION_KIND_STOP,
	"SKIP":     teksesv1.InterventionKind_INTERVENTION_KIND_SKIP,
	"BLACKOUT": teksesv1.InterventionKind_INTERVENTION_KIND_BLACKOUT,
}

var kindFromProto = func() map[teksesv1.InterventionKind]string {
	m := make(map[teksesv1.InterventionKind]string, len(kindToProto))
	for s, k := range kindToProto {
		m[k] = s
	}
	return m
}()

// EncodeBinary, bir mesajı protobuf Envelope baytlarına çevirir; Encode'un
// (JSON) ikili eşleniğidir ve aynı tür/gövde çiftlerini alır.
func EncodeBinary(msgType string, msg any) ([]byte, error) {
	env := &teksesv1.Envelope{}
	switch msgType {
	case TypeHello:
		h, ok := msg.(Hello)
		if !ok {
			return nil, fmt.Errorf("wire: %s için beklenmeyen gövde %T", msgType, msg)
		}
		env.Kind = &teksesv1.Envelope_Hello{Hello: &teksesv1.Hello{
			ProtocolVersion: h.ProtocolVersion,
			JoinCode:        h.JoinCode,
			ClientKind:      h.ClientKind,
		}}
	case TypeWelcome:
		w, ok := msg.(Welcome)
		if !ok {
			return nil, fmt.Errorf("wire: %s için beklenmeyen gövde %T", msgType, msg)
		}
		env.Kind = &teksesv1.Envelope_Welcome{Welcome: &teksesv1.Welcome{
			ServerTimeMs:    w.ServerTimeMs,
			ProtocolVersion: w.ProtocolVersion,
			RoomId:          w.RoomID,
		}}
	case TypeClockSyncRequest:
		r, ok := msg.(ClockSyncRequest)
		if !ok {
			return nil, fmt.Errorf("wire: %s için beklenmeyen gövde %T", msgType, msg)
		}
		env.Kind = &teksesv1.Envelope_ClockSyncRequest{ClockSyncRequest: &teksesv1.ClockSyncRequest{
			Seq:          r.Seq,
			ClientMonoMs: r.ClientMonoMs,
		}}
	case TypeClockSyncResponse:
		r, ok := msg.(ClockSyncResponse)
		if !ok {
			return nil, fmt.Errorf("wire: %s için beklenmeyen gövde %T", msgType, msg)
		}
		env.Kind = &teksesv1.Envelope_ClockSyncResponse{ClockSyncResponse: &teksesv1.ClockSyncResponse{
			Seq:          r.Seq,
			ClientMonoMs: r.ClientMonoMs,
			ServerRecvMs: r.ServerRecvMs,
			ServerSendMs: r.ServerSendMs,
		}}
	case TypeCueStart:
		c, ok := msg.(CueStart)
		if !ok {
			return nil, fmt.Errorf("wire: %s için beklenmeyen gövde %T", msgType, msg)
		}
		env.Kind = &teksesv1.Envelope_CueStart{CueStart: &teksesv1.CueStart{
			RunId:          c.RunID,
			CueId:          c.CueID,
			FireAtServerMs: c.FireAtServerMs,
			RepeatSeq:      c.RepeatSeq,
			Payload: &teksesv1.CuePayload{
				Color:      c.Payload.Color,
				Torch:      c.Payload.Torch,
				FlashHz:    c.Payload.FlashHz,
				DurationMs: c.Payload.DurationMs,
			},
		}}
	case TypeIntervention:
		iv, ok := msg.(Intervention)
		if !ok {
			return nil, fmt.Errorf("wire: %s için beklenmeyen gövde %T", msgType, msg)
		}
		kind, ok := kindToProto[iv.Kind]
		if !ok {
			return nil, fmt.Errorf("wire: geçersiz müdahale türü %q", iv.Kind)
		}
		env.Kind = &teksesv1.Envelope_Intervention{Intervention: &teksesv1.Intervention{
			RunId:            iv.RunID,
			Kind:             kind,
			IssuedAtServerMs: iv.IssuedAtServerMs,
		}}
	default:
		return nil, fmt.Errorf("wire: bilinmeyen mesaj türü %q", msgType)
	}
	return proto.Marshal(env)
}

// DecodeBinary, protobuf Envelope baytlarını tür ayırıcısı ve wire gövdesine
// çözer; çağıran taraf JSON yolundakiyle aynı yapılarla çalışmayı sürdürür.
func DecodeBinary(raw []byte) (msgType string, msg any, err error) {
	var env teksesv1.Envelope
	if err := proto.Unmarshal(raw, &env); err != nil {
		return "", nil, fmt.Errorf("wire: ikili zarf çözülemedi: %w", err)
	}
	switch kind := env.Kind.(type) {
	case *teksesv1.Envelope_Hello:
		return TypeHello, Hello{
			ProtocolVersion: kind.Hello.GetProtocolVersion(),
			JoinCode:        kind.Hello.GetJoinCode(),
			ClientKind:      kind.Hello.GetClientKind(),
		}, nil
	case *teksesv1.Envelope_Welcome:
		return TypeWelcome, Welcome{
			ServerTimeMs:    kind.Welcome.GetServerTimeMs(),
			ProtocolVersion: kind.Welcome.GetProtocolVersion(),
			RoomID:          kind.Welcome.GetRoomId(),
		}, nil
	case *teksesv1.Envelope_ClockSyncRequest:
		return TypeClockSyncRequest, ClockSyncRequest{
			Seq:          kind.ClockSyncRequest.GetSeq(),
			ClientMonoMs: kind.ClockSyncRequest.GetClientMonoMs(),
		}, nil
	case *teksesv1.Envelope_ClockSyncResponse:
		return TypeClockSyncResponse, ClockSyncResponse{
			Seq:          kind.ClockSyncResponse.GetSeq(),
			ClientMonoMs: kind.ClockSyncResponse.GetClientMonoMs(),
			ServerRecvMs: kind.ClockSyncResponse.GetServerRecvMs(),
			ServerSendMs: kind.ClockSyncResponse.GetServerSendMs(),
		}, nil
	case *teksesv1.Envelope_CueStart:
		p := kind.CueStart.GetPayload()
		return TypeCueStart, CueStart{
			RunID:          kind.CueStart.GetRunId(),
			CueID:          kind.CueStart.GetCueId(),
			FireAtServerMs: kind.CueStart.GetFireAtServerMs(),
			RepeatSeq:      kind.CueStart.GetRepeatSeq(),
			Payload: CuePayload{
				Color:      p.GetColor(),
				Torch:      p.GetTorch(),
				FlashHz:    p.GetFlashHz(),
				DurationMs: p.GetDurationMs(),
			},
		}, nil
	case *teksesv1.Envelope_Intervention:
		name, ok := kindFromProto[kind.Intervention.GetKind()]
		if !ok {
			return "", nil, fmt.Errorf("wire: bilinmeyen müdahale türü %v", kind.Intervention.GetKind())
		}
		return TypeIntervention, Intervention{
			RunID:            kind.Intervention.GetRunId(),
			Kind:             name,
			IssuedAtServerMs: kind.Intervention.GetIssuedAtServerMs(),
		}, nil
	default:
		return "", nil, fmt.Errorf("wire: zarf boş ya da bilinmeyen tür taşıyor")
	}
}
