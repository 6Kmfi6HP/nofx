package integration

import (
	"fmt"
	"log"
	"nofx/coordinator"
	"nofx/decision"
	"nofx/foundation"
	"nofx/intelligence"
	"nofx/market"
	"time"
)

// ThreeLayerAdapter 三层架构集成适配器
// 职责：将新的三层架构集成到现有系统中，保持向后兼容性
type ThreeLayerAdapter struct {
	// 新的三层架构组件
	executionCoordinator *coordinator.ExecutionCoordinator
	aiDecisionEngine     *intelligence.AIDecisionEngine
	dataProcessor        *foundation.DataProcessor

	// 配置
	enableNewArchitecture bool // 是否启用新架构
}

// NewThreeLayerAdapter 创建三层架构适配器
func NewThreeLayerAdapter(accountEquity float64, enableNewArchitecture bool) *ThreeLayerAdapter {
	coordinatorConfig := &coordinator.CoordinatorConfig{
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
			"BTCUSDT": true,
			"ETHUSDT": true,
			"BTCUSD":  true,
			"ETHUSD":  true,
		},
	}

	return &ThreeLayerAdapter{
		executionCoordinator:  coordinator.NewExecutionCoordinator(accountEquity, coordinatorConfig),
		aiDecisionEngine:      intelligence.NewAIDecisionEngine(),
		dataProcessor:         foundation.NewDataProcessor(),
		enableNewArchitecture: enableNewArchitecture,
	}
}

// ConvertFromLegacyContext 从旧的交易上下文转换为新的交易上下文
func (adapter *ThreeLayerAdapter) ConvertFromLegacyContext(
	legacyContext *decision.Context,
	marketDataMap map[string]*market.Data,
) *intelligence.TradingContext {
	newContext := &intelligence.TradingContext{
		CurrentTime:          legacyContext.CurrentTime,
		AccountEquity:        legacyContext.Account.TotalEquity,
		AvailableBalance:     legacyContext.Account.AvailableBalance,
		CurrentPositionCount: legacyContext.Account.PositionCount,
		MarginUsagePercent:   legacyContext.Account.MarginUsedPct,
		Positions:            make([]intelligence.PositionInfo, 0),
		Candidates:           make([]intelligence.CandidateInfo, 0),
	}

	// 获取BTC市场数据
	if btcData, exists := marketDataMap["BTCUSDT"]; exists {
		newContext.BTCPrice = btcData.CurrentPrice
		newContext.BTCChange24h = btcData.PriceChange1h // 使用1小时变化作为近似
		newContext.BTCTrend = detectSimpleTrend(btcData)
	}

	// 转换持仓信息
	for _, legacyPos := range legacyContext.Positions {
		holdingTime := calculateHoldingTime(legacyPos.UpdateTime)
		newPos := intelligence.PositionInfo{
			Symbol:               legacyPos.Symbol,
			Direction:            legacyPos.Side,
			EntryPrice:           legacyPos.EntryPrice,
			CurrentPrice:         legacyPos.MarkPrice,
			UnrealizedPnL:        legacyPos.UnrealizedPnL,
			UnrealizedPnLPercent: legacyPos.UnrealizedPnLPct,
			HoldingTime:          holdingTime,
		}
		newContext.Positions = append(newContext.Positions, newPos)
	}

	// 转换候选币种信息
	for _, candidate := range legacyContext.CandidateCoins {
		if mData, exists := marketDataMap[candidate.Symbol]; exists {
			newCandidate := intelligence.CandidateInfo{
				Symbol:       candidate.Symbol,
				CurrentPrice: mData.CurrentPrice,
				Change1h:     mData.PriceChange1h,
				Change4h:     mData.PriceChange4h,
				Change24h:    mData.PriceChange1h, // 近似值
				Volume24h:    0,                   // 需要从其他地方获取
				Volatility:   mData.ATR / mData.CurrentPrice * 100,
				Trend:        detectSimpleTrend(mData),
				RSI:          mData.RSI,
				MACD:         formatMACDStatus(mData),
			}
			newContext.Candidates = append(newContext.Candidates, newCandidate)
		}
	}

	return newContext
}

