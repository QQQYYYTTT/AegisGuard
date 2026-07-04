import { http } from "@/utils/http";

export type SandboxContext = {
  context_id: string;
  agent_id?: string;
  session_id?: string;
  source?: string;
  trusted: {
    system_prompt: string;
    tool_definitions: string[];
    memory: string;
    task_state?: string;
  };
  untrusted: {
    user_input: string;
    external_data: string;
    injected_content: string;
    source?: string;
    content_type?: string;
  };
  sm3_fingerprint: string;
  risk_score?: number;
  risk_level?: string;
  status?: string;
  isolated_at: string;
  updated_at?: string;
  expires_at?: string;
};

export type TransferRecord = {
  id: string;
  context_id?: string;
  from: string;
  to: string;
  fields: string[];
  summary: string;
  sm3_hash: string;
  risk_score?: number;
  risk_level?: string;
  action?: string;
  tool_name?: string;
  approved: boolean;
  reason?: string;
  memory_source?: string;
  promotion_reason?: string;
  timestamp: string;
};

type ApiResult<T> = { success: boolean; data: T };

export const getSandboxContext = () => {
  return http.request<ApiResult<SandboxContext>>(
    "get",
    "/aegis/sandbox/context"
  );
};

export const getTransferRecords = (params?: object) => {
  return http.request<ApiResult<TransferRecord[]>>(
    "get",
    "/aegis/sandbox/transfers",
    { params }
  );
};

export const isolateContext = (data?: object) => {
  return http.request<ApiResult<SandboxContext>>(
    "post",
    "/aegis/sandbox/isolate",
    { data }
  );
};
