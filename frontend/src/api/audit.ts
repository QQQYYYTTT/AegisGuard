import { http } from "@/utils/http";

export type AuditEvent = {
  id: string;
  request_id: string;
  timestamp: string;
  method: string;
  path: string;
  status: number;
  duration_ms: number;
  decision: string;
  risk_score: number;
  agent_id: string;
  session_id: string;
  tool_name: string;
  body_hash: string;
  event_type:
    | "input"
    | "detection"
    | "authorization"
    | "gate"
    | "sandbox"
    | "block"
    | "allow";
  description: string;
};

export type AttackChain = {
  chain_id: string;
  events: AuditEvent[];
  start_time: string;
  end_time: string;
  severity: "low" | "medium" | "high" | "critical";
  summary: string;
};

export type AuditStats = {
  total_events: number;
  today_events: number;
  attack_chains: number;
  avg_duration_ms: number;
  top_agents: { agent_id: string; count: number }[];
  decision_distribution: Record<string, number>;
};

type ApiResult<T> = { success: boolean; data: T };

export const getAuditLogs = (params?: object) => {
  return http.request<ApiResult<AuditEvent[]>>("get", "/audit/logs", {
    params
  });
};

export const getAttackChains = (params?: object) => {
  return http.request<ApiResult<AttackChain[]>>(
    "get",
    "/aegis/audit/chains",
    { params }
  );
};

export const getAuditStats = () => {
  return http.request<ApiResult<AuditStats>>("get", "/aegis/audit/stats");
};
