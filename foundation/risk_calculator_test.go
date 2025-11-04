package foundation

import (
	"math"
	"testing"
)

// TestNewRiskCalculator 测试风险计算器创建
func TestNewRiskCalculator(t *testing.T) {
	tests := []struct {
		name              string
		accountEquity     float64
		maxRiskPercent    float64
		maxMarginPercent  float64
		expectedRiskPercent  float64
		expectedMarginPercent float64
	}{
		{
			name:                  "正常配置",
			accountEquity:         10000,
			maxRiskPercent:        2.0,
			maxMarginPercent:      90.0,
			expectedRiskPercent:   2.0,
			expectedMarginPercent: 90.0,
		},
		{
			name:                  "风险百分比过高，使用默认值",
			accountEquity:         10000,
			maxRiskPercent:        15.0,
			maxMarginPercent:      90.0,
			expectedRiskPercent:   2.0,
			expectedMarginPercent: 90.0,
		},
		{
			name:                  "保证金百分比过高，使用默认值",
			accountEquity:         10000,
			maxRiskPercent:        2.0,
			maxMarginPercent:      150.0,
			expectedRiskPercent:   2.0,
			expectedMarginPercent: 90.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := NewRiskCalculator(tt.accountEquity, tt.maxRiskPercent, tt.maxMarginPercent)
			if rc.accountEquity != tt.accountEquity {
				t.Errorf("accountEquity = %v, want %v", rc.accountEquity, tt.accountEquity)
			}
			if rc.maxRiskPercentPerTrade != tt.expectedRiskPercent {
				t.Errorf("maxRiskPercentPerTrade = %v, want %v", rc.maxRiskPercentPerTrade, tt.expectedRiskPercent)
			}
			if rc.maxMarginUsagePercent != tt.expectedMarginPercent {
				t.Errorf("maxMarginUsagePercent = %v, want %v", rc.maxMarginUsagePercent, tt.expectedMarginPercent)
			}
		})
	}
}

// TestCalculateStopLoss 测试止损计算
func TestCalculateStopLoss(t *testing.T) {
	rc := NewRiskCalculator(10000, 2.0, 90.0)

	tests := []struct {
		name          string
		direction     string
		entryPrice    float64
		atrValue      float64
		atrMultiplier float64
		expectError   bool
	}{
		{
			name:          "多单止损计算",
			direction:     "long",
			entryPrice:    100.0,
			atrValue:      2.0,
			atrMultiplier: 1.5,
			expectError:   false,
		},
		{
			name:          "空单止损计算",
			direction:     "short",
			entryPrice:    100.0,
			atrValue:      2.0,
			atrMultiplier: 1.5,
			expectError:   false,
		},
		{
			name:          "无效入场价",
			direction:     "long",
			entryPrice:    0,
			atrValue:      2.0,
			atrMultiplier: 1.5,
			expectError:   true,
		},
		{
			name:          "无效方向",
			direction:     "invalid",
			entryPrice:    100.0,
			atrValue:      2.0,
			atrMultiplier: 1.5,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rc.CalculateStopLoss(tt.direction, tt.entryPrice, tt.atrValue, tt.atrMultiplier)
			if tt.expectError {
				if err == nil {
					t.Errorf("期望错误但没有返回错误")
				}
				return
			}

			if err != nil {
				t.Errorf("意外错误: %v", err)
				return
			}

			// 验证止损价格
			if tt.direction == "long" {
				expectedStopPrice := tt.entryPrice - (tt.atrValue * tt.atrMultiplier)
				if math.Abs(result.Price-expectedStopPrice) > 0.01 {
					t.Errorf("多单止损价 = %v, want %v", result.Price, expectedStopPrice)
				}
				if result.Price >= tt.entryPrice {
					t.Errorf("多单止损价应该低于入场价")
				}
			} else if tt.direction == "short" {
				expectedStopPrice := tt.entryPrice + (tt.atrValue * tt.atrMultiplier)
				if math.Abs(result.Price-expectedStopPrice) > 0.01 {
					t.Errorf("空单止损价 = %v, want %v", result.Price, expectedStopPrice)
				}
				if result.Price <= tt.entryPrice {
					t.Errorf("空单止损价应该高于入场价")
				}
			}

			// 验证风险金额
			expectedRiskAmount := rc.accountEquity * rc.maxRiskPercentPerTrade / 100
			if math.Abs(result.RiskAmount-expectedRiskAmount) > 0.01 {
				t.Errorf("风险金额 = %v, want %v", result.RiskAmount, expectedRiskAmount)
			}
		})
	}
}

