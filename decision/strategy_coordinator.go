package decision

import (
	"fmt"
	"log"
	"nofx/market"
	"nofx/trader"
)

// StrategyCoordinator 策略协调器 - 三层架构中的上层 Strategy Control 层
// 职责：
//   1. 协调底层数据处理、AI 决策和参数计算
//   2. 将 AI 的决策信号转换为具体的交易参数
//   3. 执行二次风控验证
//   4. 控制整个数据流
type StrategyCoordinator struct {
	aiCore         *AIDecisionCore
	riskCalculator *trader.RiskCalculator
	ruleEngine     *trader.RuleEngine
	dataCleaner    *market.DataCleaner

	// 配置参数
	btcEthLeverage  int
	altcoinLeverage int
	maxMarginUsage  float64
}

// NewStrategyCoordinator 创建策略协调器实例
func NewStrategyCoordinator(
	aiCore *AIDecisionCore,
	btcEthLeverage, altcoinLeverage int,
	maxMarginUsage float64,
) *StrategyCoordinator {
	return &StrategyCoordinator{
		aiCore:          aiCore,
		riskCalculator:  trader.NewRiskCalculator(),
		ruleEngine:      trader.NewRuleEngine(10.0, 20.0, maxMarginUsage, 0), // 默认风控参数
		dataCleaner:     market.NewDataCleaner(),
		btcEthLeverage:  btcEthLeverage,
		altcoinLeverage: altcoinLeverage,
		maxMarginUsage:  maxMarginUsage,
	}
}

// ProcessRequest 策略处理请求
type ProcessRequest struct {
	Context        *TradingContext
	SystemPrompt   string
	CustomPrompt   string
	OverrideBase   bool
	TemplateName   string
}

// ProcessResult 策略处理结果
type ProcessResult struct {
	// AI 分析结果
	AIAnalysis *AIAnalysisResult

	// 策略决策列表（包含完整参数）
	Decisions []StrategyDecision

	// 风险评估
	RiskAssessment *RiskAssessment

	// 思维链（保留用于日志）
	CoTTrace string
}

// RiskAssessment 风险评估
type RiskAssessment struct {
	AccountRiskPass  bool
	TotalRiskScore   float64 // 0-1，越高越危险
	WarningMessages  []string
	CriticalMessages []string
}

// Process 执行完整的策略处理流程
// 这是三层架构的主入口点，协调所有三层的工作
func (sc *StrategyCoordinator) Process(req *ProcessRequest) (*ProcessResult, error) {
	log.Printf("🎯 [策略协调器] 开始处理策略请求...")

	// ========== 第一步：底层数据清洗 ==========
	log.Printf("📊 [策略协调器] 步骤1: 数据清洗与验证...")
	if err := sc.cleanAndValidateMarketData(req.Context); err != nil {
		return nil, fmt.Errorf("数据清洗失败: %w", err)
	}

	// ========== 第二步：AI 决策分析 ==========
	log.Printf("🤖 [策略协调器] 步骤2: AI 决策分析...")
	aiRequest := &AnalyzeRequest{
		Context:      req.Context,
		SystemPrompt: req.SystemPrompt,
		CustomPrompt: req.CustomPrompt,
		OverrideBase: req.OverrideBase,
		TemplateName: req.TemplateName,
	}

	aiResult, err := sc.aiCore.Analyze(aiRequest)
	if err != nil {
		return nil, fmt.Errorf("AI 分析失败: %w", err)
	}

	// ========== 第三步：策略参数计算与二次风控 ==========
	log.Printf("💼 [策略协调器] 步骤3: 参数计算与风控验证...")
	decisions, riskAssessment := sc.calculateParametersAndValidate(aiResult, req.Context)

	result := &ProcessResult{
		AIAnalysis:     aiResult,
		Decisions:      decisions,
		RiskAssessment: riskAssessment,
		CoTTrace:       aiResult.CoTTrace,
	}

	log.Printf("✓ [策略协调器] 策略处理完成，生成 %d 个决策", len(decisions))
	return result, nil
}

