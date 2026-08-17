package config

import (
	"os"
	"path/filepath"
	"testing"

	"goldbar/internal/model"
)

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{
		"store_id": "SH001", "store_name": "上海旗舰店", "trade_date": "2026-08-17",
		"gold_price": 768.50,
		"karat_discount_rules": [{"karat":999,"rate":0.98},{"karat":990,"rate":0.95},{"karat":900,"rate":0.90},{"karat":750,"rate":0.80}],
		"craft_rate_per_gram": 12.00, "currency": "CNY"
	}`)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.StoreID != "SH001" {
		t.Errorf("store_id = %q", c.StoreID)
	}
	if c.GoldPrice != 768.5 {
		t.Errorf("gold_price = %v", c.GoldPrice)
	}
	if len(c.KaratTiers) != 4 {
		t.Fatalf("tiers = %d", len(c.KaratTiers))
	}
	if c.KaratTiers[0].Karat != 999 || c.KaratTiers[3].Karat != 750 {
		t.Errorf("tiers not sorted desc: %v", c.KaratTiers)
	}
	if c.Currency != "CNY" {
		t.Errorf("currency = %q", c.Currency)
	}
}

func TestLoadDefaultsCurrency(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeConfig(t, path, `{"store_id":"S1","store_name":"店","trade_date":"2026-08-17","gold_price":700,"karat_discount_rules":[{"karat":999,"rate":0.98}],"craft_rate_per_gram":10}`)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Currency != "CNY" {
		t.Errorf("default currency = %q", c.Currency)
	}
}

func TestLoadInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	writeConfig(t, path, `{"store_id":"","store_name":"店","trade_date":"x","gold_price":0,"karat_discount_rules":[],"craft_rate_per_gram":-1}`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
}

func TestLoadBadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	writeConfig(t, path, `{not json`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected JSON parse error, got nil")
	}
}

func TestValidateDuplicateKarat(t *testing.T) {
	c := model.Config{
		StoreID: "S", StoreName: "N", TradeDate: "D", GoldPrice: 700,
		Currency: "CNY", CraftRatePerGram: 10,
		KaratTiers: []model.KaratTier{{Karat: 999, Rate: 0.98}, {Karat: 999, Rate: 0.97}},
	}
	if err := Validate(c); err == nil {
		t.Fatal("expected duplicate karat error")
	}
}

func TestValidateBadKaratRate(t *testing.T) {
	c := model.Config{
		StoreID: "S", StoreName: "N", TradeDate: "D", GoldPrice: 700,
		Currency: "CNY", CraftRatePerGram: 10,
		KaratTiers: []model.KaratTier{{Karat: 999, Rate: 1.5}},
	}
	if err := Validate(c); err == nil {
		t.Fatal("expected bad rate error")
	}
}
