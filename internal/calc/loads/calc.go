package loads

import "fmt"

type Method string

const (
	MethodSP24 Method = "SP24"
	MethodSP22 Method = "SP22"
	MethodEC7  Method = "EC7"
)

type Input struct {
	Method      Method  `json:"method"`
	LoadGKN     float64 `json:"load_g_kn"`
	LoadQLongKN float64 `json:"load_q_long_kn"`
	LoadQShortKN float64 `json:"load_q_short_kn"`
}

type Result struct {
	DesignLoadKN float64 `json:"design_load_kn"`
	ComboName    string  `json:"combo_name"`
	Notes        string  `json:"notes"`
}

func Calculate(in Input) (Result, error) {
	if in.LoadGKN <= 0 {
		return Result{}, fmt.Errorf("invalid permanent load")
	}
	gG, gQlong, gQshort, name := factors(in.Method)
	design := in.LoadGKN*gG + in.LoadQLongKN*gQlong + in.LoadQShortKN*gQshort
	return Result{
		DesignLoadKN: design,
		ComboName:    name,
		Notes:        "Simplified combination with one permanent and two variable loads.",
	}, nil
}

func factors(method Method) (gG, gQlong, gQshort float64, name string) {
	switch method {
	case MethodSP22:
		return 1.05, 1.2, 1.3, "SP22 basic"
	case MethodEC7:
		return 1.35, 1.5, 1.5, "EC7 STR/GEO"
	default:
		return 1.1, 1.2, 1.3, "SP24 basic"
	}
}
