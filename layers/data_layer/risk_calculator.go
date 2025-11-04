package data_layer

import (
	"fmt"
	"math"
	"nofx/layers"
)

// RiskCalculator 风险计算器（底层）
// 职责：风险计算（止损、仓位、保证金）
type RiskCalculator struct {
	config layers.DataLayerConfig

	// 账户信息
	totalBalance     float64
	availableBalance float64
	usedMargin       float64

	// 统计信息
	dailyPnL         float64
	consecutiveLosses int
	circuitBreakerActive bool
}

// NewRiskCalculator 创建风险计算器
func NewRiskCalculator(config layers.DataLayerConfig) *RiskCalculator {
	return &RiskCalculator{
		config:           config,
		totalBalance:     0,
		availableBalance: 0,
		usedMargin:       0,
		dailyPnL:         0,
		consecutiveLosses: 0,
		circuitBreakerActive: false,
	}
}

// UpdateAccountInfo 更新账户信息
func (rc *RiskCalculator) UpdateAccountInfo(totalBalance, availableBalance, usedMargin float64) {
	rc.totalBalance = totalBalance
	rc.availableBalance = availableBalance
	rc.usedMargin = usedMargin
}

// UpdateDailyPnL 更新每日盈亏
func (rc *RiskCalculator) UpdateDailyPnL(pnl float64) {
	rc.dailyPnL = pnl

	// 检查熔断机制
	if rc.config.CircuitBreakerEnabled {
		maxLoss := rc.totalBalance * rc.config.MaxDailyLossPercent / 100
		if rc.dailyPnL < -maxLoss {
			rc.circuitBreakerActive = true
		}
	}
}

// RecordTradeResult 记录交易结果
func (rc *RiskCalculator) RecordTradeResult(isWin bool) {
	if isWin {
		rc.consecutiveLosses = 0
	} else {
		rc.consecutiveLosses++
	}

	// 检查连续亏损熔断
	if rc.config.CircuitBreakerEnabled {
		if rc.consecutiveLosses >= rc.config.MaxConsecutiveLosses {
			rc.circuitBreakerActive = true
		}
	}
}

// ResetCircuitBreaker 重置熔断器（手动或每日重置）
func (rc *RiskCalculator) ResetCircuitBreaker() {
	rc.circuitBreakerActive = false
	rc.consecutiveLosses = 0
	rc.dailyPnL = 0
}

// CalculateRiskMetrics 计算风险指标
// 输入：交易方向、清洗后的市场数据
// 输出：风险指标
func (rc *RiskCalculator) CalculateRiskMetrics(
	direction layers.Direction,
	marketData *layers.CleanedMarketData,
) (*layers.RiskMetrics, error) {
	if marketData == nil {
		return nil, fmt.Errorf("market data is nil")
	}

	metrics := &layers.RiskMetrics{
		Symbol: marketData.Symbol,
	}

	// 检查熔断机制
	if rc.circuitBreakerActive {
		metrics.CanTrade = false
		metrics.RiskReason = fmt.Sprintf("熔断触发: 每日亏损%.2f%% 或 连续亏损%d次",
			-rc.dailyPnL/rc.totalBalance*100, rc.consecutiveLosses)
		metrics.RiskLevel = "extreme"
		return metrics, nil
	}

	// 检查账户余额
	if rc.totalBalance <= 0 || rc.availableBalance <= 0 {
		metrics.CanTrade = false
		metrics.RiskReason = "账户余额不足"
		metrics.RiskLevel = "extreme"
		return metrics, nil
	}

	// 计算最大仓位
	maxRisk := rc.totalBalance * rc.config.MaxSingleTradeRiskPercent / 100
	metrics.MaxPositionSizeUSD = maxRisk / 0.02 // 假设2%止损

	// 限制仓位不超过账户权益的一定比例
	maxAccountRisk := rc.totalBalance * rc.config.MaxAccountRiskPercent / 100
	if metrics.MaxPositionSizeUSD > maxAccountRisk {
		metrics.MaxPositionSizeUSD = maxAccountRisk
	}

	// 计算建议杠杆
	metrics.RecommendedLeverage = rc.calculateRecommendedLeverage(marketData)

	// 计算止损止盈价格
	if direction == layers.DirectionLong {
		metrics.StopLossPrice = rc.calculateLongStopLoss(marketData)
		metrics.TakeProfitPrice = rc.calculateLongTakeProfit(marketData)
	} else if direction == layers.DirectionShort {
		metrics.StopLossPrice = rc.calculateShortStopLoss(marketData)
		metrics.TakeProfitPrice = rc.calculateShortTakeProfit(marketData)
	}

	// 计算最大亏损
	if metrics.StopLossPrice > 0 {
		priceDiff := math.Abs(marketData.CurrentPrice - metrics.StopLossPrice)
		metrics.MaxLossUSD = (priceDiff / marketData.CurrentPrice) * metrics.MaxPositionSizeUSD
	}

	// 计算所需保证金
	metrics.RequiredMargin = metrics.MaxPositionSizeUSD / float64(metrics.RecommendedLeverage)

	// 计算保证金使用率
	if rc.totalBalance > 0 {
		newUsedMargin := rc.usedMargin + metrics.RequiredMargin
		metrics.MarginUsagePercent = (newUsedMargin / rc.totalBalance) * 100
	}

	// 评估风险等级
	metrics.RiskLevel = rc.assessRiskLevel(metrics)

	// 判断是否可交易
	metrics.CanTrade, metrics.RiskReason = rc.canTrade(metrics)

	return metrics, nil
}

