package trader

import (
	"testing"
)

// TestCalculatePositionSize 测试仓位大小计算
func TestCalculatePositionSize(t *testing.T) {
	rc := NewRiskCalculator()

	params := PositionSizeParams{
		AccountEquity:  10000,  // 10000 USDT
		RiskPercentage: 2.0,    // 2% 风险
		EntryPrice:     50000,  // 入场价 50000
		StopLossPrice:  49000,  // 止损价 49000
		Leverage:       5,      // 5x 杠杆
	}

	result, err := rc.CalculatePositionSize(params)
	if err != nil {
		t.Fatalf("计算仓位大小失败: %v", err)
	}

	// 验证结果
	if result.PositionSizeUSD <= 0 {
		t.Errorf("仓位大小应大于0，实际: %.2f", result.PositionSizeUSD)
	}

	if result.MarginRequired <= 0 {
		t.Errorf("所需保证金应大于0，实际: %.2f", result.MarginRequired)
	}

	if result.RiskUSD != 200 { // 10000 * 2% = 200
		t.Errorf("风险金额错误，期望: 200，实际: %.2f", result.RiskUSD)
	}

	t.Logf("✓ 仓位大小: %.2f USD, 保证金: %.2f USD, 风险: %.2f USD",
		result.PositionSizeUSD, result.MarginRequired, result.RiskUSD)
}

// TestCalculateStopLoss 测试止损计算
func TestCalculateStopLoss(t *testing.T) {
	rc := NewRiskCalculator()

	// 测试做多止损
	params := StopLossParams{
		EntryPrice:      50000,
		IsLong:          true,
		ATR:             500, // ATR = 500
		RiskPercentage:  2.0,
		MinStopDistance: 0.5,
	}

	stopLoss, err := rc.CalculateStopLoss(params)
	if err != nil {
		t.Fatalf("计算止损失败: %v", err)
	}

	if stopLoss >= params.EntryPrice {
		t.Errorf("做多止损应低于入场价，入场: %.2f, 止损: %.2f", params.EntryPrice, stopLoss)
	}

	t.Logf("✓ 做多止损: 入场 %.2f → 止损 %.2f", params.EntryPrice, stopLoss)

	// 测试做空止损
	params.IsLong = false
	stopLoss, err = rc.CalculateStopLoss(params)
	if err != nil {
		t.Fatalf("计算止损失败: %v", err)
	}

	if stopLoss <= params.EntryPrice {
		t.Errorf("做空止损应高于入场价，入场: %.2f, 止损: %.2f", params.EntryPrice, stopLoss)
	}

	t.Logf("✓ 做空止损: 入场 %.2f → 止损 %.2f", params.EntryPrice, stopLoss)
}

// TestCalculateTakeProfit 测试止盈计算
func TestCalculateTakeProfit(t *testing.T) {
	rc := NewRiskCalculator()

	params := TakeProfitParams{
		EntryPrice:      50000,
		StopLossPrice:   49000,
		IsLong:          true,
		RiskRewardRatio: 3.0,
	}

	takeProfit, err := rc.CalculateTakeProfit(params)
	if err != nil {
		t.Fatalf("计算止盈失败: %v", err)
	}

	// 验证风险回报比
	riskDistance := params.EntryPrice - params.StopLossPrice // 1000
	rewardDistance := takeProfit - params.EntryPrice
	actualRatio := rewardDistance / riskDistance

	if actualRatio < 2.9 || actualRatio > 3.1 { // 允许小误差
		t.Errorf("风险回报比错误，期望: 3.0，实际: %.2f", actualRatio)
	}

	t.Logf("✓ 止盈计算: 入场 %.2f, 止损 %.2f, 止盈 %.2f (比率: %.2f)",
		params.EntryPrice, params.StopLossPrice, takeProfit, actualRatio)
}

// TestValidateRiskRewardRatio 测试风险回报比验证
func TestValidateRiskRewardRatio(t *testing.T) {
	rc := NewRiskCalculator()

	// 测试有效的风险回报比
	isValid, ratio, err := rc.ValidateRiskRewardRatio(50000, 49000, 53000, true, 3.0)
	if err != nil {
		t.Fatalf("验证失败: %v", err)
	}

	if !isValid {
		t.Errorf("应该通过验证，风险回报比: %.2f", ratio)
	}

	t.Logf("✓ 有效风险回报比: %.2f (通过)", ratio)

	// 测试无效的风险回报比
	isValid, ratio, err = rc.ValidateRiskRewardRatio(50000, 49000, 51000, true, 3.0)
	if err != nil {
		t.Fatalf("验证失败: %v", err)
	}

	if isValid {
		t.Errorf("不应该通过验证，风险回报比: %.2f", ratio)
	}

	t.Logf("✓ 无效风险回报比: %.2f (未通过)", ratio)
}

// TestMarginUtilization 测试保证金利用率计算
func TestMarginUtilization(t *testing.T) {
	rc := NewRiskCalculator()

	totalMarginUsed := 5000.0
	totalEquity := 10000.0

	utilization := rc.CalculateMarginUtilization(totalMarginUsed, totalEquity)

	if utilization.UtilizationRate != 50.0 {
		t.Errorf("保证金使用率错误，期望: 50.0，实际: %.2f", utilization.UtilizationRate)
	}

	if utilization.AvailableMargin != 5000.0 {
		t.Errorf("可用保证金错误，期望: 5000.0，实际: %.2f", utilization.AvailableMargin)
	}

	t.Logf("✓ 保证金利用率: %.2f%%, 可用: %.2f", utilization.UtilizationRate, utilization.AvailableMargin)
}