// ConvertToLegacyDecisions 将新的AI决策转换为旧的决策格式
func (adapter *ThreeLayerAdapter) ConvertToLegacyDecisions(
	simplifiedDecision *intelligence.SimplifiedAIDecision,
	executionPlans []*coordinator.ExecutionPlan,
) []decision.Decision {
	legacyDecisions := make([]decision.Decision, 0)

	// 转换执行计划为旧的决策格式
	for _, plan := range executionPlans {
		if plan.Status != "approved" {
			continue // 只转换已批准的计划
		}

		legacyDecision := decision.Decision{
			Symbol:          plan.Symbol,
			Action:          plan.Action,
			Leverage:        plan.Leverage,
			PositionSizeUSD: plan.QuantityUSD,
			StopLoss:        plan.StopLossPrice,
			TakeProfit:      plan.TakeProfitPrice,
			Confidence:      int(plan.AIConfidence * 100),
			RiskUSD:         plan.MarginNeeded,
			Reasoning:       plan.AIReasoning,
		}

		legacyDecisions = append(legacyDecisions, legacyDecision)
	}

	return legacyDecisions
}

// ProcessWithNewArchitecture 使用新架构处理交易决策
// 这是新架构的入口点，返回兼容旧系统的决策列表
func (adapter *ThreeLayerAdapter) ProcessWithNewArchitecture(
	legacyContext *decision.Context,
	marketDataMap map[string]*market.Data,
	aiResponse string, // AI的原始响应
) ([]decision.Decision, string, error) {
	log.Printf("[三层架构] 开始处理交易决策")

	// 步骤1：底层 - 数据验证和清洗
	log.Printf("[底层] 验证市场数据质量...")
	dataQualityOK := adapter.validateMarketData(marketDataMap)
	if !dataQualityOK {
		return nil, "", fmt.Errorf("市场数据质量检查失败")
	}

	// 步骤2：中间AI层 - 解析AI决策
	log.Printf("[中间AI层] 解析AI决策...")
	simplifiedDecision, err := adapter.aiDecisionEngine.ParseAIResponse(aiResponse)
	if err != nil {
		log.Printf("[中间AI层] AI决策解析失败，降级使用传统模式: %v", err)
		// 降级：如果解析失败，返回空决策
		return []decision.Decision{}, "AI决策解析失败，本周期暂停交易", nil
	}

	// 验证AI决策
	err = adapter.aiDecisionEngine.ValidateAIDecision(simplifiedDecision)
	if err != nil {
		log.Printf("[中间AI层] AI决策验证失败: %v", err)
		return []decision.Decision{}, fmt.Sprintf("AI决策验证失败: %v", err), nil
	}

	log.Printf("[中间AI层] 市场状态: %s, 交易机会数: %d",
		simplifiedDecision.MarketState.TrendType,
		len(simplifiedDecision.Opportunities))

	// 步骤3：上层 - 转换为执行计划并进行风控验证
	log.Printf("[上层] 转换AI决策为执行计划...")

	// 转换上下文
	newContext := adapter.ConvertFromLegacyContext(legacyContext, marketDataMap)

	// 准备账户状态
	accountState := coordinator.AccountState{
		AccountEquity:    legacyContext.Account.TotalEquity,
		AvailableBalance: legacyContext.Account.AvailableBalance,
		MarginUsed:       legacyContext.Account.MarginUsed,
		PositionCount:    legacyContext.Account.PositionCount,
		Positions:        adapter.convertPositions(legacyContext.Positions),
	}

	// 准备市场数据
	marketData := adapter.convertMarketData(marketDataMap)

	// 转换AI决策为执行计划
	executionPlans, err := adapter.executionCoordinator.ConvertAIDecisionToPlans(
		simplifiedDecision,
		accountState,
		marketData,
	)
	if err != nil {
		log.Printf("[上层] 执行计划转换失败: %v", err)
		return []decision.Decision{}, fmt.Sprintf("执行计划转换失败: %v", err), nil
	}

	log.Printf("[上层] 生成 %d 个执行计划", len(executionPlans))

	// 步骤4：上层 - 排序执行计划（先平仓后开仓）
	sortedPlans := adapter.executionCoordinator.SortPlansByPriority(executionPlans)

	// 步骤5：上层 - 生成执行报告
	executionReport := adapter.executionCoordinator.GenerateExecutionReport(sortedPlans)
	log.Printf("[上层] 批准: %d, 拒绝: %d", executionReport.ApprovedPlans, executionReport.RejectedPlans)

	// 打印风控拒绝的原因
	for _, plan := range sortedPlans {
		if plan.Status == "rejected" {
			log.Printf("[上层] %s %s 被拒绝: %v", plan.Symbol, plan.Action, plan.RiskCheckIssues)
		}
	}

	// 步骤6：转换为旧格式的决策
	legacyDecisions := adapter.ConvertToLegacyDecisions(simplifiedDecision, sortedPlans)

	// 构建思维链（包含三层架构的决策过程）
	thinkingChain := adapter.buildThinkingChain(simplifiedDecision, executionReport)

	log.Printf("[三层架构] 处理完成，返回 %d 个交易决策", len(legacyDecisions))

	return legacyDecisions, thinkingChain, nil
}