// cleanAndValidateMarketData 清洗和验证市场数据
func (sc *StrategyCoordinator) cleanAndValidateMarketData(ctx *TradingContext) error {
	validCount := 0
	warnCount := 0

	for symbol, data := range ctx.MarketDataMap {
		// 验证和清洗
		cleanedData, validation, err := sc.dataCleaner.ValidateAndClean(data)
		if err != nil {
			log.Printf("⚠️ [策略协调器] %s 数据验证失败: %v", symbol, err)
			// 从上下文中移除无效数据
			delete(ctx.MarketDataMap, symbol)
			continue
		}

		// 更新为清洗后的数据
		ctx.MarketDataMap[symbol] = cleanedData
		validCount++

		if len(validation.Warnings) > 0 {
			warnCount++
			log.Printf("⚠️ [策略协调器] %s 数据警告: %v", symbol, validation.Warnings)
		}
	}

	log.Printf("✓ [策略协调器] 数据清洗完成: 有效 %d, 警告 %d", validCount, warnCount)
	return nil
}

// calculateParametersAndValidate 计算参数并执行二次风控验证
func (sc *StrategyCoordinator) calculateParametersAndValidate(
	aiResult *AIAnalysisResult,
	ctx *TradingContext,
) ([]StrategyDecision, *RiskAssessment) {

	decisions := []StrategyDecision{}
	riskAssessment := &RiskAssessment{
		AccountRiskPass:  true,
		TotalRiskScore:   0,
		WarningMessages:  []string{},
		CriticalMessages: []string{},
	}

	// 1. 账户级别风控检查
	accountRiskResult := sc.ruleEngine.CheckAccountRisk(trader.AccountRiskParams{
		InitialBalance:    ctx.Account.TotalEquity, // 简化：使用当前净值
		CurrentEquity:     ctx.Account.TotalEquity,
		DailyPnL:          ctx.Account.TotalPnL, // 简化：使用总盈亏
		TotalPnL:          ctx.Account.TotalPnL,
		MarginUsedPercent: ctx.Account.MarginUsedPct,
		PositionCount:     ctx.Account.PositionCount,
	})

	if !accountRiskResult.Passed {
		riskAssessment.AccountRiskPass = false
		riskAssessment.CriticalMessages = append(riskAssessment.CriticalMessages, accountRiskResult.ViolatedRules...)
		log.Printf("❌ [策略协调器] 账户风控未通过: %v", accountRiskResult.ViolatedRules)
		// 如果账户风控不通过，不生成任何开仓决策
		return decisions, riskAssessment
	}

	// 2. 处理每个 AI 交易机会
	for _, signal := range aiResult.TradingOpportunities {
		decision := sc.processSignal(signal, ctx, riskAssessment)
		if decision != nil {
			decisions = append(decisions, *decision)
		}
	}

	return decisions, riskAssessment
}

// processSignal 处理单个 AI 信号，转换为完整的策略决策
func (sc *StrategyCoordinator) processSignal(
	signal AIDecisionSignal,
	ctx *TradingContext,
	riskAssessment *RiskAssessment,
) *StrategyDecision {

	// 基础决策对象
	decision := &StrategyDecision{
		Symbol:     signal.Symbol,
		Reasoning:  signal.Reasoning,
		Confidence: int(signal.Confidence * 100),
	}

	// 处理不同的动作
	switch signal.Action {
	case "BUY":
		decision.Action = "open_long"
		return sc.calculateOpenLongParameters(decision, signal, ctx, riskAssessment)

	case "SELL":
		decision.Action = "open_short"
		return sc.calculateOpenShortParameters(decision, signal, ctx, riskAssessment)

	case "CLOSE":
		// 确定是平多还是平空
		for _, pos := range ctx.Positions {
			if pos.Symbol == signal.Symbol {
				if pos.Side == "long" {
					decision.Action = "close_long"
				} else {
					decision.Action = "close_short"
				}
				return decision
			}
		}
		return nil

	case "HOLD":
		decision.Action = "hold"
		return decision

	default:
		return nil
	}
}