// TestCalculateTakeProfit 测试止盈计算
func TestCalculateTakeProfit(t *testing.T) {
	rc := NewRiskCalculator(10000, 2.0, 90.0)

	tests := []struct {
		name            string
		direction       string
		entryPrice      float64
		stopLossPrice   float64
		rewardRiskRatio float64
		expectError     bool
	}{
		{
			name:            "多单止盈计算",
			direction:       "long",
			entryPrice:      100.0,
			stopLossPrice:   97.0,
			rewardRiskRatio: 3.0,
			expectError:     false,
		},
		{
			name:            "空单止盈计算",
			direction:       "short",
			entryPrice:      100.0,
			stopLossPrice:   103.0,
			rewardRiskRatio: 3.0,
			expectError:     false,
		},
		{
			name:            "多单止损价无效",
			direction:       "long",
			entryPrice:      100.0,
			stopLossPrice:   105.0,
			rewardRiskRatio: 3.0,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rc.CalculateTakeProfit(tt.direction, tt.entryPrice, tt.stopLossPrice, tt.rewardRiskRatio)
			if tt.expectError {
				if err == nil {
					t.Errorf("期望错误但没有返回错误")
				}
				return
			}

			if err != nil {
				t.Errorf("意外错误: %v", err)
				return
			}

			// 验证止盈价格
			if tt.direction == "long" {
				riskDistance := tt.entryPrice - tt.stopLossPrice
				expectedTakeProfit := tt.entryPrice + (riskDistance * tt.rewardRiskRatio)
				if math.Abs(result-expectedTakeProfit) > 0.01 {
					t.Errorf("多单止盈价 = %v, want %v", result, expectedTakeProfit)
				}
			} else if tt.direction == "short" {
				riskDistance := tt.stopLossPrice - tt.entryPrice
				expectedTakeProfit := tt.entryPrice - (riskDistance * tt.rewardRiskRatio)
				if math.Abs(result-expectedTakeProfit) > 0.01 {
					t.Errorf("空单止盈价 = %v, want %v", result, expectedTakeProfit)
				}
			}
		})
	}
}

// TestCalculatePositionSize 测试仓位计算
func TestCalculatePositionSize(t *testing.T) {
	rc := NewRiskCalculator(10000, 2.0, 90.0)

	tests := []struct {
		name              string
		direction         string
		entryPrice        float64
		stopLossPrice     float64
		leverage          int
		currentMarginUsed float64
		confidence        float64
		expectError       bool
	}{
		{
			name:              "正常多单仓位计算",
			direction:         "long",
			entryPrice:        100.0,
			stopLossPrice:     97.0,
			leverage:          10,
			currentMarginUsed: 0,
			confidence:        0.85,
			expectError:       false,
		},
		{
			name:              "保证金不足",
			direction:         "long",
			entryPrice:        100.0,
			stopLossPrice:     97.0,
			leverage:          10,
			currentMarginUsed: 8900, // 已用89%
			confidence:        0.85,
			expectError:       true,
		},
		{
			name:              "杠杆无效",
			direction:         "long",
			entryPrice:        100.0,
			stopLossPrice:     97.0,
			leverage:          0,
			currentMarginUsed: 0,
			confidence:        0.85,
			expectError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rc.CalculatePositionSize(
				tt.direction,
				tt.entryPrice,
				tt.stopLossPrice,
				tt.leverage,
				tt.currentMarginUsed,
				tt.confidence,
			)

			if tt.expectError {
				if err == nil {
					t.Errorf("期望错误但没有返回错误")
				}
				return
			}

			if err != nil {
				t.Errorf("意外错误: %v", err)
				return
			}

			// 验证仓位大小为正
			if result.QuantityUSD <= 0 {
				t.Errorf("仓位大小应该为正: %v", result.QuantityUSD)
			}

			// 验证保证金在限制范围内
			totalMargin := tt.currentMarginUsed + result.MarginNeeded
			marginPercent := totalMargin / rc.accountEquity * 100
			if marginPercent > rc.maxMarginUsagePercent+0.01 {
				t.Errorf("保证金使用率超限: %.2f%% > %.2f%%", marginPercent, rc.maxMarginUsagePercent)
			}

			// 验证杠杆
			if result.Leverage != tt.leverage {
				t.Errorf("杠杆 = %v, want %v", result.Leverage, tt.leverage)
			}
		})
	}
}

