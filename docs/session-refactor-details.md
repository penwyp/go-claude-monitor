# Session模块重构详细实施步骤

## 前言
本文档为AGI提供详细的重构实施步骤，每个步骤都包含具体的代码示例和验证方法。

## 第一阶段：创建公共工具包

### 1.1 创建目录结构
```bash
mkdir -p internal/core/session/internal
```

### 1.2 创建timeutil.go
```go
// internal/core/session/internal/timeutil.go
package internal

import "time"

// TruncateToHour 将时间戳截断到小时
func TruncateToHour(timestamp int64) int64 {
    return (timestamp / 3600) * 3600
}

// IsWithinWindow 检查时间戳是否在窗口内
func IsWithinWindow(timestamp, windowStart, windowEnd int64) bool {
    return timestamp >= windowStart && timestamp < windowEnd
}

// FormatTimeRange 格式化时间范围
func FormatTimeRange(start, end int64) string {
    startTime := time.Unix(start, 0)
    endTime := time.Unix(end, 0)
    return fmt.Sprintf("%s-%s", 
        startTime.Format("15:04:05"),
        endTime.Format("15:04:05"))
}

// CalculateWindowEnd 计算窗口结束时间
func CalculateWindowEnd(start int64, duration time.Duration) int64 {
    return start + int64(duration.Seconds())
}
```

### 1.3 创建validation.go
```go
// internal/core/session/internal/validation.go
package internal

import (
    "fmt"
    "time"
)

// ValidateTimeRange 验证时间范围
func ValidateTimeRange(start, end int64) error {
    if start >= end {
        return fmt.Errorf("invalid time range: start(%d) >= end(%d)", start, end)
    }
    return nil
}

// ValidateWindowDuration 验证窗口持续时间
func ValidateWindowDuration(start, end int64, expected time.Duration) error {
    actual := time.Duration(end-start) * time.Second
    if actual != expected {
        return fmt.Errorf("window duration mismatch: expected %v, got %v", expected, actual)
    }
    return nil
}

// CheckWindowOverlap 检查两个窗口是否重叠
func CheckWindowOverlap(start1, end1, start2, end2 int64) bool {
    return start1 < end2 && end1 > start2
}
```

### 1.4 创建statistics.go
```go
// internal/core/session/internal/statistics.go
package internal

import "github.com/penwyp/go-claude-monitor/internal/core/model"

// CalculateTotalTokens 计算总token数
func CalculateTotalTokens(usage model.Usage) int {
    return usage.InputTokens + usage.OutputTokens + 
           usage.CacheCreationInputTokens + usage.CacheReadInputTokens
}

// MergeModelStats 合并模型统计
func MergeModelStats(stats1, stats2 *model.ModelStats) *model.ModelStats {
    if stats1 == nil {
        return stats2
    }
    if stats2 == nil {
        return stats1
    }
    return &model.ModelStats{
        Tokens: stats1.Tokens + stats2.Tokens,
        Cost:   stats1.Cost + stats2.Cost,
        Count:  stats1.Count + stats2.Count,
    }
}

// UpdateModelDistribution 更新模型分布
func UpdateModelDistribution(
    distribution map[string]*model.ModelStats,
    modelName string,
    tokens int,
    cost float64,
) {
    if distribution[modelName] == nil {
        distribution[modelName] = &model.ModelStats{}
    }
    stats := distribution[modelName]
    stats.Tokens += tokens
    stats.Cost += cost
    stats.Count++
}
```

## 第二阶段：重构types.go

