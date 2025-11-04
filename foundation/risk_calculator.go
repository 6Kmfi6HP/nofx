package foundation

import (
	"fmt"
	"math"
)

// RiskCalculator 底层风险计算器
// 职责：计算止损、仓位、保证金等风险相关参数
// 不涉及交易决策，只提供风险计算服务
type RiskCalculator struct {
	// 账户净值（用于计算风险百分比）
	accountEquity float64
	// 最大单笔风险百分比（默认1-2%）
	maxRiskPercentPerTrade float64
	// 最大保证金使用率（默认90%）
	maxMarginUsagePercent float64
}

// NewRiskCalculator 创建风险计算器实例
func NewRiskCalculator(accountEquity, maxRiskPercent, maxMarginPercent float64) *RiskCalculator {
	if maxRiskPercent <= 0 || maxRiskPercent > 10 {
		maxRiskPercent = 2.0 // 默认2%单笔风险
	}
	if maxMarginPercent <= 0 || maxMarginPercent > 100 {
		maxMarginPercent = 90.0 // 默认90%最大保证金使用率
	}
	return &RiskCalculator{
		accountEquity:          accountEquity,
		maxRiskPercentPerTrade: maxRiskPercent,
		maxMarginUsagePercent:  maxMarginPercent,
	}
}

// StopLossParams 止损参数
type StopLossParams struct {
	Price       float64 // 止损价格
	Distance    float64 // 止损距离（价格百分比）
	RiskAmount  float64 // 风险金额（USD）
	RiskPercent float64 // 风险百分比（占账户净值）
}

// PositionSizeParams 仓位大小参数
type PositionSizeParams struct {
	QuantityUSD  float64 // 仓位大小（USD）
	QuantityBase float64 // 仓位大小（基础货币数量）
	Leverage     int     // 杠杆倍数
	MarginNeeded float64 // 所需保证金（USD）
	MarginPercent float64 // 保证金占比
}

// CalculateStopLoss 计算止损价格
// direction: "long" 或 "short"
// entryPrice: 入场价格
// atrValue: ATR值（平均真实波幅）
// atrMultiplier: ATR倍数（默认1.5-2.0）
func (rc *RiskCalculator) CalculateStopLoss(direction string, entryPrice, atrValue, atrMultiplier float64) (*StopLossParams, error) {
	if entryPrice <= 0 {
		return nil, fmt.Errorf("invalid entry price: %f", entryPrice)
	}
	if atrValue <= 0 {
		return nil, fmt.Errorf("invalid ATR value: %f", atrValue)
	}
	if atrMultiplier <= 0 {
		atrMultiplier = 1.5 // 默认1.5倍ATR
	}

	// 计算止损距离
	stopDistance := atrValue * atrMultiplier

	var stopPrice float64
	var distancePercent float64

	if direction == "long" {
		// 多单止损在下方
		stopPrice = entryPrice - stopDistance
		distancePercent = stopDistance / entryPrice * 100
	} else if direction == "short" {
		// 空单止损在上方
		stopPrice = entryPrice + stopDistance
		distancePercent = stopDistance / entryPrice * 100
	} else {
		return nil, fmt.Errorf("invalid direction: %s", direction)
	}

	// 计算风险金额（基于账户净值的最大风险百分比）
	riskAmount := rc.accountEquity * rc.maxRiskPercentPerTrade / 100

	return &StopLossParams{
		Price:       stopPrice,
		Distance:    distancePercent,
		RiskAmount:  riskAmount,
		RiskPercent: rc.maxRiskPercentPerTrade,
	}, nil
}

// CalculateTakeProfit 计算止盈价格
// direction: "long" 或 "short"
// entryPrice: 入场价格
// stopLossPrice: 止损价格
// rewardRiskRatio: 风险回报比（默认3:1）
func (rc *RiskCalculator) CalculateTakeProfit(direction string, entryPrice, stopLossPrice, rewardRiskRatio float64) (float64, error) {
	if entryPrice <= 0 || stopLossPrice <= 0 {
		return 0, fmt.Errorf("invalid prices: entry=%f, stopLoss=%f", entryPrice, stopLossPrice)
	}
	if rewardRiskRatio <= 0 {
		rewardRiskRatio = 3.0 // 默认3:1
	}

	// 计算风险距离
	var riskDistance float64
	if direction == "long" {
		riskDistance = entryPrice - stopLossPrice
		if riskDistance <= 0 {
			return 0, fmt.Errorf("invalid stop loss for long position")
		}
		// 止盈在上方
		return entryPrice + (riskDistance * rewardRiskRatio), nil
	} else if direction == "short" {
		riskDistance = stopLossPrice - entryPrice
		if riskDistance <= 0 {
			return 0, fmt.Errorf("invalid stop loss for short position")
		}
		// 止盈在下方
		return entryPrice - (riskDistance * rewardRiskRatio), nil
	}

	return 0, fmt.Errorf("invalid direction: %s", direction)
}