// TestCalculateLiquidationPrice 测试强平价计算
func TestCalculateLiquidationPrice(t *testing.T) {
	rc := NewRiskCalculator(10000, 2.0, 90.0)

	tests := []struct {
		name                  string
		direction             string
		entryPrice            float64
		leverage              int
		maintenanceMarginRate float64
	}{
		{
			name:                  "多单强平价",
			direction:             "long",
			entryPrice:            100.0,
			leverage:              10,
			maintenanceMarginRate: 0.004,
		},
		{
			name:                  "空单强平价",
			direction:             "short",
			entryPrice:            100.0,
			leverage:              10,
			maintenanceMarginRate: 0.004,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rc.CalculateLiquidationPrice(tt.direction, tt.entryPrice, tt.leverage, tt.maintenanceMarginRate)
			if err != nil {
				t.Errorf("意外错误: %v", err)
				return
			}

			// 验证强平价
			if tt.direction == "long" {
				// 多单强平价应该低于入场价
				if result >= tt.entryPrice {
					t.Errorf("多单强平价应该低于入场价: %v >= %v", result, tt.entryPrice)
				}
			} else if tt.direction == "short" {
				// 空单强平价应该高于入场价
				if result <= tt.entryPrice {
					t.Errorf("空单强平价应该高于入场价: %v <= %v", result, tt.entryPrice)
				}
			}
		})
	}
}

// TestValidateMarginRequirement 测试保证金验证
func TestValidateMarginRequirement(t *testing.T) {
	rc := NewRiskCalculator(10000, 2.0, 90.0)

	tests := []struct {
		name              string
		currentMarginUsed float64
		requiredMargin    float64
		expectValid       bool
	}{
		{
			name:              "保证金充足",
			currentMarginUsed: 5000,
			requiredMargin:    1000,
			expectValid:       true,
		},
		{
			name:              "保证金刚好",
			currentMarginUsed: 8000,
			requiredMargin:    1000,
			expectValid:       true,
		},
		{
			name:              "保证金不足",
			currentMarginUsed: 8900,
			requiredMargin:    1000,
			expectValid:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, availableMargin := rc.ValidateMarginRequirement(tt.currentMarginUsed, tt.requiredMargin)
			if valid != tt.expectValid {
				t.Errorf("valid = %v, want %v", valid, tt.expectValid)
			}

			// 验证可用保证金计算
			maxAllowed := rc.accountEquity * rc.maxMarginUsagePercent / 100
			expectedAvailable := maxAllowed - tt.currentMarginUsed
			if math.Abs(availableMargin-expectedAvailable) > 0.01 {
				t.Errorf("availableMargin = %v, want %v", availableMargin, expectedAvailable)
			}
		})
	}
}

// TestGetMaxPositionValue 测试最大仓位价值计算
func TestGetMaxPositionValue(t *testing.T) {
	rc := NewRiskCalculator(10000, 2.0, 90.0)

	tests := []struct {
		name      string
		assetType string
		expectMin float64
		expectMax float64
	}{
		{
			name:      "BTC/ETH仓位限制",
			assetType: "btc_eth",
			expectMin: 50000,  // 5倍
			expectMax: 100000, // 10倍
		},
		{
			name:      "山寨币仓位限制",
			assetType: "altcoin",
			expectMin: 8000,  // 0.8倍
			expectMax: 15000, // 1.5倍
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minValue, maxValue := rc.GetMaxPositionValue(tt.assetType)
			if math.Abs(minValue-tt.expectMin) > 0.01 {
				t.Errorf("minValue = %v, want %v", minValue, tt.expectMin)
			}
			if math.Abs(maxValue-tt.expectMax) > 0.01 {
				t.Errorf("maxValue = %v, want %v", maxValue, tt.expectMax)
			}
		})
	}
}
