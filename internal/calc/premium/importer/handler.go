package importer

import (
	"encoding/json"
	"fmt"
	"net/http"

	beam "Vertex/internal/calc/beam"
	"github.com/xuri/excelize/v2"
)

type Handler struct{}

type BeamImportResult struct {
	Count   int           `json:"count"`
	Results []beam.Result `json:"results"`
}

func (h *Handler) Beam(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	f, err := excelize.OpenReader(file)
	if err != nil {
		http.Error(w, "Invalid file", http.StatusBadRequest)
		return
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil || len(rows) < 2 {
		http.Error(w, "Empty sheet", http.StatusBadRequest)
		return
	}

	var results []beam.Result
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 4 {
			continue
		}
		input, err := parseBeamRow(row)
		if err != nil {
			continue
		}
		res, err := beam.Calculate(input)
		if err != nil {
			continue
		}
		results = append(results, res)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(BeamImportResult{Count: len(results), Results: results})
}

func parseBeamRow(row []string) (beam.Input, error) {
	// expected: material, span_m, udl_kn_m, width_m, height_m(optional), fy, e, defl
	if len(row) < 4 {
		return beam.Input{}, fmt.Errorf("bad row")
	}
	material := row[0]
	span, err := toFloat(row[1])
	if err != nil {
		return beam.Input{}, err
	}
	udl, err := toFloat(row[2])
	if err != nil {
		return beam.Input{}, err
	}
	width, err := toFloat(row[3])
	if err != nil {
		return beam.Input{}, err
	}
	height := 0.0
	if len(row) > 4 && row[4] != "" {
		height, _ = toFloat(row[4])
	}
	fy := 0.0
	if len(row) > 5 && row[5] != "" {
		fy, _ = toFloat(row[5])
	}
	e := 0.0
	if len(row) > 6 && row[6] != "" {
		e, _ = toFloat(row[6])
	}
	defl := 250.0
	if len(row) > 7 && row[7] != "" {
		defl, _ = toFloat(row[7])
	}
	return beam.Input{
		Material:             material,
		SpanM:                span,
		UDLKNM:               udl,
		WidthM:               width,
		HeightM:              height,
		FyMPa:                fy,
		E_GPa:                e,
		DeflectionLimitRatio: defl,
	}, nil
}

func toFloat(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	return v, err
}
