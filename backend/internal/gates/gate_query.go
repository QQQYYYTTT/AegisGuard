package gates

import (
	"aegisguard/internal/contract"
	"aegisguard/internal/interfaces"
)

// GateQueryImpl 实现GateQuery接口
type GateQueryImpl struct {
	store *DecisionStore
}

func NewGateQuery(store *DecisionStore) contract.GateQuery {
	return &GateQueryImpl{store: store}
}

func (gq *GateQueryImpl) Overview() (*contract.GateOverview, error) {
	overview := gq.store.GetOverview()
	return &contract.GateOverview{
		MessageGate:     overview.MessageGate,
		ActionGate:      overview.ActionGate,
		ReturnGate:      overview.ReturnGate,
		RecentDecisions: overview.RecentDecisions,
	}, nil
}

func (gq *GateQueryImpl) Decisions(limit int, gateType, action string) ([]interfaces.GateDecision, error) {
	decisions := gq.store.GetRecent(limit * 2) // 获取更多，然后过滤

	filtered := make([]interfaces.GateDecision, 0, limit)
	for _, d := range decisions {
		if gateType != "" && d.GateType != gateType {
			continue
		}
		if action != "" && d.Decision.String() != action {
			continue
		}
		filtered = append(filtered, d)
		if len(filtered) >= limit {
			break
		}
	}

	return filtered, nil
}
