package ai_layer

import (
	"encoding/json"
	"fmt"
	"nofx/layers"
	"nofx/mcp"
	"time"
)

// DecisionMaker 决策制定器（AI层核心）
// 职责：整合市场分析和机会识别，输出方向和信心度（0.7-1.0）
type DecisionMaker struct {
	config            layers.AILayerConfig
	mcpClient         *mcp.Client
	marketAnalyzer    *MarketAnalyzer
	opportunityDetector *OpportunityDetector

	// 频率控制
	lastDecisionTime  time.Time
	decisionsThisHour int
}

// NewDecisionMaker 创建决策制定器
func NewDecisionMaker(config layers.AILayerConfig) (*DecisionMaker, error) {
	provider := mcp.Provider(config.Provider)
	client := mcp.NewClient(provider, config.APIKey, config.BaseURL, config.Model)

	analyzer, _ := NewMarketAnalyzer(config)
	detector, _ := NewOpportunityDetector(config)

	return &DecisionMaker{
		config:              config,
		mcpClient:           client,
		marketAnalyzer:      analyzer,
		opportunityDetector: detector,
		lastDecisionTime:    time.Time{},
		decisionsThisHour:   0,
	}, nil
}

// MakeDecision 制定AI决策（完整版，调用AI）
// 输入：清洗后的市场数据
// 输出：AI决策（包含方向和信心度）
func (dm *DecisionMaker) MakeDecision(
	marketData *layers.CleanedMarketData,
) (*layers.AIDecision, error) {
	startTime := time.Now()

	// 检查频率限制
	if !dm.checkRateLimit() {
		return &layers.AIDecision{
			Symbol:          marketData.Symbol,
			Timestamp:       startTime,
			Direction:       layers.DirectionWait,
			Confidence:      0,
			MarketCondition: layers.MarketRanging,
			Opportunity:     layers.OpportunityNone,
			ConditionReason: "频率限制：已达到每小时最大决策次数",
			OpportunityReason: "等待冷却",
		}, nil
	}

	// 步骤1：分析市场状态
	marketCondition, conditionReason, err := dm.marketAnalyzer.AnalyzeMarketCondition(marketData)
	if err != nil {
		// 如果AI调用失败，使用技术指标分析
		marketCondition, conditionReason = dm.marketAnalyzer.AnalyzeMarketConditionWithTechnicals(marketData)
	}

	// 步骤2：识别交易机会
	opportunity, oppReason, err := dm.opportunityDetector.DetectOpportunity(marketCondition, marketData)
	if err != nil {
		// 如果AI调用失败，使用技术指标识别
		opportunity, oppReason = dm.opportunityDetector.DetectOpportunityWithTechnicals(marketCondition, marketData)
	}

	// 步骤3：调用AI综合决策
	direction, confidence, chainOfThought, err := dm.callAIForDecision(
		marketCondition, conditionReason,
		opportunity, oppReason,
		marketData,
	)

	if err != nil {
		// AI调用失败，使用规则引擎
		direction, confidence = dm.makeRuleBasedDecision(marketCondition, opportunity, marketData)
		chainOfThought = "AI调用失败，使用规则引擎决策"
	}

	// 应用信心度阈值
	if confidence < dm.config.MinConfidence {
		direction = layers.DirectionWait
		chainOfThought += fmt.Sprintf("\n信心度%.2f低于阈值%.2f，改为观望",
			confidence, dm.config.MinConfidence)
	}

	// 构建决策结果
	decision := &layers.AIDecision{
		Symbol:            marketData.Symbol,
		Timestamp:         startTime,
		MarketCondition:   marketCondition,
		ConditionReason:   conditionReason,
		Opportunity:       opportunity,
		OpportunityReason: oppReason,
		Direction:         direction,
		Confidence:        confidence,
		ChainOfThought:    chainOfThought,
		ModelUsed:         dm.config.Model,
		ResponseTimeMs:    time.Since(startTime).Milliseconds(),
	}

	// 更新频率控制
	dm.updateRateLimit()

	return decision, nil
}

