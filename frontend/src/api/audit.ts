import { http } from "@/utils/http";

export type AuditEvent = {
  id: string;
  request_id: string;
  timestamp: string;
  method: string;
  path: string;
  status_code: number;
  status: number;
  duration_ms: number;
  decision: string;
  risk_score: number;
  risk_level?: string;
  gate_type?: string;
  reason?: string;
  token_status?: string;
  auth_mode?: "strict" | "compat" | "warn";
  unauthorized_allow?: boolean;
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

export type ThreatMapData = {
  target: { name: string; coord: [number, number] };
  stats: {
    total: number;
    critical: number;
    high: number;
    sources: number;
    provinces: number;
  };
  provinces: Array<{ name: string; value: number; critical: number }>;
  cities: Array<{
    name: string;
    coord: [number, number];
    value: number;
    level: "high" | "critical";
  }>;
  lines: Array<{
    from: [number, number];
    to: [number, number];
    count: number;
    level: "high" | "critical";
    latest: string;
  }>;
  generatedAt: string;
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

export const getThreatMap = (params?: { window?: string }) => {
  return http.request<ApiResult<ThreatMapData>>(
    "get",
    "/aegis/audit/threat-map",
    { params }
  );
};
