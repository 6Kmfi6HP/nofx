package layers

import (
	"nofx/market"
	"nofx/trader"
	"testing"
)

// MockTrader 模拟交易器，用于测试
type MockTrader struct {
	balance   map[string]interface{}
	positions []map[string]interface{}
}

func (m *MockTrader) GetBalance() (map[string]interface{}, error) {
	if m.balance == nil {
		m.balance = map[string]interface{}{
			"total":       10000.0,
			"available":   8000.0,
			"used_margin": 2000.0,
		}
	}
	return m.balance, nil
}

func (m *MockTrader) GetPositions() ([]map[string]interface{}, error) {
	return m.positions, nil
}

func (m *MockTrader) OpenLong(symbol string, quantity float64, leverage int) (string, error) {
	return "ORDER_123456", nil
}

func (m *MockTrader) OpenShort(symbol string, quantity float64, leverage int) (string, error) {
	return "ORDER_123457", nil
}

func (m *MockTrader) CloseLong(symbol string, quantity float64) (string, error) {
	return "ORDER_123458", nil
}

func (m *MockTrader) CloseShort(symbol string, quantity float64) (string, error) {
	return "ORDER_123459", nil
}

func (m *MockTrader) SetLeverage(symbol string, leverage int) error {
	return nil
}

func (m *MockTrader) SetStopLoss(symbol string, side string, price float64) error {
	return nil
}

func (m *MockTrader) SetTakeProfit(symbol string, side string, price float64) error {
	return nil
}

func (m *MockTrader) CancelAllOrders(symbol string) error {
	return nil
}

func (m *MockTrader) FormatQuantity(symbol string, quantity float64) (float64, error) {
	return quantity, nil
}

// getTestConfig 获取测试配置
func getTestConfig() LayerConfig {
	return LayerConfig{
		DataLayer: DataLayerConfig{
			MinDataQuality:            0.8,
			MaxAccountRiskPercent:     2.0,
			MaxSingleTradeRiskPercent: 1.0,
			DefaultLeverage:           3,
			MaxLeverage:               5,
			CircuitBreakerEnabled:     false, // 测试时关闭熔断
			MaxDailyLossPercent:       5.0,
			MaxConsecutiveLosses:      3,
		},
		AILayer: AILayerConfig{
			Provider:             "deepseek",
			Model:                "deepseek-chat",
			APIKey:               "test_key",
			MinConfidence:        0.75,
			EnableChainOfThought: false,
			MaxPromptLength:      650,
			MaxDecisionsPerHour:  100, // 测试时放宽限制
			CooldownMinutes:      0,
		},
		ExecutionLayer: ExecutionLayerConfig{
			EnableSecondaryRiskCheck: true,
			MaxSlippagePercent:       0.5,
			OrderTimeoutSeconds:      30,
			EnablePositionSizing:     true,
			PositionSizingMethod:     "fixed",
			DryRun:                   true, // 测试时使用模拟模式
			RequireManualConfirmation: false,
		},
	}
}

// getTestMarketData 获取测试市场数据
func getTestMarketData() *market.Data {
	return &market.Data{
		Symbol:        "BTCUSDT",
		CurrentPrice:  45000.0,
		PriceChange1h: 2.5,
		PriceChange4h: 5.8,
		CurrentEMA20:  44800.0,
		CurrentMACD:   0.0234,
		CurrentRSI7:   65.5,
		FundingRate:   0.0001,
		OpenInterest: &market.OIData{
			Latest:  1000000000.0,
			Average: 950000000.0,
		},
		IntradaySeries: &market.IntradayData{
			MidPrices:   []float64{44500, 44600, 44800, 45000},
			EMA20Values: []float64{44400, 44500, 44650, 44800},
			MACDValues:  []float64{0.01, 0.015, 0.020, 0.023},
			RSI7Values:  []float64{60, 62, 64, 65.5},
			RSI14Values: []float64{58, 59, 60, 62},
		},
		LongerTermContext: &market.LongerTermData{
			EMA20:         44800.0,
			EMA50:         44200.0,
			ATR3:          200.0,
			ATR14:         250.0,
			CurrentVolume: 5000000.0,
			AverageVolume: 4500000.0,
		},
	}
}

// TestOrchestrator_Creation 测试创建编排器
func TestOrchestrator_Creation(t *testing.T) {
	config := getTestConfig()
	mockTrader := &MockTrader{}

	orchestrator, err := NewOrchestrator(config, mockTrader)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	if orchestrator == nil {
		t.Fatal("Orchestrator should not be nil")
	}

	if orchestrator.dataProcessor == nil {
		t.Error("Data processor should not be nil")
	}

	if orchestrator.riskCalculator == nil {
		t.Error("Risk calculator should not be nil")
	}

	if orchestrator.decisionMaker == nil {
		t.Error("Decision maker should not be nil")
	}

	if orchestrator.paramCalculator == nil {
		t.Error("Parameter calculator should not be nil")
	}

	if orchestrator.riskValidator == nil {
		t.Error("Risk validator should not be nil")
	}

	if orchestrator.orderSender == nil {
		t.Error("Order sender should not be nil")
	}
}

