package trader

import (
	"fmt"
	"time"
)

// RuleEngine 规则引擎 - 三层架构中的底层组件
// 职责：执行硬性风控规则、熔断机制、止损触发检查
// 这些规则是非决策性的，基于明确的触发条件
type RuleEngine struct {
	maxDailyLoss    float64       // 最大日亏损百分比
	maxDrawdown     float64       // 最大回撤百分比
	maxMarginUsage  float64       // 最大保证金使用率（百分比）
	stopTradingTime time.Duration // 触发风控后的暂停时间
}

// NewRuleEngine 创建规则引擎实例
func NewRuleEngine(maxDailyLoss, maxDrawdown, maxMarginUsage float64, stopTradingTime time.Duration) *RuleEngine {
	return &RuleEngine{
		maxDailyLoss:    maxDailyLoss,
		maxDrawdown:     maxDrawdown,
		maxMarginUsage:  maxMarginUsage,
		stopTradingTime: stopTradingTime,
	}
}

// RuleCheckResult 规则检查结果
type RuleCheckResult struct {
	Passed        bool          // 是否通过
	ViolatedRules []string      // 违反的规则
	ShouldStop    bool          // 是否应该停止交易
	StopUntil     time.Time     // 停止交易直到此时间
	Severity      RuleSeverity  // 严重程度
}

// RuleSeverity 规则严重程度
type RuleSeverity string

const (
	SeverityNone     RuleSeverity = "none"     // 无违规
	SeverityWarning  RuleSeverity = "warning"  // 警告
	SeverityCritical RuleSeverity = "critical" // 严重（需要停止交易）
)

// AccountRiskParams 账户风险参数
type AccountRiskParams struct {
	InitialBalance    float64 // 初始余额
	CurrentEquity     float64 // 当前净值
	DailyPnL          float64 // 日盈亏
	TotalPnL          float64 // 总盈亏
	MarginUsedPercent float64 // 保证金使用率（百分比）
	PositionCount     int     // 持仓数量
}

// CheckAccountRisk 检查账户级别的风险规则
func (re *RuleEngine) CheckAccountRisk(params AccountRiskParams) *RuleCheckResult {
	result := &RuleCheckResult{
		Passed:        true,
		ViolatedRules: []string{},
		ShouldStop:    false,
		Severity:      SeverityNone,
	}

	// 规则1：检查日亏损
	if params.InitialBalance > 0 {
		dailyLossPercent := (params.DailyPnL / params.InitialBalance) * 100
		if dailyLossPercent < -re.maxDailyLoss {
			result.Passed = false
			result.ViolatedRules = append(result.ViolatedRules,
				fmt.Sprintf("日亏损超限: %.2f%% (上限: %.2f%%)", -dailyLossPercent, re.maxDailyLoss))
			result.ShouldStop = true
			result.Severity = SeverityCritical
			result.StopUntil = time.Now().Add(re.stopTradingTime)
		}
	}

	// 规则2：检查最大回撤
	if params.InitialBalance > 0 {
		totalPnLPercent := (params.TotalPnL / params.InitialBalance) * 100
		if totalPnLPercent < -re.maxDrawdown {
			result.Passed = false
			result.ViolatedRules = append(result.ViolatedRules,
				fmt.Sprintf("回撤超限: %.2f%% (上限: %.2f%%)", -totalPnLPercent, re.maxDrawdown))
			result.ShouldStop = true
			result.Severity = SeverityCritical
			result.StopUntil = time.Now().Add(re.stopTradingTime)
		}
	}

	// 规则3：检查保证金使用率
	if params.MarginUsedPercent > re.maxMarginUsage {
		result.Passed = false
		result.ViolatedRules = append(result.ViolatedRules,
			fmt.Sprintf("保证金使用率超限: %.2f%% (上限: %.2f%%)", params.MarginUsedPercent, re.maxMarginUsage))
		result.Severity = SeverityWarning
	}

	// 规则4：检查持仓数量（最多3个）
	if params.PositionCount > 3 {
		result.Passed = false
		result.ViolatedRules = append(result.ViolatedRules,
			fmt.Sprintf("持仓数量超限: %d (上限: 3)", params.PositionCount))
		result.Severity = SeverityWarning
	}

	return result
}

