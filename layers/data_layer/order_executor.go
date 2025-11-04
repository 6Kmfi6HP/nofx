package data_layer

import (
	"fmt"
	"nofx/layers"
	"nofx/trader"
	"time"
)

// OrderExecutor 订单执行器（底层）
// 职责：订单执行和监控
type OrderExecutor struct {
	config layers.DataLayerConfig
	trader trader.Trader // 使用现有的Trader接口
}

// NewOrderExecutor 创建订单执行器
func NewOrderExecutor(config layers.DataLayerConfig, tr trader.Trader) *OrderExecutor {
	return &OrderExecutor{
		config: config,
		trader: tr,
	}
}

// ExecuteOrder 执行订单
// 输入：执行计划
// 输出：订单结果
func (oe *OrderExecutor) ExecuteOrder(plan *layers.ExecutionPlan) (*layers.OrderResult, error) {
	startTime := time.Now()

	result := &layers.OrderResult{
		Timestamp: startTime,
	}

	// 验证执行计划
	if plan == nil {
		result.Success = false
		result.ErrorMessage = "execution plan is nil"
		return result, fmt.Errorf("execution plan is nil")
	}

	if !plan.RiskCheckPassed {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("risk check failed: %s", plan.RiskCheckReason)
		return result, fmt.Errorf("risk check failed")
	}

	// 根据动作类型执行
	var err error
	switch plan.Action {
	case "open_long":
		err = oe.executeOpenLong(plan, result)
	case "open_short":
		err = oe.executeOpenShort(plan, result)
	case "close_long":
		err = oe.executeCloseLong(plan, result)
	case "close_short":
		err = oe.executeCloseShort(plan, result)
	default:
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("unknown action: %s", plan.Action)
		return result, fmt.Errorf("unknown action: %s", plan.Action)
	}

	// 记录执行时间
	result.ExecutionTimeMs = time.Since(startTime).Milliseconds()

	if err != nil {
		result.Success = false
		if result.ErrorMessage == "" {
			result.ErrorMessage = err.Error()
		}
		return result, err
	}

	result.Success = true
	return result, nil
}

// executeOpenLong 执行开多
func (oe *OrderExecutor) executeOpenLong(plan *layers.ExecutionPlan, result *layers.OrderResult) error {
	// 设置杠杆
	if err := oe.trader.SetLeverage(plan.Symbol, plan.Leverage); err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	// 开多仓
	orderId, err := oe.trader.OpenLong(plan.Symbol, plan.Quantity, plan.Leverage)
	if err != nil {
		return fmt.Errorf("failed to open long: %w", err)
	}

	result.OrderID = orderId
	result.FilledQuantity = plan.Quantity

	// 设置止损
	if plan.StopLoss > 0 {
		if err := oe.trader.SetStopLoss(plan.Symbol, "long", plan.StopLoss); err != nil {
			// 止损设置失败不影响主订单
			result.ErrorMessage = fmt.Sprintf("warning: failed to set stop loss: %v", err)
		}
	}

	// 设置止盈
	if plan.TakeProfit > 0 {
		if err := oe.trader.SetTakeProfit(plan.Symbol, "long", plan.TakeProfit); err != nil {
			// 止盈设置失败不影响主订单
			if result.ErrorMessage != "" {
				result.ErrorMessage += "; "
			}
			result.ErrorMessage += fmt.Sprintf("warning: failed to set take profit: %v", err)
		}
	}

	return nil
}

// executeOpenShort 执行开空
func (oe *OrderExecutor) executeOpenShort(plan *layers.ExecutionPlan, result *layers.OrderResult) error {
	// 设置杠杆
	if err := oe.trader.SetLeverage(plan.Symbol, plan.Leverage); err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	// 开空仓
	orderId, err := oe.trader.OpenShort(plan.Symbol, plan.Quantity, plan.Leverage)
	if err != nil {
		return fmt.Errorf("failed to open short: %w", err)
	}

	result.OrderID = orderId
	result.FilledQuantity = plan.Quantity

	// 设置止损
	if plan.StopLoss > 0 {
		if err := oe.trader.SetStopLoss(plan.Symbol, "short", plan.StopLoss); err != nil {
			result.ErrorMessage = fmt.Sprintf("warning: failed to set stop loss: %v", err)
		}
	}

	// 设置止盈
	if plan.TakeProfit > 0 {
		if err := oe.trader.SetTakeProfit(plan.Symbol, "short", plan.TakeProfit); err != nil {
			if result.ErrorMessage != "" {
				result.ErrorMessage += "; "
			}
			result.ErrorMessage += fmt.Sprintf("warning: failed to set take profit: %v", err)
		}
	}

	return nil
}