// calculateOpenLongParameters 计算开多仓的具体参数
func (sc *StrategyCoordinator) calculateOpenLongParameters(
	decision *StrategyDecision,
	signal AIDecisionSignal,
	ctx *TradingContext,
	riskAssessment *RiskAssessment,
) *StrategyDecision {

	// 获取市场数据
	marketData, ok := ctx.MarketDataMap[decision.Symbol]
	if !ok {
		log.Printf("⚠️ [策略协调器] %s 市场数据缺失", decision.Symbol)
		return nil
	}

	// 确定杠杆
	isBTCOrETH := decision.Symbol == "BTCUSDT" || decision.Symbol == "ETHUSDT"
	if isBTCOrETH {
		decision.Leverage = sc.btcEthLeverage
	} else {
		decision.Leverage = sc.altcoinLeverage
	}

	// 计算止损价格（基于ATR或固定百分比）
	stopLossPrice, _ := sc.riskCalculator.CalculateStopLoss(trader.StopLossParams{
		EntryPrice:      marketData.CurrentPrice,
		IsLong:          true,
		ATR:             marketData.LongerTermContext.ATR14,
		RiskPercentage:  2.0, // 默认2%风险
		MinStopDistance: 0.5, // 最小0.5%
	})
	decision.StopLoss = stopLossPrice

	// 计算止盈价格（基于风险回报比）
	takeProfitPrice, _ := sc.riskCalculator.CalculateTakeProfit(trader.TakeProfitParams{
		EntryPrice:      marketData.CurrentPrice,
		StopLossPrice:   stopLossPrice,
		IsLong:          true,
		RiskRewardRatio: 3.0, // 默认1:3风险回报比
	})
	decision.TakeProfit = takeProfitPrice

	// 计算仓位大小
	positionSizeResult, _ := sc.riskCalculator.CalculatePositionSize(trader.PositionSizeParams{
		AccountEquity:  ctx.Account.TotalEquity,
		RiskPercentage: 2.0,
		EntryPrice:     marketData.CurrentPrice,
		StopLossPrice:  stopLossPrice,
		Leverage:       decision.Leverage,
	})

	if positionSizeResult != nil {
		decision.PositionSizeUSD = positionSizeResult.PositionSizeUSD
		decision.RiskUSD = positionSizeResult.RiskUSD
		decision.MarginRequired = positionSizeResult.MarginRequired
	}

	// 验证风险回报比
	isValid, ratio, _ := sc.riskCalculator.ValidateRiskRewardRatio(
		marketData.CurrentPrice, stopLossPrice, takeProfitPrice, true, 3.0)
	decision.RiskRewardRatio = ratio

	if !isValid {
		log.Printf("⚠️ [策略协调器] %s 风险回报比不足: %.2f", decision.Symbol, ratio)
		riskAssessment.WarningMessages = append(riskAssessment.WarningMessages,
			fmt.Sprintf("%s 风险回报比不足: %.2f", decision.Symbol, ratio))
		return nil
	}

	// 开仓前风控检查
	openRiskCheck := sc.ruleEngine.CheckOpenPositionRisk(trader.OpenPositionRiskParams{
		Symbol:              decision.Symbol,
		Side:                "long",
		PositionSizeUSD:     decision.PositionSizeUSD,
		Leverage:            decision.Leverage,
		AccountEquity:       ctx.Account.TotalEquity,
		CurrentPositions:    ctx.Account.PositionCount,
		AvailableMargin:     ctx.Account.AvailableBalance,
		IsBTCOrETH:          isBTCOrETH,
		MaxBTCETHLeverage:   sc.btcEthLeverage,
		MaxAltcoinLeverage:  sc.altcoinLeverage,
	})

	if !openRiskCheck.Passed {
		log.Printf("⚠️ [策略协调器] %s 开仓风控未通过: %v", decision.Symbol, openRiskCheck.ViolatedRules)
		riskAssessment.WarningMessages = append(riskAssessment.WarningMessages, openRiskCheck.ViolatedRules...)
		return nil
	}

	return decision
}

