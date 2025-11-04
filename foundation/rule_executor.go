package foundation

import (
	"fmt"
	"sync"
	"time"
)

// RuleExecutor 底层规则执行器
// 职责：触发止损、熔断机制、风控规则
// 不涉及交易决策，只执行预设规则
type RuleExecutor struct {
	mu sync.RWMutex

	// 风控配置
	maxDailyLossPercent    float64 // 最大日亏损百分比
	maxDrawdownPercent     float64 // 最大回撤百分比
	coolingPeriodDuration  time.Duration // 冷却期时长

	// 运行状态
	isTradingHalted        bool      // 是否暂停交易
	haltReason             string    // 暂停原因
	haltedAt               time.Time // 暂停时间
	canResumeAt            time.Time // 可恢复交易时间

	// 账户状态跟踪
	accountEquity          float64 // 当前账户净值
	dailyStartEquity       float64 // 日初净值
	historicalHighEquity   float64 // 历史最高净值
	lastResetTime          time.Time // 上次重置时间
}

// NewRuleExecutor 创建规则执行器实例
func NewRuleExecutor(accountEquity, maxDailyLossPercent, maxDrawdownPercent float64) *RuleExecutor {
	if maxDailyLossPercent <= 0 || maxDailyLossPercent > 50 {
		maxDailyLossPercent = 10.0 // 默认10%日亏损限制
	}
	if maxDrawdownPercent <= 0 || maxDrawdownPercent > 90 {
		maxDrawdownPercent = 20.0 // 默认20%最大回撤限制
	}

	return &RuleExecutor{
		maxDailyLossPercent:   maxDailyLossPercent,
		maxDrawdownPercent:    maxDrawdownPercent,
		coolingPeriodDuration: 1 * time.Hour, // 默认1小时冷却期
		accountEquity:         accountEquity,
		dailyStartEquity:      accountEquity,
		historicalHighEquity:  accountEquity,
		lastResetTime:         time.Now(),
		isTradingHalted:       false,
	}
}

// RuleCheckResult 规则检查结果
type RuleCheckResult struct {
	IsTradingAllowed bool     // 是否允许交易
	Violations       []string // 违规项列表
	Warnings         []string // 警告列表
	HaltReason       string   // 如果暂停，暂停原因
	CanResumeAt      time.Time // 可恢复交易时间
}

// CheckTradingRules 检查交易规则（在每次交易前调用）
func (re *RuleExecutor) CheckTradingRules(currentEquity float64) *RuleCheckResult {
	re.mu.Lock()
	defer re.mu.Unlock()

	// 更新账户净值
	re.accountEquity = currentEquity

	// 更新历史最高净值
	if currentEquity > re.historicalHighEquity {
		re.historicalHighEquity = currentEquity
	}

	// 检查是否需要重置日初净值（每天UTC 0点重置）
	now := time.Now().UTC()
	if now.Day() != re.lastResetTime.UTC().Day() {
		re.dailyStartEquity = currentEquity
		re.lastResetTime = now
	}

	result := &RuleCheckResult{
		IsTradingAllowed: true,
		Violations:       []string{},
		Warnings:         []string{},
	}

	// 检查是否在暂停期
	if re.isTradingHalted {
		if now.Before(re.canResumeAt) {
			// 仍在暂停期
			result.IsTradingAllowed = false
			result.HaltReason = re.haltReason
			result.CanResumeAt = re.canResumeAt
			remainingMinutes := int(time.Until(re.canResumeAt).Minutes())
			result.Violations = append(result.Violations,
				fmt.Sprintf("交易已暂停: %s (剩余 %d 分钟)", re.haltReason, remainingMinutes))
			return result
		}
		// 暂停期已过，自动恢复
		re.isTradingHalted = false
		re.haltReason = ""
	}

	// 规则1：检查日亏损限制
	dailyPnL := currentEquity - re.dailyStartEquity
	dailyPnLPercent := dailyPnL / re.dailyStartEquity * 100
	if dailyPnLPercent < -re.maxDailyLossPercent {
		violation := fmt.Sprintf("触发日亏损限制: %.2f%% (限制: %.2f%%)",
			-dailyPnLPercent, re.maxDailyLossPercent)
		result.Violations = append(result.Violations, violation)
		re.haltTrading(violation, re.coolingPeriodDuration)
		result.IsTradingAllowed = false
		result.HaltReason = violation
		result.CanResumeAt = re.canResumeAt
		return result
	}

	// 警告：日亏损接近限制（80%）
	if dailyPnLPercent < -re.maxDailyLossPercent*0.8 {
		warning := fmt.Sprintf("日亏损接近限制: %.2f%% / %.2f%%",
			-dailyPnLPercent, re.maxDailyLossPercent)
		result.Warnings = append(result.Warnings, warning)
	}

	// 规则2：检查最大回撤限制
	drawdown := re.historicalHighEquity - currentEquity
	drawdownPercent := drawdown / re.historicalHighEquity * 100
	if drawdownPercent > re.maxDrawdownPercent {
		violation := fmt.Sprintf("触发最大回撤限制: %.2f%% (限制: %.2f%%)",
			drawdownPercent, re.maxDrawdownPercent)
		result.Violations = append(result.Violations, violation)
		re.haltTrading(violation, re.coolingPeriodDuration)
		result.IsTradingAllowed = false
		result.HaltReason = violation
		result.CanResumeAt = re.canResumeAt
		return result
	}

	// 警告：回撤接近限制（80%）
	if drawdownPercent > re.maxDrawdownPercent*0.8 {
		warning := fmt.Sprintf("回撤接近限制: %.2f%% / %.2f%%",
			drawdownPercent, re.maxDrawdownPercent)
		result.Warnings = append(result.Warnings, warning)
	}

	return result
}