// validateMarketData 底层数据验证
func (adapter *ThreeLayerAdapter) validateMarketData(marketDataMap map[string]*market.Data) bool {
	// 简单验证：确保BTC数据存在且有效
	btcData, exists := marketDataMap["BTCUSDT"]
	if !exists {
		log.Printf("[底层] BTC数据不存在")
		return false
	}

	if btcData.CurrentPrice <= 0 || btcData.ATR <= 0 {
		log.Printf("[底层] BTC数据无效: 价格=%.2f, ATR=%.2f", btcData.CurrentPrice, btcData.ATR)
		return false
	}

	// 验证其他币种数据
	validCount := 0
	for symbol, data := range marketDataMap {
		if data.CurrentPrice > 0 && data.ATR > 0 {
			validCount++
		} else {
			log.Printf("[底层] %s 数据无效", symbol)
		}
	}

	log.Printf("[底层] 数据验证通过: %d/%d 币种有效", validCount, len(marketDataMap))
	return validCount > 0
}

// convertPositions 转换持仓格式
func (adapter *ThreeLayerAdapter) convertPositions(legacyPositions []decision.PositionInfo) []coordinator.PositionInfo {
	positions := make([]coordinator.PositionInfo, 0)
	for _, pos := range legacyPositions {
		positions = append(positions, coordinator.PositionInfo{
			Symbol:          pos.Symbol,
			Direction:       pos.Side,
			EntryPrice:      pos.EntryPrice,
			CurrentPrice:    pos.MarkPrice,
			QuantityBase:    pos.Quantity,
			PositionSizeUSD: pos.Quantity * pos.MarkPrice,
			Leverage:        pos.Leverage,
			UnrealizedPnL:   pos.UnrealizedPnL,
			StopLossPrice:   0, // 旧系统未提供
			TakeProfitPrice: 0, // 旧系统未提供
		})
	}
	return positions
}

// convertMarketData 转换市场数据格式
func (adapter *ThreeLayerAdapter) convertMarketData(marketDataMap map[string]*market.Data) map[string]coordinator.MarketData {
	result := make(map[string]coordinator.MarketData)
	for symbol, data := range marketDataMap {
		result[symbol] = coordinator.MarketData{
			Symbol:       symbol,
			CurrentPrice: data.CurrentPrice,
			ATR:          data.ATR,
			Volatility:   data.ATR / data.CurrentPrice * 100,
			Volume24h:    0, // 旧系统未提供
		}
	}
	return result
}

