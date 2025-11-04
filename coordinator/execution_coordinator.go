package coordinator

import (
	"fmt"
	"nofx/foundation"
	"nofx/intelligence"
	"time"
)

// ExecutionCoordinator 上层执行协调器
// 职责：
// 1. 接收AI决策
// 2. 根据AI决策计算具体交易参数（杠杆、仓位大小、止损止盈价格）
// 3. 二次风控验证
// 4. 发送订单到底层执行
type ExecutionCoordinator struct {
	// 底层组件
	riskCalculator *foundation.RiskCalculator
	ruleExecutor   *foundation.RuleExecutor
	dataProcessor  *foundation.DataProcessor

	// 配置
	config *CoordinatorConfig
}

// CoordinatorConfig 协调器配置
type CoordinatorConfig struct {
	// 杠杆配置
	BTCETHMinLeverage int // BTC/ETH最小杠杆
	BTCETHMaxLeverage int // BTC/ETH最大杠杆
	AltcoinMinLeverage int // 山寨币最小杠杆
	AltcoinMaxLeverage int // 山寨币最大杠杆

	// 仓位配置
	MaxPositionCount int // 最大持仓数量

	// 风控配置
	MaxRiskPercentPerTrade float64 // 单笔最大风险百分比
	MaxMarginUsagePercent  float64 // 最大保证金使用率
	RewardRiskRatio        float64 // 风险回报比

	// ATR配置
	ATRMultiplier float64 // ATR倍数（用于计算止损）

	// 资产类型映射
	BTCETHSymbols map[string]bool // BTC/ETH符号集合
}

// NewExecutionCoordinator 创建执行协调器实例
func NewExecutionCoordinator(
	accountEquity float64,
	config *CoordinatorConfig,
) *ExecutionCoordinator {
	if config == nil {
		config = getDefaultCoordinatorConfig()
	}

	riskCalculator := foundation.NewRiskCalculator(
		accountEquity,
		config.MaxRiskPercentPerTrade,
		config.MaxMarginUsagePercent,
	)

	ruleExecutor := foundation.NewRuleExecutor(
		accountEquity,
		10.0, // 最大日亏损10%
		20.0, // 最大回撤20%
	)

	dataProcessor := foundation.NewDataProcessor()

	return &ExecutionCoordinator{
		riskCalculator: riskCalculator,
		ruleExecutor:   ruleExecutor,
		dataProcessor:  dataProcessor,
		config:         config,
	}
}

// getDefaultCoordinatorConfig 获取默认配置
func getDefaultCoordinatorConfig() *CoordinatorConfig {
	return &CoordinatorConfig{
		BTCETHMinLeverage:      1,
		BTCETHMaxLeverage:      50,
		AltcoinMinLeverage:     1,
		AltcoinMaxLeverage:     20,
		MaxPositionCount:       3,
		MaxRiskPercentPerTrade: 2.0,
		MaxMarginUsagePercent:  90.0,
		RewardRiskRatio:        3.0,
		ATRMultiplier:          1.5,
		BTCETHSymbols: map[string]bool{
			"BTCUSDT":  true,
			"ETHUSDT":  true,
			"BTCUSD":   true,
			"ETHUSD":   true,
		},
	}
}

// ExecutionPlan 执行计划（由AI决策转换而来）
type ExecutionPlan struct {
	Symbol string
	Action string // "open_long", "open_short", "close_long", "close_short", "hold"

	// 由AI提供的决策信息
	AIConfidence    float64
	AIRiskLevel     string
	AIReasoning     string
	AITiming        string

	// 由协调器计算的执行参数
	Leverage      int
	QuantityUSD   float64
	QuantityBase  float64
	StopLossPrice float64
	TakeProfitPrice float64
	MarginNeeded  float64

	// 风控检查结果
	PassedRiskCheck bool
	RiskCheckIssues []string

	// 状态
	Status string // "pending", "approved", "rejected", "executed", "failed"
	ExecutedAt time.Time
	OrderID    string
}

