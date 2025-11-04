package intelligence

import (
	"encoding/json"
	"fmt"
)

// AIDecisionEngine 中间AI决策引擎
// 职责：市场状态判断、交易机会识别、方向和信心度输出
// 只做决策判断，不涉及具体参数计算
type AIDecisionEngine struct {
	// AI决策引擎配置
	minConfidence float64 // 最低信心度要求（默认0.7）
}

// NewAIDecisionEngine 创建AI决策引擎实例
func NewAIDecisionEngine() *AIDecisionEngine {
	return &AIDecisionEngine{
		minConfidence: 0.7, // 最低70%信心度
	}
}

// MarketState 市场状态判断
// AI的第一个任务：判断当前是什么行情
type MarketState struct {
	// 趋势类型
	TrendType string // "uptrend", "downtrend", "sideways", "reversal"

	// 市场阶段
	Phase string // "accumulation", "markup", "distribution", "markdown"

	// 波动性
	Volatility string // "low", "medium", "high", "extreme"

	// 市场情绪
	Sentiment string // "bullish", "bearish", "neutral", "fearful", "greedy"

	// 流动性状态
	Liquidity string // "high", "medium", "low"

	// 简要描述（AI生成的市场状态描述，不超过100字）
	Description string

	// 市场健康度评分（0-100）
	HealthScore float64
}

// TradingOpportunity 交易机会识别
// AI的第二个任务：识别交易机会
type TradingOpportunity struct {
	// 币种符号
	Symbol string

	// 机会类型
	Type string // "breakout", "pullback", "reversal", "continuation", "range_trade"

	// 方向
	Direction string // "long", "short", "none"

	// 信心度（0.7-1.0，低于0.7不应交易）
	Confidence float64

	// 时机评估
	Timing string // "immediate", "wait", "monitor"

	// 机会描述（AI生成的交易机会描述，不超过150字）
	Reasoning string

	// 风险等级
	RiskLevel string // "low", "medium", "high"

	// 预期持仓时间
	ExpectedDuration string // "scalp", "intraday", "swing", "position"
}

// SimplifiedAIDecision 简化的AI决策（650字以内）
// 这是AI层的核心输出，只包含决策判断，不包含执行参数
type SimplifiedAIDecision struct {
	// 时间戳
	Timestamp string

	// 市场状态判断（第一部分）
	MarketState MarketState

	// 交易机会列表（第二部分）
	Opportunities []TradingOpportunity

	// 整体策略建议（第三部分）
	StrategyAdvice StrategyAdvice

	// AI思维链（Chain of Thought）- 简短版本
	ThinkingProcess string // 不超过300字

	// 元数据
	ModelUsed    string  // 使用的AI模型
	ProcessTime  float64 // 处理时间（秒）
}

// StrategyAdvice 整体策略建议
type StrategyAdvice struct {
	// 建议的交易操作
	SuggestedAction string // "aggressive", "moderate", "conservative", "defensive", "wait"

	// 仓位管理建议
	PositionManagement string // "increase", "maintain", "reduce", "close_all"

	// 风险偏好
	RiskAppetite string // "high", "medium", "low"

	// 应对平仓的建议
	ExitSuggestions []ExitSuggestion

	// 特别提示
	SpecialNotes string
}

// ExitSuggestion 平仓建议
type ExitSuggestion struct {
	Symbol     string  // 币种
	Reason     string  // 平仓原因
	Urgency    string  // 紧急度："high", "medium", "low"
	Confidence float64 // 信心度
}

