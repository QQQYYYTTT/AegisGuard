// backend/internal/gates/batch_window.go
// Package gates 实现 AegisGuard 的批量窗口判定机制
// 参考 TrinityGuard 的滑动窗口设计，纯代码优化，解决并发调用 LLM 的开销问题
// 核心思想：来 100 条消息 → 批量处理 → 只调用 20 次 LLM（甚至不用 LLM，用规则引擎）
package gates

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// WindowEvent 窗口中的事件
type WindowEvent struct {
	Timestamp   time.Time              // 事件时间戳
	AgentID     string                 // Agent ID
	ToolName    string                 // 工具名称
	Params      map[string]interface{} // 工具参数
	RiskLevel   int                    // 风险等级
	Content     string                 // 内容摘要
	RequiresJud bool                   // 是否需要 LLM 判定
}

// WindowSummary 窗口摘要（用于 LLM 判定）
type WindowSummary struct {
	WindowID      string        // 窗口 ID
	StartTime     time.Time     // 窗口开始时间
	EndTime       time.Time     // 窗口结束时间
	EventCount    int           // 事件数量
	HighRiskCount int           // 高风险事件数量
	Events        []WindowEvent // 事件列表（最多 maxEvents 个）
	ActiveMonitors []string     // 当前激活的监控器
}

// BatchWindowJudge 批量窗口判定器（TrinityGuard 风格）
type BatchWindowJudge struct {
	windowSize    int           // 窗口大小（滑动窗口的事件数量）
	maxEvents     int           // 窗口摘要中最多包含的事件数
	judgeInterval time.Duration // LLM 判定间隔
	judgeFunc     JudgeFunc     // LLM 判定函数（可注入）

	mu            sync.RWMutex
	currentWindow []WindowEvent  // 当前窗口的事件
	lastJudgeTime time.Time      // 上次 LLM 判定时间
	activeMonitors map[string]bool // 激活的监控器集合

	logger *zap.Logger

	ctx    context.Context
	cancel context.CancelFunc
}

// JudgeFunc LLM 判定函数类型
type JudgeFunc func(summary *WindowSummary) (Decision, []string, string)

// NewBatchWindowJudge 创建批量窗口判定器
// 参数：
//   - windowSize: 滑动窗口大小（事件数量）
//   - maxEvents: 窗口摘要中最多包含的事件数
//   - judgeInterval: LLM 判定间隔（如 5 秒）
//   - judgeFunc: LLM 判定函数（如果为 nil，则使用规则引擎）
//
// 返回：BatchWindowJudge 实例
func NewBatchWindowJudge(windowSize, maxEvents int, judgeInterval time.Duration, judgeFunc JudgeFunc, logger *zap.Logger) *BatchWindowJudge {
	ctx, cancel := context.WithCancel(context.Background())

	if logger == nil {
		logger = zap.NewNop()
	}

	bj := &BatchWindowJudge{
		windowSize:     windowSize,
		maxEvents:      maxEvents,
		judgeInterval:  judgeInterval,
		judgeFunc:      judgeFunc,
		currentWindow:  make([]WindowEvent, 0, windowSize),
		activeMonitors: make(map[string]bool),
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
	}
	
	// 启动后台判定协程
	go bj.runPeriodicJudge()
	
	return bj
}

// AddEvent 添加事件到窗口
// 参数：
//   - event: 窗口事件
//
// 返回：当前决策（如果触发立即判定则返回 LLM 决策）
func (bj *BatchWindowJudge) AddEvent(event WindowEvent) Decision {
	bj.mu.Lock()
	defer bj.mu.Unlock()
	
	// 添加到窗口
	bj.currentWindow = append(bj.currentWindow, event)
	
	// 维护窗口大小（滑动窗口）
	if len(bj.currentWindow) > bj.windowSize {
		bj.currentWindow = bj.currentWindow[1:]
	}
	
	// 检查是否需要立即判定（高风险事件）
	if event.RiskLevel >= 70 {
		return bj.evaluateNow("high risk event detected")
	}
	
	// 检查是否达到判定间隔
	if time.Since(bj.lastJudgeTime) >= bj.judgeInterval {
		return bj.evaluateNow("judge interval reached")
	}
	
	// 默认允许（等待批量判定）
	return Allow
}

// evaluateNow 立即执行 LLM 判定
func (bj *BatchWindowJudge) evaluateNow(reason string) Decision {
	// 构建窗口摘要
	summary := bj.buildSummary()
	
	// 如果没有 LLM 判定函数，使用规则引擎
	if bj.judgeFunc == nil {
		return bj.ruleBasedJudge(summary)
	}
	
	// 调用 LLM 判定函数
	decision, monitorsToUpdate, logReason := bj.judgeFunc(summary)
	
	// 更新激活的监控器
	for _, monitor := range monitorsToUpdate {
		bj.activeMonitors[monitor] = true
	}
	
	bj.logger.Debug("[BatchWindowJudge] window judged",
		zap.String("trigger", reason),
		zap.String("decision", decision.String()),
		zap.String("reason", logReason),
	)
	
	// 更新判定时间
	bj.lastJudgeTime = time.Now()
	
	return decision
}

