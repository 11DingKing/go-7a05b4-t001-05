// Package calc implements the per-order valuation: karat discount lookup,
// old-gold discount amount, new-product weight price, craft fee and the
// resulting payable/refund split.
package calc

import (
	"errors"

	"goldbar/internal/model"
)

// ErrNoKaratTier is returned when no configured discount tier matches a karat.
var ErrNoKaratTier = errors.New("无匹配的成色折价档位")

// DiscountRate selects the discount rate for oldKarat by choosing the highest
// configured tier whose karat is <= oldKarat (tiers must be sorted desc). It
// returns false when no tier matches (e.g. karat below the lowest tier).
func DiscountRate(tiers []model.KaratTier, oldKarat int) (float64, bool) {
	for _, t := range tiers {
		if t.Karat <= oldKarat {
			return t.Rate, true
		}
	}
	return 0, false
}

// Compute valuates a single order against the store config using the locked
// daily gold price. It performs no business-guard checks; call Validate first
// to decide row eligibility. The returned error is non-nil only when no
// discount tier matches the order's karat.
func Compute(cfg model.Config, o model.Order) (model.Settlement, error) {
	rate, ok := DiscountRate(cfg.KaratTiers, o.OldKarat)
	if !ok {
		return model.Settlement{}, ErrNoKaratTier
	}
	purity := float64(o.OldKarat) / 1000.0
	oldDiscount := model.Round2(o.OldWeight * cfg.GoldPrice * purity * rate)
	newWeightPrice := model.Round2(o.NewWeight * cfg.GoldPrice)
	craftFee := model.Round2(o.NewWeight * cfg.CraftRatePerGram)
	newSalePrice := model.Round2(newWeightPrice + craftFee)
	net := model.Round2(newSalePrice - oldDiscount)

	payable, refund := 0.0, 0.0
	if net > 0 {
		payable = net
	} else if net < 0 {
		refund = -net
	}
	return model.Settlement{
		OrderID:        o.OrderID,
		Customer:       o.Customer,
		OldKarat:       o.OldKarat,
		OldWeight:      o.OldWeight,
		NewProductCode: o.NewProductCode,
		NewWeight:      o.NewWeight,
		GoldPrice:      cfg.GoldPrice,
		OldDiscount:    oldDiscount,
		NewWeightPrice: newWeightPrice,
		CraftFee:       craftFee,
		NewSalePrice:   newSalePrice,
		Payable:        payable,
		Refund:         refund,
	}, nil
}
