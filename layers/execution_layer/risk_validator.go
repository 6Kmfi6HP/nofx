package execution_layer

import (
	"fmt"
	"nofx/layers"
)

// RiskValidator 风险验证器（执行层）
// 职责：二次风控验证，在订单发送前进行最后检查
type RiskValidator struct {
	config layers.ExecutionLayerConfig

	// 统计信息
	totalValidations int
	passedValidations int
	failedValidations int
}

// NewRiskValidator 创建风险验证器
func NewRiskValidator(config layers.ExecutionLayerConfig) *RiskValidator {
	return &RiskValidator{
		config:            config,
		totalValidations:  0,
		passedValidations: 0,
		failedValidations: 0,
	}
}

// ValidateExecution 验证执行计划
// 输入：执行计划、AI决策、风险指标、市场数据
// 输出：是否通过、失败原因
func (rv *RiskValidator) ValidateExecution(
	plan *layers.ExecutionPlan,
	decision *layers.AIDecision,
	riskMetrics *layers.RiskMetrics,
	marketData *layers.CleanedMarketData,
) (bool, string) {
	rv.totalValidations++

	// 检查1：基本参数验证
	if pass, reason := rv.validateBasicParameters(plan); !pass {
		rv.failedValidations++
		return false, reason
	}

	// 检查2：风险指标验证
	if pass, reason := rv.validateRiskMetrics(plan, riskMetrics); !pass {
		rv.failedValidations++
		return false, reason
	}

	// 检查3：市场数据验证
	if pass, reason := rv.validateMarketData(plan, marketData); !pass {
		rv.failedValidations++
		return false, reason
	}

	// 检查4：AI决策一致性验证
	if pass, reason := rv.validateDecisionConsistency(plan, decision); !pass {
		rv.failedValidations++
		return false, reason
	}

	// 检查5：止损止盈合理性验证
	if pass, reason := rv.validateStopLossTakeProfit(plan, marketData); !pass {
		rv.failedValidations++
		return false, reason
	}

	// 检查6：杠杆和仓位验证
	if pass, reason := rv.validateLeverageAndPosition(plan, marketData); !pass {
		rv.failedValidations++
		return false, reason
	}

	// 所有检查通过
	rv.passedValidations++
	return true, "风控验证通过"
}

// validateBasicParameters 验证基本参数
func (rv *RiskValidator) validateBasicParameters(plan *layers.ExecutionPlan) (bool, string) {
	if plan == nil {
		return false, "执行计划为空"
	}

	if plan.Symbol == "" {
		return false, "交易对为空"
	}

	if plan.Action != "open_long" && plan.Action != "open_short" &&
		plan.Action != "close_long" && plan.Action != "close_short" {
		return false, fmt.Sprintf("无效的交易动作: %s", plan.Action)
	}

	if plan.Quantity <= 0 {
		return false, fmt.Sprintf("交易数量无效: %.6f", plan.Quantity)
	}

	if plan.QuantityUSD <= 0 {
		return false, fmt.Sprintf("交易金额无效: %.2f", plan.QuantityUSD)
	}

	if plan.Leverage < 1 || plan.Leverage > 20 {
		return false, fmt.Sprintf("杠杆倍数超出范围: %d", plan.Leverage)
	}

	return true, ""
}

// validateRiskMetrics 验证风险指标
func (rv *RiskValidator) validateRiskMetrics(plan *layers.ExecutionPlan, metrics *layers.RiskMetrics) (bool, string) {
	if metrics == nil {
		return false, "风险指标为空"
	}

	// 检查风险指标是否允许交易
	if !metrics.CanTrade {
		return false, fmt.Sprintf("风险指标禁止交易: %s", metrics.RiskReason)
	}

	// 检查风险等级
	if metrics.RiskLevel == "extreme" {
		return false, "风险等级过高(extreme)"
	}

	// 检查仓位是否超过建议
	if plan.QuantityUSD > metrics.MaxPositionSizeUSD*1.1 { // 允许10%超出
		return false, fmt.Sprintf("仓位超过建议值: %.2f > %.2f",
			plan.QuantityUSD, metrics.MaxPositionSizeUSD)
	}

	// 检查杠杆是否超过建议
	if plan.Leverage > metrics.RecommendedLeverage+1 { // 允许+1
		return false, fmt.Sprintf("杠杆超过建议值: %d > %d",
			plan.Leverage, metrics.RecommendedLeverage)
	}

	return true, ""
}

// validateMarketData 验证市场数据
func (rv *RiskValidator) validateMarketData(plan *layers.ExecutionPlan, data *layers.CleanedMarketData) (bool, string) {
	if data == nil {
		return false, "市场数据为空"
	}

	// 检查数据质量
	if !data.IsValid {
		return false, "市场数据质量不合格"
	}

	if data.DataQuality < 0.8 {
		return false, fmt.Sprintf("数据质量过低: %.2f", data.DataQuality)
	}

	// 检查价格是否异常
	if data.CurrentPrice <= 0 {
		return false, "当前价格无效"
	}

	// 检查价格波动是否异常（单小时超过20%）
	if data.PriceChange1h > 20 || data.PriceChange1h < -20 {
		return false, fmt.Sprintf("价格波动异常: %.2f%%", data.PriceChange1h)
	}

	return true, ""
}

