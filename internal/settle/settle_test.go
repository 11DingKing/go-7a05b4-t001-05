package settle

import (
	"context"
	"testing"

	"goldbar/internal/calc"
	"goldbar/internal/model"
)

func testConfig() model.Config {
	return model.Config{
		StoreID: "S", StoreName: "N", TradeDate: "2026-08-17",
		GoldPrice: 768.5, Currency: "CNY", CraftRatePerGram: 12,
		KaratTiers: []model.KaratTier{{Karat: 999, Rate: 0.98}, {Karat: 990, Rate: 0.95}, {Karat: 900, Rate: 0.90}, {Karat: 750, Rate: 0.80}},
	}
}

func mkOrder(line int, id string, karat int, oldW, newW, price float64) model.Order {
	return model.Order{LineNumber: line, OrderID: id, Customer: "C", OldKarat: karat, OldWeight: oldW, NewProductCode: "G", NewWeight: newW, GoldPrice: price}
}

func TestRunAllValid(t *testing.T) {
	cfg := testConfig()
	orders := []model.Order{
		mkOrder(2, "A1", 999, 2, 8, 768.5),
		mkOrder(3, "A2", 990, 3, 7, 768.5),
	}
	res, err := Run(context.Background(), cfg, orders, 2)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Settlements) != 2 {
		t.Fatalf("settlements = %d, want 2", len(res.Settlements))
	}
	if len(res.Errors) != 0 {
		t.Fatalf("errors = %v", res.Errors)
	}
	if res.Summary.ValidOrders != 2 {
		t.Errorf("valid = %d, want 2", res.Summary.ValidOrders)
	}
	if res.Summary.TotalOrders != 2 {
		t.Errorf("total = %d, want 2", res.Summary.TotalOrders)
	}
	// parallel must equal sequential (deterministic assembly)
	res2, _ := Run(context.Background(), cfg, orders, 1)
	if res2.Summary.TotalPayable != res.Summary.TotalPayable {
		t.Errorf("non-deterministic total: %v vs %v", res.Summary.TotalPayable, res2.Summary.TotalPayable)
	}
	if res2.Summary.TotalOldDiscount != res.Summary.TotalOldDiscount {
		t.Errorf("non-deterministic discount: %v vs %v", res.Summary.TotalOldDiscount, res2.Summary.TotalOldDiscount)
	}
}

func TestRunPartialErrors(t *testing.T) {
	cfg := testConfig()
	orders := []model.Order{
		mkOrder(2, "A1", 999, 2, 8, 768.5), // valid
		mkOrder(3, "A2", 750, 5, 5, 768.5), // karat too low -> error
		mkOrder(4, "A3", 999, 2, 8, 768.5), // valid
	}
	res, err := Run(context.Background(), cfg, orders, 2)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Settlements) != 2 {
		t.Fatalf("settlements = %d, want 2", len(res.Settlements))
	}
	if len(res.Errors) != 1 {
		t.Fatalf("errors = %d, want 1", len(res.Errors))
	}
	if res.Errors[0].Code != calc.CodeKaratTooLow {
		t.Errorf("code = %s, want %s", res.Errors[0].Code, calc.CodeKaratTooLow)
	}
	if res.Errors[0].LineNumber != 3 {
		t.Errorf("line = %d, want 3", res.Errors[0].LineNumber)
	}
	if res.Summary.ErrorOrders != 1 {
		t.Errorf("error orders = %d, want 1", res.Summary.ErrorOrders)
	}
	if res.Summary.ValidOrders != 2 {
		t.Errorf("valid = %d, want 2", res.Summary.ValidOrders)
	}
}

func TestRunPriceLockFatal(t *testing.T) {
	cfg := testConfig() // gold 768.5
	orders := []model.Order{
		mkOrder(2, "A1", 999, 2, 8, 768.5),
		mkOrder(3, "A2", 999, 2, 8, 700.0), // drift
	}
	res, err := Run(context.Background(), cfg, orders, 1)
	if err == nil {
		t.Fatal("expected price-lock error, got nil")
	}
	if res != nil {
		t.Fatal("expected nil result on fatal failure")
	}
}

func TestRunCancellation(t *testing.T) {
	cfg := testConfig()
	orders := make([]model.Order, 1000)
	for i := range orders {
		orders[i] = mkOrder(i+2, "A", 999, 2, 8, 768.5)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel
	res, err := Run(ctx, cfg, orders, 4)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if res != nil {
		t.Fatal("expected nil result on cancellation")
	}
}

func TestRunSummaryTotals(t *testing.T) {
	cfg := testConfig()
	orders := []model.Order{
		mkOrder(2, "A1", 999, 2, 8, 768.5),
		mkOrder(3, "A2", 999, 2, 8, 768.5),
	}
	res, _ := Run(context.Background(), cfg, orders, 1)
	if res.Summary.TotalOldDiscount != 3009.50 {
		t.Errorf("total_old_discount = %v, want 3009.50", res.Summary.TotalOldDiscount)
	}
	if res.Summary.TotalPayable != 9478.50 {
		t.Errorf("total_payable = %v, want 9478.50", res.Summary.TotalPayable)
	}
	if res.Summary.NetStoreReceivable != 9478.50 {
		t.Errorf("net = %v, want 9478.50", res.Summary.NetStoreReceivable)
	}
	if res.Summary.TotalRefund != 0 {
		t.Errorf("total_refund = %v, want 0", res.Summary.TotalRefund)
	}
}

func TestRunWorkersClamped(t *testing.T) {
	cfg := testConfig()
	orders := []model.Order{mkOrder(2, "A1", 999, 2, 8, 768.5)}
	// workers <= 0 must be clamped to a working value, not panic
	res, err := Run(context.Background(), cfg, orders, 0)
	if err != nil {
		t.Fatalf("Run with 0 workers: %v", err)
	}
	if len(res.Settlements) != 1 {
		t.Fatalf("settlements = %d, want 1", len(res.Settlements))
	}
	res2, err := Run(context.Background(), cfg, orders, -3)
	if err != nil {
		t.Fatalf("Run with -3 workers: %v", err)
	}
	if len(res2.Settlements) != 1 {
		t.Fatalf("settlements = %d, want 1", len(res2.Settlements))
	}
}
