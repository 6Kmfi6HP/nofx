package trader

import (
	"fmt"
	"math"
)

// RiskCalculator 风险计算器 - 三层架构中的底层组件
// 职责：计算止损点、仓位大小、保证金利用率等基础风险参数
// 不涉及决策逻辑，只提供纯粹的风险计算功能
type RiskCalculator struct{}

// NewRiskCalculator 创建风险计算器实例
func NewRiskCalculator() *RiskCalculator {
	return &RiskCalculator{}
}

// PositionSizeParams 仓位大小计算参数
type PositionSizeParams struct {
	AccountEquity   float64 // 账户净值
	RiskPercentage  float64 // 风险百分比（例如：2.0 表示 2%）
	EntryPrice      float64 // 入场价格
	StopLossPrice   float64 // 止损价格
	Leverage        int     // 杠杆倍数
}

// PositionSizeResult 仓位大小计算结果
type PositionSizeResult struct {
	PositionSizeUSD float64 // 仓位大小（美元）
	Quantity        float64 // 实际数量
	MarginRequired  float64 // 所需保证金
	RiskUSD         float64 // 风险金额（美元）
}

// CalculatePositionSize 计算合理的仓位大小
// 基于账户风险承受能力、止损距离和杠杆倍数
func (rc *RiskCalculator) CalculatePositionSize(params PositionSizeParams) (*PositionSizeResult, error) {
	if params.AccountEquity <= 0 {
		return nil, fmt.Errorf("账户净值必须大于0")
	}
	if params.EntryPrice <= 0 || params.StopLossPrice <= 0 {
		return nil, fmt.Errorf("入场价和止损价必须大于0")
	}
	if params.Leverage <= 0 {
		return nil, fmt.Errorf("杠杆倍数必须大于0")
	}

	// 计算止损距离百分比
	stopLossDistance := math.Abs(params.EntryPrice-params.StopLossPrice) / params.EntryPrice

	// 计算最大风险金额
	maxRiskUSD := params.AccountEquity * (params.RiskPercentage / 100)

	// 计算仓位大小（USD）
	// 公式：仓位大小 = 最大风险金额 / (止损距离 × 杠杆)
	// 注意：杠杆会放大风险，所以仓位大小要相应减小
	positionSizeUSD := maxRiskUSD / stopLossDistance

	// 计算实际数量
	quantity := positionSizeUSD / params.EntryPrice

	// 计算所需保证金
	marginRequired := positionSizeUSD / float64(params.Leverage)

	return &PositionSizeResult{
		PositionSizeUSD: positionSizeUSD,
		Quantity:        quantity,
		MarginRequired:  marginRequired,
		RiskUSD:         maxRiskUSD,
	}, nil
}

// StopLossParams 止损点计算参数
type StopLossParams struct {
	EntryPrice      float64 // 入场价格
	IsLong          bool    // 是否做多
	ATR             float64 // 平均真实波动范围（用于动态止损）
	RiskPercentage  float64 // 风险百分比
	MinStopDistance float64 // 最小止损距离（百分比，如 0.5 表示 0.5%）
}

// CalculateStopLoss 计算动态止损价格
// 基于ATR或固定百分比
func (rc *RiskCalculator) CalculateStopLoss(params StopLossParams) (float64, error) {
	if params.EntryPrice <= 0 {
		return 0, fmt.Errorf("入场价格必须大于0")
	}

	var stopLossPrice float64

	if params.ATR > 0 {
		// 使用ATR计算动态止损（2倍ATR）
		if params.IsLong {
			stopLossPrice = params.EntryPrice - (2 * params.ATR)
		} else {
			stopLossPrice = params.EntryPrice + (2 * params.ATR)
		}
	} else {
		// 使用固定百分比计算止损
		stopDistance := params.EntryPrice * (params.RiskPercentage / 100)
		if params.IsLong {
			stopLossPrice = params.EntryPrice - stopDistance
		} else {
			stopLossPrice = params.EntryPrice + stopDistance
		}
	}

	// 确保止损距离不小于最小距离
	minStopDistance := params.EntryPrice * (params.MinStopDistance / 100)
	actualDistance := math.Abs(params.EntryPrice - stopLossPrice)
	if actualDistance < minStopDistance {
		if params.IsLong {
			stopLossPrice = params.EntryPrice - minStopDistance
		} else {
			stopLossPrice = params.EntryPrice + minStopDistance
		}
	}

	return stopLossPrice, nil
}

