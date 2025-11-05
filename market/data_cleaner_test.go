package market

import (
	"testing"
)

// TestValidateMarketData 测试市场数据验证
func TestValidateMarketData(t *testing.T) {
	dc := NewDataCleaner()

	// 测试有效数据
	validData := &Data{
		Symbol:         "BTCUSDT",
		CurrentPrice:   50000,
		PriceChange1h:  2.5,
		PriceChange4h:  5.0,
		CurrentEMA20:   49500,
		CurrentMACD:    100,
		CurrentRSI7:    65,
		OpenInterest:   &OIData{Latest: 1000000, Average: 950000},
		FundingRate:    0.0001,
		IntradaySeries: &IntradayData{MidPrices: []float64{50000, 50100}},
		LongerTermContext: &LongerTermData{
			EMA20: 49500,
			EMA50: 49000,
			ATR3:  200,
			ATR14: 500,
		},
	}

	result := dc.ValidateMarketData(validData)
	if !result.IsValid {
		t.Errorf("有效数据应该通过验证，错误: %v", result.Errors)
	}

	t.Logf("✓ 有效数据验证通过")

	// 测试价格为零
	invalidData := &Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 0, // 无效价格
	}

	result = dc.ValidateMarketData(invalidData)
	if result.IsValid {
		t.Errorf("价格为零应该验证失败")
	}
	if len(result.Errors) == 0 {
		t.Errorf("应该有错误信息")
	}

	t.Logf("✓ 无效价格检测正常，错误: %v", result.Errors)

	// 测试 RSI 超出范围
	outOfRangeData := &Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 50000,
		CurrentRSI7:  150, // RSI 应该在 0-100
	}

	result = dc.ValidateMarketData(outOfRangeData)
	if len(result.Warnings) == 0 {
		t.Errorf("RSI 超出范围应该有警告")
	}

	t.Logf("✓ RSI 超出范围检测正常，警告: %v", result.Warnings)
}

// TestCleanMarketData 测试数据清洗
func TestCleanMarketData(t *testing.T) {
	dc := NewDataCleaner()

	// 测试 RSI 修正
	data := &Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 50000,
		CurrentRSI7:  150, // 超出范围
		OpenInterest: &OIData{Latest: -100, Average: 1000}, // 负值
	}

	cleanedData := dc.CleanMarketData(data)

	if cleanedData.CurrentRSI7 != 100 {
		t.Errorf("RSI 应该被修正为 100，实际: %.2f", cleanedData.CurrentRSI7)
	}

	if cleanedData.OpenInterest.Latest != 0 {
		t.Errorf("持仓量负值应该被修正为 0，实际: %.2f", cleanedData.OpenInterest.Latest)
	}

	t.Logf("✓ 数据清洗功能正常: RSI %.2f → %.2f, OI %.2f → %.2f",
		data.CurrentRSI7, cleanedData.CurrentRSI7,
		data.OpenInterest.Latest, cleanedData.OpenInterest.Latest)
}

// TestValidateAndClean 测试组合验证和清洗
func TestValidateAndClean(t *testing.T) {
	dc := NewDataCleaner()

	// 测试有效数据
	data := &Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 50000,
		CurrentEMA20: 49500,
		CurrentMACD:  100,
		CurrentRSI7:  150, // 会被清洗
		OpenInterest: &OIData{Latest: 1000000, Average: 950000},
	}

	cleanedData, validation, err := dc.ValidateAndClean(data)
	if err != nil {
		t.Fatalf("验证和清洗失败: %v", err)
	}

	if !validation.IsValid {
		t.Errorf("数据应该有效，错误: %v", validation.Errors)
	}

	if cleanedData.CurrentRSI7 != 100 {
		t.Errorf("RSI 应该被修正为 100，实际: %.2f", cleanedData.CurrentRSI7)
	}

	t.Logf("✓ 组合验证和清洗正常，警告数: %d", len(validation.Warnings))

	// 测试无效数据
	invalidData := &Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 0, // 无效
	}

	_, _, err = dc.ValidateAndClean(invalidData)
	if err == nil {
		t.Errorf("无效数据应该返回错误")
	}

	t.Logf("✓ 无效数据检测正常")
}

// TestCheckLiquidity 测试流动性检查
func TestCheckLiquidity(t *testing.T) {
	dc := NewDataCleaner()

	// 测试流动性充足
	data := &Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 50000,
		OpenInterest: &OIData{Latest: 1000000, Average: 950000}, // 50B USD
	}

	isValid, oiValueMillion := dc.CheckLiquidity(data, 15.0)
	if !isValid {
		t.Errorf("流动性应该充足，实际: %.2f M", oiValueMillion)
	}

	t.Logf("✓ 流动性检查通过: %.2f M USD (≥ 15M)", oiValueMillion)

	// 测试流动性不足
	data.OpenInterest.Latest = 100 // 仅 5000 USD
	isValid, oiValueMillion = dc.CheckLiquidity(data, 15.0)
	if isValid {
		t.Errorf("流动性应该不足，实际: %.2f M", oiValueMillion)
	}

	t.Logf("✓ 流动性不足检测正常: %.2f M USD (< 15M)", oiValueMillion)
}
