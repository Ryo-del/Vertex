package batch

import (
	"fmt"

	beam "Vertex/internal/calc/beam"
)

type BeamBatchInput struct {
	Items []beam.Input `json:"items"`
}

type BeamBatchResult struct {
	Results []beam.Result `json:"results"`
}

func CalculateBeam(in BeamBatchInput) (BeamBatchResult, error) {
	if len(in.Items) == 0 {
		return BeamBatchResult{}, fmt.Errorf("no items")
	}
	out := BeamBatchResult{Results: make([]beam.Result, 0, len(in.Items))}
	for _, item := range in.Items {
		res, err := beam.Calculate(item)
		if err != nil {
			return BeamBatchResult{}, err
		}
		out.Results = append(out.Results, res)
	}
	return out, nil
}
