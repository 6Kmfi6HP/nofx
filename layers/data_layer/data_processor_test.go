package data_layer

import (
	"nofx/layers"
	"nofx/market"
	"testing"
)

// TestDataProcessor_ProcessMarketData 测试市场数据处理
func TestDataProcessor_ProcessMarketData(t *testing.T) {
	config := layers.DataLayerConfig{
		MinDataQuality: 0.8,
	}
	processor := NewDataProcessor(config)

	// 创建测试数据
	rawData := &market.Data{
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
		LongerTermContext: &market.LongerTermData{
			EMA50:         44200.0,
			ATR14:         250.0,
			CurrentVolume: 5000000.0,
			AverageVolume: 4500000.0,
		},
	}

	// 测试处理
	cleaned, err := processor.ProcessMarketData(rawData)
	if err != nil {
		t.Fatalf("ProcessMarketData failed: %v", err)
	}

	// 验证结果
	if cleaned.Symbol != "BTCUSDT" {
		t.Errorf("Symbol mismatch: got %s, want BTCUSDT", cleaned.Symbol)
	}

	if cleaned.CurrentPrice != 45000.0 {
		t.Errorf("Price mismatch: got %.2f, want 45000.0", cleaned.CurrentPrice)
	}

	if !cleaned.IsValid {
		t.Errorf("Data should be valid with quality %.2f", cleaned.DataQuality)
	}

	if len(cleaned.CompressedSummary) == 0 {
		t.Error("CompressedSummary should not be empty")
	}

	if len(cleaned.CompressedSummary) > 650 {
		t.Errorf("CompressedSummary too long: %d characters", len(cleaned.CompressedSummary))
	}
}

// TestDataProcessor_ProcessMarketData_NilInput 测试nil输入
func TestDataProcessor_ProcessMarketData_NilInput(t *testing.T) {
	config := layers.DataLayerConfig{MinDataQuality: 0.8}
	processor := NewDataProcessor(config)

	_, err := processor.ProcessMarketData(nil)
	if err == nil {
		t.Error("Expected error for nil input, got nil")
	}
}

// TestDataProcessor_ProcessMarketData_LowQuality 测试低质量数据
func TestDataProcessor_ProcessMarketData_LowQuality(t *testing.T) {
	config := layers.DataLayerConfig{MinDataQuality: 0.8}
	processor := NewDataProcessor(config)

	// 低质量数据（价格为0）
	rawData := &market.Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 0, // 无效价格
		CurrentRSI7:  -10, // 无效RSI
	}

	cleaned, err := processor.ProcessMarketData(rawData)
	if err != nil {
		t.Fatalf("ProcessMarketData failed: %v", err)
	}

	if cleaned.IsValid {
		t.Errorf("Data should be invalid with quality %.2f", cleaned.DataQuality)
	}

	if cleaned.DataQuality >= 0.8 {
		t.Errorf("Data quality should be low: got %.2f", cleaned.DataQuality)
	}
}

// TestDataProcessor_BatchProcessMarketData 测试批量处理
func TestDataProcessor_BatchProcessMarketData(t *testing.T) {
	config := layers.DataLayerConfig{MinDataQuality: 0.8}
	processor := NewDataProcessor(config)

	rawDataList := []*market.Data{
		{
			Symbol:       "BTCUSDT",
			CurrentPrice: 45000.0,
			CurrentEMA20: 44800.0,
			CurrentRSI7:  65.5,
		},
		{
			Symbol:       "ETHUSDT",
			CurrentPrice: 2500.0,
			CurrentEMA20: 2480.0,
			CurrentRSI7:  55.0,
		},
	}

	cleaned, err := processor.BatchProcessMarketData(rawDataList)
	if err != nil {
		t.Fatalf("BatchProcessMarketData failed: %v", err)
	}

	if len(cleaned) == 0 {
		t.Error("Expected at least one cleaned data")
	}
}

// BenchmarkDataProcessor_ProcessMarketData 性能测试
func BenchmarkDataProcessor_ProcessMarketData(b *testing.B) {
	config := layers.DataLayerConfig{MinDataQuality: 0.8}
	processor := NewDataProcessor(config)

	rawData := &market.Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 45000.0,
		CurrentEMA20: 44800.0,
		CurrentRSI7:  65.5,
		LongerTermContext: &market.LongerTermData{
			EMA50: 44200.0,
			ATR14: 250.0,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = processor.ProcessMarketData(rawData)
	}
}
