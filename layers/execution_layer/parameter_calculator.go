package execution_layer

import (
	"fmt"
	"math"
	"nofx/layers"
)

// ParameterCalculator 参数计算器（执行层）
// 职责：根据AI决策计算具体交易参数
type ParameterCalculator struct {
	config layers.ExecutionLayerConfig
}

// NewParameterCalculator 创建参数计算器
func NewParameterCalculator(config layers.ExecutionLayerConfig) *ParameterCalculator {
	return &ParameterCalculator{
		config: config,
	}
}

// CalculateParameters 计算交易参数
// 输入：AI决策、风险指标、清洗后的市场数据
// 输出：具体的交易参数（数量、价格等）
func (pc *ParameterCalculator) CalculateParameters(
	decision *layers.AIDecision,
	riskMetrics *layers.RiskMetrics,
	marketData *layers.CleanedMarketData,
) (map[string]interface{}, error) {
	if decision == nil || riskMetrics == nil || marketData == nil {
		return nil, fmt.Errorf("invalid input parameters")
	}

	params := make(map[string]interface{})

	// 1. 确定交易动作
	action := pc.determineAction(decision)
	params["action"] = action

	// 2. 计算仓位大小
	quantity, quantityUSD := pc.calculatePositionSize(decision, riskMetrics, marketData)
	params["quantity"] = quantity
	params["quantity_usd"] = quantityUSD

	// 3. 确定杠杆
	leverage := pc.calculateLeverage(decision, riskMetrics, marketData)
	params["leverage"] = leverage

	// 4. 计算止损价格
	stopLoss := pc.calculateStopLoss(decision, riskMetrics, marketData)
	params["stop_loss"] = stopLoss

	// 5. 计算止盈价格
	takeProfit := pc.calculateTakeProfit(decision, riskMetrics, marketData)
	params["take_profit"] = takeProfit

	// 6. 计算滑点容忍度
	maxSlippage := pc.config.MaxSlippagePercent
	params["max_slippage_percent"] = maxSlippage

	// 7. 设置超时时间
	params["timeout_seconds"] = pc.config.OrderTimeoutSeconds

	// 8. 优先级
	priority := pc.determinePriority(decision)
	params["priority"] = priority

	return params, nil
}

// determineAction 确定交易动作
func (pc *ParameterCalculator) determineAction(decision *layers.AIDecision) string {
	switch decision.Direction {
	case layers.DirectionLong:
		return "open_long"
	case layers.DirectionShort:
		return "open_short"
	case layers.DirectionWait:
		return "wait"
	default:
		return "wait"
	}
}

// calculatePositionSize 计算仓位大小
func (pc *ParameterCalculator) calculatePositionSize(
	decision *layers.AIDecision,
	riskMetrics *layers.RiskMetrics,
	marketData *layers.CleanedMarketData,
) (float64, float64) {
	// 基础仓位：风险指标建议的最大仓位
	basePositionUSD := riskMetrics.MaxPositionSizeUSD

	// 根据信心度调整仓位
	// 信心度 0.7 -> 50%仓位
	// 信心度 0.85 -> 75%仓位
	// 信心度 1.0 -> 100%仓位
	confidenceMultiplier := 0.0
	if decision.Confidence >= 0.7 && decision.Confidence <= 1.0 {
		// 线性映射：0.7->0.5, 0.85->0.75, 1.0->1.0
		confidenceMultiplier = (decision.Confidence-0.7)/0.3*0.5 + 0.5
	} else if decision.Confidence < 0.7 {
		confidenceMultiplier = 0.5
	} else {
		confidenceMultiplier = 1.0
	}

	adjustedPositionUSD := basePositionUSD * confidenceMultiplier

	// 仓位大小方法
	if pc.config.EnablePositionSizing {
		switch pc.config.PositionSizingMethod {
		case "fixed":
			// 固定仓位
			adjustedPositionUSD = basePositionUSD * 0.5

		case "kelly":
			// Kelly准则（简化版）
			// f = (bp - q) / b
			// b = 赔率, p = 胜率, q = 败率
			winRate := decision.Confidence
			lossRate := 1 - winRate
			payoffRatio := 2.0 // 假设盈亏比2:1

			kellyFraction := (payoffRatio*winRate - lossRate) / payoffRatio
			if kellyFraction < 0 {
				kellyFraction = 0
			}
			if kellyFraction > 0.25 { // Kelly的1/4
				kellyFraction = 0.25
			}

			adjustedPositionUSD = basePositionUSD * kellyFraction / 0.25

		case "volatility":
			// 基于波动率的仓位
			if marketData.ATR > 0 && marketData.CurrentPrice > 0 {
				volatility := marketData.ATR / marketData.CurrentPrice
				// 波动率越高，仓位越小
				volMultiplier := 1.0 / (1.0 + volatility*10)
				adjustedPositionUSD = basePositionUSD * volMultiplier
			}
		}
	}

	// 计算实际数量
	quantity := 0.0
	if marketData.CurrentPrice > 0 {
		quantity = adjustedPositionUSD / marketData.CurrentPrice
	}

	return quantity, adjustedPositionUSD
}

