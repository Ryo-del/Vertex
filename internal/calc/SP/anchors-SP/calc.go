package anchors

import (
	"fmt"
	"math"
)

type Input struct {
	BoltDiameterMM float64 `json:"bolt_diameter_mm"`
	BoltCount      int     `json:"bolt_count"`
	FyMPa          float64 `json:"fy_mpa"`
	GammaM         float64 `json:"gamma_m"`
	TensionKN      float64 `json:"tension_kn"`
	ShearKN        float64 `json:"shear_kn"`
}

type Result struct {
	TensionCapacityKN float64 `json:"tension_capacity_kn"`
	ShearCapacityKN   float64 `json:"shear_capacity_kn"`
	Utilization       float64 `json:"utilization"`
	OK                bool    `json:"ok"`
	Notes             string  `json:"notes"`
}

func Calculate(in Input) (Result, error) {
	if in.BoltDiameterMM <= 0 || in.BoltCount <= 0 || in.FyMPa <= 0 {
		return Result{}, fmt.Errorf("invalid input")
	}
	if in.GammaM <= 0 {
		in.GammaM = 1.25
	}
	area := math.Pi * in.BoltDiameterMM * in.BoltDiameterMM / 4.0 // mm2
	n := float64(in.BoltCount)
	nrd := n * area * in.FyMPa / in.GammaM / 1000.0 // kN
	vrd := n * 0.6 * area * in.FyMPa / in.GammaM / 1000.0
	util := 0.0
	if nrd > 0 && vrd > 0 {
		util = math.Pow(in.TensionKN/nrd, 2) + math.Pow(in.ShearKN/vrd, 2)
	}
	return Result{
		TensionCapacityKN: nrd,
		ShearCapacityKN:   vrd,
		Utilization:       util,
		OK:                util <= 1.0,
		Notes:             "Placeholder. Anchors are not covered by SP63; concrete failure modes not included.",
	}, nil
}
