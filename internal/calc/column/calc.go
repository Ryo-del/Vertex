package column

import (
	"fmt"
	"math"
)

type Input struct {
	LengthM  float64 `json:"length_m"`
	KFactor  float64 `json:"k_factor"`
	WidthM   float64 `json:"width_m"`
	HeightM  float64 `json:"height_m"`
	E_GPa    float64 `json:"e_gpa"`
	LoadKN   float64 `json:"load_kn"`
}

type Result struct {
	IxxMM4     float64 `json:"ixx_mm4"`
	PcrKN      float64 `json:"pcr_kn"`
	Utilization float64 `json:"utilization"`
	OK         bool    `json:"ok"`
	Notes      string  `json:"notes"`
}

func Calculate(in Input) (Result, error) {
	if in.LengthM <= 0 || in.WidthM <= 0 || in.HeightM <= 0 || in.LoadKN <= 0 {
		return Result{}, fmt.Errorf("invalid input")
	}
	if in.KFactor <= 0 {
		in.KFactor = 1.0
	}
	if in.E_GPa <= 0 {
		in.E_GPa = 200
	}

	b := in.WidthM * 1000.0
	h := in.HeightM * 1000.0
	I := b * math.Pow(h, 3) / 12.0
	L := in.LengthM * 1000.0
	E := in.E_GPa * 1000.0
	pcr := (math.Pi * math.Pi * E * I) / math.Pow(in.KFactor*L, 2) / 1000.0 // kN
	util := in.LoadKN / pcr

	return Result{
		IxxMM4:     I,
		PcrKN:      pcr,
		Utilization: util,
		OK:         util <= 1.0,
		Notes:      "Euler buckling check for pinned column.",
	}, nil
}