// validateDecisionConsistency 验证决策一致性
func (rv *RiskValidator) validateDecisionConsistency(plan *layers.ExecutionPlan, decision *layers.AIDecision) (bool, string) {
	if decision == nil {
		return false, "AI决策为空"
	}

	// 检查执行计划的动作是否与AI决策一致
	expectedAction := ""
	switch decision.Direction {
	case layers.DirectionLong:
		expectedAction = "open_long"
	case layers.DirectionShort:
		expectedAction = "open_short"
	case layers.DirectionWait:
		expectedAction = "wait"
	}

	if plan.Action != expectedAction {
		return false, fmt.Sprintf("执行动作与AI决策不一致: %s != %s",
			plan.Action, expectedAction)
	}

	// 检查信心度
	if decision.Confidence < 0.7 {
		return false, fmt.Sprintf("AI信心度过低: %.2f", decision.Confidence)
	}

	// 检查机会类型
	if decision.Opportunity == layers.OpportunityNone && plan.Action != "wait" {
		return false, "无交易机会但计划执行交易"
	}

	return true, ""
}

// validateStopLossTakeProfit 验证止损止盈
func (rv *RiskValidator) validateStopLossTakeProfit(plan *layers.ExecutionPlan, data *layers.CleanedMarketData) (bool, string) {
	currentPrice := data.CurrentPrice

	// 开多单验证
	if plan.Action == "open_long" {
		// 止损必须低于当前价格
		if plan.StopLoss >= currentPrice {
			return false, fmt.Sprintf("多单止损价格无效: %.2f >= %.2f",
				plan.StopLoss, currentPrice)
		}

		// 止盈必须高于当前价格
		if plan.TakeProfit > 0 && plan.TakeProfit <= currentPrice {
			return false, fmt.Sprintf("多单止盈价格无效: %.2f <= %.2f",
				plan.TakeProfit, currentPrice)
		}

		// 止损不能太远（超过10%）
		stopLossPercent := (currentPrice - plan.StopLoss) / currentPrice * 100
		if stopLossPercent > 10 {
			return false, fmt.Sprintf("止损距离过大: %.2f%%", stopLossPercent)
		}

		// 止损不能太近（小于0.5%）
		if stopLossPercent < 0.5 {
			return false, fmt.Sprintf("止损距离过小: %.2f%%", stopLossPercent)
		}
	}

	// 开空单验证
	if plan.Action == "open_short" {
		// 止损必须高于当前价格
		if plan.StopLoss <= currentPrice {
			return false, fmt.Sprintf("空单止损价格无效: %.2f <= %.2f",
				plan.StopLoss, currentPrice)
		}

		// 止盈必须低于当前价格
		if plan.TakeProfit > 0 && plan.TakeProfit >= currentPrice {
			return false, fmt.Sprintf("空单止盈价格无效: %.2f >= %.2f",
				plan.TakeProfit, currentPrice)
		}

		// 止损不能太远（超过10%）
		stopLossPercent := (plan.StopLoss - currentPrice) / currentPrice * 100
		if stopLossPercent > 10 {
			return false, fmt.Sprintf("止损距离过大: %.2f%%", stopLossPercent)
		}

		// 止损不能太近（小于0.5%）
		if stopLossPercent < 0.5 {
			return false, fmt.Sprintf("止损距离过小: %.2f%%", stopLossPercent)
		}
	}

	return true, ""
}

// validateLeverageAndPosition 验证杠杆和仓位
func (rv *RiskValidator) validateLeverageAndPosition(plan *layers.ExecutionPlan, data *layers.CleanedMarketData) (bool, string) {
	// 高波动市场限制杠杆
	if data.ATR > 0 && data.CurrentPrice > 0 {
		volatility := data.ATR / data.CurrentPrice * 100

		if volatility > 5 && plan.Leverage > 3 {
			return false, fmt.Sprintf("高波动市场(%.2f%%)杠杆过高: %d",
				volatility, plan.Leverage)
		}
	}

	// 超买超卖区域限制仓位
	if data.RSI14 > 80 && plan.Action == "open_long" {
		return false, "RSI严重超买，不适合做多"
	}

	if data.RSI14 < 20 && plan.Action == "open_short" {
		return false, "RSI严重超卖，不适合做空"
	}

	// 资金费率检查
	if data.FundingRate > 0.01 && plan.Action == "open_long" {
		// 资金费率过高，做多成本高
		// 警告但不阻止
	}

	if data.FundingRate < -0.01 && plan.Action == "open_short" {
		// 资金费率过低，做空成本高
		// 警告但不阻止
	}

	return true, ""
}

// GetValidationStats 获取验证统计
func (rv *RiskValidator) GetValidationStats() map[string]interface{} {
	passRate := 0.0
	if rv.totalValidations > 0 {
		passRate = float64(rv.passedValidations) / float64(rv.totalValidations) * 100
	}

	return map[string]interface{}{
		"total_validations":  rv.totalValidations,
		"passed_validations": rv.passedValidations,
		"failed_validations": rv.failedValidations,
		"pass_rate":          passRate,
	}
}

// ResetStats 重置统计
func (rv *RiskValidator) ResetStats() {
	rv.totalValidations = 0
	rv.passedValidations = 0
	rv.failedValidations = 0
}