// callAIForDecision 调用AI进行综合决策
func (dm *DecisionMaker) callAIForDecision(
	condition layers.MarketCondition,
	conditionReason string,
	opportunity layers.TradingOpportunity,
	oppReason string,
	marketData *layers.CleanedMarketData,
) (layers.Direction, float64, string, error) {
	// 构建提示词
	systemPrompt := `你是顶级交易决策专家。基于市场分析和机会识别，给出最终交易决策。
只返回JSON格式：
{"direction":"long/short/wait","confidence":0.75,"reasoning":"简短推理过程"}`

	userPrompt := fmt.Sprintf(`市场分析：
状态：%s
原因：%s

机会识别：
机会：%s
原因：%s

市场数据：%s

要求：
1. direction: long(做多)/short(做空)/wait(观望)
2. confidence: 0.7-1.0 (低于0.7不建议交易)
3. reasoning: 100字以内的推理过程

只返回JSON`,
		condition, conditionReason,
		opportunity, oppReason,
		marketData.CompressedSummary)

	// 限制prompt长度
	if len(userPrompt) > dm.config.MaxPromptLength {
		userPrompt = userPrompt[:dm.config.MaxPromptLength-3] + "..."
	}

	// 调用AI
	response, err := dm.mcpClient.SendMessage(systemPrompt, userPrompt)
	if err != nil {
		return layers.DirectionWait, 0, "", fmt.Errorf("AI call failed: %w", err)
	}

	// 解析响应
	var result struct {
		Direction  string  `json:"direction"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return dm.parseDecisionFromText(response)
	}

	// 验证direction
	direction := layers.Direction(result.Direction)
	if direction != layers.DirectionLong && direction != layers.DirectionShort && direction != layers.DirectionWait {
		direction = layers.DirectionWait
	}

	// 验证confidence
	confidence := result.Confidence
	if confidence < 0.7 {
		confidence = 0.7
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	return direction, confidence, result.Reasoning, nil
}

// makeRuleBasedDecision 基于规则的决策（回退方案）
func (dm *DecisionMaker) makeRuleBasedDecision(
	condition layers.MarketCondition,
	opportunity layers.TradingOpportunity,
	marketData *layers.CleanedMarketData,
) (layers.Direction, float64) {
	// 根据机会类型决定方向
	direction := layers.DirectionWait
	confidence := 0.75

	switch opportunity {
	case layers.OpportunityLongEntry:
		direction = layers.DirectionLong
		confidence = dm.calculateConfidence(marketData, true)

	case layers.OpportunityShortEntry:
		direction = layers.DirectionShort
		confidence = dm.calculateConfidence(marketData, false)

	case layers.OpportunityScalp:
		// 剥头皮：根据短期RSI决定
		if marketData.RSI7 < 30 {
			direction = layers.DirectionLong
			confidence = 0.75
		} else if marketData.RSI7 > 70 {
			direction = layers.DirectionShort
			confidence = 0.75
		}

	default:
		direction = layers.DirectionWait
		confidence = 0
	}

	return direction, confidence
}

// calculateConfidence 计算信心度
func (dm *DecisionMaker) calculateConfidence(data *layers.CleanedMarketData, isLong bool) float64 {
	confidence := 0.75 // 基础信心度

	// RSI确认
	if isLong && data.RSI14 < 40 {
		confidence += 0.05
	} else if !isLong && data.RSI14 > 60 {
		confidence += 0.05
	}

	// MACD确认
	if isLong && data.MACD > data.MACDSignal {
		confidence += 0.05
	} else if !isLong && data.MACD < data.MACDSignal {
		confidence += 0.05
	}

	// EMA趋势确认
	if isLong && data.CurrentPrice > data.EMA20 {
		confidence += 0.05
	} else if !isLong && data.CurrentPrice < data.EMA20 {
		confidence += 0.05
	}

	// 成交量确认
	if data.VolumeChange > 30 {
		confidence += 0.05
	}

	// 限制最大信心度
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// parseDecisionFromText 从文本解析决策
func (dm *DecisionMaker) parseDecisionFromText(text string) (layers.Direction, float64, string, error) {
	direction := layers.DirectionWait
	confidence := 0.75

	if contains(text, "long") || contains(text, "做多") || contains(text, "买入") {
		direction = layers.DirectionLong
	} else if contains(text, "short") || contains(text, "做空") || contains(text, "卖出") {
		direction = layers.DirectionShort
	} else if contains(text, "wait") || contains(text, "观望") || contains(text, "等待") {
		direction = layers.DirectionWait
	}

	return direction, confidence, "从文本解析的决策", nil
}

// checkRateLimit 检查频率限制
func (dm *DecisionMaker) checkRateLimit() bool {
	now := time.Now()

	// 检查是否在新的小时
	if now.Sub(dm.lastDecisionTime) > time.Hour {
		dm.decisionsThisHour = 0
	}

	// 检查是否达到限制
	if dm.decisionsThisHour >= dm.config.MaxDecisionsPerHour {
		return false
	}

	// 检查冷却时间
	if now.Sub(dm.lastDecisionTime) < time.Duration(dm.config.CooldownMinutes)*time.Minute {
		return false
	}

	return true
}

// updateRateLimit 更新频率限制
func (dm *DecisionMaker) updateRateLimit() {
	dm.lastDecisionTime = time.Now()
	dm.decisionsThisHour++
}

// GetRateLimitStatus 获取频率限制状态
func (dm *DecisionMaker) GetRateLimitStatus() map[string]interface{} {
	return map[string]interface{}{
		"decisions_this_hour":    dm.decisionsThisHour,
		"max_decisions_per_hour": dm.config.MaxDecisionsPerHour,
		"last_decision_time":     dm.lastDecisionTime,
		"cooldown_minutes":       dm.config.CooldownMinutes,
		"can_decide_now":         dm.checkRateLimit(),
	}
}

// ResetRateLimit 重置频率限制（用于测试或手动重置）
func (dm *DecisionMaker) ResetRateLimit() {
	dm.decisionsThisHour = 0
	dm.lastDecisionTime = time.Time{}
}