// PositionRiskParams 持仓风险参数
type PositionRiskParams struct {
	Symbol            string  // 币种
	Side              string  // 方向 (long/short)
	EntryPrice        float64 // 入场价格
	CurrentPrice      float64 // 当前价格
	StopLossPrice     float64 // 止损价格
	LiquidationPrice  float64 // 强平价格
	UnrealizedPnLPct  float64 // 未实现盈亏百分比
	HoldingTimeMinutes int     // 持仓时长（分钟）
}

// CheckPositionRisk 检查单个持仓的风险规则
func (re *RuleEngine) CheckPositionRisk(params PositionRiskParams) *RuleCheckResult {
	result := &RuleCheckResult{
		Passed:        true,
		ViolatedRules: []string{},
		ShouldStop:    false,
		Severity:      SeverityNone,
	}

	// 规则1：检查是否触及止损
	stopLossTriggered := false
	if params.Side == "long" && params.CurrentPrice <= params.StopLossPrice {
		stopLossTriggered = true
	} else if params.Side == "short" && params.CurrentPrice >= params.StopLossPrice {
		stopLossTriggered = true
	}

	if stopLossTriggered {
		result.Passed = false
		result.ViolatedRules = append(result.ViolatedRules,
			fmt.Sprintf("%s 触发止损: 当前价格 %.4f, 止损价格 %.4f", params.Symbol, params.CurrentPrice, params.StopLossPrice))
		result.Severity = SeverityWarning
	}

	// 规则2：检查是否接近强平价
	// 如果当前价格距离强平价小于20%的距离，发出警告
	if params.LiquidationPrice > 0 {
		var distanceToLiquidation float64
		if params.Side == "long" {
			distanceToLiquidation = ((params.CurrentPrice - params.LiquidationPrice) / params.EntryPrice) * 100
		} else {
			distanceToLiquidation = ((params.LiquidationPrice - params.CurrentPrice) / params.EntryPrice) * 100
		}

		if distanceToLiquidation < 5 { // 距离强平小于5%
			result.Passed = false
			result.ViolatedRules = append(result.ViolatedRules,
				fmt.Sprintf("%s 接近强平价: 距离 %.2f%%", params.Symbol, distanceToLiquidation))
			result.Severity = SeverityCritical
		}
	}

	// 规则3：检查持仓时长过长（超过24小时）
	if params.HoldingTimeMinutes > 24*60 {
		result.ViolatedRules = append(result.ViolatedRules,
			fmt.Sprintf("%s 持仓时长过长: %d 小时", params.Symbol, params.HoldingTimeMinutes/60))
		result.Severity = SeverityWarning
	}

	return result
}

// OpenPositionRiskParams 开仓风险检查参数
type OpenPositionRiskParams struct {
	Symbol           string  // 币种
	Side             string  // 方向
	PositionSizeUSD  float64 // 仓位大小（美元）
	Leverage         int     // 杠杆倍数
	AccountEquity    float64 // 账户净值
	CurrentPositions int     // 当前持仓数量
	AvailableMargin  float64 // 可用保证金
	IsBTCOrETH       bool    // 是否为BTC或ETH
	MaxBTCETHLeverage int    // BTC/ETH最大杠杆
	MaxAltcoinLeverage int   // 山寨币最大杠杆
}