// CalculatePositionSize 计算仓位大小
// direction: "long" 或 "short"
// entryPrice: 入场价格
// stopLossPrice: 止损价格
// leverage: 杠杆倍数
// currentMarginUsed: 当前已使用保证金（USD）
// confidence: 信心度（0.7-1.0，影响仓位大小）
func (rc *RiskCalculator) CalculatePositionSize(
	direction string,
	entryPrice, stopLossPrice float64,
	leverage int,
	currentMarginUsed float64,
	confidence float64,
) (*PositionSizeParams, error) {
	if entryPrice <= 0 || stopLossPrice <= 0 {
		return nil, fmt.Errorf("invalid prices")
	}
	if leverage < 1 || leverage > 100 {
		return nil, fmt.Errorf("invalid leverage: %d", leverage)
	}
	if confidence < 0.7 || confidence > 1.0 {
		confidence = 0.85 // 默认信心度
	}

	// 计算止损距离百分比
	var stopDistancePercent float64
	if direction == "long" {
		stopDistancePercent = (entryPrice - stopLossPrice) / entryPrice
	} else {
		stopDistancePercent = (stopLossPrice - entryPrice) / entryPrice
	}

	if stopDistancePercent <= 0 {
		return nil, fmt.Errorf("invalid stop loss distance")
	}

	// 基于风险金额计算仓位
	// 风险金额 = 仓位大小 * 止损距离百分比
	// 仓位大小 = 风险金额 / 止损距离百分比
	maxRiskAmount := rc.accountEquity * rc.maxRiskPercentPerTrade / 100
	basePositionSize := maxRiskAmount / stopDistancePercent

	// 根据信心度调整仓位（信心度越高，仓位越大）
	// 0.7信心 -> 70%仓位, 0.85信心 -> 85%仓位, 1.0信心 -> 100%仓位
	adjustedPositionSize := basePositionSize * confidence

	// 计算所需保证金
	marginNeeded := adjustedPositionSize / float64(leverage)

	// 检查保证金是否超限
	totalMarginAfter := currentMarginUsed + marginNeeded
	marginPercentAfter := totalMarginAfter / rc.accountEquity * 100

	if marginPercentAfter > rc.maxMarginUsagePercent {
		// 保证金超限，缩小仓位
		availableMargin := rc.accountEquity * rc.maxMarginUsagePercent / 100 - currentMarginUsed
		if availableMargin <= 0 {
			return nil, fmt.Errorf("no available margin")
		}
		adjustedPositionSize = availableMargin * float64(leverage)
		marginNeeded = availableMargin
		marginPercentAfter = rc.maxMarginUsagePercent
	}

	// 计算基础货币数量
	quantityBase := adjustedPositionSize / entryPrice

	return &PositionSizeParams{
		QuantityUSD:   adjustedPositionSize,
		QuantityBase:  quantityBase,
		Leverage:      leverage,
		MarginNeeded:  marginNeeded,
		MarginPercent: marginPercentAfter,
	}, nil
}

// ValidateMarginRequirement 验证保证金要求
// 返回是否满足保证金要求，以及可用保证金金额
func (rc *RiskCalculator) ValidateMarginRequirement(currentMarginUsed, requiredMargin float64) (bool, float64) {
	maxAllowedMargin := rc.accountEquity * rc.maxMarginUsagePercent / 100
	availableMargin := maxAllowedMargin - currentMarginUsed

	if availableMargin < requiredMargin {
		return false, availableMargin
	}
	return true, availableMargin
}

