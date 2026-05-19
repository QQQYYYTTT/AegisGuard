package gates

import (
	"sync"

	"aegisguard/internal/contract"
	"aegisguard/internal/interfaces"
)

// DecisionStore 存储门控决策记录
// 使用环形缓冲区（ring buffer）实现 O(1) 插入
type DecisionStore struct {
	mu      sync.RWMutex
	buffer  []interfaces.GateDecision
	start   int // 环形缓冲区中最早元素的索引
	count   int // 当前有效元素数量
	maxSize int
}

func NewDecisionStore(maxSize int) *DecisionStore {
	return &DecisionStore{
		buffer:  make([]interfaces.GateDecision, maxSize),
		maxSize: maxSize,
	}
}

func (ds *DecisionStore) Add(decision interfaces.GateDecision) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.count < ds.maxSize {
		idx := (ds.start + ds.count) % ds.maxSize
		ds.buffer[idx] = decision
		ds.count++
	} else {
		ds.buffer[ds.start] = decision
		ds.start = (ds.start + 1) % ds.maxSize
	}
}

func (ds *DecisionStore) GetRecent(limit int) []interfaces.GateDecision {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	if ds.count == 0 {
		return nil
	}

	if limit <= 0 || limit > ds.count {
		limit = ds.count
	}

	result := make([]interfaces.GateDecision, limit)
	// 从最新（末尾）向最旧（开头）遍历
	for i := 0; i < limit; i++ {
		idx := (ds.start + ds.count - 1 - i) % ds.maxSize
		if idx < 0 {
			idx += ds.maxSize
		}
		result[i] = ds.buffer[idx]
	}
	return result
}

func (ds *DecisionStore) GetOverview() *contract.GateOverview {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	overview := &contract.GateOverview{
		MessageGate:     make(map[string]int),
		ActionGate:      make(map[string]int),
		ReturnGate:      make(map[string]int),
		RecentDecisions: make([]interfaces.GateDecision, 0),
	}

	if ds.count == 0 {
		return overview
	}

	// 遍历环形缓冲区中的所有有效元素
	for i := 0; i < ds.count; i++ {
		idx := (ds.start + i) % ds.maxSize
		d := ds.buffer[idx]

		var gateMap map[string]int
		switch d.GateType {
		case "message":
			gateMap = overview.MessageGate
		case "action":
			gateMap = overview.ActionGate
		case "return":
			gateMap = overview.ReturnGate
		default:
			continue
		}
		gateMap[d.Decision.String()]++
	}

	// 填充 RecentDecisions（最近的 10 条，从新到旧）
	recentLimit := ds.count
	if recentLimit > 10 {
		recentLimit = 10
	}
	overview.RecentDecisions = make([]interfaces.GateDecision, recentLimit)
	for i := 0; i < recentLimit; i++ {
		idx := (ds.start + ds.count - 1 - i) % ds.maxSize
		if idx < 0 {
			idx += ds.maxSize
		}
		overview.RecentDecisions[i] = ds.buffer[idx]
	}

	return overview
}