// buildThinkingChain 构建思维链
func (adapter *ThreeLayerAdapter) buildThinkingChain(
	simplifiedDecision *intelligence.SimplifiedAIDecision,
	executionReport *coordinator.ExecutionReport,
) string {
	chain := fmt.Sprintf(`=== 三层架构交易决策 ===

【中间AI层 - 市场状态判断】
- 趋势类型: %s
- 市场阶段: %s
- 波动性: %s
- 市场情绪: %s
- 市场健康度: %.1f/100
- 状态描述: %s

【中间AI层 - 交易机会识别】
识别到 %d 个交易机会:
`,
		simplifiedDecision.MarketState.TrendType,
		simplifiedDecision.MarketState.Phase,
		simplifiedDecision.MarketState.Volatility,
		simplifiedDecision.MarketState.Sentiment,
		simplifiedDecision.MarketState.HealthScore,
		simplifiedDecision.MarketState.Description,
		len(simplifiedDecision.Opportunities),
	)

	for i, opp := range simplifiedDecision.Opportunities {
		chain += fmt.Sprintf("%d. %s %s (信心度: %.2f, 风险: %s)\n   时机: %s\n   理由: %s\n\n",
			i+1, opp.Symbol, opp.Direction, opp.Confidence, opp.RiskLevel, opp.Timing, opp.Reasoning)
	}

	chain += fmt.Sprintf(`
【中间AI层 - 策略建议】
- 建议操作: %s
- 仓位管理: %s
- 风险偏好: %s
%s

【上层代码层 - 参数计算与风控验证】
总计划数: %d
- 批准: %d 个
- 拒绝: %d 个

`,
		simplifiedDecision.StrategyAdvice.SuggestedAction,
		simplifiedDecision.StrategyAdvice.PositionManagement,
		simplifiedDecision.StrategyAdvice.RiskAppetite,
		simplifiedDecision.StrategyAdvice.SpecialNotes,
		executionReport.TotalPlans,
		executionReport.ApprovedPlans,
		executionReport.RejectedPlans,
	)

	// 添加执行计划详情
	for i, plan := range executionReport.Plans {
		status := "✓"
		if plan.Status == "rejected" {
			status = "✗"
		}
		chain += fmt.Sprintf("%s %d. %s %s | 杠杆: %dx | 仓位: $%.2f | 止损: $%.4f | 止盈: $%.4f\n",
			status, i+1, plan.Symbol, plan.Action, plan.Leverage, plan.QuantityUSD, plan.StopLossPrice, plan.TakeProfitPrice)

		if plan.Status == "rejected" {
			chain += fmt.Sprintf("   拒绝原因: %v\n", plan.RiskCheckIssues)
		}
	}

	chain += "\n=== AI思维链 ===\n" + simplifiedDecision.ThinkingProcess

	return chain
}

// detectSimpleTrend 简单趋势检测
func detectSimpleTrend(data *market.Data) string {
	if data.PriceChange4h > 2 {
		return "up"
	} else if data.PriceChange4h < -2 {
		return "down"
	}
	return "sideways"
}

// calculateHoldingTime 计算持仓时间
func calculateHoldingTime(updateTime int64) string {
	if updateTime == 0 {
		return "未知"
	}

	duration := time.Now().Unix() - updateTime/1000
	hours := duration / 3600
	minutes := (duration % 3600) / 60

	if hours > 24 {
		days := hours / 24
		return fmt.Sprintf("%d天%d小时", days, hours%24)
	} else if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes)
	} else {
		return fmt.Sprintf("%d分钟", minutes)
	}
}

// formatMACDStatus 格式化MACD状态
func formatMACDStatus(data *market.Data) string {
	if data.MACD > 0 {
		return "bullish"
	} else if data.MACD < 0 {
		return "bearish"
	}
	return "neutral"
}

// IsNewArchitectureEnabled 检查是否启用新架构
func (adapter *ThreeLayerAdapter) IsNewArchitectureEnabled() bool {
	return adapter.enableNewArchitecture
}

// UpdateAccountEquity 更新账户净值
func (adapter *ThreeLayerAdapter) UpdateAccountEquity(newEquity float64) {
	adapter.executionCoordinator.UpdateAccountEquity(newEquity)
}

// GetRiskStatus 获取风险状态
func (adapter *ThreeLayerAdapter) GetRiskStatus(currentEquity float64) map[string]interface{} {
	return adapter.executionCoordinator.GetRiskStatus(currentEquity)
}
