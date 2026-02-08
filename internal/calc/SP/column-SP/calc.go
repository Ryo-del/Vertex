package column

import (
	"fmt"
)

type Input struct {
	WidthMM  float64 `json:"width_mm"`
	HeightMM float64 `json:"height_mm"`
	RbMPa    float64 `json:"rb_mpa"`
	RsMPa    float64 `json:"rs_mpa"`
	AsMM2    float64 `json:"as_mm2"`
	LoadKN   float64 `json:"load_kn"`
}

type Result struct {
	CapacityKN  float64 `json:"capacity_kn"`
	Utilization float64 `json:"utilization"`
	OK          bool    `json:"ok"`
	Notes       string  `json:"notes"`
}

func Calculate(in Input) (Result, error) {
	if in.WidthMM <= 0 || in.HeightMM <= 0 || in.RbMPa <= 0 || in.RsMPa <= 0 || in.LoadKN <= 0 {
		return Result{}, fmt.Errorf("invalid input")
	}
	if in.AsMM2 < 0 {
		in.AsMM2 = 0
	}
	A := in.WidthMM * in.HeightMM
	Ac := A - in.AsMM2
	if Ac < 0 {
		return Result{}, fmt.Errorf("invalid steel area")
	}
	// Axial capacity for centrally loaded column (simplified)
	capacity := (in.RbMPa*Ac + in.RsMPa*in.AsMM2) / 1000.0
	util := in.LoadKN / capacity
	return Result{
		CapacityKN:  capacity,
		Utilization: util,
		OK:          util <= 1.0,
		Notes:       "Simplified RC axial capacity per SP63.",
	}, nil
}
