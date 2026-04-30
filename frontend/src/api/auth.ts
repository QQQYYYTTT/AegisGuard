import { http } from "@/utils/http";

export type TokenInfo = {
  token_id: string;
  tool_name: string;
  scope: string;
  agent_id: string;
  session_id: string;
  task_id: string;
  expires_at: string;
  nonce: string;
  risk_level: "low" | "medium" | "high" | "critical";
  schema_hash: string;
  max_calls: number;
  call_count: number;
  signature: string;
  signed: boolean;
  verified: boolean;
  verification_checks: {
    signature_valid: boolean;
    expiry_valid: boolean;
    nonce_valid: boolean;
    call_budget_ok: boolean;
    schema_hash_match: boolean;
    scope_match: boolean;
    risk_level_ok: boolean;
  };
};

export type AuthStatus = {
  sm2_active: boolean;
  sm3_active: boolean;
  sm4_active: boolean;
  key_expires_at: string;
  active_tokens: number;
  revoked_tokens: number;
};

type ApiResult<T> = { success: boolean; data: T };

export const getTokenInfo = (params?: object) => {
  return http.request<ApiResult<TokenInfo>>("get", "/aegis/auth/token", {
    params
  });
};

export const issueToken = (data?: object) => {
  return http.request<ApiResult<TokenInfo>>("post", "/aegis/auth/token", {
    data
  });
};

export const verifyToken = (data?: object) => {
  return http.request<
    ApiResult<{ valid: boolean; checks: Record<string, boolean> }>
  >("post", "/aegis/auth/verify", { data });
};

export const getAuthStatus = () => {
  return http.request<ApiResult<AuthStatus>>("get", "/aegis/auth/status");
};
