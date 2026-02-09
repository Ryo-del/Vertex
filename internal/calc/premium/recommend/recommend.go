package recommend

import "fmt"

type WeldRecommendInput struct {
	ShearKN      float64 `json:"shear_kn"`
	WeldLengthMM float64 `json:"weld_length_mm"`
	FvwMPa       float64 `json:"fvw_mpa"`
	GammaM       float64 `json:"gamma_m"`
}

type WeldRecommendResult struct {
	RequiredSizeMM float64 `json:"required_size_mm"`
	Notes          string  `json:"notes"`
}

func WeldSize(in WeldRecommendInput) (WeldRecommendResult, error) {
	if in.ShearKN <= 0 || in.WeldLengthMM <= 0 {
		return WeldRecommendResult{}, fmt.Errorf("invalid input")
	}
	if in.FvwMPa <= 0 {
		in.FvwMPa = 180
	}
	if in.GammaM <= 0 {
		in.GammaM = 1.25
	}
	// V = 0.7*s*L*fvw/gamma /1000
	s := (in.ShearKN * 1000.0 * in.GammaM) / (0.7 * in.WeldLengthMM * in.FvwMPa)
	if s < 3 {
		s = 3
	}
	return WeldRecommendResult{
		RequiredSizeMM: s,
		Notes:          "Recommended fillet weld size for shear.",
	}, nil
}
