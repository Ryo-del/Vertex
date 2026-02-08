package slab

import (
	"fmt"
	"math"
)

type Input struct {
	MomentKNmPerM float64 `json:"moment_knm_per_m"`
	EffectiveDepthMM float64 `json:"effective_depth_mm"`
	FydMPa        float64 `json:"fyd_mpa"`
	BarDiameterMM float64 `json:"bar_diameter_mm"`
}

type Result struct {
	AsRequiredMM2PerM float64 `json:"as_required_mm2_per_m"`
	BarAreaMM2        float64 `json:"bar_area_mm2"`
	SpacingMM         float64 `json:"spacing_mm"`
	Notes             string  `json:"notes"`
}

func Calculate(in Input) (Result, error) {
	if in.MomentKNmPerM <= 0 || in.EffectiveDepthMM <= 0 || in.FydMPa <= 0 || in.BarDiameterMM <= 0 {
		return Result{}, fmt.Errorf("invalid input")
	}
	z := 0.9 * in.EffectiveDepthMM
	As := (in.MomentKNmPerM * 1e6) / (0.87 * in.FydMPa * z)
	barArea := math.Pi * in.BarDiameterMM * in.BarDiameterMM / 4.0
	spacing := barArea * 1000.0 / As
	return Result{
		AsRequiredMM2PerM: As,
		BarAreaMM2:        barArea,
		SpacingMM:         spacing,
		Notes:             "Single-layer slab reinforcement estimate.",
	}, nil
}
