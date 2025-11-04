package layers

import (
	"fmt"
	"nofx/layers/ai_layer"
	"nofx/layers/data_layer"
	"nofx/layers/execution_layer"
	"nofx/market"
	"nofx/trader"
	"time"
)

// Orchestrator 三层架构编排器
// 负责协调底层、AI层、执行层的工作流程
type Orchestrator struct {
	config LayerConfig

	// 底层代码层
	dataProcessor  *data_layer.DataProcessor
	riskCalculator *data_layer.RiskCalculator
	orderExecutor  *data_layer.OrderExecutor

	// AI层
	decisionMaker *ai_layer.DecisionMaker

	// 执行层
	paramCalculator *execution_layer.ParameterCalculator
	riskValidator   *execution_layer.RiskValidator
	orderSender     *execution_layer.OrderSender

	// 统计信息
	totalExecutions   int
	successfulTrades  int
	failedTrades      int
	rejectedByRisk    int
}

// NewOrchestrator 创建编排器
func NewOrchestrator(config LayerConfig, tr trader.Trader) (*Orchestrator, error) {
	// 初始化底层
	dataProcessor := data_layer.NewDataProcessor(config.DataLayer)
	riskCalculator := data_layer.NewRiskCalculator(config.DataLayer)
	orderExecutor := data_layer.NewOrderExecutor(config.DataLayer, tr)

	// 初始化AI层
	decisionMaker, err := ai_layer.NewDecisionMaker(config.AILayer)
	if err != nil {
		return nil, fmt.Errorf("failed to create decision maker: %w", err)
	}

	// 初始化执行层
	paramCalculator := execution_layer.NewParameterCalculator(config.ExecutionLayer)
	riskValidator := execution_layer.NewRiskValidator(config.ExecutionLayer)
	orderSender := execution_layer.NewOrderSender(config.ExecutionLayer, orderExecutor)

	return &Orchestrator{
		config:          config,
		dataProcessor:   dataProcessor,
		riskCalculator:  riskCalculator,
		orderExecutor:   orderExecutor,
		decisionMaker:   decisionMaker,
		paramCalculator: paramCalculator,
		riskValidator:   riskValidator,
		orderSender:     orderSender,
		totalExecutions: 0,
		successfulTrades: 0,
		failedTrades:    0,
		rejectedByRisk:  0,
	}, nil
}

