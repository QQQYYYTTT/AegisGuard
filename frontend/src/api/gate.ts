import { http } from "@/utils/http";

export type Decision = "Allow" | "Block" | "Degrade" | "Deny" | "HumanApproval";

export type GateDecision = {
  request_id: string;
  timestamp: string;
  gate_type: "message" | "action" | "return";
  decision: Decision;
  risk_score: number;
  risk_level: "low" | "medium" | "high" | "critical";
  matched_rules: string[];
  reason: string;
  tool_name?: string;
  agent_id?: string;
};

type RawGateOverview = {
  message_gate: Record<string, number>;
  action_gate: Record<string, number>;
  return_gate: Record<string, number>;
  recent_decisions: GateDecision[];
};

export type GateOverview = {
  message_gate: GateSummary;
  action_gate: GateSummary;
  return_gate: GateSummary;
  recent_decisions: GateDecision[];
};

export type GateSummary = {
  status: string;
  today_count: number;
  block_count: number;
  decision_counts: Record<string, number>;
};

export type GateEvaluateRequest =
  | {
      type: "message" | "return";
      body?: Record<string, unknown>;
      content?: string;
    }
  | {
      type: "action";
      tool_name: string;
      params?: Record<string, unknown>;
      headers?: Record<string, string>;
    };

type ApiResult<T> = { success: boolean; data: T };

export function normalizeRiskScore(score: number) {
  if (!Number.isFinite(score)) return 0;
  if (score > 0 && score <= 1) return Math.round(score * 100);
  return Math.max(0, Math.min(100, Math.round(score)));
}

export function normalizeGateDecision(decision: GateDecision): GateDecision {
  return {
    ...decision,
    risk_score: normalizeRiskScore(decision.risk_score)
  };
}

export function normalizeGateOverview(raw: RawGateOverview): GateOverview {
  const buildSummary = (counts: Record<string, number>): GateSummary => {
    const total = Object.values(counts || {}).reduce((sum, value) => sum + value, 0);
    return {
      status: "online",
      today_count: total,
      block_count: (counts?.Block || 0) + (counts?.Deny || 0),
      decision_counts: counts || {}
    };
  };

  return {
    message_gate: buildSummary(raw.message_gate),
    action_gate: buildSummary(raw.action_gate),
    return_gate: buildSummary(raw.return_gate),
    recent_decisions: (raw.recent_decisions || []).map(normalizeGateDecision)
  };
}

export const getGateOverview = () => {
  return http.request<ApiResult<RawGateOverview>>("get", "/aegis/gate/overview");
};

export const getGateDecisions = (params?: object) => {
  return http.request<ApiResult<GateDecision[]>>("get", "/aegis/gate/decisions", {
    params
  });
};

export const evaluateGate = (data?: GateEvaluateRequest) => {
  return http.request<ApiResult<GateDecision>>("post", "/aegis/gate/evaluate", {
    data
  });
};
