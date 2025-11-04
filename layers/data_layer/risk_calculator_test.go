package data_layer

import (
	"nofx/layers"
	"testing"
)

// TestRiskCalculator_CalculateRiskMetrics 测试风险计算
func TestRiskCalculator_CalculateRiskMetrics(t *testing.T) {
	config := layers.DataLayerConfig{
		MaxAccountRiskPercent:     2.0,
		MaxSingleTradeRiskPercent: 1.0,
		DefaultLeverage:           3,
		MaxLeverage:               5,
		CircuitBreakerEnabled:     true,
		MaxDailyLossPercent:       5.0,
		MaxConsecutiveLosses:      3,
	}

	calculator := NewRiskCalculator(config)
	calculator.UpdateAccountInfo(10000.0, 8000.0, 2000.0)

	marketData := &layers.CleanedMarketData{
		Symbol:       "BTCUSDT",
		CurrentPrice: 45000.0,
		EMA20:        44800.0,
		EMA50:        44200.0,
		RSI14:        65.0,
		ATR:          250.0,
	}

	// 测试做多
	metrics, err := calculator.CalculateRiskMetrics(layers.DirectionLong, marketData)
	if err != nil {
		t.Fatalf("CalculateRiskMetrics failed: %v", err)
	}

	if metrics.Symbol != "BTCUSDT" {
		t.Errorf("Symbol mismatch: got %s", metrics.Symbol)
	}

	if !metrics.CanTrade {
		t.Errorf("Should be able to trade: %s", metrics.RiskReason)
	}

	if metrics.RecommendedLeverage < 1 || metrics.RecommendedLeverage > 5 {
		t.Errorf("Invalid leverage: %d", metrics.RecommendedLeverage)
	}

	if metrics.MaxPositionSizeUSD <= 0 {
		t.Errorf("Invalid max position size: %.2f", metrics.MaxPositionSizeUSD)
	}

	if metrics.StopLossPrice >= marketData.CurrentPrice {
		t.Errorf("Long stop loss should be below current price: %.2f >= %.2f",
			metrics.StopLossPrice, marketData.CurrentPrice)
	}

	if metrics.TakeProfitPrice <= marketData.CurrentPrice {
		t.Errorf("Long take profit should be above current price: %.2f <= %.2f",
			metrics.TakeProfitPrice, marketData.CurrentPrice)
	}
}

// TestRiskCalculator_CircuitBreaker 测试熔断机制
func TestRiskCalculator_CircuitBreaker(t *testing.T) {
	config := layers.DataLayerConfig{
		CircuitBreakerEnabled: true,
		MaxDailyLossPercent:   5.0,
		MaxConsecutiveLosses:  3,
		DefaultLeverage:       3,
		MaxLeverage:           5,
	}

	calculator := NewRiskCalculator(config)
	calculator.UpdateAccountInfo(10000.0, 8000.0, 2000.0)

	// 测试日亏损熔断
	calculator.UpdateDailyPnL(-600.0) // 亏损6%

	marketData := &layers.CleanedMarketData{
		Symbol:       "BTCUSDT",
		CurrentPrice: 45000.0,
		ATR:          250.0,
	}

	metrics, _ := calculator.CalculateRiskMetrics(layers.DirectionLong, marketData)
	if metrics.CanTrade {
		t.Error("Should not be able to trade when circuit breaker is active")
	}

	// 重置熔断器
	calculator.ResetCircuitBreaker()
	metrics, _ = calculator.CalculateRiskMetrics(layers.DirectionLong, marketData)
	if !metrics.CanTrade {
		t.Error("Should be able to trade after circuit breaker reset")
	}

	// 测试连续亏损熔断
	calculator.RecordTradeResult(false) // 亏损1
	calculator.RecordTradeResult(false) // 亏损2
	calculator.RecordTradeResult(false) // 亏损3

	metrics, _ = calculator.CalculateRiskMetrics(layers.DirectionLong, marketData)
	if metrics.CanTrade {
		t.Error("Should not be able to trade after 3 consecutive losses")
	}
}

// TestRiskCalculator_ShortDirection 测试做空风险计算
func TestRiskCalculator_ShortDirection(t *testing.T) {
	config := layers.DataLayerConfig{
		DefaultLeverage: 3,
		MaxLeverage:     5,
	}

	calculator := NewRiskCalculator(config)
	calculator.UpdateAccountInfo(10000.0, 8000.0, 2000.0)

	marketData := &layers.CleanedMarketData{
		Symbol:       "BTCUSDT",
		CurrentPrice: 45000.0,
		ATR:          250.0,
	}

	metrics, err := calculator.CalculateRiskMetrics(layers.DirectionShort, marketData)
	if err != nil {
		t.Fatalf("CalculateRiskMetrics failed: %v", err)
	}

	if metrics.StopLossPrice <= marketData.CurrentPrice {
		t.Errorf("Short stop loss should be above current price: %.2f <= %.2f",
			metrics.StopLossPrice, marketData.CurrentPrice)
	}

	if metrics.TakeProfitPrice >= marketData.CurrentPrice {
		t.Errorf("Short take profit should be below current price: %.2f >= %.2f",
			metrics.TakeProfitPrice, marketData.CurrentPrice)
	}
}

// TestRiskCalculator_InsufficientBalance 测试余额不足
func TestRiskCalculator_InsufficientBalance(t *testing.T) {
	config := layers.DataLayerConfig{DefaultLeverage: 3, MaxLeverage: 5}
	calculator := NewRiskCalculator(config)
	calculator.UpdateAccountInfo(10000.0, 100.0, 9900.0) // 可用余额很少

	marketData := &layers.CleanedMarketData{
		Symbol:       "BTCUSDT",
		CurrentPrice: 45000.0,
		ATR:          250.0,
	}

	metrics, _ := calculator.CalculateRiskMetrics(layers.DirectionLong, marketData)
	if metrics.CanTrade {
		t.Error("Should not be able to trade with insufficient balance")
	}
}

// BenchmarkRiskCalculator_CalculateRiskMetrics 性能测试
func BenchmarkRiskCalculator_CalculateRiskMetrics(b *testing.B) {
	config := layers.DataLayerConfig{
		DefaultLeverage: 3,
		MaxLeverage:     5,
	}
	calculator := NewRiskCalculator(config)
	calculator.UpdateAccountInfo(10000.0, 8000.0, 2000.0)

	marketData := &layers.CleanedMarketData{
		Symbol:       "BTCUSDT",
		CurrentPrice: 45000.0,
		ATR:          250.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = calculator.CalculateRiskMetrics(layers.DirectionLong, marketData)
	}
}
