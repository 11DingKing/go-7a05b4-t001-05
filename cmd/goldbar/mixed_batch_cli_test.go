package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const mixedConfigJSON = `{
  "store_id": "SH001",
  "store_name": "上海旗舰店",
  "trade_date": "2026-08-17",
  "gold_price": 768.50,
  "karat_discount_rules": [
    {"karat": 999, "rate": 0.98},
    {"karat": 990, "rate": 0.95},
    {"karat": 900, "rate": 0.90},
    {"karat": 750, "rate": 0.80}
  ],
  "craft_rate_per_gram": 12.00,
  "currency": "CNY"
}
`

// mixedRecordsCSV has two settleable orders and one row that must be rejected
// by the row-level guards (karat below 900).
const mixedRecordsCSV = `order_id,customer,old_karat,old_weight,new_product_code,new_weight,gold_price
A001,张三,999,2.000,G001,8.000,768.50
A002,李四,990,3.500,G002,7.000,768.50
A003,王五,750,1.000,G003,5.000,768.50
`

func mixedFixture(t *testing.T) (cfgPath, recPath, outDir string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, "config.json")
	recPath = filepath.Join(dir, "records.csv")
	outDir = filepath.Join(dir, "out")
	if err := os.WriteFile(cfgPath, []byte(mixedConfigJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recPath, []byte(mixedRecordsCSV), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath, recPath, outDir
}

func readCSVRows(t *testing.T, path string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	rows, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return rows
}

// TestSettleMixedBatchDetailHasOnlySettledOrders runs the settle subcommand on a
// batch with one rejected row and asserts that detail.csv contains exactly the
// settleable orders and no blank filler row.
func TestSettleMixedBatchDetailHasOnlySettledOrders(t *testing.T) {
	cfgPath, recPath, outDir := mixedFixture(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"settle", "-config", cfgPath, "-input", recPath, "-outdir", outDir},
		strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("settle exit code = %d, want 2\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}

	rows := readCSVRows(t, filepath.Join(outDir, "detail.csv"))
	if len(rows) != 3 {
		t.Fatalf("detail.csv has %d row(s) %q, want 3 (header + A001 + A002)", len(rows), rows)
	}
	gotIDs := []string{rows[1][0], rows[2][0]}
	if gotIDs[0] != "A001" || gotIDs[1] != "A002" {
		t.Errorf("detail.csv order ids = %q, want [A001 A002]", gotIDs)
	}
	for i, row := range rows[1:] {
		if strings.TrimSpace(row[0]) == "" {
			t.Errorf("detail.csv data row %d has an empty order id: %q", i+1, row)
		}
	}

	errRows := readCSVRows(t, filepath.Join(outDir, "errors.csv"))
	if len(errRows) != 2 {
		t.Fatalf("errors.csv has %d row(s) %q, want 2 (header + A003)", len(errRows), errRows)
	}
	if errRows[1][1] != "A003" {
		t.Errorf("errors.csv order id = %q, want A003", errRows[1][1])
	}
}

// TestSettleMixedBatchSummaryCounts asserts the store daily summary counters and
// the console summary line for the same run.
func TestSettleMixedBatchSummaryCounts(t *testing.T) {
	cfgPath, recPath, outDir := mixedFixture(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"settle", "-config", cfgPath, "-input", recPath, "-outdir", outDir},
		strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("settle exit code = %d, want 2\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}

	raw, err := os.ReadFile(filepath.Join(outDir, "summary.json"))
	if err != nil {
		t.Fatalf("read summary.json: %v", err)
	}
	var summary struct {
		TotalOrders      int     `json:"total_orders"`
		ValidOrders      int     `json:"valid_orders"`
		ErrorOrders      int     `json:"error_orders"`
		TotalOldDiscount float64 `json:"total_old_discount"`
		TotalPayable     float64 `json:"total_payable"`
	}
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatalf("parse summary.json: %v", err)
	}
	if summary.TotalOrders != 3 {
		t.Errorf("total_orders = %d, want 3", summary.TotalOrders)
	}
	if summary.ValidOrders != 2 {
		t.Errorf("valid_orders = %d, want 2", summary.ValidOrders)
	}
	if summary.ErrorOrders != 1 {
		t.Errorf("error_orders = %d, want 1", summary.ErrorOrders)
	}
	if summary.ValidOrders+summary.ErrorOrders != summary.TotalOrders {
		t.Errorf("valid %d + error %d != total %d", summary.ValidOrders, summary.ErrorOrders, summary.TotalOrders)
	}
	if summary.TotalOldDiscount != 4034.46 {
		t.Errorf("total_old_discount = %v, want 4034.46", summary.TotalOldDiscount)
	}
	if summary.TotalPayable != 7673.04 {
		t.Errorf("total_payable = %v, want 7673.04", summary.TotalPayable)
	}
	if !strings.Contains(stdout.String(), "订单 3 (有效 2 / 异常 1)") {
		t.Errorf("console summary line missing from stdout:\n%s", stdout.String())
	}
}

// TestValidateMixedBatchReportsRejectedRow is the contrast case: the validate
// subcommand on the very same inputs.
func TestValidateMixedBatchReportsRejectedRow(t *testing.T) {
	cfgPath, recPath, _ := mixedFixture(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"validate", "-config", cfgPath, "-input", recPath},
		strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("validate exit code = %d, want 2\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "异常 第4行 A003") {
		t.Errorf("rejected row not reported on stdout:\n%s", stdout.String())
	}
}