### 2.1 定义嵌套结构体
```go
// internal/core/session/types_refactored.go
package session

// WindowInfo 窗口相关信息
type WindowInfo struct {
    WindowStartTime  *int64 `json:"window_start_time,omitempty"`
    IsWindowDetected bool   `json:"is_window_detected"`
    WindowSource     string `json:"window_source"`
    WindowPriority   int    `json:"window_priority"`
}

// MetricsInfo 指标信息
type MetricsInfo struct {
    TimeRemaining    time.Duration    `json:"time_remaining"`
    TokensPerMinute  float64          `json:"tokens_per_minute"`
    CostPerHour      float64          `json:"cost_per_hour"`
    CostPerMinute    float64          `json:"cost_per_minute"`
    BurnRate         float64          `json:"burn_rate"`
    BurnRateSnapshot *model.BurnRate  `json:"burn_rate_snapshot,omitempty"`
}

// ProjectionInfo 预测信息
type ProjectionInfo struct {
    ProjectedTokens  int                    `json:"projected_tokens"`
    ProjectedCost    float64                `json:"projected_cost"`
    PredictedEndTime int64                  `json:"predicted_end_time"`
    ProjectionData   map[string]interface{} `json:"projection_data"`
}

// 更新Session结构体，使用嵌套结构
type Session struct {
    // 基础信息
    ID               string `json:"id"`
    StartTime        int64  `json:"start_time"`
    StartHour        int64  `json:"start_hour"`
    EndTime          int64  `json:"end_time"`
    ActualEndTime    *int64 `json:"actual_end_time,omitempty"`
    IsActive         bool   `json:"is_active"`
    IsGap            bool   `json:"is_gap"`
    
    // 项目信息
    ProjectName      string                    `json:"project_name"`
    Projects         map[string]*ProjectStats  `json:"projects"`
    
    // 窗口信息（使用嵌套结构）
    Window           WindowInfo                `json:"window"`
    
    // 统计信息
    TotalTokens       int                      `json:"total_tokens"`
    TotalCost         float64                  `json:"total_cost"`
    MessageCount      int                      `json:"message_count"`
    SentMessageCount  int                      `json:"sent_message_count"`
    ModelDistribution map[string]*model.ModelStats `json:"model_distribution"`
    PerModelStats     map[string]map[string]interface{} `json:"per_model_stats"`
    HourlyMetrics     []*model.HourlyMetric    `json:"hourly_metrics"`
    
    // 指标信息（使用嵌套结构）
    Metrics          MetricsInfo               `json:"metrics"`
    
    // 预测信息（使用嵌套结构）
    Projection       ProjectionInfo            `json:"projection"`
    
    // 其他信息
    ResetTime        int64                     `json:"reset_time"`
    FirstEntryTime   int64                     `json:"first_entry_time"`
    LimitMessages    []map[string]interface{}  `json:"limit_messages"`
}
```

### 2.2 创建兼容性辅助函数
```go
// 为了保持向后兼容，提供getter/setter
func (s *Session) GetWindowStartTime() *int64 {
    return s.Window.WindowStartTime
}

func (s *Session) SetWindowStartTime(t *int64) {
    s.Window.WindowStartTime = t
}

// 类似地为其他字段提供兼容性方法...
```

## 第三阶段：优化detector.go

### 3.1 提取统计更新函数
```go
// internal/core/session/detector_helpers.go
package session

import (
    "github.com/penwyp/go-claude-monitor/internal/core/session/internal"
)

// updateProjectStatistics 更新项目统计
func (d *SessionDetector) updateProjectStatistics(
    projectStats *ProjectStats,
    tl timeline.TimestampedLog,
    totalTokens int,
    cost float64,
) {
    projectStats.TotalTokens += totalTokens
    projectStats.MessageCount++
    projectStats.TotalCost += cost
    
    if tl.Log.Type == "message:sent" {
        projectStats.SentMessageCount++
    }
    
    // 更新模型分布
    modelName := util.SimplifyModelName(tl.Log.Message.Model)
    internal.UpdateModelDistribution(
        projectStats.ModelDistribution,
        modelName,
        totalTokens,
        cost,
    )
}

// updateSessionStatistics 更新会话统计
func (d *SessionDetector) updateSessionStatistics(
    session *Session,
    tl timeline.TimestampedLog,
    totalTokens int,
    cost float64,
) {
    session.TotalTokens += totalTokens
    session.TotalCost += cost
    session.MessageCount++
    
    if tl.Log.Type == "message:sent" {
        session.SentMessageCount++
    }
    
    // 更新模型分布
    modelName := util.SimplifyModelName(tl.Log.Message.Model)
    internal.UpdateModelDistribution(
        session.ModelDistribution,
        modelName,
        totalTokens,
        cost,
    )
}
```

