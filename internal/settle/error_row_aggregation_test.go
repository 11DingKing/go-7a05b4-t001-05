package settle

import (
	"context"
	"testing"

	"goldbar/internal/calc"
	"goldbar/internal/model"
)

// TestRunKeepsErrorRowsOutOfSettlements settles a mixed batch (two valid orders,
// two rows that must be rejected by the row-level guards) and asserts that the
// rejected rows appear only in the error report, never as settlement rows.
func TestRunKeepsErrorRowsOutOfSettlements(t *testing.T) {
	cfg := testConfig()
	orders := []model.Order{
		mkOrder(2, "A1", 999, 2, 8, 768.5),   // valid
		mkOrder(3, "A2", 750, 5, 5, 768.5),   // karat below 900
		mkOrder(4, "A3", 990, 3.5, 7, 768.5), // valid
		mkOrder(5, "A4", 999, -1, 5, 768.5),  // negative old weight
	}

	res, err := Run(context.Background(), cfg, orders, 2)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	gotIDs := make([]string, 0, len(res.Settlements))
	for _, s := range res.Settlements {
		gotIDs = append(gotIDs, s.OrderID)
	}
	want := []string{"A1", "A3"}
	if len(gotIDs) != len(want) {
		t.Fatalf("settlement order ids = %q, want %q", gotIDs, want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Errorf("settlement[%d] order id = %q, want %q", i, gotIDs[i], want[i])
		}
	}
	for i, s := range res.Settlements {
		if s.OrderID == "" || s.Customer == "" || s.OldKarat == 0 || s.NewWeight == 0 {
			t.Errorf("settlement[%d] is a blank row: %+v", i, s)
		}
	}

	if len(res.Errors) != 2 {
		t.Fatalf("errors = %d (%v), want 2", len(res.Errors), res.Errors)
	}
	if res.Errors[0].Code != calc.CodeKaratTooLow || res.Errors[0].LineNumber != 3 {
		t.Errorf("errors[0] = %+v, want KARAT_TOO_LOW on line 3", res.Errors[0])
	}
	if res.Errors[1].Code != calc.CodeNegativeWeight || res.Errors[1].LineNumber != 5 {
		t.Errorf("errors[1] = %+v, want NEGATIVE_WEIGHT on line 5", res.Errors[1])
	}
}

// TestRunSummaryCountsAddUpOnMixedBatch asserts the daily summary counters of
// the same mixed batch: valid + error must equal the total number of input rows.
func TestRunSummaryCountsAddUpOnMixedBatch(t *testing.T) {
	cfg := testConfig()
	orders := []model.Order{
		mkOrder(2, "A1", 999, 2, 8, 768.5),   // valid
		mkOrder(3, "A2", 750, 5, 5, 768.5),   // karat below 900
		mkOrder(4, "A3", 990, 3.5, 7, 768.5), // valid
		mkOrder(5, "A4", 999, -1, 5, 768.5),  // negative old weight
	}

	res, err := Run(context.Background(), cfg, orders, 3)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	s := res.Summary
	if s.TotalOrders != 4 {
		t.Errorf("total_orders = %d, want 4", s.TotalOrders)
	}
	if s.ValidOrders != 2 {
		t.Errorf("valid_orders = %d, want 2", s.ValidOrders)
	}
	if s.ErrorOrders != 2 {
		t.Errorf("error_orders = %d, want 2", s.ErrorOrders)
	}
	if s.ValidOrders+s.ErrorOrders != s.TotalOrders {
		t.Errorf("valid %d + error %d != total %d", s.ValidOrders, s.ErrorOrders, s.TotalOrders)
	}
	// 1504.75 + 2529.71 over the two valid orders only.
	if s.TotalOldDiscount != 4034.46 {
		t.Errorf("total_old_discount = %v, want 4034.46", s.TotalOldDiscount)
	}
	if s.TotalPayable != 7673.04 {
		t.Errorf("total_payable = %v, want 7673.04", s.TotalPayable)
	}
}

// TestRunFullyValidBatchUnaffected is the contrast case: a batch without any
// rejected row.
func TestRunFullyValidBatchUnaffected(t *testing.T) {
	cfg := testConfig()
	orders := []model.Order{
		mkOrder(2, "A1", 999, 2, 8, 768.5),
		mkOrder(3, "A3", 990, 3.5, 7, 768.5),
	}

	res, err := Run(context.Background(), cfg, orders, 2)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Settlements) != 2 {
		t.Fatalf("settlements = %d, want 2", len(res.Settlements))
	}
	if res.Summary.ValidOrders != 2 || res.Summary.ErrorOrders != 0 || res.Summary.TotalOrders != 2 {
		t.Errorf("summary counts = %d/%d/%d, want valid 2 / error 0 / total 2",
			res.Summary.ValidOrders, res.Summary.ErrorOrders, res.Summary.TotalOrders)
	}
}