// haltTrading 暂停交易（内部方法，已持有锁）
func (re *RuleExecutor) haltTrading(reason string, duration time.Duration) {
	re.isTradingHalted = true
	re.haltReason = reason
	re.haltedAt = time.Now()
	re.canResumeAt = re.haltedAt.Add(duration)
}

// ManualHaltTrading 手动暂停交易
func (re *RuleExecutor) ManualHaltTrading(reason string, duration time.Duration) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.haltTrading(reason, duration)
}

// ManualResumeTrading 手动恢复交易
func (re *RuleExecutor) ManualResumeTrading() {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.isTradingHalted = false
	re.haltReason = ""
}

// StopLossTrigger 止损触发器
type StopLossTrigger struct {
	Symbol         string
	Direction      string  // "long" 或 "short"
	CurrentPrice   float64
	StopLossPrice  float64
	PositionSize   float64
	ShouldTrigger  bool
	TriggerReason  string
}

// CheckStopLossTrigger 检查是否触发止损
func (re *RuleExecutor) CheckStopLossTrigger(
	symbol, direction string,
	currentPrice, stopLossPrice, positionSize float64,
) *StopLossTrigger {
	trigger := &StopLossTrigger{
		Symbol:        symbol,
		Direction:     direction,
		CurrentPrice:  currentPrice,
		StopLossPrice: stopLossPrice,
		PositionSize:  positionSize,
		ShouldTrigger: false,
	}

	if stopLossPrice <= 0 {
		// 未设置止损
		return trigger
	}

	if direction == "long" {
		// 多单：当前价格低于止损价
		if currentPrice <= stopLossPrice {
			trigger.ShouldTrigger = true
			trigger.TriggerReason = fmt.Sprintf("多单止损触发: 当前价 %.4f <= 止损价 %.4f",
				currentPrice, stopLossPrice)
		}
	} else if direction == "short" {
		// 空单：当前价格高于止损价
		if currentPrice >= stopLossPrice {
			trigger.ShouldTrigger = true
			trigger.TriggerReason = fmt.Sprintf("空单止损触发: 当前价 %.4f >= 止损价 %.4f",
				currentPrice, stopLossPrice)
		}
	}

	return trigger
}

// TakeProfitTrigger 止盈触发器
type TakeProfitTrigger struct {
	Symbol         string
	Direction      string
	CurrentPrice   float64
	TakeProfitPrice float64
	PositionSize   float64
	ShouldTrigger  bool
	TriggerReason  string
}