// ConvertAIDecisionToPlans 将AI决策转换为执行计划
func (ec *ExecutionCoordinator) ConvertAIDecisionToPlans(
	aiDecision *intelligence.SimplifiedAIDecision,
	currentAccount AccountState,
	marketData map[string]MarketData,
) ([]*ExecutionPlan, error) {
	plans := make([]*ExecutionPlan, 0)

	// 首先检查是否允许交易
	ruleCheck := ec.ruleExecutor.CheckTradingRules(currentAccount.AccountEquity)
	if !ruleCheck.IsTradingAllowed {
		return nil, fmt.Errorf("trading halted: %s", ruleCheck.HaltReason)
	}

	// 更新风险计算器的账户净值
	ec.riskCalculator.UpdateAccountEquity(currentAccount.AccountEquity)

	// 处理平仓建议
	for _, exitSuggestion := range aiDecision.StrategyAdvice.ExitSuggestions {
		plan := ec.createExitPlan(exitSuggestion, currentAccount)
		if plan != nil {
			plans = append(plans, plan)
		}
	}

	// 处理开仓机会
	for _, opportunity := range aiDecision.Opportunities {
		// 只处理信心度≥0.7的机会
		if opportunity.Confidence < 0.7 {
			continue
		}

		// 只处理immediate时机的机会
		if opportunity.Timing != "immediate" {
			continue
		}

		// 只处理long和short方向
		if opportunity.Direction != "long" && opportunity.Direction != "short" {
			continue
		}

		plan, err := ec.createOpenPlan(opportunity, currentAccount, marketData)
		if err != nil {
			// 记录错误但继续处理其他机会
			fmt.Printf("Failed to create plan for %s: %v\n", opportunity.Symbol, err)
			continue
		}

		plans = append(plans, plan)
	}

	return plans, nil
}

// createExitPlan 创建平仓计划
func (ec *ExecutionCoordinator) createExitPlan(
	exitSuggestion intelligence.ExitSuggestion,
	currentAccount AccountState,
) *ExecutionPlan {
	// 查找对应的持仓
	var position *PositionInfo
	for i := range currentAccount.Positions {
		if currentAccount.Positions[i].Symbol == exitSuggestion.Symbol {
			position = &currentAccount.Positions[i]
			break
		}
	}

	if position == nil {
		return nil // 未找到持仓
	}

	action := ""
	if position.Direction == "long" {
		action = "close_long"
	} else if position.Direction == "short" {
		action = "close_short"
	} else {
		return nil
	}

	plan := &ExecutionPlan{
		Symbol:          exitSuggestion.Symbol,
		Action:          action,
		AIConfidence:    exitSuggestion.Confidence,
		AIReasoning:     exitSuggestion.Reason,
		QuantityBase:    position.QuantityBase,
		QuantityUSD:     position.PositionSizeUSD,
		PassedRiskCheck: true, // 平仓默认通过风控
		Status:          "approved",
	}

	return plan
}

// createOpenPlan 创建开仓计划
func (ec *ExecutionCoordinator) createOpenPlan(
	opportunity intelligence.TradingOpportunity,
	currentAccount AccountState,
	marketData map[string]MarketData,
) (*ExecutionPlan, error) {
	// 获取市场数据
	market, exists := marketData[opportunity.Symbol]
	if !exists {
		return nil, fmt.Errorf("market data not found for %s", opportunity.Symbol)
	}

	plan := &ExecutionPlan{
		Symbol:       opportunity.Symbol,
		AIConfidence: opportunity.Confidence,
		AIRiskLevel:  opportunity.RiskLevel,
		AIReasoning:  opportunity.Reasoning,
		AITiming:     opportunity.Timing,
		Status:       "pending",
	}

	// 确定操作类型
	if opportunity.Direction == "long" {
		plan.Action = "open_long"
	} else if opportunity.Direction == "short" {
		plan.Action = "open_short"
	} else {
		return nil, fmt.Errorf("invalid direction: %s", opportunity.Direction)
	}

	// 1. 检查持仓数量限制
	if currentAccount.PositionCount >= ec.config.MaxPositionCount {
		plan.Status = "rejected"
		plan.PassedRiskCheck = false
		plan.RiskCheckIssues = append(plan.RiskCheckIssues,
			fmt.Sprintf("持仓数量已达上限: %d/%d", currentAccount.PositionCount, ec.config.MaxPositionCount))
		return plan, nil
	}

	// 2. 确定资产类型并选择杠杆
	assetType := ec.getAssetType(opportunity.Symbol)
	leverage := ec.calculateLeverage(opportunity, assetType)
	plan.Leverage = leverage

	// 3. 计算止损价格
	stopLossParams, err := ec.riskCalculator.CalculateStopLoss(
		opportunity.Direction,
		market.CurrentPrice,
		market.ATR,
		ec.config.ATRMultiplier,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate stop loss: %v", err)
	}
	plan.StopLossPrice = stopLossParams.Price

	// 4. 计算止盈价格
	takeProfitPrice, err := ec.riskCalculator.CalculateTakeProfit(
		opportunity.Direction,
		market.CurrentPrice,
		plan.StopLossPrice,
		ec.config.RewardRiskRatio,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate take profit: %v", err)
	}
	plan.TakeProfitPrice = takeProfitPrice

	// 5. 计算仓位大小
	positionSizeParams, err := ec.riskCalculator.CalculatePositionSize(
		opportunity.Direction,
		market.CurrentPrice,
		plan.StopLossPrice,
		leverage,
		currentAccount.MarginUsed,
		opportunity.Confidence,
	)
	if err != nil {
		plan.Status = "rejected"
		plan.PassedRiskCheck = false
		plan.RiskCheckIssues = append(plan.RiskCheckIssues, err.Error())
		return plan, nil
	}

	plan.QuantityUSD = positionSizeParams.QuantityUSD
	plan.QuantityBase = positionSizeParams.QuantityBase
	plan.MarginNeeded = positionSizeParams.MarginNeeded

	// 6. 二次风控验证
	riskCheckResult := ec.performRiskCheck(plan, currentAccount, market)
	plan.PassedRiskCheck = riskCheckResult.Passed
	plan.RiskCheckIssues = riskCheckResult.Issues

	if plan.PassedRiskCheck {
		plan.Status = "approved"
	} else {
		plan.Status = "rejected"
	}

	return plan, nil
}

