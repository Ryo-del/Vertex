package beam

import (
	"fmt"
	"math"
)

type Input struct {
	MomentKNM        float64 `json:"moment_knm"`
	WidthMM          float64 `json:"width_mm"`
	EffectiveDepthMM float64 `json:"effective_depth_mm"`
	RbMPa            float64 `json:"rb_mpa"`
	RsMPa            float64 `json:"rs_mpa"`
	XiR              float64 `json:"xi_r"`
}

type Result struct {
	CompressionZoneXMM float64 `json:"compression_zone_x_mm"`
	AsRequiredMM2      float64 `json:"as_required_mm2"`
	OK                bool    `json:"ok"`
	Notes             string  `json:"notes"`
}

// Simplified SP63 rectangular section design (single reinforcement).
func Calculate(in Input) (Result, error) {
	if in.MomentKNM <= 0 || in.WidthMM <= 0 || in.EffectiveDepthMM <= 0 || in.RbMPa <= 0 || in.RsMPa <= 0 {
		return Result{}, fmt.Errorf("invalid input")
	}
	if in.XiR <= 0 {
		in.XiR = 0.45
	}

	M := in.MomentKNM * 1e6 // N*mm
	b := in.WidthMM
	h0 := in.EffectiveDepthMM
	Rb := in.RbMPa

	// Solve: M = Rb*b*(h0*x - 0.5*x^2)
	// => 0.5*Rb*b*x^2 - Rb*b*h0*x + M = 0
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
	ok := x <= in.XiR*h0

	return Result{
		CompressionZoneXMM: x,
		AsRequiredMM2:      As,
		OK:                ok,
		Notes:             "RC beam flexure per SP63 simplified rectangular section.",
	}, nil
}
