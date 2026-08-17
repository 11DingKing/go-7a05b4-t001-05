// Package config loads and validates the store settlement parameters.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"goldbar/internal/model"
)

// Load reads, parses and validates the store configuration file.
func Load(path string) (model.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Config{}, fmt.Errorf("读取配置文件 %q 失败: %w", path, err)
	}
	return Parse(data)
}

// Parse decodes JSON configuration, applies defaults and validates it.
func Parse(data []byte) (model.Config, error) {
	var c model.Config
	if err := json.Unmarshal(data, &c); err != nil {
		return model.Config{}, fmt.Errorf("解析配置 JSON 失败: %w", err)
	}
	if c.Currency == "" {
		c.Currency = "CNY"
	}
	if err := Validate(c); err != nil {
		return model.Config{}, err
	}
	sort.Slice(c.KaratTiers, func(i, j int) bool {
		return c.KaratTiers[i].Karat > c.KaratTiers[j].Karat
	})
	return c, nil
}

// Validate checks that all configuration parameters are usable for settlement.
func Validate(c model.Config) error {
	if c.StoreID == "" {
		return fmt.Errorf("store_id 不能为空")
	}
	if c.StoreName == "" {
		return fmt.Errorf("store_name 不能为空")
	}
	if c.TradeDate == "" {
		return fmt.Errorf("trade_date 不能为空")
	}
	if c.GoldPrice <= 0 {
		return fmt.Errorf("gold_price 必须大于 0, 当前 %v", c.GoldPrice)
	}
	if c.CraftRatePerGram < 0 {
		return fmt.Errorf("craft_rate_per_gram 不能为负, 当前 %v", c.CraftRatePerGram)
	}
	if c.Currency == "" {
		return fmt.Errorf("currency 不能为空")
	}
	if len(c.KaratTiers) == 0 {
		return fmt.Errorf("karat_discount_rules 不能为空")
	}
	seen := make(map[int]bool, len(c.KaratTiers))
	for _, t := range c.KaratTiers {
		if t.Karat <= 0 || t.Karat > 1000 {
			return fmt.Errorf("成色 %d 必须在 (0, 1000] 范围内", t.Karat)
		}
		if t.Rate <= 0 || t.Rate > 1 {
			return fmt.Errorf("成色 %d 的折价率 %v 必须在 (0, 1] 范围内", t.Karat, t.Rate)
		}
		if seen[t.Karat] {
			return fmt.Errorf("成色 %d 重复定义", t.Karat)
		}
		seen[t.Karat] = true
	}
	return nil
}