### 3.2 实现SessionBuilder模式
```go
// internal/core/session/builders.go
package session

type SessionBuilder struct {
    session *Session
}

func NewSessionBuilder(id string) *SessionBuilder {
    return &SessionBuilder{
        session: &Session{
            ID:                id,
            Projects:          make(map[string]*ProjectStats),
            ModelDistribution: make(map[string]*model.ModelStats),
            PerModelStats:     make(map[string]map[string]interface{}),
            HourlyMetrics:     make([]*model.HourlyMetric, 0),
            LimitMessages:     make([]map[string]interface{}, 0),
        },
    }
}

func (b *SessionBuilder) WithTimeWindow(start, end int64) *SessionBuilder {
    b.session.StartTime = start
    b.session.StartHour = internal.TruncateToHour(start)
    b.session.EndTime = end
    b.session.ResetTime = end
    b.session.PredictedEndTime = end
    return b
}

func (b *SessionBuilder) WithWindowInfo(source string, detected bool) *SessionBuilder {
    b.session.Window = WindowInfo{
        WindowStartTime:  &b.session.StartTime,
        IsWindowDetected: detected,
        WindowSource:     source,
    }
    return b
}

func (b *SessionBuilder) Build() *Session {
    // 初始化嵌套结构
    if b.session.Projection.ProjectionData == nil {
        b.session.Projection.ProjectionData = make(map[string]interface{})
    }
    return b.session
}
```

### 3.3 实现窗口策略模式
```go
// internal/core/session/strategies.go
package session

type WindowSourceStrategy interface {
    GetPriority() int
    GetSource() string
    ValidateWindow(start, end int64) bool
}

type LimitMessageStrategy struct{}

func (s *LimitMessageStrategy) GetPriority() int { return 10 }
func (s *LimitMessageStrategy) GetSource() string { return "limit_message" }
func (s *LimitMessageStrategy) ValidateWindow(start, end int64) bool {
    // 限制消息窗口总是有效的
    return true
}

type GapDetectionStrategy struct {
    sessionDuration time.Duration
}

func (s *GapDetectionStrategy) GetPriority() int { return 5 }
func (s *GapDetectionStrategy) GetSource() string { return "gap" }
func (s *GapDetectionStrategy) ValidateWindow(start, end int64) bool {
    return end-start == int64(s.sessionDuration.Seconds())
}

// 策略注册器
type StrategyRegistry struct {
    strategies map[string]WindowSourceStrategy
}

func NewStrategyRegistry() *StrategyRegistry {
    return &StrategyRegistry{
        strategies: make(map[string]WindowSourceStrategy),
    }
}

func (r *StrategyRegistry) Register(name string, strategy WindowSourceStrategy) {
    r.strategies[name] = strategy
}

func (r *StrategyRegistry) GetStrategy(source string) WindowSourceStrategy {
    return r.strategies[source]
}
```

## 第四阶段：简化window_history.go

### 4.1 提取验证辅助函数
```go
// internal/core/session/window_validation.go
package session

import (
    "github.com/penwyp/go-claude-monitor/internal/core/session/internal"
)

// validateTimeBounds 验证时间边界
func validateTimeBounds(timestamp, minTime, maxTime int64) error {
    if timestamp < minTime {
        return fmt.Errorf("timestamp %d is before minimum allowed time %d", timestamp, minTime)
    }
    if timestamp > maxTime {
        return fmt.Errorf("timestamp %d is after maximum allowed time %d", timestamp, maxTime)
    }
    return nil
}

// isSameDay 检查两个时间戳是否在同一天
func isSameDay(time1, time2 int64) bool {
    t1 := time.Unix(time1, 0)
    t2 := time.Unix(time2, 0)
    return t1.Format("2006-01-02") == t2.Format("2006-01-02")
}

// adjustWindowForOverlap 调整窗口避免重叠
func adjustWindowForOverlap(proposed, existing WindowRecord) (int64, int64) {
    if internal.CheckWindowOverlap(
        proposed.StartTime, proposed.EndTime,
        existing.StartTime, existing.EndTime,
    ) {
        // 调整到现有窗口之后
        newStart := existing.EndTime
        newEnd := newStart + constants.SessionDurationSeconds
        return newStart, newEnd
    }
    return proposed.StartTime, proposed.EndTime
}
```

