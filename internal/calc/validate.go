package calc

import (
	"fmt"

	"goldbar/internal/model"
)

// Guard codes for row-level business errors.
const (
	CodeKaratTooLow      = "KARAT_TOO_LOW"
	CodeNegativeWeight   = "NEGATIVE_WEIGHT"
	CodeDiscountOverSale = "DISCOUNT_OVER_SALE"
	CodeNoKaratTier      = "NO_KARAT_TIER"
)

// Validate applies the per-row business guards. A non-nil *LineError means the
// row must be excluded from the summary and reported with its source line
// number:
//   - 成色低于 900
//   - 克重为负 (旧金或新饰品)
//   - 旧金折现额超过新品售价
func Validate(cfg model.Config, o model.Order) *model.LineError {
	if o.OldKarat < 900 {
		return &model.LineError{
			LineNumber: o.LineNumber,
			OrderID:    o.OrderID,
			Code:       CodeKaratTooLow,
			Message:    fmt.Sprintf("成色 %d 低于 900", o.OldKarat),
		}
	}
	if o.OldWeight < 0 || o.NewWeight < 0 {
		return &model.LineError{
			LineNumber: o.LineNumber,
			OrderID:    o.OrderID,
			Code:       CodeNegativeWeight,
			Message:    fmt.Sprintf("克重为负: old_weight=%g new_weight=%g", o.OldWeight, o.NewWeight),
		}
	}
	if _, ok := DiscountRate(cfg.KaratTiers, o.OldKarat); !ok {
		return &model.LineError{
			LineNumber: o.LineNumber,
			OrderID:    o.OrderID,
			Code:       CodeNoKaratTier,
			Message:    fmt.Sprintf("成色 %d 无匹配折价档位", o.OldKarat),
		}
	}
	s, err := Compute(cfg, o)
	if err != nil {
		return &model.LineError{
			LineNumber: o.LineNumber,
			OrderID:    o.OrderID,
			Code:       CodeNoKaratTier,
			Message:    err.Error(),
		}
	}
	if s.OldDiscount > s.NewSalePrice {
		return &model.LineError{
			LineNumber: o.LineNumber,
			OrderID:    o.OrderID,
			Code:       CodeDiscountOverSale,
			Message:    fmt.Sprintf("旧金折现额 %.2f 超过新品售价 %.2f", s.OldDiscount, s.NewSalePrice),
		}
	}
	return nil
}
