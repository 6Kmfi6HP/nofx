package execution_layer

import (
	"nofx/layers"
	"testing"
	"time"
)

// TestRiskValidator_ValidateExecution 测试风险验证
func TestRiskValidator_ValidateExecution(t *testing.T) {
	config := layers.ExecutionLayerConfig{
		EnableSecondaryRiskCheck: true,
	}
	validator := NewRiskValidator(config)

	// 准备测试数据
	marketData := &layers.CleanedMarketData{
		Symbol:       "BTCUSDT",
		CurrentPrice: 45000.0,
		DataQuality:  0.95,
		IsValid:      true,
		RSI14:        55.0,
		PriceChange1h: 2.5,
		ATR:          250.0,
	}

	riskMetrics := &layers.RiskMetrics{
		Symbol:              "BTCUSDT",
		CanTrade:            true,
		RiskLevel:           "low",
		MaxPositionSizeUSD:  500.0,
		RecommendedLeverage: 3,
		StopLossPrice:       44500.0,
		TakeProfitPrice:     46000.0,
	}

	aiDecision := &layers.AIDecision{
		Symbol:          "BTCUSDT",
		Direction:       layers.DirectionLong,
		Confidence:      0.85,
		MarketCondition: layers.MarketTrending,
		Opportunity:     layers.OpportunityLongEntry,
	}

	executionPlan := &layers.ExecutionPlan{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Quantity:        0.01,
		QuantityUSD:     450.0,
		Leverage:        3,
		StopLoss:        44500.0,
		TakeProfit:      46000.0,
		RiskCheckPassed: true,
	}

	// 测试验证通过
	passed, reason := validator.ValidateExecution(executionPlan, aiDecision, riskMetrics, marketData)
	if !passed {
		t.Errorf("Validation should pass, but failed: %s", reason)
	}

	stats := validator.GetValidationStats()
	if stats["total_validations"].(int) != 1 {
		t.Errorf("Total validations should be 1, got %d", stats["total_validations"])
	}
	if stats["passed_validations"].(int) != 1 {
		t.Errorf("Passed validations should be 1, got %d", stats["passed_validations"])
	}
}

// TestRiskValidator_InvalidAction 测试无效动作
func TestRiskValidator_InvalidAction(t *testing.T) {
	config := layers.ExecutionLayerConfig{EnableSecondaryRiskCheck: true}
	validator := NewRiskValidator(config)

	marketData := &layers.CleanedMarketData{
		Symbol:      "BTCUSDT",
		CurrentPrice: 45000.0,
		DataQuality: 0.95,
		IsValid:     true,
	}

	riskMetrics := &layers.RiskMetrics{
		Symbol:   "BTCUSDT",
		CanTrade: true,
	}

	aiDecision := &layers.AIDecision{
		Symbol:     "BTCUSDT",
		Direction:  layers.DirectionLong,
		Confidence: 0.85,
	}

	executionPlan := &layers.ExecutionPlan{
		Symbol:   "BTCUSDT",
		Action:   "invalid_action", // 无效动作
		Quantity: 0.01,
	}

	passed, reason := validator.ValidateExecution(executionPlan, aiDecision, riskMetrics, marketData)
	if passed {
		t.Error("Validation should fail for invalid action")
	}
	if len(reason) == 0 {
		t.Error("Should have failure reason")
	}
}

// TestRiskValidator_LowConfidence 测试低信心度
func TestRiskValidator_LowConfidence(t *testing.T) {
	config := layers.ExecutionLayerConfig{EnableSecondaryRiskCheck: true}
	validator := NewRiskValidator(config)

	marketData := &layers.CleanedMarketData{
		Symbol:      "BTCUSDT",
		CurrentPrice: 45000.0,
		DataQuality: 0.95,
		IsValid:     true,
		RSI14:       55.0,
	}

	riskMetrics := &layers.RiskMetrics{
		Symbol:              "BTCUSDT",
		CanTrade:            true,
		RiskLevel:           "low",
		RecommendedLeverage: 3,
		StopLossPrice:       44500.0,
	}

	aiDecision := &layers.AIDecision{
		Symbol:     "BTCUSDT",
		Direction:  layers.DirectionLong,
		Confidence: 0.6, // 低于0.7
	}

	executionPlan := &layers.ExecutionPlan{
		Symbol:   "BTCUSDT",
		Action:   "open_long",
		Quantity: 0.01,
		Leverage: 3,
		StopLoss: 44500.0,
	}

	passed, _ := validator.ValidateExecution(executionPlan, aiDecision, riskMetrics, marketData)
	if passed {
		t.Error("Validation should fail for low confidence")
	}
}