// getAssetType 获取资产类型
func (ec *ExecutionCoordinator) getAssetType(symbol string) string {
	if ec.config.BTCETHSymbols[symbol] {
		return "btc_eth"
	}
	return "altcoin"
}

// calculateLeverage 计算杠杆倍数
// 根据AI的风险等级和信心度调整杠杆
func (ec *ExecutionCoordinator) calculateLeverage(
	opportunity intelligence.TradingOpportunity,
	assetType string,
) int {
	var minLeverage, maxLeverage int

	if assetType == "btc_eth" {
		minLeverage = ec.config.BTCETHMinLeverage
		maxLeverage = ec.config.BTCETHMaxLeverage
	} else {
		minLeverage = ec.config.AltcoinMinLeverage
		maxLeverage = ec.config.AltcoinMaxLeverage
	}

	// 基础杠杆：根据资产类型
	baseLeverage := (minLeverage + maxLeverage) / 2

	// 根据信心度调整（0.7-1.0 -> 0.7-1.0倍调整）
	confidenceMultiplier := opportunity.Confidence

	// 根据风险等级调整
	riskMultiplier := 1.0
	switch opportunity.RiskLevel {
	case "low":
		riskMultiplier = 1.2
	case "medium":
		riskMultiplier = 1.0
	case "high":
		riskMultiplier = 0.8
	}

	// 计算最终杠杆
	leverage := int(float64(baseLeverage) * confidenceMultiplier * riskMultiplier)

	// 限制在最小和最大杠杆之间
	if leverage < minLeverage {
		leverage = minLeverage
	}
	if leverage > maxLeverage {
		leverage = maxLeverage
	}

	return leverage
}

// RiskCheckResult 风控检查结果
type RiskCheckResult struct {
	Passed bool
	Issues []string
}

// performRiskCheck 执行二次风控检查
func (ec *ExecutionCoordinator) performRiskCheck(
	plan *ExecutionPlan,
	currentAccount AccountState,
	market MarketData,
) *RiskCheckResult {
	result := &RiskCheckResult{
		Passed: true,
		Issues: []string{},
	}

	// 检查1：保证金是否充足
	valid, availableMargin := ec.riskCalculator.ValidateMarginRequirement(
		currentAccount.MarginUsed,
		plan.MarginNeeded,
	)
	if !valid {
		result.Passed = false
		result.Issues = append(result.Issues,
			fmt.Sprintf("保证金不足: 需要 $%.2f, 可用 $%.2f", plan.MarginNeeded, availableMargin))
	}

	// 检查2：止损距离是否合理（不能太近或太远）
	stopDistancePercent := 0.0
	if plan.Action == "open_long" {
		stopDistancePercent = (market.CurrentPrice - plan.StopLossPrice) / market.CurrentPrice * 100
	} else {
		stopDistancePercent = (plan.StopLossPrice - market.CurrentPrice) / market.CurrentPrice * 100
	}

	if stopDistancePercent < 0.5 {
		result.Passed = false
		result.Issues = append(result.Issues,
			fmt.Sprintf("止损距离过近: %.2f%% (最小0.5%%)", stopDistancePercent))
	}
	if stopDistancePercent > 10.0 {
		result.Passed = false
		result.Issues = append(result.Issues,
			fmt.Sprintf("止损距离过远: %.2f%% (最大10%%)", stopDistancePercent))
	}

	// 检查3：仓位大小是否在合理范围内
	minPositionValue, maxPositionValue := ec.riskCalculator.GetMaxPositionValue(
		ec.getAssetType(plan.Symbol),
	)
	if plan.QuantityUSD < minPositionValue*0.5 {
		result.Passed = false
		result.Issues = append(result.Issues,
			fmt.Sprintf("仓位过小: $%.2f (最小 $%.2f)", plan.QuantityUSD, minPositionValue*0.5))
	}
	if plan.QuantityUSD > maxPositionValue*1.2 {
		result.Passed = false
		result.Issues = append(result.Issues,
			fmt.Sprintf("仓位过大: $%.2f (最大 $%.2f)", plan.QuantityUSD, maxPositionValue*1.2))
	}

	// 检查4：杠杆是否合理
	assetType := ec.getAssetType(plan.Symbol)
	leverageValidation := ec.ruleExecutor.ValidateLeverage(plan.Leverage, assetType)
	if !leverageValidation.IsValid {
		// 自动调整杠杆
		plan.Leverage = leverageValidation.AdjustedLeverage
		result.Issues = append(result.Issues, leverageValidation.Message)
	}

	// 检查5：价格数据是否有效
	if market.CurrentPrice <= 0 || market.ATR <= 0 {
		result.Passed = false
		result.Issues = append(result.Issues, "市场数据无效")
	}

	return result
}

