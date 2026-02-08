package slab

import (
	"fmt"
	"math"
)

type Input struct {
	MomentKNmPerM     float64 `json:"moment_knm_per_m"`
	EffectiveDepthMM  float64 `json:"effective_depth_mm"`
	RbMPa             float64 `json:"rb_mpa"`
	RsMPa             float64 `json:"rs_mpa"`
	BarDiameterMM     float64 `json:"bar_diameter_mm"`
	XiR               float64 `json:"xi_r"`
}

type Result struct {
	AsRequiredMM2PerM float64 `json:"as_required_mm2_per_m"`
	BarAreaMM2        float64 `json:"bar_area_mm2"`
	SpacingMM         float64 `json:"spacing_mm"`
	OK                bool    `json:"ok"`
	Notes             string  `json:"notes"`
}

func Calculate(in Input) (Result, error) {
	if in.MomentKNmPerM <= 0 || in.EffectiveDepthMM <= 0 || in.RbMPa <= 0 || in.RsMPa <= 0 || in.BarDiameterMM <= 0 {
		return Result{}, fmt.Errorf("invalid input")
	}
	if in.XiR <= 0 {
		in.XiR = 0.45
	}

	b := 1000.0 // 1 m strip
	h0 := in.EffectiveDepthMM
	Rb := in.RbMPa
	M := in.MomentKNmPerM * 1e6

	A := 0.5 * Rb * b
	B := -Rb * b * h0
	C := M
	D := B*B - 4*A*C
	if D < 0 {
		return Result{}, fmt.Errorf("no solution for compression zone")
	}
	x1 := (-B - math.Sqrt(D)) / (2 * A)
	x2 := (-B + math.Sqrt(D)) / (2 * A)
	x := x1
	if x <= 0 || x > h0 {
		x = x2
	}
	if x <= 0 || x > h0 {
		return Result{}, fmt.Errorf("invalid compression zone")
	}

	As := (Rb * b * x) / in.RsMPa
	barArea := math.Pi * in.BarDiameterMM * in.BarDiameterMM / 4.0
	spacing := barArea * 1000.0 / As
	ok := x <= in.XiR*h0
	return Result{
		AsRequiredMM2PerM: As,
		BarAreaMM2:        barArea,
		SpacingMM:         spacing,
		OK:                ok,
		Notes:             "RC slab flexure per SP63 simplified rectangular section.",
	}, nil
}