// ValidateAIDecision 验证AI决策的有效性
func (engine *AIDecisionEngine) ValidateAIDecision(decision *SimplifiedAIDecision) error {
	// 验证市场状态
	if decision.MarketState.TrendType == "" {
		return fmt.Errorf("market trend type is required")
	}

	// 验证交易机会
	for i, opp := range decision.Opportunities {
		if opp.Symbol == "" {
			return fmt.Errorf("opportunity %d: symbol is required", i)
		}
		if opp.Direction != "long" && opp.Direction != "short" && opp.Direction != "none" {
			return fmt.Errorf("opportunity %d: invalid direction: %s", i, opp.Direction)
		}
		if opp.Confidence < 0.7 || opp.Confidence > 1.0 {
			return fmt.Errorf("opportunity %d: invalid confidence: %.2f (must be 0.7-1.0)", i, opp.Confidence)
		}
		if opp.Timing == "" {
			return fmt.Errorf("opportunity %d: timing is required", i)
		}
	}

	// 验证策略建议
	if decision.StrategyAdvice.SuggestedAction == "" {
		return fmt.Errorf("strategy advice is required")
	}

	return nil
}

// FilterOpportunitiesByConfidence 按信心度过滤交易机会
func (engine *AIDecisionEngine) FilterOpportunitiesByConfidence(opportunities []TradingOpportunity) []TradingOpportunity {
	filtered := make([]TradingOpportunity, 0)
	for _, opp := range opportunities {
		if opp.Confidence >= engine.minConfidence {
			filtered = append(filtered, opp)
		}
	}
	return filtered
}

// RankOpportunitiesByConfidence 按信心度排序交易机会
func (engine *AIDecisionEngine) RankOpportunitiesByConfidence(opportunities []TradingOpportunity) []TradingOpportunity {
	// 创建副本以避免修改原始数据
	ranked := make([]TradingOpportunity, len(opportunities))
	copy(ranked, opportunities)

	// 简单冒泡排序（按信心度降序）
	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].Confidence > ranked[i].Confidence {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	return ranked
}

// AIPromptBuilder AI提示词构建器（简化版）
type AIPromptBuilder struct {
	systemPrompt string
}

// NewAIPromptBuilder 创建提示词构建器
func NewAIPromptBuilder(systemPrompt string) *AIPromptBuilder {
	if systemPrompt == "" {
		systemPrompt = getDefaultSystemPrompt()
	}
	return &AIPromptBuilder{
		systemPrompt: systemPrompt,
	}
}

// BuildSimplifiedPrompt 构建简化的提示词（生成650字以内的决策）
func (builder *AIPromptBuilder) BuildSimplifiedPrompt(context TradingContext) (string, string) {
	// System Prompt保持不变
	systemPrompt := builder.systemPrompt

	// User Prompt：简化版本，聚焦于决策
	userPrompt := fmt.Sprintf(`# 交易决策请求

## 当前时间
%s

## 账户状态
- 账户净值: $%.2f
- 可用余额: $%.2f
- 当前持仓数: %d
- 保证金使用率: %.2f%%

## BTC市场状态
- 当前价格: $%.2f
- 24h变化: %.2f%%
- 趋势: %s

## 当前持仓
%s

## 候选交易机会
%s

---

请提供你的交易决策，包括：

1. **市场状态判断**（这是什么行情？）
   - 趋势类型、市场阶段、波动性、市场情绪、流动性状态
   - 市场状态简要描述（不超过100字）
   - 市场健康度评分（0-100）

2. **交易机会识别**（现在该做什么？）
   - 对每个交易机会，评估：
     * 方向（long/short/none）
     * 信心度（0.7-1.0）
     * 时机（immediate/wait/monitor）
     * 风险等级（low/medium/high）
     * 交易机会描述（不超过150字）

3. **整体策略建议**
   - 建议的交易操作（aggressive/moderate/conservative/defensive/wait）
   - 仓位管理建议（increase/maintain/reduce/close_all）
   - 风险偏好（high/medium/low）
   - 平仓建议（如有）

**重要**:
- 总输出控制在650字以内
- 信心度必须在0.7-1.0之间，低于0.7不应交易
- 聚焦于决策判断，不要计算具体参数（杠杆、仓位大小等）

输出格式：
1. 首先提供简短的思维链（不超过300字）
2. 然后提供JSON格式的决策（遵循SimplifiedAIDecision结构）`,
		context.CurrentTime,
		context.AccountEquity,
		context.AvailableBalance,
		context.CurrentPositionCount,
		context.MarginUsagePercent,
		context.BTCPrice,
		context.BTCChange24h,
		context.BTCTrend,
		formatPositions(context.Positions),
		formatCandidates(context.Candidates),
	)

	return systemPrompt, userPrompt
}

