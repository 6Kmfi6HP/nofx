package execution_layer

import (
	"fmt"
	"nofx/layers"
	"nofx/layers/data_layer"
	"time"
)

// OrderSender 订单发送器（执行层）
// 职责：发送订单到交易所
type OrderSender struct {
	config        layers.ExecutionLayerConfig
	orderExecutor *data_layer.OrderExecutor
}

// NewOrderSender 创建订单发送器
func NewOrderSender(config layers.ExecutionLayerConfig, executor *data_layer.OrderExecutor) *OrderSender {
	return &OrderSender{
		config:        config,
		orderExecutor: executor,
	}
}

// SendOrder 发送订单
// 输入：执行计划（已通过风控验证）
// 输出：订单结果
func (os *OrderSender) SendOrder(plan *layers.ExecutionPlan) (*layers.OrderResult, error) {
	// 检查是否为模拟模式
	if os.config.DryRun {
		return os.dryRunOrder(plan)
	}

	// 检查是否需要人工确认
	if os.config.RequireManualConfirmation {
		// TODO: 实现人工确认机制
		// 可以通过API或WebSocket通知用户，等待确认
		fmt.Printf("[Order Sender] Waiting for manual confirmation for %s\n", plan.Symbol)
	}

	// 执行订单
	return os.orderExecutor.ExecuteOrder(plan)
}

// dryRunOrder 模拟执行订单
func (os *OrderSender) dryRunOrder(plan *layers.ExecutionPlan) (*layers.OrderResult, error) {
	result := &layers.OrderResult{
		Success:        true,
		OrderID:        fmt.Sprintf("DRYRUN-%d", time.Now().Unix()),
		FilledQuantity: plan.Quantity,
		AvgPrice:       0, // 模拟模式无实际成交价
		ExecutionTimeMs: 10,
		Timestamp:      time.Now(),
		ErrorMessage:   "[DRY RUN] This is a simulated order",
	}

	fmt.Printf("[Dry Run] Order: %s %s %.6f @ leverage %d, SL: %.2f, TP: %.2f\n",
		plan.Action, plan.Symbol, plan.Quantity, plan.Leverage,
		plan.StopLoss, plan.TakeProfit)

	return result, nil
}

// PrepareExecutionPlan 准备执行计划
// 输入：AI决策、风险指标、交易参数、验证结果
// 输出：执行计划
func (os *OrderSender) PrepareExecutionPlan(
	decision *layers.AIDecision,
	riskMetrics *layers.RiskMetrics,
	params map[string]interface{},
	riskCheckPassed bool,
	riskCheckReason string,
) *layers.ExecutionPlan {
	plan := &layers.ExecutionPlan{
		Symbol:             decision.Symbol,
		Timestamp:          time.Now(),
		Action:             params["action"].(string),
		Quantity:           params["quantity"].(float64),
		QuantityUSD:        params["quantity_usd"].(float64),
		Leverage:           params["leverage"].(int),
		StopLoss:           params["stop_loss"].(float64),
		TakeProfit:         params["take_profit"].(float64),
		MaxSlippagePercent: params["max_slippage_percent"].(float64),
		TimeoutSeconds:     params["timeout_seconds"].(int),
		RiskCheckPassed:    riskCheckPassed,
		RiskCheckReason:    riskCheckReason,
		Priority:           params["priority"].(string),
		SourceDecision:     decision,
	}

	return plan
}

// BatchSendOrders 批量发送订单
func (os *OrderSender) BatchSendOrders(plans []*layers.ExecutionPlan) ([]*layers.OrderResult, error) {
	results := make([]*layers.OrderResult, 0, len(plans))

	for _, plan := range plans {
		if plan == nil || !plan.RiskCheckPassed {
			continue
		}

		result, err := os.SendOrder(plan)
		if err != nil {
			// 记录错误但继续处理其他订单
			fmt.Printf("[Order Sender] Failed to send order for %s: %v\n", plan.Symbol, err)
			continue
		}

		results = append(results, result)

		// 批量发送时添加延迟，避免触发交易所限流
		time.Sleep(100 * time.Millisecond)
	}

	return results, nil
}

// CancelOrder 取消订单
func (os *OrderSender) CancelOrder(symbol string, orderID string) error {
	return os.orderExecutor.CancelOrder(orderID, symbol)
}

// GetOrderStatus 获取订单状态
func (os *OrderSender) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	return os.orderExecutor.GetOrderStatus(orderID, symbol)
}

// FormatExecutionPlan 格式化执行计划（用于日志）
func (os *OrderSender) FormatExecutionPlan(plan *layers.ExecutionPlan) string {
	return fmt.Sprintf(
		"[Execution Plan] %s %s | Qty: %.6f (%.2f USD) | Leverage: %dx | "+
			"SL: %.2f | TP: %.2f | Priority: %s | Risk Check: %v (%s)",
		plan.Symbol,
		plan.Action,
		plan.Quantity,
		plan.QuantityUSD,
		plan.Leverage,
		plan.StopLoss,
		plan.TakeProfit,
		plan.Priority,
		plan.RiskCheckPassed,
		plan.RiskCheckReason,
	)
}

// FormatOrderResult 格式化订单结果（用于日志）
func (os *OrderSender) FormatOrderResult(result *layers.OrderResult) string {
	status := "FAILED"
	if result.Success {
		status = "SUCCESS"
	}

	return fmt.Sprintf(
		"[Order Result] %s | OrderID: %s | Filled: %.6f | Time: %dms | Error: %s",
		status,
		result.OrderID,
		result.FilledQuantity,
		result.ExecutionTimeMs,
		result.ErrorMessage,
	)
}
