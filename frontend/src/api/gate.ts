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

export type GateOverview = {
  message_gate: { status: string; today_count: number; block_count: number };
  action_gate: { status: string; today_count: number; block_count: number };
  return_gate: { status: string; today_count: number; block_count: number };
  recent_decisions: GateDecision[];
};

type ApiResult<T> = { success: boolean; data: T };

export const getGateOverview = () => {
  return http.request<ApiResult<GateOverview>>("get", "/aegis/gate/overview");
};

export const getGateDecisions = (params?: object) => {
  return http.request<ApiResult<GateDecision[]>>("get", "/aegis/gate/decisions", {
    params
  });
};

export const evaluateGate = (data?: object) => {
  return http.request<ApiResult<GateDecision>>("post", "/aegis/gate/evaluate", {
    data
  });
};