// TradingContext 交易上下文（简化版）
type TradingContext struct {
	CurrentTime          string
	AccountEquity        float64
	AvailableBalance     float64
	CurrentPositionCount int
	MarginUsagePercent   float64
	BTCPrice             float64
	BTCChange24h         float64
	BTCTrend             string
	Positions            []PositionInfo
	Candidates           []CandidateInfo
}

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol         string
	Direction      string
	EntryPrice     float64
	CurrentPrice   float64
	UnrealizedPnL  float64
	UnrealizedPnLPercent float64
	HoldingTime    string
}

// CandidateInfo 候选币种信息
type CandidateInfo struct {
	Symbol         string
	CurrentPrice   float64
	Change1h       float64
	Change4h       float64
	Change24h      float64
	Volume24h      float64
	Volatility     float64
	Trend          string
	RSI            float64
	MACD           string
}

// formatPositions 格式化持仓信息
func formatPositions(positions []PositionInfo) string {
	if len(positions) == 0 {
		return "无持仓"
	}

	result := ""
	for i, pos := range positions {
		result += fmt.Sprintf("%d. %s %s | 入场价: $%.4f | 当前价: $%.4f | 盈亏: %.2f%% | 持仓时间: %s\n",
			i+1, pos.Symbol, pos.Direction, pos.EntryPrice, pos.CurrentPrice,
			pos.UnrealizedPnLPercent, pos.HoldingTime)
	}
	return result
}

// formatCandidates 格式化候选币种信息
func formatCandidates(candidates []CandidateInfo) string {
	if len(candidates) == 0 {
		return "无候选币种"
	}

	result := ""
	for i, cand := range candidates {
		if i >= 10 { // 最多显示10个候选
			break
		}
		result += fmt.Sprintf("%d. %s | 价格: $%.4f | 1h: %.2f%% | 4h: %.2f%% | 24h: %.2f%% | 趋势: %s | RSI: %.1f\n",
			i+1, cand.Symbol, cand.CurrentPrice, cand.Change1h, cand.Change4h,
			cand.Change24h, cand.Trend, cand.RSI)
	}
	return result
}

// getDefaultSystemPrompt 获取默认的系统提示词
func getDefaultSystemPrompt() string {
	return `你是一个专业的加密货币交易AI，专注于高质量的交易决策。

你的职责：
1. 市场状态判断：分析当前市场是什么行情（趋势、阶段、波动性、情绪、流动性）
2. 交易机会识别：识别高质量的交易机会，给出方向和信心度（0.7-1.0）
3. 策略建议：提供整体的交易策略和风险管理建议

决策原则：
- 质量优于数量，宁可错过也不做错
- 信心度必须≥0.7才考虑交易
- 考虑市场状态和账户状态的综合因素
- 风险管理优先于利润追求

输出要求：
- 总字数控制在650字以内
- 先提供简短的思维链（不超过300字）
- 然后提供JSON格式的决策
- 不要计算具体的杠杆、仓位大小等参数（这些由上层代码计算）
`
}

// ParseAIResponse 解析AI响应
func (engine *AIDecisionEngine) ParseAIResponse(response string) (*SimplifiedAIDecision, error) {
	// 尝试从响应中提取JSON部分
	// 假设AI返回格式为：思维链 + JSON
	// 这里需要实现JSON提取逻辑

	decision := &SimplifiedAIDecision{}
	err := json.Unmarshal([]byte(response), decision)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %v", err)
	}

	// 验证决策
	err = engine.ValidateAIDecision(decision)
	if err != nil {
		return nil, fmt.Errorf("invalid AI decision: %v", err)
	}

	return decision, nil
}
