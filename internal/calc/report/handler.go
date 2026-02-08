package report

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/phpdave11/gofpdf"
)

type Input struct {
	Project string `json:"project"`
	Author  string `json:"author"`
	Title   string `json:"title"`
	Notes   string `json:"notes"`
}

type Handler struct{}

func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	var input Input
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	if input.Title == "" {
		input.Title = "Engineering Report"
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.Cell(0, 10, input.Title)
	pdf.Ln(12)
	pdf.SetFont("Helvetica", "", 11)
	pdf.Cell(0, 6, fmt.Sprintf("Project: %s", input.Project))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Author: %s", input.Author))
	pdf.Ln(6)
	pdf.Cell(0, 6, fmt.Sprintf("Date: %s", time.Now().Format("2006-01-02")))
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "", 11)
	pdf.MultiCell(0, 6, input.Notes, "", "L", false)

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=\"report.pdf\"")
	if err := pdf.Output(w); err != nil {
		http.Error(w, "Report generation error", http.StatusInternalServerError)
		return
	}
}
