package trader

import (
	"fmt"
	"log"
	"nofx/market"
)

// OrderExecutor 订单执行器 - 三层架构中的底层组件
// 职责：封装所有订单执行逻辑，提供统一的执行接口
// 这是底层与交易所交互的唯一入口
type OrderExecutor struct {
	trader         Trader // 交易器接口
	isCrossMargin  bool   // 是否使用全仓模式
}

// NewOrderExecutor 创建订单执行器实例
func NewOrderExecutor(trader Trader, isCrossMargin bool) *OrderExecutor {
	return &OrderExecutor{
		trader:        trader,
		isCrossMargin: isCrossMargin,
	}
}

// OpenLongParams 开多仓参数
type OpenLongParams struct {
	Symbol        string  // 币种
	Quantity      float64 // 数量
	Leverage      int     // 杠杆倍数
	StopLoss      float64 // 止损价格
	TakeProfit    float64 // 止盈价格
}

// OpenLongResult 开多仓结果
type OpenLongResult struct {
	OrderID   int64                  // 订单ID
	Symbol    string                 // 币种
	Quantity  float64                // 实际成交数量
	OrderData map[string]interface{} // 原始订单数据
}

// ExecuteOpenLong 执行开多仓操作
func (e *OrderExecutor) ExecuteOpenLong(params OpenLongParams) (*OpenLongResult, error) {
	log.Printf("  📈 [执行器] 开多仓: %s, 数量: %.4f, 杠杆: %dx", params.Symbol, params.Quantity, params.Leverage)

	// 设置仓位模式
	if err := e.trader.SetMarginMode(params.Symbol, e.isCrossMargin); err != nil {
		log.Printf("  ⚠️ [执行器] 设置仓位模式失败: %v (继续执行)", err)
	}

	// 设置杠杆
	if err := e.trader.SetLeverage(params.Symbol, params.Leverage); err != nil {
		return nil, fmt.Errorf("设置杠杆失败: %w", err)
	}

	// 开仓
	order, err := e.trader.OpenLong(params.Symbol, params.Quantity, params.Leverage)
	if err != nil {
		return nil, fmt.Errorf("开多仓失败: %w", err)
	}

	// 获取订单ID
	var orderID int64
	if id, ok := order["orderId"].(int64); ok {
		orderID = id
	}

	log.Printf("  ✓ [执行器] 开仓成功，订单ID: %v", orderID)

	// 设置止损止盈
	if params.StopLoss > 0 {
		if err := e.trader.SetStopLoss(params.Symbol, "LONG", params.Quantity, params.StopLoss); err != nil {
			log.Printf("  ⚠ [执行器] 设置止损失败: %v", err)
		} else {
			log.Printf("  ✓ [执行器] 设置止损: %.4f", params.StopLoss)
		}
	}

	if params.TakeProfit > 0 {
		if err := e.trader.SetTakeProfit(params.Symbol, "LONG", params.Quantity, params.TakeProfit); err != nil {
			log.Printf("  ⚠ [执行器] 设置止盈失败: %v", err)
		} else {
			log.Printf("  ✓ [执行器] 设置止盈: %.4f", params.TakeProfit)
		}
	}

	return &OpenLongResult{
		OrderID:   orderID,
		Symbol:    params.Symbol,
		Quantity:  params.Quantity,
		OrderData: order,
	}, nil
}

// OpenShortParams 开空仓参数
type OpenShortParams struct {
	Symbol        string  // 币种
	Quantity      float64 // 数量
	Leverage      int     // 杠杆倍数
	StopLoss      float64 // 止损价格
	TakeProfit    float64 // 止盈价格
}

// OpenShortResult 开空仓结果
type OpenShortResult struct {
	OrderID   int64                  // 订单ID
	Symbol    string                 // 币种
	Quantity  float64                // 实际成交数量
	OrderData map[string]interface{} // 原始订单数据
}

