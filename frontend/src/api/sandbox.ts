import { http } from "@/utils/http";

export type SandboxContext = {
  context_id: string;
  trusted: {
    system_prompt: string;
    tool_definitions: string[];
    memory: string;
  };
  untrusted: {
    user_input: string;
    external_data: string;
    injected_content: string;
  };
  sm3_fingerprint: string;
  isolated_at: string;
};

export type TransferRecord = {
  id: string;
  from: "trusted" | "untrusted";
  to: "trusted" | "untrusted";
  fields: string[];
  summary: string;
  sm3_hash: string;
  approved: boolean;
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
