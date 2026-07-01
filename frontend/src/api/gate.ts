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
  token_status?: string;
  auth_mode?: "strict" | "compat" | "warn";
  unauthorized_allow?: boolean;
};

type RawGateSummary = Record<string, unknown>;

type RawGateOverview = {
  message_gate: RawGateSummary;
  action_gate: RawGateSummary;
  return_gate: RawGateSummary;
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
      agent_id?: string;
      body?: Record<string, unknown>;
      content?: string;
    }
  | {
      type: "action";
      agent_id?: string;
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
  const readNumber = (value: unknown) => (typeof value === "number" && Number.isFinite(value) ? value : undefined);

  const buildSummary = (summary: RawGateSummary): GateSummary => {
    const status = typeof summary?.status === "string" ? summary.status : "online";
    const todayCount =
      readNumber(summary?.today_count) !== undefined
        ? (readNumber(summary?.today_count) as number)
        : Object.values(summary || {}).reduce<number>((sum, value) => {
            const numeric = readNumber(value);
            return numeric !== undefined ? sum + numeric : sum;
          }, 0);
    const blockCount =
      readNumber(summary?.block_count) !== undefined
        ? (readNumber(summary?.block_count) as number)
        : (readNumber(summary?.Block) || 0) + (readNumber(summary?.Deny) || 0);
    const decisionCounts =
      summary?.decision_counts && typeof summary.decision_counts === "object"
        ? (summary.decision_counts as Record<string, number>)
        : (summary as Record<string, number>);

    return {
      status,
      today_count: todayCount,
      block_count: blockCount,
      decision_counts: decisionCounts || {}
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
