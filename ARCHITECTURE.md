# GoLang AI Trading System - Three-Layer Architecture

## 架构概述

本项目采用严格的三层架构设计，将数据处理、AI决策和策略控制完全解耦，实现高内聚低耦合的系统设计。

## 三层架构详解

### 第一层：底层代码层 (Data & Execution Layer)

**位置**: `market/` 和 `trader/` 包

**职责**:
- 所有 I/O 和原子操作
- 数据获取与清洗
- 风险计算核心（止损点、仓位大小、保证金利用率）
- 规则引擎执行（触发硬性止损、熔断机制）
- 订单发送和实时状态监控

**核心组件**:

1. **market/data_cleaner.go** - 数据清洗和验证
   - 验证市场数据完整性和合理性
   - 清洗异常数据（修正超出范围的值）
   - 流动性检查

2. **trader/risk_calculator.go** - 风险计算核心
   - 计算合理的仓位大小
   - 计算动态止损/止盈价格
   - 保证金利用率计算
   - 风险回报比验证
   - 强平价格计算

3. **trader/rule_engine.go** - 规则引擎
   - 账户级别风控（日亏损、最大回撤、保证金使用率）
   - 持仓级别风控（止损触发、接近强平、持仓时长）
   - 开仓前风控（持仓数量、杠杆倍数、仓位大小、可用保证金）
   - 熔断机制（连续亏损、快速亏损）

4. **trader/executor.go** - 统一订单执行器
   - 开多仓/开空仓执行
   - 平仓执行
   - 止损止盈设置
   - 订单状态查询

**接口定义**:
- 只接收来自上层的结构化指令
- 返回原始执行结果和基础状态
- 不涉及决策逻辑

---

### 第二层：中间 AI 层 (AI Decision Layer)

**位置**: `decision/` 包

**职责**:
- 纯粹的智能决策引擎
- 与底层执行和上层参数计算完全解耦

**核心功能（仅限三项）**:
1. 市场状态判断（趋势、震荡、突破）
2. 交易机会识别（基于当前状态和输入数据）
3. 输出结构化决策：`{"action": "BUY/SELL/HOLD", "confidence": 0.0 - 1.0}`

**核心组件**:

1. **decision/types.go** - 类型定义
   - AIDecisionSignal: 纯AI决策信号
   - AIAnalysisResult: AI完整分析结果
   - TradingContext: 交易上下文
   - StrategyDecision: 策略决策（包含完整参数）

2. **decision/ai_core.go** - 纯AI决策引擎
   - Analyze(): 主分析入口，接收交易上下文
   - buildAIInputPrompt(): 构建AI输入
   - parseAIResponse(): 解析AI响应
   - inferMarketState(): 推断市场状态

**输入/输出**:
- 输入：清洗后的市场数据（TradingContext）
- 输出：结构化的决策信号（AIDecisionSignal）

---

### 第三层：上层代码层 (Strategy Control Layer)

**位置**: `decision/` 包

**职责**:
- 策略逻辑、参数计算、二次风控和协调
- 接收AI决策信号
- 计算具体交易参数（入场价、止损距离、手数）
- 执行二次风控验证
- 协调整个数据流

**核心组件**:

1. **decision/strategy_coordinator.go** - 策略协调器
   - Process(): 主处理入口，协调三层工作
   - cleanAndValidateMarketData(): 数据清洗与验证
   - calculateParametersAndValidate(): 参数计算与风控验证
   - processSignal(): 处理单个AI信号
   - calculateOpenLongParameters(): 计算开多仓参数
   - calculateOpenShortParameters(): 计算开空仓参数

**处理流程**:
1. 第一步：调用底层数据清洗器验证数据
2. 第二步：调用AI核心进行决策分析
3. 第三步：基于AI信号计算具体参数并执行二次风控

---

## 数据流设计

```
市场原始数据
    ↓
[底层] DataCleaner.Validate() - 数据验证和清洗
    ↓
[AI层] AICore.Analyze() - 市场分析和决策
    ↓
[上层] StrategyCoordinator.Process() - 参数计算和风控
    ↓
[上层] RiskCalculator.Calculate() - 风险参数计算
    ↓
[底层] RuleEngine.Check() - 规则验证
    ↓
[底层] Executor.Execute() - 订单执行
    ↓
交易所
```

## 关键设计原则

### 1. 单一职责原则
每一层只负责自己的核心功能：
- 底层：数据和执行
- AI层：分析和决策
- 上层：策略和协调

### 2. 依赖倒置原则
上层依赖底层的抽象接口，而不是具体实现

### 3. 开闭原则
对扩展开放，对修改封闭。可以轻松添加新的：
- 数据清洗规则
- 风险计算方法
- AI决策模型
- 策略逻辑

### 4. 接口隔离原则
各层之间通过清晰定义的数据结构通信：
- `TradingContext`: 传递给AI的上下文
- `AIDecisionSignal`: AI的输出信号
- `StrategyDecision`: 策略的最终决策

## 优势

1. **高内聚**：每个模块职责明确，代码组织清晰
2. **低耦合**：层与层之间通过标准接口通信，易于修改和扩展
3. **可测试**：每层都可以独立测试，已提供完整的单元测试
4. **可维护**：清晰的架构使代码易于理解和维护
5. **可扩展**：可以轻松替换或增强任何一层的实现

## 单元测试

所有核心组件都提供了单元测试：
- `trader/risk_calculator_test.go` - 风险计算器测试
- `trader/rule_engine_test.go` - 规则引擎测试
- `market/data_cleaner_test.go` - 数据清洗器测试

运行测试：
```bash
go test ./trader -v
go test ./market -v
```

## 向后兼容性

为了保持向后兼容，保留了原有的类型别名：
- `Context` = `TradingContext`
- `Decision` = `StrategyDecision`
- `FullDecision` 结构保持不变

现有的 `decision/engine.go` 和 `trader/auto_trader.go` 可以逐步迁移到新架构，无需一次性重写所有代码。

## 未来改进方向

1. 完全迁移 `engine.go` 到新的三层架构
2. 重构 `auto_trader.go` 使用策略协调器
3. 添加更多的风险计算策略
4. 增强AI决策的市场状态分析
5. 添加更多的规则引擎规则
