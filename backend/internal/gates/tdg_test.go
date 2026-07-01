// backend/internal/gates/tdg_test.go
package gates

import (
	"sync"
	"testing"
	"time"
)

// 测试样例：TDG - 不同工具种类数上限
// 用低危工具名（read_* 前缀），避免与高危前置校验规则相互干扰。
func TestTDGMaxDistinctNodesEnforced(t *testing.T) {
	tdg := newTDG(2, 100)

	if allowed, reason := tdg.ValidateCall("read_a"); !allowed {
		t.Fatalf("first distinct tool should be allowed: %s", reason)
	}
	tdg.RecordCall("read_a")

	if allowed, reason := tdg.ValidateCall("read_b"); !allowed {
		t.Fatalf("second distinct tool should be allowed: %s", reason)
	}
	tdg.RecordCall("read_b")

	if allowed, _ := tdg.ValidateCall("read_c"); allowed {
		t.Fatalf("third distinct tool should exceed max_nodes cap")
	}

	// 已出现过的工具不受种类数上限影响
	if allowed, reason := tdg.ValidateCall("read_a"); !allowed {
		t.Fatalf("previously seen tool should still be allowed after cap reached: %s", reason)
	}
}

// 测试样例：TDG - 连续重复调用上限
// 用低危工具名（read_* 前缀），避免与高危前置校验规则相互干扰。
func TestTDGConsecutiveRepeatEnforced(t *testing.T) {
	tdg := newTDG(100, 3)

	for i := 0; i < 3; i++ {
		allowed, reason := tdg.ValidateCall("read_a")
		if !allowed {
			t.Fatalf("call %d should be allowed: %s", i+1, reason)
		}
		tdg.RecordCall("read_a")
	}

	if allowed, _ := tdg.ValidateCall("read_a"); allowed {
		t.Fatalf("4th consecutive call should exceed max_repeat cap")
	}

	// 切换到另一个工具后再切回，重复计数应被重置
	tdg.RecordCall("read_a")
	if allowed, reason := tdg.ValidateCall("read_b"); !allowed {
		t.Fatalf("switching tool should be allowed: %s", reason)
	}
	tdg.RecordCall("read_b")
	if allowed, reason := tdg.ValidateCall("read_a"); !allowed {
		t.Fatalf("returning to tool a after interruption should reset repeat count: %s", reason)
	}
}

// 测试样例：TDG - 高危工具无前置低危调用时被拒绝
func TestTDGHighRiskToolRequiresPriorLowerRiskCall(t *testing.T) {
	tdg := newTDG(100, 100)

	if allowed, reason := tdg.ValidateCall("transfer_report"); allowed {
		t.Fatalf("high-risk tool as the very first call should be denied, reason=%q", reason)
	}

	tdg.RecordCall("search_records") // 低危调用铺垫
	if allowed, reason := tdg.ValidateCall("transfer_report"); !allowed {
		t.Fatalf("high-risk tool after a preceding lower-risk call should be allowed: %s", reason)
	}
}

// 测试样例：TDG - 中危调用也能作为高危工具的合法铺垫
func TestTDGHighRiskToolAllowedAfterMidRiskCall(t *testing.T) {
	tdg := newTDG(100, 100)
	tdg.RecordCall("create_invoice") // 中危调用（write 类别）
	if allowed, reason := tdg.ValidateCall("transfer_report"); !allowed {
		t.Fatalf("high-risk tool after a mid-risk call should be allowed: %s", reason)
	}
}

// 测试样例：TDG - 未识别的工具名按最高风险层级保守处理
func TestTDGUnrecognizedToolDefaultsToHighRiskTier(t *testing.T) {
	tdg := newTDG(100, 100)
	if allowed, reason := tdg.ValidateCall("weird_custom_tool_xyz"); allowed {
		t.Fatalf("unrecognized tool should default to high-risk tier and require precedent, reason=%q", reason)
	}
}

// 测试样例：TDGRegistry - GetOrCreate 原子性（防止并发场景下同一 TraceID 产生两个拓扑图）
func TestTDGRegistryGetOrCreateAtomic(t *testing.T) {
	reg := NewTDGRegistry(50, 5, time.Minute)
	defer reg.Close()

	const goroutines = 50
	results := make([]*TDG, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx] = reg.GetOrCreate("trace-shared")
		}(i)
	}
	wg.Wait()

	first := results[0]
	for i, tdg := range results {
		if tdg != first {
			t.Fatalf("goroutine %d got a different TDG instance for the same trace ID", i)
		}
	}
}

// 测试样例：TDGRegistry - 过期 Trace 自动清理
func TestTDGRegistrySweepExpiresOldTraces(t *testing.T) {
	reg := &TDGRegistry{ttl: time.Millisecond, maxNodes: 50, maxRepeat: 5, stopCh: make(chan struct{})}
	tdg := reg.GetOrCreate("trace-expiring")
	tdg.RecordCall("a")

	time.Sleep(5 * time.Millisecond)
	reg.sweep()

	if _, ok := reg.traces.Load("trace-expiring"); ok {
		t.Fatalf("expected expired trace to be swept")
	}
}

// 测试样例：TDGRegistry - 未过期 Trace 不受影响
func TestTDGRegistrySweepKeepsFreshTraces(t *testing.T) {
	reg := &TDGRegistry{ttl: time.Hour, maxNodes: 50, maxRepeat: 5, stopCh: make(chan struct{})}
	reg.GetOrCreate("trace-fresh")

	reg.sweep()

	if _, ok := reg.traces.Load("trace-fresh"); !ok {
		t.Fatalf("fresh trace should not be swept")
	}
}