// ExecuteTradingCycle 执行完整的交易周期
// 这是三层架构的核心流程：
// 市场数据 → 底层处理 → AI判断 → 上层执行 → 交易所
func (o *Orchestrator) ExecuteTradingCycle(rawMarketData *market.Data) (*TradingCycleResult, error) {
	o.totalExecutions++
	startTime := time.Now()

	result := &TradingCycleResult{
		StartTime: startTime,
		Symbol:    rawMarketData.Symbol,
		Success:   false,
	}

	fmt.Printf("\n========================================\n")
	fmt.Printf("🔄 开始交易周期: %s\n", rawMarketData.Symbol)
	fmt.Printf("========================================\n")

	// ============================================
	// 第一层：底层代码层（数据与执行）
	// ============================================
	fmt.Printf("\n📊 [底层] 数据处理中...\n")

	// 1.1 数据获取和清洗
	cleanedData, err := o.dataProcessor.ProcessMarketData(rawMarketData)
	if err != nil {
		result.Error = fmt.Sprintf("数据处理失败: %v", err)
		return result, err
	}
	result.CleanedData = cleanedData

	fmt.Printf("   ✓ 数据清洗完成 | 质量: %.2f | 摘要长度: %d字符\n",
		cleanedData.DataQuality, len(cleanedData.CompressedSummary))

	// 1.2 获取账户信息
	balance, err := o.orderExecutor.GetAccountBalance()
	if err != nil {
		result.Error = fmt.Sprintf("获取账户信息失败: %v", err)
		return result, err
	}

	totalBalance := balance["total"].(float64)
	availableBalance := balance["available"].(float64)
	usedMargin := balance["used_margin"].(float64)

	o.riskCalculator.UpdateAccountInfo(totalBalance, availableBalance, usedMargin)

	fmt.Printf("   ✓ 账户信息 | 总余额: %.2f | 可用: %.2f | 保证金: %.2f\n",
		totalBalance, availableBalance, usedMargin)

	// ============================================
	// 第二层：AI层（决策与判断）
	// ============================================
	fmt.Printf("\n🤖 [AI层] 智能决策中...\n")

	// 2.1 AI决策（市场状态判断 + 交易机会识别 + 方向和信心度）
	aiDecision, err := o.decisionMaker.MakeDecision(cleanedData)
	if err != nil {
		result.Error = fmt.Sprintf("AI决策失败: %v", err)
		return result, err
	}
	result.AIDecision = aiDecision

	fmt.Printf("   ✓ 市场状态: %s (%s)\n",
		aiDecision.MarketCondition, aiDecision.ConditionReason)
	fmt.Printf("   ✓ 交易机会: %s (%s)\n",
		aiDecision.Opportunity, aiDecision.OpportunityReason)
	fmt.Printf("   ✓ 决策方向: %s | 信心度: %.2f\n",
		aiDecision.Direction, aiDecision.Confidence)
	fmt.Printf("   ✓ AI耗时: %dms\n", aiDecision.ResponseTimeMs)

	// 如果AI决策为观望，则结束流程
	if aiDecision.Direction == DirectionWait {
		result.Success = true
		result.Message = "AI决策：观望，不执行交易"
		result.Duration = time.Since(startTime)
		fmt.Printf("\n⏸️  决策结果：观望\n")
		return result, nil
	}

	// 1.3 风险计算（根据AI决策方向）
	fmt.Printf("\n📊 [底层] 风险计算中...\n")
	riskMetrics, err := o.riskCalculator.CalculateRiskMetrics(aiDecision.Direction, cleanedData)
	if err != nil {
		result.Error = fmt.Sprintf("风险计算失败: %v", err)
		return result, err
	}
	result.RiskMetrics = riskMetrics

	fmt.Printf("   ✓ 风险等级: %s\n", riskMetrics.RiskLevel)
	fmt.Printf("   ✓ 建议杠杆: %dx | 最大仓位: %.2f USD\n",
		riskMetrics.RecommendedLeverage, riskMetrics.MaxPositionSizeUSD)
	fmt.Printf("   ✓ 止损: %.2f | 止盈: %.2f | 最大亏损: %.2f USD\n",
		riskMetrics.StopLossPrice, riskMetrics.TakeProfitPrice, riskMetrics.MaxLossUSD)

	// 检查是否可交易
	if !riskMetrics.CanTrade {
		result.Success = true
		result.Message = fmt.Sprintf("风险检查阻止交易: %s", riskMetrics.RiskReason)
		result.Duration = time.Since(startTime)
		o.rejectedByRisk++
		fmt.Printf("\n❌ 风险检查不通过：%s\n", riskMetrics.RiskReason)
		return result, nil
	}

	// ============================================
	// 第三层：执行层（参数与风控）
	// ============================================
	fmt.Printf("\n⚡ [执行层] 准备交易参数...\n")

	// 3.1 计算交易参数
	params, err := o.paramCalculator.CalculateParameters(aiDecision, riskMetrics, cleanedData)
	if err != nil {
		result.Error = fmt.Sprintf("参数计算失败: %v", err)
		return result, err
	}

	fmt.Printf("   ✓ 交易动作: %s\n", params["action"])
	fmt.Printf("   ✓ 交易数量: %.6f (%.2f USD)\n",
		params["quantity"], params["quantity_usd"])
	fmt.Printf("   ✓ 杠杆: %dx | 优先级: %s\n",
		params["leverage"], params["priority"])

	// 3.2 准备执行计划
	executionPlan := o.orderSender.PrepareExecutionPlan(
		aiDecision, riskMetrics, params, true, "初步检查通过")
	result.ExecutionPlan = executionPlan

	// 3.3 二次风控验证
	fmt.Printf("\n🛡️  [执行层] 二次风控验证...\n")
	riskCheckPassed, riskCheckReason := o.riskValidator.ValidateExecution(
		executionPlan, aiDecision, riskMetrics, cleanedData)

	executionPlan.RiskCheckPassed = riskCheckPassed
	executionPlan.RiskCheckReason = riskCheckReason

	fmt.Printf("   %s 验证结果: %s\n",
		map[bool]string{true: "✓", false: "✗"}[riskCheckPassed],
		riskCheckReason)

	if !riskCheckPassed {
		result.Success = true
		result.Message = fmt.Sprintf("二次风控验证失败: %s", riskCheckReason)
		result.Duration = time.Since(startTime)
		o.rejectedByRisk++
		fmt.Printf("\n❌ 二次风控不通过：%s\n", riskCheckReason)
		return result, nil
	}

	// 3.4 发送订单
	fmt.Printf("\n📤 [执行层] 发送订单到交易所...\n")
	orderResult, err := o.orderSender.SendOrder(executionPlan)
	if err != nil {
		result.Error = fmt.Sprintf("订单发送失败: %v", err)
		o.failedTrades++
		fmt.Printf("\n❌ 订单失败：%v\n", err)
		return result, err
	}
	result.OrderResult = orderResult

	// 更新统计
	if orderResult.Success {
		o.successfulTrades++
		result.Success = true
		result.Message = "交易执行成功"
		fmt.Printf("\n✅ 交易成功！\n")
		fmt.Printf("   订单ID: %s\n", orderResult.OrderID)
		fmt.Printf("   成交量: %.6f\n", orderResult.FilledQuantity)
		fmt.Printf("   执行耗时: %dms\n", orderResult.ExecutionTimeMs)
	} else {
		o.failedTrades++
		result.Error = orderResult.ErrorMessage
		fmt.Printf("\n❌ 交易失败：%s\n", orderResult.ErrorMessage)
	}

	result.Duration = time.Since(startTime)
	fmt.Printf("\n总耗时: %dms\n", result.Duration.Milliseconds())
	fmt.Printf("========================================\n")

	return result, nil
}

