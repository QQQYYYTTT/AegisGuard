// backend/internal/gates/tdg.go
package gates

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// TDG 是单个 Trace 内工具调用的有向邻接表（Tool Dependency Graph）。
//
// Phase 2（借鉴 IPIGuard 的 TDG 思想）：网关在运行时以纯 Go 状态机记录每个 Trace 的
// 工具调用拓扑，用于识别偏离正常调用模式的行为。与论文原版不同，本系统不具备
// LLM 生成的先验执行计划，因此不做"计划内/计划外"的白名单比对，
// 而是用两类可观测的拓扑异常信号替代（详见 ValidateCall 注释与 PLAN.md Phase 2 实现说明）。
type TDG struct {
	mu           sync.Mutex
	nodes        map[string]int                 // 工具名 -> 累计调用次数；key 集合即已出现的不同工具
	edges        map[string]map[string]struct{} // u -> v 表示调用序列中 v 紧随 u 之后出现过
	order        []string                        // 调用顺序（按时间），供后续审计/导出使用
	lastTool     string
	lastRepeat   int
	sawLowerRisk bool // 本 Trace 内是否已出现过风险层级低于 highRiskTier 的调用
	maxNodes     int
	maxRepeat    int
	lastActivity time.Time
}

func newTDG(maxNodes, maxRepeat int) *TDG {
	return &TDG{
		nodes:        make(map[string]int),
		edges:        make(map[string]map[string]struct{}),
		maxNodes:     maxNodes,
		maxRepeat:    maxRepeat,
		lastActivity: time.Now(),
	}
}

// ValidateCall 校验当前工具调用是否符合拓扑约束，返回 (是否放行, 违规原因)。
//
// 校验规则：
//  1. 不同工具种类上限：单个 Trace 内累计出现的不同工具种类超过 maxNodes 视为异常
//     （对应原计划"调用次数上限检查"，防止注入驱动的无限攻击面扩张）。
//  2. 连续重复调用上限：同一工具被连续调用达到 maxRepeat 次后再次调用视为异常
//     （对应 DRIFT/IPIGuard 论文中"注入驱动的异常调用循环"这一最常见的计划外表现）。
//  3. 高危工具前置校验：风险层级达到 highRiskTier 的工具（复用 Phase 1 的工具风险分层，
//     见 policy.go 的 resolveToolRiskTier）若是本 Trace 内第一次出现的、且此前没有任何
//     更低风险层级的调用作为铺垫，视为异常。这是对 IPIGuard"调用需遵循任务依赖顺序"这一
//     核心思想在不引入 LLM 规划前提下的工程近似：真实的 TDG 需要先由 LLM 生成执行计划，
//     而网关层不参与规划、也不做推理，因此改为约束"破坏性操作前必须有过铺垫性调用"这一更粗粒度、
//     但同样能拦截"注入指令诱导 Agent 跳过侦察直接执行破坏性操作"这类典型攻击的顺序信号。
func (tdg *TDG) ValidateCall(toolName string) (bool, string) {
	tdg.mu.Lock()
	defer tdg.mu.Unlock()

	if _, seen := tdg.nodes[toolName]; !seen && len(tdg.nodes) >= tdg.maxNodes {
		return false, fmt.Sprintf("distinct tool count exceeds max_nodes_per_trace (%d)", tdg.maxNodes)
	}
	if tdg.lastTool == toolName && tdg.lastRepeat+1 >= tdg.maxRepeat {
		return false, fmt.Sprintf("tool %q repeated consecutively beyond limit (%d)", toolName, tdg.maxRepeat)
	}
	if resolveToolRiskTier(toolName) >= highRiskTier && !tdg.sawLowerRisk {
		return false, fmt.Sprintf("high-impact tool %q invoked without any preceding lower-risk call in this trace", toolName)
	}
	return true, ""
}

// RecordCall 记录一次工具调用，更新节点、边、调用顺序与风险层级状态。
// 无论 ValidateCall 是否放行都应调用，以便拓扑数据持续积累，供 log-only 阶段分析真实调用模式。
func (tdg *TDG) RecordCall(toolName string) {
	tdg.mu.Lock()
	defer tdg.mu.Unlock()

	if tdg.lastTool != "" {
		if tdg.edges[tdg.lastTool] == nil {
			tdg.edges[tdg.lastTool] = make(map[string]struct{})
		}
		tdg.edges[tdg.lastTool][toolName] = struct{}{}
	}
	if tdg.lastTool == toolName {
		tdg.lastRepeat++
	} else {
		tdg.lastTool = toolName
		tdg.lastRepeat = 0
	}
	if resolveToolRiskTier(toolName) < highRiskTier {
		tdg.sawLowerRisk = true
	}
	tdg.nodes[toolName]++
	tdg.order = append(tdg.order, toolName)
	tdg.lastActivity = time.Now()
}

func (tdg *TDG) touchedAt() time.Time {
	tdg.mu.Lock()
	defer tdg.mu.Unlock()
	return tdg.lastActivity
}

// TDGRegistry 是全局 TraceID -> TDG 映射，按 TTL 周期性清理过期拓扑，防止内存泄漏。
type TDGRegistry struct {
	traces    sync.Map // map[string]*TDG
	ttl       time.Duration
	maxNodes  int
	maxRepeat int
	stopCh    chan struct{}
	stopOnce  sync.Once
}

// NewTDGRegistry 创建一个 TDG 注册表，并启动后台清理协程。
func NewTDGRegistry(maxNodes, maxRepeat int, ttl time.Duration) *TDGRegistry {
	if maxNodes <= 0 {
		maxNodes = 50
	}
	if maxRepeat <= 0 {
		maxRepeat = 5
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	reg := &TDGRegistry{
		ttl:       ttl,
		maxNodes:  maxNodes,
		maxRepeat: maxRepeat,
		stopCh:    make(chan struct{}),
	}
	go reg.cleanupLoop()
	return reg
}

// GetOrCreate 原子获取或创建某个 TraceID 对应的 TDG。
// 使用 sync.Map.LoadOrStore 保证高并发场景下同一 TraceID 不会被实例化为两个不同的拓扑图，
// 避免调用链记录断裂（工程审计要求）。
func (reg *TDGRegistry) GetOrCreate(traceID string) *TDG {
	if val, ok := reg.traces.Load(traceID); ok {
		return val.(*TDG)
	}
	candidate := newTDG(reg.maxNodes, reg.maxRepeat)
	actual, _ := reg.traces.LoadOrStore(traceID, candidate)
	return actual.(*TDG)
}

func (reg *TDGRegistry) cleanupLoop() {
	interval := reg.ttl / 2
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			reg.sweep()
		case <-reg.stopCh:
			return
		}
	}
}

func (reg *TDGRegistry) sweep() {
	now := time.Now()
	reg.traces.Range(func(key, value any) bool {
		tdg := value.(*TDG)
		if now.Sub(tdg.touchedAt()) > reg.ttl {
			reg.traces.Delete(key)
		}
		return true
	})
}

// Close 停止后台清理协程，用于测试与优雅关闭。
func (reg *TDGRegistry) Close() {
	reg.stopOnce.Do(func() {
		close(reg.stopCh)
	})
}

// TDGSettings 是 Phase 2 TDG 拓扑校验的开关配置。
type TDGSettings struct {
	Enabled   bool
	Mode      string // "log-only" | "enforce"
	MaxNodes  int
	MaxRepeat int
	TTL       time.Duration
}

func normalizeTDGMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "enforce":
		return "enforce"
	default:
		return "log-only"
	}
}
