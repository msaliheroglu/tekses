package clocksync

import "testing"

// makeSample, gerçek ofseti trueOffset olan bir sunucuya karşı, gidiş ve
// dönüş gecikmeleri verilen bir değişim örneği üretir.
func makeSample(t0, trueOffset, upDelay, downDelay, serverProc int64) Sample {
	t1 := t0 + upDelay + trueOffset
	t2 := t1 + serverProc
	t3 := t2 - trueOffset + downDelay
	return Sample{T0: t0, T1: t1, T2: t2, T3: t3}
}

func TestSampleOffsetSymmetricDelay(t *testing.T) {
	// Simetrik gecikmede tek örnek bile gerçek ofseti bulur.
	s := makeSample(1000, 500, 20, 20, 3)
	if got := s.Offset(); got != 500 {
		t.Fatalf("Offset() = %d, beklenen 500", got)
	}
	if got := s.RTT(); got != 40 {
		t.Fatalf("RTT() = %d, beklenen 40", got)
	}
}

func TestEstimatePrefersLowRTT(t *testing.T) {
	// Yüksek RTT'li, asimetrik gecikmeli (dolayısıyla saptırılmış) örnekler
	// elenmeli; temiz düşük RTT'li örneklerin medyanı kazanmalı.
	var e Estimator
	trueOffset := int64(-1234)
	for i := int64(0); i < 5; i++ {
		e.Add(makeSample(1000+i*100, trueOffset, 15, 15, 2)) // temiz
	}
	for i := int64(0); i < 5; i++ {
		e.Add(makeSample(2000+i*100, trueOffset, 400, 20, 2)) // saptırılmış
	}

	est, err := e.Estimate()
	if err != nil {
		t.Fatal(err)
	}
	if est.OffsetMs != trueOffset {
		t.Fatalf("OffsetMs = %d, beklenen %d", est.OffsetMs, trueOffset)
	}
	if est.UsedSamples != 5 {
		t.Fatalf("UsedSamples = %d, beklenen 5", est.UsedSamples)
	}
	if est.BestRTTMs != 30 {
		t.Fatalf("BestRTTMs = %d, beklenen 30", est.BestRTTMs)
	}
}

func TestEstimateDropsNegativeRTT(t *testing.T) {
	var e Estimator
	e.Add(Sample{T0: 100, T1: 50, T2: 51, T3: 90}) // RTT < 0 → atılır
	if e.Len() != 0 {
		t.Fatalf("Len() = %d, beklenen 0", e.Len())
	}
	if _, err := e.Estimate(); err != ErrNoSamples {
		t.Fatalf("err = %v, beklenen ErrNoSamples", err)
	}
}

func TestEstimateSingleSample(t *testing.T) {
	var e Estimator
	e.Add(makeSample(0, 42, 10, 10, 1))
	est, err := e.Estimate()
	if err != nil {
		t.Fatal(err)
	}
	if est.OffsetMs != 42 {
		t.Fatalf("OffsetMs = %d, beklenen 42", est.OffsetMs)
	}
}
