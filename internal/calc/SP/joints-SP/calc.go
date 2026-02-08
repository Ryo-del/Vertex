package joints

import (
	"fmt"
)

type Input struct {
	WeldSizeMM float64 `json:"weld_size_mm"`
	WeldLengthMM float64 `json:"weld_length_mm"`
	FvwMPa     float64 `json:"fvw_mpa"`
	GammaM     float64 `json:"gamma_m"`
	ShearKN    float64 `json:"shear_kn"`
}

type Result struct {
	CapacityKN  float64 `json:"capacity_kn"`
	Utilization float64 `json:"utilization"`
	OK          bool    `json:"ok"`
	Notes       string  `json:"notes"`
}

func Calculate(in Input) (Result, error) {
	if in.WeldSizeMM <= 0 || in.WeldLengthMM <= 0 || in.ShearKN <= 0 {
		return Result{}, fmt.Errorf("invalid input")
	}
	if in.FvwMPa <= 0 {
		in.FvwMPa = 180
	}
	if in.GammaM <= 0 {
		in.GammaM = 1.25
	}
	// Throat thickness a = 0.7 * s
	a := 0.7 * in.WeldSizeMM
	capacity := (a * in.WeldLengthMM * in.FvwMPa / in.GammaM) / 1000.0
	util := in.ShearKN / capacity
	return Result{
		CapacityKN:  capacity,
		Utilization: util,
		OK:          util <= 1.0,
		Notes:       "Placeholder. Weld design is not covered by SP63.",
	}, nil
}
