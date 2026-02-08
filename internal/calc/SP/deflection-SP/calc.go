package deflection

import (
	"fmt"
	"math"
)

type Input struct {
	SpanM                float64 `json:"span_m"`
	UDLKNM               float64 `json:"udl_kn_m"`
	E_GPa                float64 `json:"e_gpa"`
	WidthM               float64 `json:"width_m"`
	HeightM              float64 `json:"height_m"`
	DeflectionLimitRatio float64 `json:"deflection_limit_ratio"`
}

type Result struct {
	DeflectionMM     float64 `json:"deflection_mm"`
	DeflectionLimitMM float64 `json:"deflection_limit_mm"`
	OK               bool    `json:"ok"`
	Notes            string  `json:"notes"`
}

func Calculate(in Input) (Result, error) {
	if in.SpanM <= 0 || in.UDLKNM <= 0 || in.WidthM <= 0 || in.HeightM <= 0 {
		return Result{}, fmt.Errorf("invalid input")
	}
	if in.DeflectionLimitRatio <= 0 {
		in.DeflectionLimitRatio = 250
	}
	if in.E_GPa <= 0 {
		in.E_GPa = 30
	}

	b := in.WidthM * 1000.0
	h := in.HeightM * 1000.0
	I := b * math.Pow(h, 3) / 12.0
	E := in.E_GPa * 1000.0 // MPa
	L := in.SpanM * 1000.0
	w := in.UDLKNM // 1 kN/m = 1 N/mm
	defl := 5.0 * w * math.Pow(L, 4) / (384.0 * E * I)
	limit := L / in.DeflectionLimitRatio
	return Result{
		DeflectionMM:      defl,
		DeflectionLimitMM: limit,
		OK:                defl <= limit,
		Notes:             "Deflection check per SP63 assumptions (simplified).",
	}, nil
}
