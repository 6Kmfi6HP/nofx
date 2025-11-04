package foundation

import (
	"testing"
	"time"
)

// TestNewRuleExecutor 测试规则执行器创建
func TestNewRuleExecutor(t *testing.T) {
	re := NewRuleExecutor(10000, 10.0, 20.0)

	if re.accountEquity != 10000 {
		t.Errorf("accountEquity = %v, want 10000", re.accountEquity)
	}
	if re.maxDailyLossPercent != 10.0 {
		t.Errorf("maxDailyLossPercent = %v, want 10.0", re.maxDailyLossPercent)
	}
	if re.maxDrawdownPercent != 20.0 {
		t.Errorf("maxDrawdownPercent = %v, want 20.0", re.maxDrawdownPercent)
	}
	if re.isTradingHalted {
		t.Errorf("交易不应该被暂停")
	}
}

// TestCheckTradingRules_Normal 测试正常情况下的交易规则
func TestCheckTradingRules_Normal(t *testing.T) {
	re := NewRuleExecutor(10000, 10.0, 20.0)

	// 正常情况：账户净值增长
	result := re.CheckTradingRules(10500)

	if !result.IsTradingAllowed {
		t.Errorf("交易应该被允许")
	}
	if len(result.Violations) > 0 {
		t.Errorf("不应该有违规项: %v", result.Violations)
	}
}

// TestCheckTradingRules_DailyLoss 测试日亏损限制
func TestCheckTradingRules_DailyLoss(t *testing.T) {
	re := NewRuleExecutor(10000, 10.0, 20.0)

	// 触发日亏损限制：亏损11%
	result := re.CheckTradingRules(8900)

	if result.IsTradingAllowed {
		t.Errorf("交易应该被暂停")
	}
	if len(result.Violations) == 0 {
		t.Errorf("应该有违规项")
	}
	if result.HaltReason == "" {
		t.Errorf("应该有暂停原因")
	}

	// 验证交易已暂停
	if !re.isTradingHalted {
		t.Errorf("交易应该被标记为暂停")
	}
}

// TestCheckTradingRules_Drawdown 测试最大回撤限制
func TestCheckTradingRules_Drawdown(t *testing.T) {
	re := NewRuleExecutor(10000, 10.0, 20.0)

	// 先让账户增长到12000（设置历史最高）
	re.CheckTradingRules(12000)

	// 然后回撤到9500（回撤超过20%）
	result := re.CheckTradingRules(9500)

	if result.IsTradingAllowed {
		t.Errorf("交易应该被暂停")
	}
	if len(result.Violations) == 0 {
		t.Errorf("应该有违规项")
	}
}

// TestCheckTradingRules_Warning 测试警告情况
func TestCheckTradingRules_Warning(t *testing.T) {
	re := NewRuleExecutor(10000, 10.0, 20.0)

	// 亏损8%（接近10%限制的80%）
	result := re.CheckTradingRules(9200)

	if !result.IsTradingAllowed {
		t.Errorf("交易应该被允许")
	}
	if len(result.Warnings) == 0 {
		t.Errorf("应该有警告")
	}
}

// TestCheckStopLossTrigger 测试止损触发
func TestCheckStopLossTrigger(t *testing.T) {
	re := NewRuleExecutor(10000, 10.0, 20.0)

	tests := []struct {
		name             string
		direction        string
		currentPrice     float64
		stopLossPrice    float64
		expectTrigger    bool
	}{
		{
			name:          "多单止损触发",
			direction:     "long",
			currentPrice:  96.0,
			stopLossPrice: 97.0,
			expectTrigger: true,
		},
		{
			name:          "多单止损未触发",
			direction:     "long",
			currentPrice:  98.0,
			stopLossPrice: 97.0,
			expectTrigger: false,
		},
		{
			name:          "空单止损触发",
			direction:     "short",
			currentPrice:  104.0,
			stopLossPrice: 103.0,
			expectTrigger: true,
		},
		{
			name:          "空单止损未触发",
			direction:     "short",
			currentPrice:  102.0,
			stopLossPrice: 103.0,
			expectTrigger: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger := re.CheckStopLossTrigger("BTCUSDT", tt.direction, tt.currentPrice, tt.stopLossPrice, 1.0)
			if trigger.ShouldTrigger != tt.expectTrigger {
				t.Errorf("ShouldTrigger = %v, want %v", trigger.ShouldTrigger, tt.expectTrigger)
			}
		})
	}
}

// TestCheckTakeProfitTrigger 测试止盈触发
func TestCheckTakeProfitTrigger(t *testing.T) {
	re := NewRuleExecutor(10000, 10.0, 20.0)

	tests := []struct {
		name             string
		direction        string
		currentPrice     float64
		takeProfitPrice  float64
		expectTrigger    bool
	}{
		{
			name:            "多单止盈触发",
			direction:       "long",
			currentPrice:    110.0,
			takeProfitPrice: 109.0,
			expectTrigger:   true,
		},
		{
			name:            "多单止盈未触发",
			direction:       "long",
			currentPrice:    108.0,
			takeProfitPrice: 109.0,
			expectTrigger:   false,
		},
		{
			name:            "空单止盈触发",
			direction:       "short",
			currentPrice:    90.0,
			takeProfitPrice: 91.0,
			expectTrigger:   true,
		},
		{
			name:            "空单止盈未触发",
			direction:       "short",
			currentPrice:    92.0,
			takeProfitPrice: 91.0,
			expectTrigger:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger := re.CheckTakeProfitTrigger("BTCUSDT", tt.direction, tt.currentPrice, tt.takeProfitPrice, 1.0)
			if trigger.ShouldTrigger != tt.expectTrigger {
				t.Errorf("ShouldTrigger = %v, want %v", trigger.ShouldTrigger, tt.expectTrigger)
			}
		})
	}
}

