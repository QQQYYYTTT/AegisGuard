package gates

import (
	"sync"

	"aegisguard/internal/contract"
	"aegisguard/internal/interfaces"
)

// DecisionStore 存储门控决策记录
type DecisionStore struct {
	mu       sync.RWMutex
	decisions []interfaces.GateDecision
	maxSize  int
}

func NewDecisionStore(maxSize int) *DecisionStore {
	return &DecisionStore{
		decisions: make([]interfaces.GateDecision, 0, maxSize),
		maxSize:   maxSize,
	}
}

func (ds *DecisionStore) Add(decision interfaces.GateDecision) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.decisions = append(ds.decisions, decision)
	if len(ds.decisions) > ds.maxSize {
		// 移除最旧的
		copy(ds.decisions, ds.decisions[1:])
		ds.decisions = ds.decisions[:len(ds.decisions)-1]
	}
}

func (ds *DecisionStore) GetRecent(limit int) []interfaces.GateDecision {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	if limit <= 0 || limit > len(ds.decisions) {
		limit = len(ds.decisions)
	}

	result := make([]interfaces.GateDecision, limit)
	copy(result, ds.decisions[len(ds.decisions)-limit:])
	return result
}

func (ds *DecisionStore) GetOverview() *contract.GateOverview {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	overview := &contract.GateOverview{
		MessageGate:    make(map[string]int),
		ActionGate:     make(map[string]int),
		ReturnGate:     make(map[string]int),
		RecentDecisions: ds.GetRecent(10),
	}

	for _, d := range ds.decisions {
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
		gateMap[d.Decision]++
	}

	return overview
}