// UpdateAccountInfo 更新账户信息
func (o *Orchestrator) UpdateAccountInfo(totalBalance, availableBalance, usedMargin float64) {
	o.riskCalculator.UpdateAccountInfo(totalBalance, availableBalance, usedMargin)
}

// UpdateDailyPnL 更新每日盈亏
func (o *Orchestrator) UpdateDailyPnL(pnl float64) {
	o.riskCalculator.UpdateDailyPnL(pnl)
}

// RecordTradeResult 记录交易结果
func (o *Orchestrator) RecordTradeResult(isWin bool) {
	o.riskCalculator.RecordTradeResult(isWin)
}

// ResetCircuitBreaker 重置熔断器
func (o *Orchestrator) ResetCircuitBreaker() {
	o.riskCalculator.ResetCircuitBreaker()
}

// GetStats 获取统计信息
func (o *Orchestrator) GetStats() map[string]interface{} {
	winRate := 0.0
	totalTrades := o.successfulTrades + o.failedTrades
	if totalTrades > 0 {
		winRate = float64(o.successfulTrades) / float64(totalTrades) * 100
	}

	return map[string]interface{}{
		"total_executions":    o.totalExecutions,
		"successful_trades":   o.successfulTrades,
		"failed_trades":       o.failedTrades,
		"rejected_by_risk":    o.rejectedByRisk,
		"win_rate":            winRate,
		"circuit_breaker":     o.riskCalculator.GetCircuitBreakerStatus(),
		"account_risk":        o.riskCalculator.GetAccountRiskSummary(),
		"validation_stats":    o.riskValidator.GetValidationStats(),
		"rate_limit_status":   o.decisionMaker.GetRateLimitStatus(),
	}
}

// TradingCycleResult 交易周期结果
type TradingCycleResult struct {
	StartTime     time.Time
	Duration      time.Duration
	Symbol        string
	Success       bool
	Message       string
	Error         string

	// 各层的输出
	CleanedData   *CleanedMarketData
	AIDecision    *AIDecision
	RiskMetrics   *RiskMetrics
	ExecutionPlan *ExecutionPlan
	OrderResult   *OrderResult
}

// Summary 生成结果摘要
func (r *TradingCycleResult) Summary() string {
	status := "✅ 成功"
	if !r.Success {
		status = "❌ 失败"
	}

	summary := fmt.Sprintf(
		"%s | %s | 耗时: %dms",
		status, r.Symbol, r.Duration.Milliseconds())

	if r.AIDecision != nil {
		summary += fmt.Sprintf(" | AI: %s (%.2f)",
			r.AIDecision.Direction, r.AIDecision.Confidence)
	}

	if r.OrderResult != nil && r.OrderResult.Success {
		summary += fmt.Sprintf(" | 订单: %s", r.OrderResult.OrderID)
	}

	if r.Error != "" {
		summary += fmt.Sprintf(" | 错误: %s", r.Error)
	} else if r.Message != "" {
		summary += fmt.Sprintf(" | %s", r.Message)
	}

	return summary
}