// calculateOpenShortParameters 计算开空仓的具体参数
func (sc *StrategyCoordinator) calculateOpenShortParameters(
	decision *StrategyDecision,
	signal AIDecisionSignal,
	ctx *TradingContext,
	riskAssessment *RiskAssessment,
) *StrategyDecision {

	// 获取市场数据
	marketData, ok := ctx.MarketDataMap[decision.Symbol]
	if !ok {
		log.Printf("⚠️ [策略协调器] %s 市场数据缺失", decision.Symbol)
		return nil
	}

	// 确定杠杆
	isBTCOrETH := decision.Symbol == "BTCUSDT" || decision.Symbol == "ETHUSDT"
	if isBTCOrETH {
		decision.Leverage = sc.btcEthLeverage
	} else {
		decision.Leverage = sc.altcoinLeverage
	}

	// 计算止损价格
	stopLossPrice, _ := sc.riskCalculator.CalculateStopLoss(trader.StopLossParams{
		EntryPrice:      marketData.CurrentPrice,
		IsLong:          false,
		ATR:             marketData.LongerTermContext.ATR14,
		RiskPercentage:  2.0,
		MinStopDistance: 0.5,
	})
	decision.StopLoss = stopLossPrice

	// 计算止盈价格
	takeProfitPrice, _ := sc.riskCalculator.CalculateTakeProfit(trader.TakeProfitParams{
		EntryPrice:      marketData.CurrentPrice,
		StopLossPrice:   stopLossPrice,
		IsLong:          false,
		RiskRewardRatio: 3.0,
	})
	decision.TakeProfit = takeProfitPrice

	// 计算仓位大小
	positionSizeResult, _ := sc.riskCalculator.CalculatePositionSize(trader.PositionSizeParams{
		AccountEquity:  ctx.Account.TotalEquity,
		RiskPercentage: 2.0,
		EntryPrice:     marketData.CurrentPrice,
		StopLossPrice:  stopLossPrice,
		Leverage:       decision.Leverage,
	})

	if positionSizeResult != nil {
		decision.PositionSizeUSD = positionSizeResult.PositionSizeUSD
		decision.RiskUSD = positionSizeResult.RiskUSD
		decision.MarginRequired = positionSizeResult.MarginRequired
	}

	// 验证风险回报比
	isValid, ratio, _ := sc.riskCalculator.ValidateRiskRewardRatio(
		marketData.CurrentPrice, stopLossPrice, takeProfitPrice, false, 3.0)
	decision.RiskRewardRatio = ratio

	if !isValid {
		log.Printf("⚠️ [策略协调器] %s 风险回报比不足: %.2f", decision.Symbol, ratio)
		riskAssessment.WarningMessages = append(riskAssessment.WarningMessages,
			fmt.Sprintf("%s 风险回报比不足: %.2f", decision.Symbol, ratio))
		return nil
	}

	// 开仓前风控检查
	openRiskCheck := sc.ruleEngine.CheckOpenPositionRisk(trader.OpenPositionRiskParams{
		Symbol:              decision.Symbol,
		Side:                "short",
		PositionSizeUSD:     decision.PositionSizeUSD,
		Leverage:            decision.Leverage,
		AccountEquity:       ctx.Account.TotalEquity,
		CurrentPositions:    ctx.Account.PositionCount,
		AvailableMargin:     ctx.Account.AvailableBalance,
		IsBTCOrETH:          isBTCOrETH,
		MaxBTCETHLeverage:   sc.btcEthLeverage,
		MaxAltcoinLeverage:  sc.altcoinLeverage,
	})

	if !openRiskCheck.Passed {
		log.Printf("⚠️ [策略协调器] %s 开仓风控未通过: %v", decision.Symbol, openRiskCheck.ViolatedRules)
		riskAssessment.WarningMessages = append(riskAssessment.WarningMessages, openRiskCheck.ViolatedRules...)
		return nil
	}

	return decision
}