// calculateRecommendedLeverage 计算建议杠杆
func (rc *RiskCalculator) calculateRecommendedLeverage(data *layers.CleanedMarketData) int {
	baseLeverage := rc.config.DefaultLeverage

	// 根据波动率调整杠杆
	// ATR越大，波动率越高，杠杆越低
	if data.ATR > 0 && data.CurrentPrice > 0 {
		volatility := data.ATR / data.CurrentPrice * 100 // 波动率百分比

		if volatility > 5 {
			baseLeverage = int(math.Max(1, float64(baseLeverage)-2))
		} else if volatility > 3 {
			baseLeverage = int(math.Max(1, float64(baseLeverage)-1))
		}
	}

	// 根据RSI调整
	if data.RSI14 > 70 || data.RSI14 < 30 {
		// 超买超卖区域，降低杠杆
		baseLeverage = int(math.Max(1, float64(baseLeverage)-1))
	}

	// 限制最大杠杆
	if baseLeverage > rc.config.MaxLeverage {
		baseLeverage = rc.config.MaxLeverage
	}

	return baseLeverage
}

// calculateLongStopLoss 计算做多止损价格
func (rc *RiskCalculator) calculateLongStopLoss(data *layers.CleanedMarketData) float64 {
	// 方法1：基于ATR
	if data.ATR > 0 {
		return data.CurrentPrice - (data.ATR * 2)
	}

	// 方法2：基于百分比（2%）
	return data.CurrentPrice * 0.98
}

// calculateLongTakeProfit 计算做多止盈价格
func (rc *RiskCalculator) calculateLongTakeProfit(data *layers.CleanedMarketData) float64 {
	// 风险收益比 1:2
	stopLoss := rc.calculateLongStopLoss(data)
	risk := data.CurrentPrice - stopLoss
	return data.CurrentPrice + (risk * 2)
}

// calculateShortStopLoss 计算做空止损价格
func (rc *RiskCalculator) calculateShortStopLoss(data *layers.CleanedMarketData) float64 {
	// 方法1：基于ATR
	if data.ATR > 0 {
		return data.CurrentPrice + (data.ATR * 2)
	}

	// 方法2：基于百分比（2%）
	return data.CurrentPrice * 1.02
}

// calculateShortTakeProfit 计算做空止盈价格
func (rc *RiskCalculator) calculateShortTakeProfit(data *layers.CleanedMarketData) float64 {
	// 风险收益比 1:2
	stopLoss := rc.calculateShortStopLoss(data)
	risk := stopLoss - data.CurrentPrice
	return data.CurrentPrice - (risk * 2)
}

// assessRiskLevel 评估风险等级
func (rc *RiskCalculator) assessRiskLevel(metrics *layers.RiskMetrics) string {
	score := 0

	// 保证金使用率
	if metrics.MarginUsagePercent > 80 {
		score += 3
	} else if metrics.MarginUsagePercent > 60 {
		score += 2
	} else if metrics.MarginUsagePercent > 40 {
		score += 1
	}

	// 杠杆倍数
	if metrics.RecommendedLeverage >= 5 {
		score += 2
	} else if metrics.RecommendedLeverage >= 3 {
		score += 1
	}

	// 最大亏损比例
	if rc.totalBalance > 0 {
		lossPercent := metrics.MaxLossUSD / rc.totalBalance * 100
		if lossPercent > 2 {
			score += 2
		} else if lossPercent > 1 {
			score += 1
		}
	}

	// 连续亏损
	if rc.consecutiveLosses >= 2 {
		score += 1
	}

	// 评级
	if score >= 5 {
		return "extreme"
	} else if score >= 3 {
		return "high"
	} else if score >= 1 {
		return "medium"
	}
	return "low"
}

// canTrade 判断是否可交易
func (rc *RiskCalculator) canTrade(metrics *layers.RiskMetrics) (bool, string) {
	// 检查熔断
	if rc.circuitBreakerActive {
		return false, "熔断机制触发"
	}

	// 检查余额
	if rc.availableBalance < metrics.RequiredMargin {
		return false, fmt.Sprintf("可用余额不足，需要%.2f，可用%.2f",
			metrics.RequiredMargin, rc.availableBalance)
	}

	// 检查保证金使用率
	if metrics.MarginUsagePercent > 90 {
		return false, fmt.Sprintf("保证金使用率过高：%.1f%%", metrics.MarginUsagePercent)
	}

	// 检查风险等级
	if metrics.RiskLevel == "extreme" {
		return false, "风险等级过高"
	}

	// 检查止损价格
	if metrics.StopLossPrice <= 0 {
		return false, "止损价格无效"
	}

	return true, "风控检查通过"
}

// GetCircuitBreakerStatus 获取熔断器状态
func (rc *RiskCalculator) GetCircuitBreakerStatus() map[string]interface{} {
	return map[string]interface{}{
		"active":             rc.circuitBreakerActive,
		"daily_pnl":          rc.dailyPnL,
		"daily_pnl_percent":  rc.dailyPnL / rc.totalBalance * 100,
		"consecutive_losses": rc.consecutiveLosses,
		"max_daily_loss_percent": rc.config.MaxDailyLossPercent,
		"max_consecutive_losses": rc.config.MaxConsecutiveLosses,
	}
}

// GetAccountRiskSummary 获取账户风险摘要
func (rc *RiskCalculator) GetAccountRiskSummary() map[string]interface{} {
	marginUsagePercent := 0.0
	if rc.totalBalance > 0 {
		marginUsagePercent = rc.usedMargin / rc.totalBalance * 100
	}

	return map[string]interface{}{
		"total_balance":       rc.totalBalance,
		"available_balance":   rc.availableBalance,
		"used_margin":         rc.usedMargin,
		"margin_usage_percent": marginUsagePercent,
		"daily_pnl":           rc.dailyPnL,
		"consecutive_losses":  rc.consecutiveLosses,
		"circuit_breaker_active": rc.circuitBreakerActive,
	}
}
