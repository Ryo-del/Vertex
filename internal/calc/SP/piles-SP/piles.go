package piles

import (
	"fmt"
	"math"
)

type Method string

const (
	MethodSP24 Method = "SP24"
	MethodSP22 Method = "SP22"
	MethodEC7  Method = "EC7"
)

type Layer struct {
	FromDepthM float64 `json:"from_depth_m"`
	ToDepthM   float64 `json:"to_depth_m"`
	SoilType   string  `json:"soil_type"`
	GammaKNM3  float64 `json:"gamma_kn_m3"`
	FiKPa      float64 `json:"fi_kpa"`
	Alpha      float64 `json:"alpha"`
}

type Input struct {
	Method      Method  `json:"method"`
	PileType    string  `json:"pile_type"`
	SideM       float64 `json:"side_m"`
	DiameterM   float64 `json:"diameter_m"`
	LengthM     float64 `json:"length_m"`
	ToeDepthM   float64 `json:"toe_depth_m"`
	BaseQbKPa   float64 `json:"base_qb_kpa"`
	LoadGKN     float64 `json:"load_g_kn"`
	LoadQLongKN float64 `json:"load_q_long_kn"`
	LoadQShortKN float64 `json:"load_q_short_kn"`
	Layers      []Layer `json:"layers"`
}

type Result struct {
	DesignLoadKN       float64 `json:"design_load_kn"`
	DesignResistanceKN float64 `json:"design_resistance_kn"`
	ShaftResistanceKN  float64 `json:"shaft_resistance_kn"`
	BaseResistanceKN   float64 `json:"base_resistance_kn"`
	PileCount          int     `json:"pile_count"`
	AverageLoadPerPile float64 `json:"average_load_per_pile"`
	MethodUsed         Method  `json:"method_used"`
	Notes              string  `json:"notes"`
}

func Calculate(input Input) (Result, error) {
	if input.LengthM <= 0 {
		return Result{}, fmt.Errorf("invalid pile length")
	}
	if input.PileType != "square" && input.PileType != "round" {
		return Result{}, fmt.Errorf("invalid pile type")
	}
	if input.PileType == "square" && input.SideM <= 0 {
		return Result{}, fmt.Errorf("invalid square side")
	}
	if input.PileType == "round" && input.DiameterM <= 0 {
		return Result{}, fmt.Errorf("invalid diameter")
	}
	if len(input.Layers) == 0 {
		return Result{}, fmt.Errorf("no layers provided")
	}

	perimeter := 0.0
	area := 0.0
	if input.PileType == "square" {
		perimeter = 4 * input.SideM
		area = input.SideM * input.SideM
	} else {
		perimeter = math.Pi * input.DiameterM
		area = math.Pi * input.DiameterM * input.DiameterM / 4
	}

	shaft := 0.0
	top := input.ToeDepthM - input.LengthM
	bottom := input.ToeDepthM
	for _, layer := range input.Layers {
		overlap := math.Min(layer.ToDepthM, bottom) - math.Max(layer.FromDepthM, top)
		if overlap <= 0 {
			continue
		}
		fi := layer.FiKPa
		alpha := layer.Alpha
		if alpha == 0 {
			alpha = 1
		}
		shaft += fi * alpha * perimeter * overlap
	}

	base := input.BaseQbKPa * area
	totalResistance := shaft + base

	gammaG, gammaQLong, gammaQShort, gammaR := factors(input.Method)
	designLoad := input.LoadGKN*gammaG + input.LoadQLongKN*gammaQLong + input.LoadQShortKN*gammaQShort
	designResistance := totalResistance / gammaR

	pileCount := 0
	avg := 0.0
	if designResistance > 0 {
		pileCount = int(math.Ceil(designLoad / designResistance))
		if pileCount < 1 {
			pileCount = 1
		}
		avg = designLoad / float64(pileCount)
	}

	return Result{
		DesignLoadKN:       designLoad,
		DesignResistanceKN: designResistance,
		ShaftResistanceKN:  shaft,
		BaseResistanceKN:   base,
		PileCount:          pileCount,
		AverageLoadPerPile: avg,
		MethodUsed:         input.Method,
		Notes:              "Placeholder. Pile foundations are not governed by SP63.",
	}, nil
}

func factors(method Method) (gammaG, gammaQLong, gammaQShort, gammaR float64) {
	switch method {
	case MethodSP22:
		return 1.05, 1.2, 1.3, 1.2
	case MethodEC7:
		return 1.35, 1.5, 1.5, 1.1
	default:
		return 1.1, 1.2, 1.3, 1.25
	}
}