// TestCheckPositionLimit 测试持仓数量限制
func TestCheckPositionLimit(t *testing.T) {
	re := NewRuleExecutor(10000, 10.0, 20.0)

	tests := []struct {
		name                 string
		currentPositionCount int
		maxPositionCount     int
		expectWithinLimit    bool
	}{
		{
			name:                 "持仓正常",
			currentPositionCount: 2,
			maxPositionCount:     3,
			expectWithinLimit:    true,
		},
		{
			name:                 "持仓达到上限",
			currentPositionCount: 3,
			maxPositionCount:     3,
			expectWithinLimit:    false,
		},
		{
			name:                 "持仓超过上限",
			currentPositionCount: 4,
			maxPositionCount:     3,
			expectWithinLimit:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check := re.CheckPositionLimit(tt.currentPositionCount, tt.maxPositionCount)
			if check.IsWithinLimit != tt.expectWithinLimit {
				t.Errorf("IsWithinLimit = %v, want %v", check.IsWithinLimit, tt.expectWithinLimit)
			}
		})
	}
}

// TestValidateLeverage 测试杠杆验证
func TestValidateLeverage(t *testing.T) {
	re := NewRuleExecutor(10000, 10.0, 20.0)

	tests := []struct {
		name              string
		requestedLeverage int
		assetType         string
		expectValid       bool
		expectAdjusted    int
	}{
		{
			name:              "BTC正常杠杆",
			requestedLeverage: 25,
			assetType:         "btc_eth",
			expectValid:       true,
			expectAdjusted:    25,
		},
		{
			name:              "BTC杠杆过高",
			requestedLeverage: 100,
			assetType:         "btc_eth",
			expectValid:       false,
			expectAdjusted:    50,
		},
		{
			name:              "山寨币正常杠杆",
			requestedLeverage: 15,
			assetType:         "altcoin",
			expectValid:       true,
			expectAdjusted:    15,
		},
		{
			name:              "山寨币杠杆过高",
			requestedLeverage: 50,
			assetType:         "altcoin",
			expectValid:       false,
			expectAdjusted:    20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validation := re.ValidateLeverage(tt.requestedLeverage, tt.assetType)
			if validation.IsValid != tt.expectValid {
				t.Errorf("IsValid = %v, want %v", validation.IsValid, tt.expectValid)
			}
			if validation.AdjustedLeverage != tt.expectAdjusted {
				t.Errorf("AdjustedLeverage = %v, want %v", validation.AdjustedLeverage, tt.expectAdjusted)
			}
		})
	}
}

// TestCheckTrailingStop 测试移动止损
func TestCheckTrailingStop(t *testing.T) {
	re := NewRuleExecutor(10000, 10.0, 20.0)

	config := &TrailingStopConfig{
		ActivationProfitPercent: 2.0,
		TrailingPercent:         1.0,
	}

	tests := []struct {
		name             string
		direction        string
		entryPrice       float64
		currentPrice     float64
		currentStopLoss  float64
		highestPrice     float64
		expectActivated  bool
		expectShouldUpdate bool
	}{
		{
			name:               "多单未激活",
			direction:          "long",
			entryPrice:         100.0,
			currentPrice:       101.0,
			currentStopLoss:    97.0,
			highestPrice:       101.0,
			expectActivated:    false,
			expectShouldUpdate: false,
		},
		{
			name:               "多单已激活并更新止损",
			direction:          "long",
			entryPrice:         100.0,
			currentPrice:       103.0,
			currentStopLoss:    97.0,
			highestPrice:       103.0,
			expectActivated:    true,
			expectShouldUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger := re.CheckTrailingStop(
				tt.direction,
				tt.entryPrice,
				tt.currentPrice,
				tt.currentStopLoss,
				tt.highestPrice,
				config,
			)

			if trigger.IsActivated != tt.expectActivated {
				t.Errorf("IsActivated = %v, want %v", trigger.IsActivated, tt.expectActivated)
			}
			if trigger.ShouldUpdate != tt.expectShouldUpdate {
				t.Errorf("ShouldUpdate = %v, want %v", trigger.ShouldUpdate, tt.expectShouldUpdate)
			}
		})
	}
}

// TestManualHaltAndResume 测试手动暂停和恢复
func TestManualHaltAndResume(t *testing.T) {
	re := NewRuleExecutor(10000, 10.0, 20.0)

	// 手动暂停
	re.ManualHaltTrading("测试暂停", 1*time.Minute)

	result := re.CheckTradingRules(10000)
	if result.IsTradingAllowed {
		t.Errorf("交易应该被暂停")
	}

	// 手动恢复
	re.ManualResumeTrading()

	result = re.CheckTradingRules(10000)
	if !result.IsTradingAllowed {
		t.Errorf("交易应该被允许")
	}
}

// TestGetStatus 测试获取状态
func TestGetStatus(t *testing.T) {
	re := NewRuleExecutor(10000, 10.0, 20.0)

	// 触发日亏损
	re.CheckTradingRules(8900)

	status := re.GetStatus()

	if !status["is_trading_halted"].(bool) {
		t.Errorf("状态应该显示交易已暂停")
	}

	if status["account_equity"].(float64) != 8900 {
		t.Errorf("账户净值 = %v, want 8900", status["account_equity"])
	}
}