### 4.2 重构ValidateNewWindow函数
```go
// 使用辅助函数简化ValidateNewWindow
func (m *WindowHistoryManager) ValidateNewWindow(proposedStart, proposedEnd int64) (int64, int64, bool) {
    m.history.mu.RLock()
    defer m.history.mu.RUnlock()
    
    // 步骤1：验证时间边界
    currentTime := time.Now().Unix()
    minTime := currentTime - constants.LimitWindowRetentionSeconds
    maxTime := currentTime + constants.MaxFutureWindowSeconds
    
    if err := validateTimeBounds(proposedEnd, minTime, maxTime); err != nil {
        util.LogWarn(fmt.Sprintf("Window validation failed: %v", err))
        return proposedStart, proposedEnd, false
    }
    
    // 步骤2：处理限制消息窗口（最高优先级）
    validStart, validEnd := m.handleLimitMessageWindows(proposedStart, proposedEnd)
    
    // 步骤3：处理其他窗口
    validStart, validEnd = m.handleOtherWindows(validStart, validEnd)
    
    // 步骤4：确保窗口持续时间正确
    if err := internal.ValidateWindowDuration(validStart, validEnd, constants.SessionDuration); err != nil {
        validEnd = validStart + constants.SessionDurationSeconds
    }
    
    isValid := validStart == proposedStart
    return validStart, validEnd, isValid
}

// 处理限制消息窗口
func (m *WindowHistoryManager) handleLimitMessageWindows(start, end int64) (int64, int64) {
    for _, record := range m.history.Windows {
        if !record.IsLimitReached || record.Source != "limit_message" {
            continue
        }
        
        if !isSameDay(start, record.StartTime) {
            continue
        }
        
        start, end = adjustWindowForOverlap(
            WindowRecord{StartTime: start, EndTime: end},
            record,
        )
    }
    return start, end
}
```

## 第五阶段：重构limit_parser.go

### 5.1 定义策略接口
```go
// internal/core/session/limit_strategies.go
package session

type LimitParseStrategy interface {
    CanParse(content string) bool
    Parse(log model.ConversationLog) *LimitInfo
    GetPriority() int
}

// Opus限制策略
type OpusLimitStrategy struct {
    opusPattern *regexp.Regexp
    waitPattern *regexp.Regexp
}

func NewOpusLimitStrategy() *OpusLimitStrategy {
    return &OpusLimitStrategy{
        opusPattern: regexp.MustCompile(`(?i)(opus).*(rate\s*limit|limit\s*exceeded)`),
        waitPattern: regexp.MustCompile(`(?i)wait\s+(\d+)\s+minutes?`),
    }
}

func (s *OpusLimitStrategy) CanParse(content string) bool {
    return s.opusPattern.MatchString(strings.ToLower(content))
}

func (s *OpusLimitStrategy) GetPriority() int { return 10 }

func (s *OpusLimitStrategy) Parse(log model.ConversationLog) *LimitInfo {
    if !s.CanParse(log.Content) {
        return nil
    }
    
    limit := buildBaseLimitInfo("opus_limit", log)
    
    // 提取等待时间
    if matches := s.waitPattern.FindStringSubmatch(log.Content); len(matches) > 1 {
        waitMinutes := parseWaitMinutes(matches[1])
        limit.WaitMinutes = &waitMinutes
        resetTime := calculateResetTime(limit.Timestamp, waitMinutes)
        limit.ResetTime = &resetTime
    }
    
    return limit
}
```

### 5.2 提取公共函数
```go
// internal/core/session/limit_helpers.go
package session

// buildBaseLimitInfo 构建基础限制信息
func buildBaseLimitInfo(limitType string, log model.ConversationLog) *LimitInfo {
    timestamp, _ := time.Parse(time.RFC3339, log.Timestamp)
    return &LimitInfo{
        Type:      limitType,
        Timestamp: timestamp.Unix(),
        Content:   log.Content,
        RequestID: log.RequestId,
        SessionID: log.SessionId,
        Model:     log.Message.Model,
    }
}

// extractResetTimestamp 提取重置时间戳
func extractResetTimestamp(content string, pattern *regexp.Regexp) *int64 {
    if matches := pattern.FindStringSubmatch(content); len(matches) > 1 {
        timestamp := parseTimestamp(matches[1])
        return &timestamp
    }
    return nil
}

// parseTimestamp 解析时间戳（处理毫秒）
func parseTimestamp(timestampStr string) int64 {
    var timestamp int64
    fmt.Sscanf(timestampStr, "%d", &timestamp)
    if timestamp > 1e12 {
        timestamp = timestamp / 1000
    }
    return timestamp
}

// calculateResetTime 计算重置时间
func calculateResetTime(baseTime int64, waitMinutes int) int64 {
    return baseTime + int64(waitMinutes*60)
}
```

