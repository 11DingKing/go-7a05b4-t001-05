package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goldbar/internal/model"
)

func sampleResult() *model.BatchResult {
	cfg := model.Config{
		StoreID: "S", StoreName: "N", TradeDate: "2026-08-17", GoldPrice: 768.5,
		Currency: "CNY", CraftRatePerGram: 12,
		KaratTiers: []model.KaratTier{{Karat: 999, Rate: 0.98}, {Karat: 990, Rate: 0.95}, {Karat: 900, Rate: 0.90}, {Karat: 750, Rate: 0.80}},
	}
	return &model.BatchResult{Config: cfg}
}

func validSettlement() model.Settlement {
	return model.Settlement{
		OrderID: "A1", Customer: "C", OldKarat: 999, OldWeight: 2,
		NewProductCode: "G", NewWeight: 8, GoldPrice: 768.5,
		OldDiscount: 1504.75, NewWeightPrice: 6148, CraftFee: 96,
		NewSalePrice: 6244, Payable: 4739.25, Refund: 0,
	}
}

func TestBuildFilesWithErrors(t *testing.T) {
	r := sampleResult()
	r.Settlements = []model.Settlement{validSettlement()}
	r.Errors = []model.LineError{{LineNumber: 5, OrderID: "A2", Code: "KARAT_TOO_LOW", Message: "成色 750 低于 900"}}
	files := Build(r)
	if len(files) != 3 {
		t.Fatalf("files = %d, want 3", len(files))
	}
	if _, ok := files[DetailFile]; !ok {
		t.Error("missing detail.csv")
	}
	if _, ok := files[SummaryFile]; !ok {
		t.Error("missing summary.json")
	}
	if _, ok := files[ErrorFile]; !ok {
		t.Error("missing errors.csv")
	}
	if detail, _ := files[DetailFile]; !strings.Contains(string(detail), "补差") {
		t.Errorf("detail missing direction: %s", detail)
	}
}

func TestBuildNoErrorsOmitsErrorFile(t *testing.T) {
	r := sampleResult()
	r.Settlements = []model.Settlement{validSettlement()}
	files := Build(r)
	if _, ok := files[ErrorFile]; ok {
		t.Error("errors.csv should be omitted when no errors")
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
}

func TestWriteAllIdempotent(t *testing.T) {
	r := sampleResult()
	r.Settlements = []model.Settlement{validSettlement()}
	r.Summary = model.Summary{
		StoreID: "S", StoreName: "N", TradeDate: "2026-08-17", GoldPrice: 768.5,
		Currency: "CNY", TotalOrders: 1, ValidOrders: 1,
		TotalPayable: 4739.25, NetStoreReceivable: 4739.25,
	}
	dir := t.TempDir()
	if err := WriteAll(dir, r); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(dir, DetailFile))
	if err != nil {
		t.Fatalf("read detail: %v", err)
	}
	if err := WriteAll(dir, r); err != nil {
		t.Fatalf("WriteAll 2: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(dir, DetailFile))
	if err != nil {
		t.Fatalf("read detail 2: %v", err)
	}
	if string(first) != string(second) {
		t.Error("detail.csv is not idempotent across runs")
	}
	if _, err := os.Stat(filepath.Join(dir, ErrorFile)); !os.IsNotExist(err) {
		t.Error("stale errors.csv should be removed when there are no errors")
	}
}

func TestWriteAllWithErrors(t *testing.T) {
	r := sampleResult()
	r.Settlements = []model.Settlement{validSettlement()}
	r.Errors = []model.LineError{{LineNumber: 5, OrderID: "A2", Code: "KARAT_TOO_LOW", Message: "成色 750 低于 900"}}
	r.Summary = model.Summary{
		StoreID: "S", StoreName: "N", TradeDate: "x", GoldPrice: 768.5, Currency: "CNY",
		TotalOrders: 2, ValidOrders: 1, ErrorOrders: 1,
		TotalPayable: 4739.25, NetStoreReceivable: 4739.25,
	}
	dir := t.TempDir()
	if err := WriteAll(dir, r); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ErrorFile)); err != nil {
		t.Errorf("errors.csv should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, DetailFile)); err != nil {
		t.Errorf("detail.csv should exist: %v", err)
	}
}

func TestCommitFilesStageFailureLeavesNoFinal(t *testing.T) {
	dir := t.TempDir()
	// Pre-create the temp path as a directory so staging OpenFile fails with
	// EISDIR. This simulates a write failure regardless of user privileges.
	tmpPath := filepath.Join(dir, "."+DetailFile+".swp")
	if err := os.Mkdir(tmpPath, 0o755); err != nil {
		t.Fatal(err)
	}
	err := CommitFiles(dir, map[string][]byte{DetailFile: []byte("data")})
	if err == nil {
		t.Fatal("expected staging failure, got nil")
	}
	// Phase 1 failed before any rename, so no final file must exist.
	if _, err := os.Stat(filepath.Join(dir, DetailFile)); !os.IsNotExist(err) {
		t.Error("final detail.csv must not exist when staging fails")
	}
}

func TestCommitFilesPathIsFileFails(t *testing.T) {
	dir := t.TempDir()
	fileAsDir := filepath.Join(dir, "notadir")
	if err := os.WriteFile(fileAsDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CommitFiles(fileAsDir, map[string][]byte{DetailFile: []byte("data")})
	if err == nil {
		t.Fatal("expected MkdirAll failure when target path is a file")
	}
}