// executeCloseLong 执行平多
func (oe *OrderExecutor) executeCloseLong(plan *layers.ExecutionPlan, result *layers.OrderResult) error {
	orderId, err := oe.trader.CloseLong(plan.Symbol, plan.Quantity)
	if err != nil {
		return fmt.Errorf("failed to close long: %w", err)
	}

	result.OrderID = orderId
	result.FilledQuantity = plan.Quantity

	return nil
}

// executeCloseShort 执行平空
func (oe *OrderExecutor) executeCloseShort(plan *layers.ExecutionPlan, result *layers.OrderResult) error {
	orderId, err := oe.trader.CloseShort(plan.Symbol, plan.Quantity)
	if err != nil {
		return fmt.Errorf("failed to close short: %w", err)
	}

	result.OrderID = orderId
	result.FilledQuantity = plan.Quantity

	return nil
}

// MonitorOrder 监控订单状态
func (oe *OrderExecutor) MonitorOrder(orderID string, symbol string, timeoutSeconds int) error {
	// TODO: 实现订单监控逻辑
	// 1. 定期查询订单状态
	// 2. 检查是否成交
	// 3. 检查是否超时
	// 4. 处理部分成交
	return nil
}

// CancelOrder 取消订单
func (oe *OrderExecutor) CancelOrder(orderID string, symbol string) error {
	// 使用现有的取消所有订单功能
	return oe.trader.CancelAllOrders(symbol)
}

// GetOrderStatus 获取订单状态
func (oe *OrderExecutor) GetOrderStatus(orderID string, symbol string) (map[string]interface{}, error) {
	// TODO: 实现订单状态查询
	// 需要扩展Trader接口
	return map[string]interface{}{
		"order_id": orderID,
		"symbol":   symbol,
		"status":   "unknown",
	}, nil
}

// EmergencyCloseAllPositions 紧急平仓所有持仓
func (oe *OrderExecutor) EmergencyCloseAllPositions() error {
	// 获取所有持仓
	positions, err := oe.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// 平掉所有持仓
	for _, pos := range positions {
		symbol, ok := pos["symbol"].(string)
		if !ok {
			continue
		}

		side, ok := pos["side"].(string)
		if !ok {
			continue
		}

		qty, ok := pos["quantity"].(float64)
		if !ok {
			continue
		}

		// 平仓
		if side == "long" {
			_, _ = oe.trader.CloseLong(symbol, qty)
		} else if side == "short" {
			_, _ = oe.trader.CloseShort(symbol, qty)
		}
	}

	return nil
}

// GetActivePositions 获取活跃持仓
func (oe *OrderExecutor) GetActivePositions() ([]map[string]interface{}, error) {
	return oe.trader.GetPositions()
}

// GetAccountBalance 获取账户余额
func (oe *OrderExecutor) GetAccountBalance() (map[string]interface{}, error) {
	return oe.trader.GetBalance()
}

// ValidateOrderParameters 验证订单参数
func (oe *OrderExecutor) ValidateOrderParameters(plan *layers.ExecutionPlan) error {
	if plan.Symbol == "" {
		return fmt.Errorf("symbol is empty")
	}

	if plan.Quantity <= 0 {
		return fmt.Errorf("quantity must be positive: %f", plan.Quantity)
	}

	if plan.Leverage < 1 || plan.Leverage > oe.config.MaxLeverage {
		return fmt.Errorf("invalid leverage: %d (max: %d)", plan.Leverage, oe.config.MaxLeverage)
	}

	if plan.Action != "open_long" && plan.Action != "open_short" &&
		plan.Action != "close_long" && plan.Action != "close_short" {
		return fmt.Errorf("invalid action: %s", plan.Action)
	}

	return nil
}

// DryRunOrder 模拟执行订单（不实际下单）
func (oe *OrderExecutor) DryRunOrder(plan *layers.ExecutionPlan) (*layers.OrderResult, error) {
	result := &layers.OrderResult{
		Success:        true,
		OrderID:        fmt.Sprintf("DRYRUN-%d", time.Now().Unix()),
		FilledQuantity: plan.Quantity,
		Timestamp:      time.Now(),
		ExecutionTimeMs: 0,
	}

	// 验证参数
	if err := oe.ValidateOrderParameters(plan); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	return result, nil
}