// AccountState 账户状态
type AccountState struct {
	AccountEquity   float64
	AvailableBalance float64
	MarginUsed      float64
	PositionCount   int
	Positions       []PositionInfo
}

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol          string
	Direction       string
	EntryPrice      float64
	CurrentPrice    float64
	QuantityBase    float64
	PositionSizeUSD float64
	Leverage        int
	UnrealizedPnL   float64
	StopLossPrice   float64
	TakeProfitPrice float64
}

// MarketData 市场数据
type MarketData struct {
	Symbol       string
	CurrentPrice float64
	ATR          float64
	Volatility   float64
	Volume24h    float64
}

// SortPlansByPriority 按优先级排序执行计划（先平仓，后开仓）
func (ec *ExecutionCoordinator) SortPlansByPriority(plans []*ExecutionPlan) []*ExecutionPlan {
	sorted := make([]*ExecutionPlan, 0, len(plans))

	// 第一组：平仓操作
	for _, plan := range plans {
		if plan.Action == "close_long" || plan.Action == "close_short" {
			sorted = append(sorted, plan)
		}
	}

	// 第二组：开仓操作（按信心度排序）
	openPlans := make([]*ExecutionPlan, 0)
	for _, plan := range plans {
		if plan.Action == "open_long" || plan.Action == "open_short" {
			openPlans = append(openPlans, plan)
		}
	}

	// 按信心度排序开仓计划
	for i := 0; i < len(openPlans); i++ {
		for j := i + 1; j < len(openPlans); j++ {
			if openPlans[j].AIConfidence > openPlans[i].AIConfidence {
				openPlans[i], openPlans[j] = openPlans[j], openPlans[i]
			}
		}
	}

	sorted = append(sorted, openPlans...)

	return sorted
}

// GenerateExecutionReport 生成执行报告
func (ec *ExecutionCoordinator) GenerateExecutionReport(plans []*ExecutionPlan) *ExecutionReport {
	report := &ExecutionReport{
		Timestamp:     time.Now().Format("2006-01-02 15:04:05"),
		TotalPlans:    len(plans),
		ApprovedPlans: 0,
		RejectedPlans: 0,
		Plans:         plans,
	}

	for _, plan := range plans {
		if plan.Status == "approved" {
			report.ApprovedPlans++
		} else if plan.Status == "rejected" {
			report.RejectedPlans++
		}
	}

	return report
}

// ExecutionReport 执行报告
type ExecutionReport struct {
	Timestamp     string
	TotalPlans    int
	ApprovedPlans int
	RejectedPlans int
	Plans         []*ExecutionPlan
}

// UpdateAccountEquity 更新账户净值（动态调整风险参数）
func (ec *ExecutionCoordinator) UpdateAccountEquity(newEquity float64) {
	ec.riskCalculator.UpdateAccountEquity(newEquity)
}

// GetRiskStatus 获取风险状态
func (ec *ExecutionCoordinator) GetRiskStatus(currentEquity float64) map[string]interface{} {
	ruleCheck := ec.ruleExecutor.CheckTradingRules(currentEquity)

	return map[string]interface{}{
		"is_trading_allowed": ruleCheck.IsTradingAllowed,
		"violations":         ruleCheck.Violations,
		"warnings":           ruleCheck.Warnings,
		"halt_reason":        ruleCheck.HaltReason,
		"can_resume_at":      ruleCheck.CanResumeAt,
	}
}