// ExecuteOpenShort 执行开空仓操作
func (e *OrderExecutor) ExecuteOpenShort(params OpenShortParams) (*OpenShortResult, error) {
	log.Printf("  📉 [执行器] 开空仓: %s, 数量: %.4f, 杠杆: %dx", params.Symbol, params.Quantity, params.Leverage)

	// 设置仓位模式
	if err := e.trader.SetMarginMode(params.Symbol, e.isCrossMargin); err != nil {
		log.Printf("  ⚠️ [执行器] 设置仓位模式失败: %v (继续执行)", err)
	}

	// 设置杠杆
	if err := e.trader.SetLeverage(params.Symbol, params.Leverage); err != nil {
		return nil, fmt.Errorf("设置杠杆失败: %w", err)
	}

	// 开仓
	order, err := e.trader.OpenShort(params.Symbol, params.Quantity, params.Leverage)
	if err != nil {
		return nil, fmt.Errorf("开空仓失败: %w", err)
	}

	// 获取订单ID
	var orderID int64
	if id, ok := order["orderId"].(int64); ok {
		orderID = id
	}

	log.Printf("  ✓ [执行器] 开仓成功，订单ID: %v", orderID)

	// 设置止损止盈
	if params.StopLoss > 0 {
		if err := e.trader.SetStopLoss(params.Symbol, "SHORT", params.Quantity, params.StopLoss); err != nil {
			log.Printf("  ⚠ [执行器] 设置止损失败: %v", err)
		} else {
			log.Printf("  ✓ [执行器] 设置止损: %.4f", params.StopLoss)
		}
	}

	if params.TakeProfit > 0 {
		if err := e.trader.SetTakeProfit(params.Symbol, "SHORT", params.Quantity, params.TakeProfit); err != nil {
			log.Printf("  ⚠ [执行器] 设置止盈失败: %v", err)
		} else {
			log.Printf("  ✓ [执行器] 设置止盈: %.4f", params.TakeProfit)
		}
	}

	return &OpenShortResult{
		OrderID:   orderID,
		Symbol:    params.Symbol,
		Quantity:  params.Quantity,
		OrderData: order,
	}, nil
}

// ClosePositionParams 平仓参数
type ClosePositionParams struct {
	Symbol   string  // 币种
	Side     string  // 方向 (long/short)
	Quantity float64 // 数量（0表示全部平仓）
}

// ClosePositionResult 平仓结果
type ClosePositionResult struct {
	OrderID     int64                  // 订单ID
	Symbol      string                 // 币种
	Side        string                 // 方向
	ClosePrice  float64                // 平仓价格
	OrderData   map[string]interface{} // 原始订单数据
}

// ExecuteClosePosition 执行平仓操作
func (e *OrderExecutor) ExecuteClosePosition(params ClosePositionParams) (*ClosePositionResult, error) {
	log.Printf("  🔄 [执行器] 平%s仓: %s", params.Side, params.Symbol)

	// 获取当前价格
	marketData, err := market.Get(params.Symbol)
	if err != nil {
		return nil, fmt.Errorf("获取市场价格失败: %w", err)
	}
	closePrice := marketData.CurrentPrice

	var order map[string]interface{}
	if params.Side == "long" {
		order, err = e.trader.CloseLong(params.Symbol, params.Quantity)
	} else if params.Side == "short" {
		order, err = e.trader.CloseShort(params.Symbol, params.Quantity)
	} else {
		return nil, fmt.Errorf("无效的持仓方向: %s", params.Side)
	}

	if err != nil {
		return nil, fmt.Errorf("平仓失败: %w", err)
	}

	// 获取订单ID
	var orderID int64
	if id, ok := order["orderId"].(int64); ok {
		orderID = id
	}

	log.Printf("  ✓ [执行器] 平仓成功，订单ID: %v", orderID)

	return &ClosePositionResult{
		OrderID:    orderID,
		Symbol:     params.Symbol,
		Side:       params.Side,
		ClosePrice: closePrice,
		OrderData:  order,
	}, nil
}

// CancelAllOrders 取消指定币种的所有挂单
func (e *OrderExecutor) CancelAllOrders(symbol string) error {
	log.Printf("  🗑️ [执行器] 取消 %s 的所有挂单", symbol)

	if err := e.trader.CancelAllOrders(symbol); err != nil {
		return fmt.Errorf("取消挂单失败: %w", err)
	}

	log.Printf("  ✓ [执行器] 取消挂单成功")
	return nil
}

// GetCurrentPrice 获取当前市场价格
func (e *OrderExecutor) GetCurrentPrice(symbol string) (float64, error) {
	return e.trader.GetMarketPrice(symbol)
}

// CheckExistingPosition 检查是否存在同方向持仓
// 返回：是否存在，错误信息
func (e *OrderExecutor) CheckExistingPosition(symbol, side string) (bool, error) {
	positions, err := e.trader.GetPositions()
	if err != nil {
		return false, fmt.Errorf("获取持仓失败: %w", err)
	}

	for _, pos := range positions {
		if pos["symbol"] == symbol && pos["side"] == side {
			return true, nil
		}
	}

	return false, nil
}

// GetAccountBalance 获取账户余额信息
func (e *OrderExecutor) GetAccountBalance() (map[string]interface{}, error) {
	return e.trader.GetBalance()
}

// GetAllPositions 获取所有持仓
func (e *OrderExecutor) GetAllPositions() ([]map[string]interface{}, error) {
	return e.trader.GetPositions()
}
