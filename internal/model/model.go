// Package model defines the core domain types for the gold trade-in
// settlement tool: input orders, store configuration, per-row errors,
// per-order settlements and the store daily summary.
package model

import (
	"fmt"
	"math"
)

// Order is a single trade-in record parsed from the POS export file.
type Order struct {
	LineNumber     int     // 1-based source line (header is line 1)
	OrderID        string  // 订单号
	Customer       string  // 客户/分户
	OldKarat       int     // 旧金成色 (千分比, 例如 999)
	OldWeight      float64 // 旧金克重 (克)
	NewProductCode string  // 新饰品编码
	NewWeight      float64 // 新饰品克重 (克)
	GoldPrice      float64 // 记录中的金价 (元/克), 必须与配置锁定一致
}

// KaratTier maps a karat fineness to its post-tax-policy discount rate.
type KaratTier struct {
	Karat int     `json:"karat"`
	Rate  float64 `json:"rate"` // 折价率, (0, 1]
}

// Config holds the store's daily settlement parameters.
type Config struct {
	StoreID          string      `json:"store_id"`
	StoreName        string      `json:"store_name"`
	TradeDate        string      `json:"trade_date"`
	GoldPrice        float64     `json:"gold_price"`           // 当日金价 (元/克)
	KaratTiers       []KaratTier `json:"karat_discount_rules"` // 成色折价规则, 按成色降序
	CraftRatePerGram float64     `json:"craft_rate_per_gram"`  // 工艺费 (元/克)
	Currency         string      `json:"currency"`
}

// Settlement is the computed result for a valid order.
type Settlement struct {
	OrderID        string  `json:"order_id"`
	Customer       string  `json:"customer"`
	OldKarat       int     `json:"old_karat"`
	OldWeight      float64 `json:"old_weight"`
	NewProductCode string  `json:"new_product_code"`
	NewWeight      float64 `json:"new_weight"`
	GoldPrice      float64 `json:"gold_price"`

	OldDiscount    float64 `json:"old_discount"`     // 旧金折现额
	NewWeightPrice float64 `json:"new_weight_price"` // 新饰品克重价
	CraftFee       float64 `json:"craft_fee"`        // 工艺费
	NewSalePrice   float64 `json:"new_sale_price"`   // 新品售价 (克重价 + 工艺费)
	Payable        float64 `json:"payable"`          // 应补差价
	Refund         float64 `json:"refund"`           // 应退金额
}

// Direction returns a human-readable label for the settlement outcome.
func (s Settlement) Direction() string {
	switch {
	case s.Payable > 0:
		return "补差"
	case s.Refund > 0:
		return "退款"
	default:
		return "平账"
	}
}

// LineError is a per-row validation or calculation error. Rows with a non-nil
// LineError are excluded from the summary and reported with their line number.
type LineError struct {
	LineNumber int    `json:"line_number"`
	OrderID    string `json:"order_id"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

// Error implements the error interface.
func (e LineError) Error() string {
	return fmt.Sprintf("第 %d 行 订单 %s [%s]: %s", e.LineNumber, e.OrderID, e.Code, e.Message)
}

// Summary is the store daily settlement aggregate over all valid orders.
type Summary struct {
	StoreID            string  `json:"store_id"`
	StoreName          string  `json:"store_name"`
	TradeDate          string  `json:"trade_date"`
	GoldPrice          float64 `json:"gold_price"`
	Currency           string  `json:"currency"`
	TotalOrders        int     `json:"total_orders"`
	ValidOrders        int     `json:"valid_orders"`
	ErrorOrders        int     `json:"error_orders"`
	TotalOldDiscount   float64 `json:"total_old_discount"`
	TotalNewSalePrice  float64 `json:"total_new_sale_price"`
	TotalCraftFee      float64 `json:"total_craft_fee"`
	TotalPayable       float64 `json:"total_payable"`
	TotalRefund        float64 `json:"total_refund"`
	NetStoreReceivable float64 `json:"net_store_receivable"`
}

// Add accumulates a settlement into the running summary totals.
func (s *Summary) Add(set Settlement) {
	s.ValidOrders++
	s.TotalOldDiscount += set.OldDiscount
	s.TotalNewSalePrice += set.NewSalePrice
	s.TotalCraftFee += set.CraftFee
	s.TotalPayable += set.Payable
	s.TotalRefund += set.Refund
}

// Finalize rounds all summary totals to cents and derives the net receivable.
func (s *Summary) Finalize() {
	s.TotalOldDiscount = Round2(s.TotalOldDiscount)
	s.TotalNewSalePrice = Round2(s.TotalNewSalePrice)
	s.TotalCraftFee = Round2(s.TotalCraftFee)
	s.TotalPayable = Round2(s.TotalPayable)
	s.TotalRefund = Round2(s.TotalRefund)
	s.NetStoreReceivable = Round2(s.TotalPayable - s.TotalRefund)
}

// BatchResult bundles the full outcome of a settlement run.
type BatchResult struct {
	Config      Config
	Settlements []Settlement
	Errors      []LineError
	Summary     Summary
}

// Round2 rounds a monetary value to two decimal places (cents).
func Round2(v float64) float64 {
	return math.Round(v*100) / 100
}
