// Package report renders the settlement result into structured output files
// (per-order detail CSV, store summary JSON, per-row error CSV) and writes
// them atomically so failed runs leave no half-finished files.
package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"

	"goldbar/internal/model"
)

// Output file names written to the output directory.
const (
	DetailFile  = "detail.csv"
	SummaryFile = "summary.json"
	ErrorFile   = "errors.csv"
)

// Build produces the complete set of output files as name->content. errors.csv
// is only included when there are row errors.
func Build(r *model.BatchResult) map[string][]byte {
	files := make(map[string][]byte, 3)
	files[DetailFile] = buildDetail(r.Settlements)
	files[SummaryFile] = buildSummary(r.Summary)
	if len(r.Errors) > 0 {
		files[ErrorFile] = buildErrors(r.Errors)
	}
	return files
}

func buildDetail(settlements []model.Settlement) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"order_id", "customer", "old_karat", "old_weight",
		"new_product_code", "new_weight", "gold_price",
		"old_discount", "new_weight_price", "craft_fee",
		"new_sale_price", "payable", "refund", "direction",
	})
	for _, s := range settlements {
		_ = w.Write([]string{
			s.OrderID,
			s.Customer,
			fmt.Sprintf("%d", s.OldKarat),
			fmt.Sprintf("%.3f", s.OldWeight),
			s.NewProductCode,
			fmt.Sprintf("%.3f", s.NewWeight),
			fmt.Sprintf("%.2f", s.GoldPrice),
			fmt.Sprintf("%.2f", s.OldDiscount),
			fmt.Sprintf("%.2f", s.NewWeightPrice),
			fmt.Sprintf("%.2f", s.CraftFee),
			fmt.Sprintf("%.2f", s.NewSalePrice),
			fmt.Sprintf("%.2f", s.Payable),
			fmt.Sprintf("%.2f", s.Refund),
			s.Direction(),
		})
	}
	w.Flush()
	return buf.Bytes()
}

func buildSummary(s model.Summary) []byte {
	data, _ := json.MarshalIndent(s, "", "  ")
	return data
}

func buildErrors(errs []model.LineError) []byte {
	sorted := make([]model.LineError, len(errs))
	copy(sorted, errs)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].LineNumber < sorted[j].LineNumber
	})
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"line_number", "order_id", "code", "message"})
	for _, e := range sorted {
		_ = w.Write([]string{
			fmt.Sprintf("%d", e.LineNumber),
			e.OrderID,
			e.Code,
			e.Message,
		})
	}
	w.Flush()
	return buf.Bytes()
}