// CalculateLiquidationPrice 计算强平价格
// direction: "long" 或 "short"
// entryPrice: 入场价格
// leverage: 杠杆倍数
// maintenanceMarginRate: 维持保证金率（默认0.4%，即0.004）
func (rc *RiskCalculator) CalculateLiquidationPrice(direction string, entryPrice float64, leverage int, maintenanceMarginRate float64) (float64, error) {
	if entryPrice <= 0 || leverage < 1 {
		return 0, fmt.Errorf("invalid parameters")
	}
	if maintenanceMarginRate <= 0 {
		maintenanceMarginRate = 0.004 // 默认0.4%
	}

	// 强平价格计算公式
	// 多单：强平价 = 入场价 * (1 - 1/杠杆 + 维持保证金率)
	// 空单：强平价 = 入场价 * (1 + 1/杠杆 - 维持保证金率)
	leverageFloat := float64(leverage)

	if direction == "long" {
		return entryPrice * (1 - 1/leverageFloat + maintenanceMarginRate), nil
	} else if direction == "short" {
		return entryPrice * (1 + 1/leverageFloat - maintenanceMarginRate), nil
	}

	return 0, fmt.Errorf("invalid direction: %s", direction)
}

// RiskMetrics 风险指标
type RiskMetrics struct {
	CurrentMarginUsagePercent float64 // 当前保证金使用率
	AvailableMargin           float64 // 可用保证金
	TotalRiskAmount           float64 // 总风险金额
	TotalRiskPercent          float64 // 总风险百分比
	MaxDrawdownPercent        float64 // 最大回撤百分比
	DailyPnLPercent           float64 // 日盈亏百分比
}

// CalculateRiskMetrics 计算综合风险指标
func (rc *RiskCalculator) CalculateRiskMetrics(
	currentMarginUsed float64,
	openPositions []OpenPosition,
	dailyPnL float64,
	historicalHighEquity float64,
) *RiskMetrics {
	// 保证金使用率
	marginUsagePercent := currentMarginUsed / rc.accountEquity * 100
	availableMargin := rc.accountEquity*rc.maxMarginUsagePercent/100 - currentMarginUsed
	if availableMargin < 0 {
		availableMargin = 0
	}

	// 计算总风险金额（所有持仓的潜在亏损）
	totalRiskAmount := 0.0
	for _, pos := range openPositions {
		// 风险 = 仓位大小 * 止损距离百分比
		var stopDistancePercent float64
		if pos.Direction == "long" {
			stopDistancePercent = (pos.EntryPrice - pos.StopLossPrice) / pos.EntryPrice
		} else {
			stopDistancePercent = (pos.StopLossPrice - pos.EntryPrice) / pos.EntryPrice
		}
		if stopDistancePercent > 0 {
			totalRiskAmount += pos.PositionSizeUSD * stopDistancePercent
		}
	}
	totalRiskPercent := totalRiskAmount / rc.accountEquity * 100

	// 最大回撤
	maxDrawdownPercent := 0.0
	if historicalHighEquity > 0 {
		maxDrawdownPercent = (historicalHighEquity - rc.accountEquity) / historicalHighEquity * 100
	}

	// 日盈亏百分比
	dailyPnLPercent := dailyPnL / rc.accountEquity * 100

	return &RiskMetrics{
		CurrentMarginUsagePercent: marginUsagePercent,
		AvailableMargin:           availableMargin,
		TotalRiskAmount:           totalRiskAmount,
		TotalRiskPercent:          totalRiskPercent,
		MaxDrawdownPercent:        math.Max(0, maxDrawdownPercent),
		DailyPnLPercent:           dailyPnLPercent,
	}
}

// OpenPosition 持仓信息（用于风险计算）
type OpenPosition struct {
	Symbol          string
	Direction       string  // "long" 或 "short"
	EntryPrice      float64
	StopLossPrice   float64
	PositionSizeUSD float64
	Leverage        int
}

// UpdateAccountEquity 更新账户净值（用于动态调整风险参数）
func (rc *RiskCalculator) UpdateAccountEquity(newEquity float64) {
	if newEquity > 0 {
		rc.accountEquity = newEquity
	}
}

// GetMaxRiskPerTrade 获取单笔最大风险金额
func (rc *RiskCalculator) GetMaxRiskPerTrade() float64 {
	return rc.accountEquity * rc.maxRiskPercentPerTrade / 100
}

// GetMaxPositionValue 获取最大仓位价值（基于账户净值）
// assetType: "btc_eth" 或 "altcoin"
func (rc *RiskCalculator) GetMaxPositionValue(assetType string) (float64, float64) {
	if assetType == "btc_eth" {
		// BTC/ETH: 5-10倍账户净值
		return rc.accountEquity * 5, rc.accountEquity * 10
	}
	// 山寨币: 0.8-1.5倍账户净值
	return rc.accountEquity * 0.8, rc.accountEquity * 1.5
}
