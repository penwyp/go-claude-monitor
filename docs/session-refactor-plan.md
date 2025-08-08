# Session模块重构计划

## 概述
本文档详细描述了session模块的重构计划，旨在通过提取函数、创建公共结构体和应用设计模式来优化代码结构，同时**保持所有业务流程完全不变**。

## 核心原则
1. **不改变任何业务流程** - 所有重构仅涉及代码组织，不改变逻辑
2. **保持功能完全等同** - 重构前后行为必须一致
3. **提高可读性** - 通过提取和命名使代码意图更清晰
4. **增强可维护性** - 通过设计模式和模块化提高维护效率

## 文件分析与优化策略

### 1. types.go (74行)
**问题**: Session结构体包含40+字段，过于庞大和复杂

**优化策略**: 提取嵌套结构体
```go
// 窗口相关信息
type WindowInfo struct {
    WindowStartTime  *int64
    IsWindowDetected bool
    WindowSource     string
    WindowPriority   int
}

// 指标信息
type MetricsInfo struct {
    TimeRemaining    time.Duration
    TokensPerMinute  float64
    CostPerHour      float64
    CostPerMinute    float64
    BurnRate         float64
    BurnRateSnapshot *model.BurnRate
}

// 预测信息
type ProjectionInfo struct {
    ProjectedTokens  int
    ProjectedCost    float64
    PredictedEndTime int64
    ProjectionData   map[string]interface{}
}
```

### 2. detector.go (929行)
**问题**: 
- `detectSessionsFromGlobalTimeline`函数169行，职责过多
- 大量重复的统计更新逻辑
- 调试日志分散

**优化策略**:

#### 2.1 提取辅助函数
```go
// 提取token计算
func calculateTotalTokens(usage model.Usage) int

// 提取统计更新
func updateModelStats(stats *model.ModelStats, tokens int, cost float64)

// 提取窗口时间计算
func calculateWindowBoundaries(timestamp int64, duration time.Duration) (start, end int64)

// 提取验证逻辑
func validateTokenDiscrepancy(sessionTokens, timelineTokens int64) error
```

#### 2.2 应用Builder模式
```go
type SessionBuilder struct {
    session *Session
}

func NewSessionBuilder(id string) *SessionBuilder
func (b *SessionBuilder) WithWindow(start, end int64) *SessionBuilder
func (b *SessionBuilder) WithSource(source string) *SessionBuilder
func (b *SessionBuilder) Build() *Session
```

#### 2.3 应用Strategy模式处理不同窗口来源
```go
type WindowSourceStrategy interface {
    ProcessWindow(candidate WindowCandidate) WindowCandidate
    GetPriority() int
}

type LimitMessageStrategy struct{}
type GapDetectionStrategy struct{}
type FirstMessageStrategy struct{}
```

### 3. window_history.go (583行)
**问题**: 
- `ValidateNewWindow`函数127行，逻辑复杂
- 重复的时间边界检查
- 未使用的`MergeAccountWindows`函数

**优化策略**:

#### 3.1 提取验证函数
```go
// 时间边界验证
func validateTimeBounds(timestamp int64, minTime, maxTime int64) error

// 窗口重叠检查
func checkWindowOverlap(window1, window2 WindowRecord) bool

// 冲突调整
func adjustWindowForConflict(proposed, existing WindowRecord) WindowRecord

// 日期比较
func isSameDay(time1, time2 int64) bool
```

#### 3.2 创建时间格式化工具
```go
type TimeFormatter struct {
    format string
}

func (f *TimeFormatter) Format(unixTime int64) string
func (f *TimeFormatter) FormatRange(start, end int64) string
```

#### 3.3 删除死代码
- 移除`MergeAccountWindows`函数（未被调用）

### 4. limit_parser.go (276行)
**问题**: 
- 多个解析函数结构相似
- 正则表达式分散
- 类型转换逻辑重复

**优化策略**:

#### 4.1 策略模式重构
```go
type LimitParseStrategy interface {
    CanParse(content string) bool
    Parse(log model.ConversationLog) *LimitInfo
}

type StrategyRegistry struct {
    strategies []LimitParseStrategy
}

func (r *StrategyRegistry) Parse(log model.ConversationLog) *LimitInfo
```

#### 4.2 提取公共函数
```go
// 提取重置时间
func extractResetTime(content string, pattern *regexp.Regexp) *int64

// 构建限制信息
func buildLimitInfo(limitType string, log model.ConversationLog) *LimitInfo

// 时间戳转换
func normalizeTimestamp(timestamp int64) int64
```

### 5. calculator.go (114行)
**问题**: 计算逻辑可以更模块化

**优化策略**:

#### 5.1 提取计算函数
```go
// 利用率计算
func calculateUtilizationRate(actual, expected float64) float64

// 预测限制时间
func predictTimeToLimit(current, rate, limit float64) time.Duration

// 调整预测值
func capProjection(projected, limit float64) float64
```

## 公共工具包设计

### session/internal/timeutil.go
```go
// 时间处理工具函数
func TruncateToHour(timestamp int64) int64
func IsWithinDuration(timestamp int64, duration time.Duration) bool
func FormatDuration(seconds int64) string
```

### session/internal/validation.go
```go
// 通用验证逻辑
func ValidateTimeRange(start, end int64) error
func ValidateWindowDuration(start, end int64, expected time.Duration) error
```

### session/internal/statistics.go
```go
// 统计辅助函数
func SumTokens(entries []model.Usage) int
func CalculateAverage(values []float64) float64
func MergeModelStats(stats1, stats2 *model.ModelStats) *model.ModelStats
```

## 实施步骤

### 第一阶段：创建公共工具包
1. 创建internal目录结构
2. 提取通用函数到工具包
3. 更新现有代码引用

### 第二阶段：重构types.go
1. 定义嵌套结构体
2. 更新Session结构体
3. 确保向后兼容

### 第三阶段：优化detector.go
1. 提取辅助函数
2. 实现Builder模式
3. 应用Strategy模式
4. 简化主函数

### 第四阶段：简化window_history.go
1. 提取验证函数
2. 创建格式化工具
3. 删除死代码

### 第五阶段：重构limit_parser.go
1. 实现策略模式
2. 提取公共函数
3. 简化主解析逻辑

### 第六阶段：优化calculator.go
1. 提取计算函数
2. 简化条件逻辑

### 第七阶段：测试验证
1. 运行所有单元测试
2. 进行集成测试
3. 性能对比测试

## 预期成果

### 代码质量提升
- 单个函数长度减少60%以上
- 代码复用率提高40%
- 圈复杂度降低50%

### 可维护性改善
- 更清晰的代码结构
- 更好的模块化
- 更容易的单元测试

### 性能保持
- 执行效率不变或略有提升
- 内存使用保持相同水平

## 风险控制

1. **渐进式重构** - 每次只改动一个文件
2. **充分测试** - 每步都运行测试确保功能一致
3. **版本控制** - 每个阶段创建独立commit
4. **回滚计划** - 保持可以快速回滚的能力

## 成功标准

1. 所有现有测试通过
2. 功能行为完全一致
3. 代码覆盖率不降低
4. 性能指标不退化