// CheckTakeProfitTrigger 检查是否触发止盈
func (re *RuleExecutor) CheckTakeProfitTrigger(
	symbol, direction string,
	currentPrice, takeProfitPrice, positionSize float64,
) *TakeProfitTrigger {
	trigger := &TakeProfitTrigger{
		Symbol:          symbol,
		Direction:       direction,
		CurrentPrice:    currentPrice,
		TakeProfitPrice: takeProfitPrice,
		PositionSize:    positionSize,
		ShouldTrigger:   false,
	}

	if takeProfitPrice <= 0 {
		// 未设置止盈
		return trigger
	}

	if direction == "long" {
		// 多单：当前价格高于止盈价
		if currentPrice >= takeProfitPrice {
			trigger.ShouldTrigger = true
			trigger.TriggerReason = fmt.Sprintf("多单止盈触发: 当前价 %.4f >= 止盈价 %.4f",
				currentPrice, takeProfitPrice)
		}
	} else if direction == "short" {
		// 空单：当前价格低于止盈价
		if currentPrice <= takeProfitPrice {
			trigger.ShouldTrigger = true
			trigger.TriggerReason = fmt.Sprintf("空单止盈触发: 当前价 %.4f <= 止盈价 %.4f",
				currentPrice, takeProfitPrice)
		}
	}

	return trigger
}

// PositionLimitCheck 持仓数量限制检查
type PositionLimitCheck struct {
	CurrentPositionCount int
	MaxPositionCount     int
	IsWithinLimit        bool
	Message              string
}

// CheckPositionLimit 检查持仓数量限制
func (re *RuleExecutor) CheckPositionLimit(currentPositionCount, maxPositionCount int) *PositionLimitCheck {
	check := &PositionLimitCheck{
		CurrentPositionCount: currentPositionCount,
		MaxPositionCount:     maxPositionCount,
		IsWithinLimit:        currentPositionCount < maxPositionCount,
	}

	if !check.IsWithinLimit {
		check.Message = fmt.Sprintf("持仓数量已达上限: %d / %d",
			currentPositionCount, maxPositionCount)
	} else {
		check.Message = fmt.Sprintf("持仓数量正常: %d / %d",
			currentPositionCount, maxPositionCount)
	}

	return check
}

// LeverageValidation 杠杆验证
type LeverageValidation struct {
	RequestedLeverage int
	MaxAllowedLeverage int
	IsValid           bool
	AdjustedLeverage  int
	Message           string
}

// ValidateLeverage 验证杠杆倍数
// assetType: "btc_eth" 或 "altcoin"
func (re *RuleExecutor) ValidateLeverage(requestedLeverage int, assetType string) *LeverageValidation {
	validation := &LeverageValidation{
		RequestedLeverage: requestedLeverage,
	}

	// 设置最大杠杆
	if assetType == "btc_eth" {
		validation.MaxAllowedLeverage = 50 // BTC/ETH最大50倍
	} else {
		validation.MaxAllowedLeverage = 20 // 山寨币最大20倍
	}

	// 验证杠杆
	if requestedLeverage < 1 {
		validation.IsValid = false
		validation.AdjustedLeverage = 1
		validation.Message = fmt.Sprintf("杠杆倍数过低，已调整为 1x")
	} else if requestedLeverage > validation.MaxAllowedLeverage {
		validation.IsValid = false
		validation.AdjustedLeverage = validation.MaxAllowedLeverage
		validation.Message = fmt.Sprintf("杠杆倍数超限，已调整为 %dx (最大 %dx)",
			validation.AdjustedLeverage, validation.MaxAllowedLeverage)
	} else {
		validation.IsValid = true
		validation.AdjustedLeverage = requestedLeverage
		validation.Message = fmt.Sprintf("杠杆倍数正常: %dx", requestedLeverage)
	}

	return validation
}

// TrailingStopConfig 移动止损配置
type TrailingStopConfig struct {
	ActivationProfitPercent float64 // 激活移动止损的盈利百分比
	TrailingPercent         float64 // 回撤百分比（触发止损）
}

// TrailingStopTrigger 移动止损触发器
type TrailingStopTrigger struct {
	IsActivated     bool
	HighestPrice    float64 // 开仓后的最高价（多单）或最低价（空单）
	NewStopLoss     float64 // 新的止损价
	ShouldUpdate    bool    // 是否应该更新止损
	ShouldTrigger   bool    // 是否触发止损
	Message         string
}