// TakeProfitParams 止盈点计算参数
type TakeProfitParams struct {
	EntryPrice        float64 // 入场价格
	StopLossPrice     float64 // 止损价格
	IsLong            bool    // 是否做多
	RiskRewardRatio   float64 // 风险回报比（如 3.0 表示 1:3）
}

// CalculateTakeProfit 计算止盈价格
// 基于风险回报比
func (rc *RiskCalculator) CalculateTakeProfit(params TakeProfitParams) (float64, error) {
	if params.EntryPrice <= 0 || params.StopLossPrice <= 0 {
		return 0, fmt.Errorf("入场价和止损价必须大于0")
	}
	if params.RiskRewardRatio <= 0 {
		return 0, fmt.Errorf("风险回报比必须大于0")
	}

	// 计算止损距离
	riskDistance := math.Abs(params.EntryPrice - params.StopLossPrice)

	// 计算止盈距离（风险距离 × 风险回报比）
	rewardDistance := riskDistance * params.RiskRewardRatio

	var takeProfitPrice float64
	if params.IsLong {
		takeProfitPrice = params.EntryPrice + rewardDistance
	} else {
		takeProfitPrice = params.EntryPrice - rewardDistance
	}

	return takeProfitPrice, nil
}

// MarginUtilization 保证金利用率计算
type MarginUtilization struct {
	TotalMarginUsed  float64 // 总保证金使用量
	TotalEquity      float64 // 账户净值
	UtilizationRate  float64 // 利用率（百分比）
	AvailableMargin  float64 // 可用保证金
}

// CalculateMarginUtilization 计算当前保证金利用率
func (rc *RiskCalculator) CalculateMarginUtilization(totalMarginUsed, totalEquity float64) *MarginUtilization {
	if totalEquity <= 0 {
		return &MarginUtilization{
			TotalMarginUsed:  totalMarginUsed,
			TotalEquity:      totalEquity,
			UtilizationRate:  0,
			AvailableMargin:  0,
		}
	}

	utilizationRate := (totalMarginUsed / totalEquity) * 100
	availableMargin := totalEquity - totalMarginUsed

	return &MarginUtilization{
		TotalMarginUsed:  totalMarginUsed,
		TotalEquity:      totalEquity,
		UtilizationRate:  utilizationRate,
		AvailableMargin:  availableMargin,
	}
}

// ValidateRiskRewardRatio 验证风险回报比是否满足要求
func (rc *RiskCalculator) ValidateRiskRewardRatio(entryPrice, stopLoss, takeProfit float64, isLong bool, minRatio float64) (bool, float64, error) {
	if entryPrice <= 0 || stopLoss <= 0 || takeProfit <= 0 {
		return false, 0, fmt.Errorf("价格参数必须大于0")
	}

	var riskDistance, rewardDistance float64

	if isLong {
		riskDistance = entryPrice - stopLoss
		rewardDistance = takeProfit - entryPrice
	} else {
		riskDistance = stopLoss - entryPrice
		rewardDistance = entryPrice - takeProfit
	}

	if riskDistance <= 0 {
		return false, 0, fmt.Errorf("止损设置不合理")
	}

	ratio := rewardDistance / riskDistance

	return ratio >= minRatio, ratio, nil
}

// LiquidationPrice 计算强平价格
type LiquidationPriceParams struct {
	EntryPrice   float64 // 入场价格
	Leverage     int     // 杠杆倍数
	IsLong       bool    // 是否做多
	MarginRatio  float64 // 维持保证金率（如 0.5% = 0.005）
}

// CalculateLiquidationPrice 计算理论强平价格
func (rc *RiskCalculator) CalculateLiquidationPrice(params LiquidationPriceParams) (float64, error) {
	if params.EntryPrice <= 0 {
		return 0, fmt.Errorf("入场价格必须大于0")
	}
	if params.Leverage <= 0 {
		return 0, fmt.Errorf("杠杆倍数必须大于0")
	}

	// 计算强平距离百分比
	// 强平距离 = (1 / 杠杆) - 维持保证金率
	liquidationDistancePercent := (1.0 / float64(params.Leverage)) - params.MarginRatio

	var liquidationPrice float64
	if params.IsLong {
		// 做多：强平价格 = 入场价格 × (1 - 强平距离)
		liquidationPrice = params.EntryPrice * (1 - liquidationDistancePercent)
	} else {
		// 做空：强平价格 = 入场价格 × (1 + 强平距离)
		liquidationPrice = params.EntryPrice * (1 + liquidationDistancePercent)
	}

	return liquidationPrice, nil
}