// calculateLeverage 计算杠杆
func (pc *ParameterCalculator) calculateLeverage(
	decision *layers.AIDecision,
	riskMetrics *layers.RiskMetrics,
	marketData *layers.CleanedMarketData,
) int {
	// 使用风险指标推荐的杠杆
	baseLeverage := riskMetrics.RecommendedLeverage

	// 根据信心度微调
	if decision.Confidence < 0.8 {
		// 低信心度，降低杠杆
		baseLeverage = int(math.Max(1, float64(baseLeverage)-1))
	}

	// 根据市场状态调整
	switch decision.MarketCondition {
	case layers.MarketVolatile:
		// 高波动，降低杠杆
		baseLeverage = int(math.Max(1, float64(baseLeverage)-1))

	case layers.MarketTrending:
		// 趋势市场，可以保持推荐杠杆
		// 不变

	case layers.MarketBreakout:
		// 突破市场，可以略微提高杠杆
		// 但要谨慎
		// 不变
	}

	return baseLeverage
}

// calculateStopLoss 计算止损价格
func (pc *ParameterCalculator) calculateStopLoss(
	decision *layers.AIDecision,
	riskMetrics *layers.RiskMetrics,
	marketData *layers.CleanedMarketData,
) float64 {
	// 使用风险指标计算的止损价格
	stopLoss := riskMetrics.StopLossPrice

	// 根据市场状态微调
	if decision.MarketCondition == layers.MarketVolatile {
		// 高波动市场，适当放宽止损
		if decision.Direction == layers.DirectionLong {
			stopLoss = stopLoss * 0.98 // 再降2%
		} else if decision.Direction == layers.DirectionShort {
			stopLoss = stopLoss * 1.02 // 再提2%
		}
	}

	return stopLoss
}

// calculateTakeProfit 计算止盈价格
func (pc *ParameterCalculator) calculateTakeProfit(
	decision *layers.AIDecision,
	riskMetrics *layers.RiskMetrics,
	marketData *layers.CleanedMarketData,
) float64 {
	// 使用风险指标计算的止盈价格
	takeProfit := riskMetrics.TakeProfitPrice

	// 根据信心度调整目标
	if decision.Confidence > 0.9 {
		// 高信心度，可以设置更激进的止盈
		if decision.Direction == layers.DirectionLong {
			risk := marketData.CurrentPrice - riskMetrics.StopLossPrice
			takeProfit = marketData.CurrentPrice + risk*2.5 // 2.5倍风险收益比
		} else if decision.Direction == layers.DirectionShort {
			risk := riskMetrics.StopLossPrice - marketData.CurrentPrice
			takeProfit = marketData.CurrentPrice - risk*2.5
		}
	}

	return takeProfit
}

// determinePriority 确定订单优先级
func (pc *ParameterCalculator) determinePriority(decision *layers.AIDecision) string {
	// 高信心度 + 突破或趋势 = 高优先级
	if decision.Confidence >= 0.9 {
		if decision.MarketCondition == layers.MarketBreakout ||
			decision.MarketCondition == layers.MarketTrending {
			return "high"
		}
	}

	// 中等信心度或震荡市场 = 普通优先级
	if decision.Confidence >= 0.75 {
		return "normal"
	}

	// 低信心度 = 低优先级
	return "low"
}

// AdjustParametersForRisk 根据风险调整参数
func (pc *ParameterCalculator) AdjustParametersForRisk(
	params map[string]interface{},
	riskLevel string,
) map[string]interface{} {
	adjusted := make(map[string]interface{})
	for k, v := range params {
		adjusted[k] = v
	}

	switch riskLevel {
	case "extreme":
		// 极端风险：取消交易
		adjusted["action"] = "wait"

	case "high":
		// 高风险：减半仓位，降低杠杆
		if qty, ok := adjusted["quantity"].(float64); ok {
			adjusted["quantity"] = qty * 0.5
		}
		if qtyUSD, ok := adjusted["quantity_usd"].(float64); ok {
			adjusted["quantity_usd"] = qtyUSD * 0.5
		}
		if leverage, ok := adjusted["leverage"].(int); ok {
			adjusted["leverage"] = int(math.Max(1, float64(leverage)-1))
		}

	case "medium":
		// 中等风险：略微减小仓位
		if qty, ok := adjusted["quantity"].(float64); ok {
			adjusted["quantity"] = qty * 0.8
		}
		if qtyUSD, ok := adjusted["quantity_usd"].(float64); ok {
			adjusted["quantity_usd"] = qtyUSD * 0.8
		}

	case "low":
		// 低风险：保持原参数
		// 不变
	}

	return adjusted
}

// FormatParameters 格式化参数为字符串（用于日志）
func (pc *ParameterCalculator) FormatParameters(params map[string]interface{}) string {
	return fmt.Sprintf(
		"Action: %v, Quantity: %.6f, QuantityUSD: %.2f, Leverage: %v, "+
			"StopLoss: %.2f, TakeProfit: %.2f, Priority: %v",
		params["action"],
		params["quantity"],
		params["quantity_usd"],
		params["leverage"],
		params["stop_loss"],
		params["take_profit"],
		params["priority"],
	)
}