// CheckTrailingStop 检查移动止损
func (re *RuleExecutor) CheckTrailingStop(
	direction string,
	entryPrice, currentPrice, currentStopLoss, highestPrice float64,
	config *TrailingStopConfig,
) *TrailingStopTrigger {
	trigger := &TrailingStopTrigger{
		HighestPrice: highestPrice,
	}

	if config == nil {
		config = &TrailingStopConfig{
			ActivationProfitPercent: 2.0, // 默认盈利2%激活
			TrailingPercent:         1.0, // 默认回撤1%触发
		}
	}

	if direction == "long" {
		// 多单：更新最高价
		if currentPrice > highestPrice {
			trigger.HighestPrice = currentPrice
		}

		// 计算当前盈利百分比
		profitPercent := (trigger.HighestPrice - entryPrice) / entryPrice * 100

		if profitPercent >= config.ActivationProfitPercent {
			// 激活移动止损
			trigger.IsActivated = true

			// 计算新的止损价（最高价回撤一定百分比）
			trigger.NewStopLoss = trigger.HighestPrice * (1 - config.TrailingPercent/100)

			// 如果新止损价高于当前止损价，更新止损
			if trigger.NewStopLoss > currentStopLoss {
				trigger.ShouldUpdate = true
				trigger.Message = fmt.Sprintf("移动止损: %.4f -> %.4f (盈利: %.2f%%)",
					currentStopLoss, trigger.NewStopLoss, profitPercent)
			}

			// 检查是否触发止损
			if currentPrice <= trigger.NewStopLoss {
				trigger.ShouldTrigger = true
				trigger.Message = fmt.Sprintf("移动止损触发: 当前价 %.4f <= 止损价 %.4f",
					currentPrice, trigger.NewStopLoss)
			}
		}
	} else if direction == "short" {
		// 空单：更新最低价
		if highestPrice == 0 || currentPrice < highestPrice {
			trigger.HighestPrice = currentPrice
		}

		// 计算当前盈利百分比
		profitPercent := (entryPrice - trigger.HighestPrice) / entryPrice * 100

		if profitPercent >= config.ActivationProfitPercent {
			// 激活移动止损
			trigger.IsActivated = true

			// 计算新的止损价（最低价上涨一定百分比）
			trigger.NewStopLoss = trigger.HighestPrice * (1 + config.TrailingPercent/100)

			// 如果新止损价低于当前止损价，更新止损
			if trigger.NewStopLoss < currentStopLoss {
				trigger.ShouldUpdate = true
				trigger.Message = fmt.Sprintf("移动止损: %.4f -> %.4f (盈利: %.2f%%)",
					currentStopLoss, trigger.NewStopLoss, profitPercent)
			}

			// 检查是否触发止损
			if currentPrice >= trigger.NewStopLoss {
				trigger.ShouldTrigger = true
				trigger.Message = fmt.Sprintf("移动止损触发: 当前价 %.4f >= 止损价 %.4f",
					currentPrice, trigger.NewStopLoss)
			}
		}
	}

	return trigger
}

// GetStatus 获取规则执行器状态
func (re *RuleExecutor) GetStatus() map[string]interface{} {
	re.mu.RLock()
	defer re.mu.RUnlock()

	dailyPnL := re.accountEquity - re.dailyStartEquity
	dailyPnLPercent := dailyPnL / re.dailyStartEquity * 100
	drawdown := re.historicalHighEquity - re.accountEquity
	drawdownPercent := drawdown / re.historicalHighEquity * 100

	status := map[string]interface{}{
		"is_trading_halted":       re.isTradingHalted,
		"halt_reason":             re.haltReason,
		"account_equity":          re.accountEquity,
		"daily_start_equity":      re.dailyStartEquity,
		"historical_high_equity":  re.historicalHighEquity,
		"daily_pnl":               dailyPnL,
		"daily_pnl_percent":       dailyPnLPercent,
		"drawdown":                drawdown,
		"drawdown_percent":        drawdownPercent,
		"max_daily_loss_percent":  re.maxDailyLossPercent,
		"max_drawdown_percent":    re.maxDrawdownPercent,
	}

	if re.isTradingHalted {
		status["halted_at"] = re.haltedAt
		status["can_resume_at"] = re.canResumeAt
		status["remaining_minutes"] = int(time.Until(re.canResumeAt).Minutes())
	}

	return status
}

// ResetDailyStats 重置日统计（手动调用，用于测试或特殊情况）
func (re *RuleExecutor) ResetDailyStats() {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.dailyStartEquity = re.accountEquity
	re.lastResetTime = time.Now()
}