// TestRiskValidator_InvalidStopLoss 测试无效止损
func TestRiskValidator_InvalidStopLoss(t *testing.T) {
	config := layers.ExecutionLayerConfig{EnableSecondaryRiskCheck: true}
	validator := NewRiskValidator(config)

	marketData := &layers.CleanedMarketData{
		Symbol:       "BTCUSDT",
		CurrentPrice: 45000.0,
		DataQuality:  0.95,
		IsValid:      true,
		RSI14:        55.0,
	}

	riskMetrics := &layers.RiskMetrics{
		Symbol:              "BTCUSDT",
		CanTrade:            true,
		RiskLevel:           "low",
		RecommendedLeverage: 3,
	}

	aiDecision := &layers.AIDecision{
		Symbol:     "BTCUSDT",
		Direction:  layers.DirectionLong,
		Confidence: 0.85,
	}

	// 做多但止损价格高于当前价
	executionPlan := &layers.ExecutionPlan{
		Symbol:   "BTCUSDT",
		Action:   "open_long",
		Quantity: 0.01,
		Leverage: 3,
		StopLoss: 46000.0, // 错误：止损应该低于当前价
	}

	passed, _ := validator.ValidateExecution(executionPlan, aiDecision, riskMetrics, marketData)
	if passed {
		t.Error("Validation should fail for invalid stop loss")
	}
}

// TestRiskValidator_HighVolatility 测试高波动限制
func TestRiskValidator_HighVolatility(t *testing.T) {
	config := layers.ExecutionLayerConfig{EnableSecondaryRiskCheck: true}
	validator := NewRiskValidator(config)

	marketData := &layers.CleanedMarketData{
		Symbol:       "BTCUSDT",
		CurrentPrice: 45000.0,
		DataQuality:  0.95,
		IsValid:      true,
		RSI14:        55.0,
		ATR:          3000.0, // 高ATR，波动率>5%
	}

	riskMetrics := &layers.RiskMetrics{
		Symbol:              "BTCUSDT",
		CanTrade:            true,
		RiskLevel:           "low",
		RecommendedLeverage: 3,
		StopLossPrice:       44500.0,
	}

	aiDecision := &layers.AIDecision{
		Symbol:     "BTCUSDT",
		Direction:  layers.DirectionLong,
		Confidence: 0.85,
	}

	executionPlan := &layers.ExecutionPlan{
		Symbol:   "BTCUSDT",
		Action:   "open_long",
		Quantity: 0.01,
		Leverage: 5, // 高波动市场用高杠杆
		StopLoss: 44500.0,
	}

	passed, _ := validator.ValidateExecution(executionPlan, aiDecision, riskMetrics, marketData)
	if passed {
		t.Error("Validation should fail for high leverage in high volatility market")
	}
}

// BenchmarkRiskValidator_ValidateExecution 性能测试
func BenchmarkRiskValidator_ValidateExecution(b *testing.B) {
	config := layers.ExecutionLayerConfig{EnableSecondaryRiskCheck: true}
	validator := NewRiskValidator(config)

	marketData := &layers.CleanedMarketData{
		Symbol:       "BTCUSDT",
		CurrentPrice: 45000.0,
		DataQuality:  0.95,
		IsValid:      true,
		RSI14:        55.0,
	}

	riskMetrics := &layers.RiskMetrics{
		Symbol:              "BTCUSDT",
		CanTrade:            true,
		RiskLevel:           "low",
		RecommendedLeverage: 3,
		StopLossPrice:       44500.0,
	}

	aiDecision := &layers.AIDecision{
		Symbol:     "BTCUSDT",
		Direction:  layers.DirectionLong,
		Confidence: 0.85,
	}

	executionPlan := &layers.ExecutionPlan{
		Symbol:   "BTCUSDT",
		Action:   "open_long",
		Quantity: 0.01,
		Leverage: 3,
		StopLoss: 44500.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.ValidateExecution(executionPlan, aiDecision, riskMetrics, marketData)
	}
}