// CheckOpenPositionRisk 检查开仓前的风险规则
func (re *RuleEngine) CheckOpenPositionRisk(params OpenPositionRiskParams) *RuleCheckResult {
	result := &RuleCheckResult{
		Passed:        true,
		ViolatedRules: []string{},
		ShouldStop:    false,
		Severity:      SeverityNone,
	}

	// 规则1：检查持仓数量
	if params.CurrentPositions >= 3 {
		result.Passed = false
		result.ViolatedRules = append(result.ViolatedRules, "持仓数量已达上限3个")
		result.Severity = SeverityWarning
		return result
	}

	// 规则2：检查杠杆倍数
	maxLeverage := params.MaxAltcoinLeverage
	if params.IsBTCOrETH {
		maxLeverage = params.MaxBTCETHLeverage
	}

	if params.Leverage > maxLeverage {
		result.Passed = false
		result.ViolatedRules = append(result.ViolatedRules,
			fmt.Sprintf("杠杆倍数超限: %d倍 (上限: %d倍)", params.Leverage, maxLeverage))
		result.Severity = SeverityWarning
	}

	// 规则3：检查仓位大小
	var maxPositionValue float64
	if params.IsBTCOrETH {
		maxPositionValue = params.AccountEquity * 10 // BTC/ETH最多10倍账户净值
	} else {
		maxPositionValue = params.AccountEquity * 1.5 // 山寨币最多1.5倍账户净值
	}

	if params.PositionSizeUSD > maxPositionValue {
		result.Passed = false
		coinType := "山寨币"
		if params.IsBTCOrETH {
			coinType = "BTC/ETH"
		}
		result.ViolatedRules = append(result.ViolatedRules,
			fmt.Sprintf("%s仓位大小超限: %.0f USDT (上限: %.0f USDT)", coinType, params.PositionSizeUSD, maxPositionValue))
		result.Severity = SeverityWarning
	}

	// 规则4：检查可用保证金
	requiredMargin := params.PositionSizeUSD / float64(params.Leverage)
	if requiredMargin > params.AvailableMargin {
		result.Passed = false
		result.ViolatedRules = append(result.ViolatedRules,
			fmt.Sprintf("可用保证金不足: 需要 %.0f USDT, 可用 %.0f USDT", requiredMargin, params.AvailableMargin))
		result.Severity = SeverityCritical
	}

	return result
}

// CircuitBreakerParams 熔断机制参数
type CircuitBreakerParams struct {
	RecentLossCount    int     // 最近连续亏损次数
	RecentLossThreshold int     // 连续亏损阈值
	QuickLossPercent   float64 // 快速亏损百分比（如10分钟内亏损5%）
	QuickLossThreshold float64 // 快速亏损阈值
}

// CheckCircuitBreaker 检查熔断机制
// 当连续亏损或快速亏损达到阈值时，触发熔断
func (re *RuleEngine) CheckCircuitBreaker(params CircuitBreakerParams) *RuleCheckResult {
	result := &RuleCheckResult{
		Passed:        true,
		ViolatedRules: []string{},
		ShouldStop:    false,
		Severity:      SeverityNone,
	}

	// 规则1：检查连续亏损次数
	if params.RecentLossCount >= params.RecentLossThreshold {
		result.Passed = false
		result.ViolatedRules = append(result.ViolatedRules,
			fmt.Sprintf("连续亏损触发熔断: %d次 (阈值: %d次)", params.RecentLossCount, params.RecentLossThreshold))
		result.ShouldStop = true
		result.Severity = SeverityCritical
		result.StopUntil = time.Now().Add(re.stopTradingTime)
	}

	// 规则2：检查快速亏损
	if params.QuickLossPercent > params.QuickLossThreshold {
		result.Passed = false
		result.ViolatedRules = append(result.ViolatedRules,
			fmt.Sprintf("快速亏损触发熔断: %.2f%% (阈值: %.2f%%)", params.QuickLossPercent, params.QuickLossThreshold))
		result.ShouldStop = true
		result.Severity = SeverityCritical
		result.StopUntil = time.Now().Add(re.stopTradingTime)
	}

	return result
}