// buildSummary 构建窗口摘要
func (bj *BatchWindowJudge) buildSummary() *WindowSummary {
	now := time.Now()
	
	// 选择代表性事件（最多 maxEvents 个）
	events := bj.currentWindow
	if len(events) > bj.maxEvents {
		// 选择最近的事件和高风险事件
		events = selectRepresentativeEvents(events, bj.maxEvents)
	}
	
	// 统计高风险事件数量
	highRiskCount := 0
	for _, event := range events {
		if event.RiskLevel >= 70 {
			highRiskCount++
		}
	}
	
	// 构建摘要
	summary := &WindowSummary{
		WindowID:       fmt.Sprintf("window-%d", now.Unix()),
		StartTime:      now.Add(-bj.judgeInterval),
		EndTime:        now,
		EventCount:     len(bj.currentWindow),
		HighRiskCount:  highRiskCount,
		Events:         events,
		ActiveMonitors: bj.getActiveMonitorsList(),
	}
	
	return summary
}

// selectRepresentativeEvents 选择代表性事件
func selectRepresentativeEvents(events []WindowEvent, maxCount int) []WindowEvent {
	if len(events) <= maxCount {
		return events
	}
	
	// 优先选择高风险事件和最近事件
	selected := make([]WindowEvent, 0, maxCount)
	
	// 先选高风险事件
	for _, event := range events {
		if event.RiskLevel >= 70 && len(selected) < maxCount {
			selected = append(selected, event)
		}
	}
	
	// 再选最近事件
	if len(selected) < maxCount {
		for i := len(events) - 1; i >= 0 && len(selected) < maxCount; i-- {
			selected = append(selected, events[i])
		}
	}
	
	return selected
}

// getActiveMonitorsList 获取激活的监控器列表
func (bj *BatchWindowJudge) getActiveMonitorsList() []string {
	list := make([]string, 0, len(bj.activeMonitors))
	for monitor := range bj.activeMonitors {
		list = append(list, monitor)
	}
	return list
}

// runPeriodicJudge 后台定期判定协程
func (bj *BatchWindowJudge) runPeriodicJudge() {
	ticker := time.NewTicker(bj.judgeInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-bj.ctx.Done():
			return
		case <-ticker.C:
			bj.mu.Lock()
			if len(bj.currentWindow) > 0 {
				bj.evaluateNow("periodic judge")
			}
			bj.mu.Unlock()
		}
	}
}

// ruleBasedJudge 基于规则的判定（LLM 的 fallback）
func (bj *BatchWindowJudge) ruleBasedJudge(summary *WindowSummary) Decision {
	// 规则 1：窗口内高风险事件过多，直接阻断
	if summary.HighRiskCount >= 3 {
		return Block
	}
	
	// 规则 2：事件频率异常（可能 DoS）
	if summary.EventCount >= bj.windowSize {
		return HumanApproval
	}
	
	// 规则 3：根据活跃监控器决策
	if len(summary.ActiveMonitors) > 0 {
		return HumanApproval
	}
	
	// 默认允许
	return Allow
}

// EnableMonitor 启用指定监控器
func (bj *BatchWindowJudge) EnableMonitor(monitorName string) {
	bj.mu.Lock()
	defer bj.mu.Unlock()
	bj.activeMonitors[monitorName] = true
}

// DisableMonitor 禁用指定监控器
func (bj *BatchWindowJudge) DisableMonitor(monitorName string) {
	bj.mu.Lock()
	defer bj.mu.Unlock()
	delete(bj.activeMonitors, monitorName)
}

// GetActiveMonitors 获取当前激活的监控器列表
func (bj *BatchWindowJudge) GetActiveMonitors() []string {
	bj.mu.RLock()
	defer bj.mu.RUnlock()
	return bj.getActiveMonitorsList()
}

// GetWindowStatus 获取窗口状态
func (bj *BatchWindowJudge) GetWindowStatus() map[string]interface{} {
	bj.mu.RLock()
	defer bj.mu.RUnlock()
	
	return map[string]interface{}{
		"window_size":      len(bj.currentWindow),
		"max_events":       bj.maxEvents,
		"active_monitors":  bj.getActiveMonitorsList(),
		"last_judge_time":  bj.lastJudgeTime,
		"judge_interval":   bj.judgeInterval,
	}
}

// Close 关闭判定器
func (bj *BatchWindowJudge) Close() {
	bj.cancel()
}