### 5.3 重构LimitParser使用策略
```go
type LimitParser struct {
    strategies []LimitParseStrategy
}

func NewLimitParser() *LimitParser {
    return &LimitParser{
        strategies: []LimitParseStrategy{
            NewOpusLimitStrategy(),
            NewGeneralLimitStrategy(),
            NewSystemLimitStrategy(),
        },
    }
}

func (p *LimitParser) ParseLogs(logs []model.ConversationLog) []LimitInfo {
    var limits []LimitInfo
    
    for _, log := range logs {
        // 按优先级尝试各个策略
        for _, strategy := range p.strategies {
            if limit := strategy.Parse(log); limit != nil {
                limits = append(limits, *limit)
                break // 找到匹配的策略就停止
            }
        }
    }
    
    return limits
}
```

## 第六阶段：优化calculator.go

### 6.1 提取计算辅助函数
```go
// internal/core/session/calc_helpers.go
package session

// calculateUtilizationRate 计算利用率
func calculateUtilizationRate(actual, expected float64) float64 {
    if expected <= 0 {
        return 0
    }
    return (actual / expected) * 100
}

// predictTimeToLimit 预测达到限制的时间
func predictTimeToLimit(current, rate, limit float64) time.Duration {
    if rate <= 0 || current >= limit {
        return 0
    }
    remaining := limit - current
    minutes := remaining / rate
    return time.Duration(minutes) * time.Minute
}

// capValue 限制值不超过上限
func capValue(value, limit float64) float64 {
    if limit > 0 && value > limit {
        return limit
    }
    return value
}
```

### 6.2 重构计算逻辑
```go
func (c *MetricsCalculator) Calculate(session *Session) {
    if session == nil {
        return
    }
    
    // 排序指标
    c.sortHourlyMetrics(session)
    
    // 计算利用率
    utilization := c.calculateSessionUtilization(session)
    
    // 更新燃烧率
    c.updateBurnRate(session, utilization)
    
    // 预测限制时间
    c.predictLimitTime(session)
    
    // 调整预测值
    c.adjustProjections(session)
}

func (c *MetricsCalculator) calculateSessionUtilization(session *Session) float64 {
    elapsed := time.Since(time.Unix(session.StartTime, 0))
    if elapsed.Minutes() <= 0 {
        return 0
    }
    
    if c.planLimits.TokenLimit > 0 {
        expectedRate := float64(c.planLimits.TokenLimit) / (5 * 60)
        return calculateUtilizationRate(session.Metrics.TokensPerMinute, expectedRate)
    }
    
    if c.planLimits.CostLimit > 0 {
        expectedRate := c.planLimits.CostLimit / 5
        return calculateUtilizationRate(session.Metrics.CostPerHour, expectedRate)
    }
    
    return 0
}
```

## 验证步骤

### 单元测试验证
每个重构步骤后运行：
```bash
go test ./internal/core/session/... -v
```

### 集成测试验证
```bash
go test ./internal/application/top/... -v
```

### 性能测试
```bash
go test -bench=. ./internal/core/session/...
```

### 功能验证
```bash
# 构建并运行
make build
./bin/go-claude-monitor top --plan max5
```

## 回滚计划

如果任何步骤出现问题：
```bash
# 回滚到上一个稳定版本
git checkout HEAD~1

# 或回滚到特定标记
git checkout stable-before-refactor
```

## 注意事项

1. **保持接口稳定** - 公共API不能改变
2. **渐进式重构** - 每次只改一小部分
3. **充分测试** - 每步都要验证
4. **文档更新** - 同步更新相关文档
5. **性能监控** - 确保没有性能退化