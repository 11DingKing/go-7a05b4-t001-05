// Package settle orchestrates the batch settlement: gold-price locking,
// parallel per-order valuation, cancellation support and summary aggregation.
package settle

import (
	"context"
	"fmt"
	"sync"

	"goldbar/internal/calc"
	"goldbar/internal/model"
)

// Run performs the batch settlement over orders using cfg. workers controls
// parallelism (clamped to >= 1). The context enables cancellation: on cancel
// no result is returned. A non-nil error means a fatal batch failure (exit 1)
// such as a price-lock violation or cancellation. Partial row errors are
// returned inside BatchResult.Errors, not as a returned error.
func Run(ctx context.Context, cfg model.Config, orders []model.Order, workers int) (*model.BatchResult, error) {
	if workers < 1 {
		workers = 1
	}
	if err := checkPriceLock(cfg, orders); err != nil {
		return nil, err
	}

	n := len(orders)
	settlements := make([]model.Settlement, n)
	rowErrs := make([]*model.LineError, n)

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
loop:
	for i, o := range orders {
		select {
		case <-ctx.Done():
			break loop
		case sem <- struct{}{}:
		}
		if err := ctx.Err(); err != nil {
			wg.Wait()
			return nil, err
		}
		wg.Add(1)
		go func(idx int, ord model.Order) {
			defer wg.Done()
			defer func() { <-sem }()
			if e := calc.Validate(cfg, ord); e != nil {
				rowErrs[idx] = e
				return
			}
			s, err := calc.Compute(cfg, ord)
			if err != nil {
				rowErrs[idx] = &model.LineError{
					LineNumber: ord.LineNumber,
					OrderID:    ord.OrderID,
					Code:       calc.CodeNoKaratTier,
					Message:    err.Error(),
				}
				return
			}
			settlements[idx] = s
		}(i, o)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result := &model.BatchResult{Config: cfg}
	result.Summary = model.Summary{
		StoreID:     cfg.StoreID,
		StoreName:   cfg.StoreName,
		TradeDate:   cfg.TradeDate,
		GoldPrice:   cfg.GoldPrice,
		Currency:    cfg.Currency,
		TotalOrders: n,
	}
	for i := 0; i < n; i++ {
		if rowErrs[i] != nil {
			result.Errors = append(result.Errors, *rowErrs[i])
			continue
		}
		result.Settlements = append(result.Settlements, settlements[i])
		result.Summary.Add(settlements[i])
	}
	result.Summary.ErrorOrders = len(result.Errors)
	result.Summary.Finalize()
	return result, nil
}

// checkPriceLock enforces that every order uses the same gold price as the
// configured daily price, preventing cross-period price drift within a file.
func checkPriceLock(cfg model.Config, orders []model.Order) error {
	locked := model.Round2(cfg.GoldPrice)
	for _, o := range orders {
		if model.Round2(o.GoldPrice) != locked {
			return fmt.Errorf("金价锁定失败: 第 %d 行订单 %s 金价 %.4f 与当日锁定金价 %.4f 不一致",
				o.LineNumber, o.OrderID, o.GoldPrice, cfg.GoldPrice)
		}
	}
	return nil
}
