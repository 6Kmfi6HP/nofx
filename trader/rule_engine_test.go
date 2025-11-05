package trader

import (
	"testing"
	"time"
)

// TestCheckAccountRisk 测试账户风险检查
func TestCheckAccountRisk(t *testing.T) {
	re := NewRuleEngine(10.0, 20.0, 90.0, 60*time.Minute)

	// 测试正常情况
	params := AccountRiskParams{
		InitialBalance:    10000,
		CurrentEquity:     9500,
		DailyPnL:          -300,
		TotalPnL:          -500,
		MarginUsedPercent: 50,
		PositionCount:     2,
	}

	result := re.CheckAccountRisk(params)
	if !result.Passed {
		t.Errorf("正常情况应该通过，违规: %v", result.ViolatedRules)
	}

	t.Logf("✓ 正常账户风险检查通过")

	// 测试日亏损超限
	params.DailyPnL = -1200 // 12% 亏损，超过 10%
	result = re.CheckAccountRisk(params)
	if result.Passed {
		t.Errorf("日亏损超限应该不通过")
	}
	if result.Severity != SeverityCritical {
		t.Errorf("日亏损超限应该是严重级别")
	}

	t.Logf("✓ 日亏损超限检测正常，违规: %v", result.ViolatedRules)

	// 测试回撤超限
	params.DailyPnL = 0
	params.TotalPnL = -2100 // 21% 回撤，超过 20%
	result = re.CheckAccountRisk(params)
	if result.Passed {
		t.Errorf("回撤超限应该不通过")
	}

	t.Logf("✓ 回撤超限检测正常，违规: %v", result.ViolatedRules)

	// 测试保证金超限
	params.TotalPnL = 0
	params.MarginUsedPercent = 95 // 95% 使用率，超过 90%
	result = re.CheckAccountRisk(params)
	if result.Passed {
		t.Errorf("保证金超限应该不通过")
	}

	t.Logf("✓ 保证金超限检测正常，违规: %v", result.ViolatedRules)

	// 测试持仓数量超限
	params.MarginUsedPercent = 50
	params.PositionCount = 4 // 4个持仓，超过 3个
	result = re.CheckAccountRisk(params)
	if result.Passed {
		t.Errorf("持仓数量超限应该不通过")
	}

	t.Logf("✓ 持仓数量超限检测正常，违规: %v", result.ViolatedRules)
}

// TestCheckOpenPositionRisk 测试开仓风险检查
func TestCheckOpenPositionRisk(t *testing.T) {
	re := NewRuleEngine(10.0, 20.0, 90.0, 60*time.Minute)

	// 测试正常开仓
	params := OpenPositionRiskParams{
		Symbol:             "BTCUSDT",
		Side:               "long",
		PositionSizeUSD:    50000,
		Leverage:           5,
		AccountEquity:      10000,
		CurrentPositions:   1,
		AvailableMargin:    8000,
		IsBTCOrETH:         true,
		MaxBTCETHLeverage:  5,
		MaxAltcoinLeverage: 3,
	}

	result := re.CheckOpenPositionRisk(params)
	if !result.Passed {
		t.Errorf("正常开仓应该通过，违规: %v", result.ViolatedRules)
	}

	t.Logf("✓ 正常开仓检查通过")

	// 测试持仓数量已满
	params.CurrentPositions = 3
	result = re.CheckOpenPositionRisk(params)
	if result.Passed {
		t.Errorf("持仓数量已满应该不通过")
	}

	t.Logf("✓ 持仓数量已满检测正常，违规: %v", result.ViolatedRules)

	// 测试杠杆超限
	params.CurrentPositions = 1
	params.Leverage = 10
	result = re.CheckOpenPositionRisk(params)
	if result.Passed {
		t.Errorf("杠杆超限应该不通过")
	}

	t.Logf("✓ 杠杆超限检测正常，违规: %v", result.ViolatedRules)

	// 测试仓位大小超限
	params.Leverage = 5
	params.PositionSizeUSD = 150000 // 15倍账户净值，超过 10倍
	result = re.CheckOpenPositionRisk(params)
	if result.Passed {
		t.Errorf("仓位大小超限应该不通过")
	}

	t.Logf("✓ 仓位大小超限检测正常，违规: %v", result.ViolatedRules)

	// 测试保证金不足
	params.PositionSizeUSD = 50000
	params.AvailableMargin = 5000 // 需要 10000，但只有 5000
	result = re.CheckOpenPositionRisk(params)
	if result.Passed {
		t.Errorf("保证金不足应该不通过")
	}
	if result.Severity != SeverityCritical {
		t.Errorf("保证金不足应该是严重级别")
	}

	t.Logf("✓ 保证金不足检测正常，违规: %v", result.ViolatedRules)
}

// TestCheckCircuitBreaker 测试熔断机制
func TestCheckCircuitBreaker(t *testing.T) {
	re := NewRuleEngine(10.0, 20.0, 90.0, 60*time.Minute)

	// 测试正常情况
	params := CircuitBreakerParams{
		RecentLossCount:     2,
		RecentLossThreshold: 5,
		QuickLossPercent:    3.0,
		QuickLossThreshold:  5.0,
	}

	result := re.CheckCircuitBreaker(params)
	if !result.Passed {
		t.Errorf("正常情况应该通过")
	}

	t.Logf("✓ 正常情况熔断检查通过")

	// 测试连续亏损触发熔断
	params.RecentLossCount = 5
	result = re.CheckCircuitBreaker(params)
	if result.Passed {
		t.Errorf("连续亏损应该触发熔断")
	}
	if !result.ShouldStop {
		t.Errorf("应该停止交易")
	}

	t.Logf("✓ 连续亏损熔断检测正常，违规: %v", result.ViolatedRules)

	// 测试快速亏损触发熔断
	params.RecentLossCount = 2
	params.QuickLossPercent = 6.0 // 超过 5%
	result = re.CheckCircuitBreaker(params)
	if result.Passed {
		t.Errorf("快速亏损应该触发熔断")
	}
	if !result.ShouldStop {
		t.Errorf("应该停止交易")
	}

	t.Logf("✓ 快速亏损熔断检测正常，违规: %v", result.ViolatedRules)
}
