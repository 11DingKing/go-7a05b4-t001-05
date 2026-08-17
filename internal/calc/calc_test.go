package calc

import (
	"testing"

	"goldbar/internal/model"
)

func sampleConfig() model.Config {
	return model.Config{
		StoreID: "S", StoreName: "N", TradeDate: "2026-08-17",
		GoldPrice: 768.5, Currency: "CNY", CraftRatePerGram: 12,
		KaratTiers: []model.KaratTier{
			{Karat: 999, Rate: 0.98}, {Karat: 990, Rate: 0.95}, {Karat: 900, Rate: 0.90}, {Karat: 750, Rate: 0.80},
		},
	}
}

func TestDiscountRateLookup(t *testing.T) {
	cfg := sampleConfig()
	cases := []struct {
		karat int
		rate  float64
		ok    bool
	}{
		{karat: 999, rate: 0.98, ok: true},
		{karat: 995, rate: 0.95, ok: true},
		{karat: 990, rate: 0.95, ok: true},
		{karat: 950, rate: 0.90, ok: true},
		{karat: 900, rate: 0.90, ok: true},
		{karat: 800, rate: 0.80, ok: true},
		{karat: 700, rate: 0, ok: false},
	}
	for _, c := range cases {
		r, ok := DiscountRate(cfg.KaratTiers, c.karat)
		if ok != c.ok || (ok && r != c.rate) {
			t.Errorf("karat %d: got (%v,%v) want (%v,%v)", c.karat, r, ok, c.rate, c.ok)
		}
	}
}

func TestComputePayable(t *testing.T) {
	cfg := sampleConfig()
	o := model.Order{LineNumber: 2, OrderID: "A1", Customer: "C", OldKarat: 999, OldWeight: 2, NewProductCode: "G1", NewWeight: 8, GoldPrice: 768.5}
	s, err := Compute(cfg, o)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if s.OldDiscount != 1504.75 {
		t.Errorf("old_discount = %v, want 1504.75", s.OldDiscount)
	}
	if s.NewWeightPrice != 6148.00 {
		t.Errorf("new_weight_price = %v, want 6148.00", s.NewWeightPrice)
	}
	if s.CraftFee != 96.00 {
		t.Errorf("craft_fee = %v, want 96.00", s.CraftFee)
	}
	if s.NewSalePrice != 6244.00 {
		t.Errorf("new_sale_price = %v, want 6244.00", s.NewSalePrice)
	}
	if s.Payable != 4739.25 {
		t.Errorf("payable = %v, want 4739.25", s.Payable)
	}
	if s.Refund != 0 {
		t.Errorf("refund = %v, want 0", s.Refund)
	}
	if s.Direction() != "补差" {
		t.Errorf("direction = %q, want 补差", s.Direction())
	}
}

func TestComputeRefund(t *testing.T) {
	cfg := sampleConfig()
	o := model.Order{LineNumber: 2, OrderID: "A2", Customer: "C", OldKarat: 999, OldWeight: 10, NewProductCode: "G1", NewWeight: 8, GoldPrice: 768.5}
	s, err := Compute(cfg, o)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if s.OldDiscount != 7523.77 {
		t.Errorf("old_discount = %v, want 7523.77", s.OldDiscount)
	}
	if s.Refund != 1279.77 {
		t.Errorf("refund = %v, want 1279.77", s.Refund)
	}
	if s.Payable != 0 {
		t.Errorf("payable = %v, want 0", s.Payable)
	}
	if s.Direction() != "退款" {
		t.Errorf("direction = %q, want 退款", s.Direction())
	}
}

func TestComputeNoKaratTier(t *testing.T) {
	cfg := sampleConfig()
	o := model.Order{LineNumber: 2, OrderID: "A3", OldKarat: 700, OldWeight: 5, NewWeight: 5, GoldPrice: 768.5}
	if _, err := Compute(cfg, o); err != ErrNoKaratTier {
		t.Fatalf("expected ErrNoKaratTier, got %v", err)
	}
}

func TestValidateGuards(t *testing.T) {
	cfg := sampleConfig()
	cases := []struct {
		name  string
		order model.Order
		code  string
	}{
		{"karat low", model.Order{LineNumber: 2, OrderID: "X", OldKarat: 750, OldWeight: 5, NewWeight: 5, GoldPrice: 768.5}, CodeKaratTooLow},
		{"neg old weight", model.Order{LineNumber: 3, OrderID: "X", OldKarat: 999, OldWeight: -1, NewWeight: 5, GoldPrice: 768.5}, CodeNegativeWeight},
		{"neg new weight", model.Order{LineNumber: 4, OrderID: "X", OldKarat: 999, OldWeight: 5, NewWeight: -2, GoldPrice: 768.5}, CodeNegativeWeight},
		{"discount over sale", model.Order{LineNumber: 5, OrderID: "X", OldKarat: 999, OldWeight: 10, NewWeight: 1, GoldPrice: 768.5}, CodeDiscountOverSale},
	}
	for _, c := range cases {
		e := Validate(cfg, c.order)
		if e == nil {
			t.Errorf("%s: expected error, got nil", c.name)
			continue
		}
		if e.Code != c.code {
			t.Errorf("%s: code = %s, want %s", c.name, e.Code, c.code)
		}
		if e.LineNumber != c.order.LineNumber {
			t.Errorf("%s: line = %d, want %d", c.name, e.LineNumber, c.order.LineNumber)
		}
	}
}

func TestValidateOK(t *testing.T) {
	cfg := sampleConfig()
	o := model.Order{LineNumber: 2, OrderID: "X", OldKarat: 999, OldWeight: 2, NewWeight: 8, GoldPrice: 768.5}
	if e := Validate(cfg, o); e != nil {
		t.Fatalf("unexpected error: %v", e)
	}
}
