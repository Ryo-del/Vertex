package beam

import (
	"fmt"
	"math"
)

type Input struct {
	Material             string  `json:"material"` // steel or rc
	FyMPa                float64 `json:"fy_mpa"`
	E_GPa                float64 `json:"e_gpa"`
	SpanM                float64 `json:"span_m"`
	UDLKNM               float64 `json:"udl_kn_m"`
	WidthM               float64 `json:"width_m"`
	HeightM              float64 `json:"height_m"`
	DeflectionLimitRatio float64 `json:"deflection_limit_ratio"`
}

type Result struct {
	MaxMomentKNM     float64 `json:"max_moment_knm"`
	RequiredHeightM  float64 `json:"required_height_m"`
	StressMPa        float64 `json:"stress_mpa"`
	DeflectionMM     float64 `json:"deflection_mm"`
	DeflectionLimitM float64 `json:"deflection_limit_mm"`
	OKStress         bool    `json:"ok_stress"`
	OKDeflection     bool    `json:"ok_deflection"`
	Notes            string  `json:"notes"`
}

func Calculate(in Input) (Result, error) {
	if in.SpanM <= 0 || in.UDLKNM <= 0 || in.WidthM <= 0 {
		return Result{}, fmt.Errorf("invalid input")
	}
	if in.DeflectionLimitRatio <= 0 {
		in.DeflectionLimitRatio = 250
	}
	if in.E_GPa <= 0 {
		if in.Material == "rc" {
			in.E_GPa = 30
		} else {
			in.E_GPa = 200
		}
	}
	if in.FyMPa <= 0 {
		if in.Material == "rc" {
			in.FyMPa = 14
		} else {
			in.FyMPa = 235
		}
	}

	// Simply supported beam, UDL: M = w L^2 / 8
	M := in.UDLKNM * in.SpanM * in.SpanM / 8.0

	bmm := in.WidthM * 1000.0
	hmm := in.HeightM * 1000.0

	// Required height if not provided
	if hmm <= 0 {
		Wreq := (M * 1e6) / in.FyMPa
		hmm = math.Sqrt(6.0 * Wreq / bmm)
	}

	// Section modulus for rectangle
	W := bmm * hmm * hmm / 6.0
	stress := (M * 1e6) / W

	// Deflection for UDL: 5 w L^4 / (384 E I)
	I := bmm * math.Pow(hmm, 3) / 12.0
	wNmm := in.UDLKNM // 1 kN/m = 1 N/mm
	Lmm := in.SpanM * 1000.0
	E := in.E_GPa * 1000.0 // MPa
	defl := 5.0 * wNmm * math.Pow(Lmm, 4) / (384.0 * E * I)
	deflLimit := Lmm / in.DeflectionLimitRatio

	return Result{
		MaxMomentKNM:     M,
		RequiredHeightM:  hmm / 1000.0,
		StressMPa:        stress,
		DeflectionMM:     defl,
		DeflectionLimitM: deflLimit,
		OKStress:         stress <= in.FyMPa,
		OKDeflection:     defl <= deflLimit,
		Notes:            "Simplified beam check (UDL, simply supported).",
	}, nil
}
