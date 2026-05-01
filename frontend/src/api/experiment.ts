import { http } from "@/utils/http";

export type ExperimentSummary = {
  run_id: string;
  benchmark: string;
  attack: string;
  metrics: {
    total: number;
    asr: number;
    asr_count: number;
    asr_total: number;
    rr: number;
    rr_count: number;
    pna: number;
    bp: number;
    fnr: number;
    fpr: number;
    average_latency_ms: number;
  };
  file: string;
  modified: string;
};

export type ExperimentRecord = {
  run_id: string;
  benchmark_family: string;
  benchmark_suite: string;
  asb_attack: string;
  case_id: string;
  scenario: string;
  agent_name: string;
  defense: string;
  attack_success: boolean;
  refused: boolean;
  task_success: boolean;
  latency_ms: number;
  asr: number;
  rr: number;
  pna: number;
  bp: number;
  fnr: number;
  fpr: number;
};

export type ThreeGateResult = {
  case: string;
  title: string;
  expected_focus: string;
  use_llm: boolean;
  stopped: boolean;
  final_output: string;
  trace: Array<{
    hard: {
      stage: string;
      action: string;
      reason: string;
      triggered_rules: string[];
      risk_score: number;
      decision_basis: string;
      safe_text?: string;
      allowed_tools?: string[];
    };
    combined: {
      combined_action: string;
      combined_risk_score?: number;
      decision_basis: string;
      llm_judgement?: {
        recommended_action: string;
        risk_score: number;
        reason: string;
      };
    };
  }>;
  generated_at: string;
};

export type AttackFamilyStats = {
  attack: string;
  runs: number;
  total_cases: number;
  avg_asr: number;
  avg_rr: number;
};

type ApiResult<T> = { success: boolean; data: T; total?: number };

export const getExperimentSummaries = () => {
  return http.request<ApiResult<ExperimentSummary[]>>("get", "/api/experiments/summaries");
};

export const getExperimentSummary = (runId: string) => {
  return http.request<ApiResult<Record<string, any>>>("get", `/api/experiments/summary/${runId}`);
};

export const getExperimentRecords = (runId: string) => {
  return http.request<ApiResult<ExperimentRecord[]>>("get", `/api/experiments/records/${runId}`);
};

export const getThreeGateResult = () => {
  return http.request<ApiResult<ThreeGateResult>>("get", "/api/experiments/three-gate");
};

export const getAttackFamilyStats = () => {
  return http.request<ApiResult<AttackFamilyStats[]>>("get", "/api/experiments/attack-families");
};
