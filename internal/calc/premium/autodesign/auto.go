package autodesign

import (
	"fmt"

	beam "Vertex/internal/calc/beam"
)

type BeamAutoInput struct {
	Material             string  `json:"material"`
	FyMPa                float64 `json:"fy_mpa"`
	E_GPa                float64 `json:"e_gpa"`
	SpanM                float64 `json:"span_m"`
	UDLKNM               float64 `json:"udl_kn_m"`
	WidthM               float64 `json:"width_m"`
	DeflectionLimitRatio float64 `json:"deflection_limit_ratio"`
}

type BeamAutoResult struct {
	RequiredHeightM float64 `json:"required_height_m"`
	StressMPa       float64 `json:"stress_mpa"`
	DeflectionMM    float64 `json:"deflection_mm"`
	OKStress        bool    `json:"ok_stress"`
	OKDeflection    bool    `json:"ok_deflection"`
	Notes           string  `json:"notes"`
}

func Beam(in BeamAutoInput) (BeamAutoResult, error) {
	if in.SpanM <= 0 || in.UDLKNM <= 0 || in.WidthM <= 0 {
		return BeamAutoResult{}, fmt.Errorf("invalid input")
	}
	res, err := beam.Calculate(beam.Input{
		Material:             in.Material,
		FyMPa:                in.FyMPa,
		E_GPa:                in.E_GPa,
		SpanM:                in.SpanM,
		UDLKNM:               in.UDLKNM,
		WidthM:               in.WidthM,
		HeightM:              0,
		DeflectionLimitRatio: in.DeflectionLimitRatio,
	})
	if err != nil {
		return BeamAutoResult{}, err
	}
	return BeamAutoResult{
		RequiredHeightM: res.RequiredHeightM,
		StressMPa:       res.StressMPa,
		DeflectionMM:    res.DeflectionMM,
		OKStress:        res.OKStress,
		OKDeflection:    res.OKDeflection,
		Notes:           "Auto-sized beam (height selected to satisfy stress).",
	}, nil
}