// TestOrchestrator_ExecuteTradingCycle_DryRun 测试交易周期（模拟模式）
func TestOrchestrator_ExecuteTradingCycle_DryRun(t *testing.T) {
	config := getTestConfig()
	config.ExecutionLayer.DryRun = true // 确保是模拟模式
	mockTrader := &MockTrader{}

	orchestrator, err := NewOrchestrator(config, mockTrader)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}

	marketData := getTestMarketData()

	// 注意：因为没有真实的AI API，这个测试会使用规则引擎
	result, err := orchestrator.ExecuteTradingCycle(marketData)
	if err != nil {
		t.Logf("Trading cycle completed with error (expected in test): %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if result.Symbol != "BTCUSDT" {
		t.Errorf("Symbol mismatch: got %s, want BTCUSDT", result.Symbol)
	}

	// 验证数据处理
	if result.CleanedData == nil {
		t.Error("Cleaned data should not be nil")
	}

	if result.Duration == 0 {
		t.Error("Duration should not be zero")
	}
}

// TestOrchestrator_UpdateAccountInfo 测试更新账户信息
func TestOrchestrator_UpdateAccountInfo(t *testing.T) {
	config := getTestConfig()
	mockTrader := &MockTrader{}

	orchestrator, _ := NewOrchestrator(config, mockTrader)

	orchestrator.UpdateAccountInfo(10000.0, 8000.0, 2000.0)

	// 验证账户信息已更新
	stats := orchestrator.GetStats()
	accountRisk := stats["account_risk"].(map[string]interface{})

	if accountRisk["total_balance"].(float64) != 10000.0 {
		t.Errorf("Total balance mismatch: got %.2f", accountRisk["total_balance"].(float64))
	}
}

// TestOrchestrator_GetStats 测试获取统计信息
func TestOrchestrator_GetStats(t *testing.T) {
	config := getTestConfig()
	mockTrader := &MockTrader{}

	orchestrator, _ := NewOrchestrator(config, mockTrader)

	stats := orchestrator.GetStats()

	// 验证统计字段存在
	if _, ok := stats["total_executions"]; !ok {
		t.Error("Stats should contain total_executions")
	}

	if _, ok := stats["successful_trades"]; !ok {
		t.Error("Stats should contain successful_trades")
	}

	if _, ok := stats["failed_trades"]; !ok {
		t.Error("Stats should contain failed_trades")
	}

	if _, ok := stats["rejected_by_risk"]; !ok {
		t.Error("Stats should contain rejected_by_risk")
	}

	if _, ok := stats["win_rate"]; !ok {
		t.Error("Stats should contain win_rate")
	}
}

// TestOrchestrator_CircuitBreaker 测试熔断机制
func TestOrchestrator_CircuitBreaker(t *testing.T) {
	config := getTestConfig()
	config.DataLayer.CircuitBreakerEnabled = true
	mockTrader := &MockTrader{}

	orchestrator, _ := NewOrchestrator(config, mockTrader)
	orchestrator.UpdateAccountInfo(10000.0, 8000.0, 2000.0)

	// 触发日亏损熔断
	orchestrator.UpdateDailyPnL(-600.0) // 6%亏损

	stats := orchestrator.GetStats()
	circuitBreaker := stats["circuit_breaker"].(map[string]interface{})

	if !circuitBreaker["active"].(bool) {
		t.Error("Circuit breaker should be active after exceeding daily loss limit")
	}

	// 重置熔断器
	orchestrator.ResetCircuitBreaker()

	stats = orchestrator.GetStats()
	circuitBreaker = stats["circuit_breaker"].(map[string]interface{})

	if circuitBreaker["active"].(bool) {
		t.Error("Circuit breaker should be inactive after reset")
	}
}

// TestOrchestrator_RecordTradeResult 测试记录交易结果
func TestOrchestrator_RecordTradeResult(t *testing.T) {
	config := getTestConfig()
	mockTrader := &MockTrader{}

	orchestrator, _ := NewOrchestrator(config, mockTrader)

	// 记录盈利
	orchestrator.RecordTradeResult(true)

	stats := orchestrator.GetStats()
	circuitBreaker := stats["circuit_breaker"].(map[string]interface{})

	if circuitBreaker["consecutive_losses"].(int) != 0 {
		t.Error("Consecutive losses should be 0 after a win")
	}

	// 记录连续亏损
	orchestrator.RecordTradeResult(false)
	orchestrator.RecordTradeResult(false)

	stats = orchestrator.GetStats()
	circuitBreaker = stats["circuit_breaker"].(map[string]interface{})

	if circuitBreaker["consecutive_losses"].(int) != 2 {
		t.Errorf("Consecutive losses should be 2, got %d",
			circuitBreaker["consecutive_losses"].(int))
	}
}

// BenchmarkOrchestrator_ExecuteTradingCycle 性能测试
func BenchmarkOrchestrator_ExecuteTradingCycle(b *testing.B) {
	config := getTestConfig()
	config.ExecutionLayer.DryRun = true
	mockTrader := &MockTrader{}

	orchestrator, _ := NewOrchestrator(config, mockTrader)
	marketData := getTestMarketData()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = orchestrator.ExecuteTradingCycle(marketData)
	}
}
