import { http } from "@/utils/http";

export type PolicyRule = {
  id: string;
  name: string;
  description: string;
  gate_type: "message" | "action" | "return";
  condition: string;
  action: "Allow" | "Block" | "Degrade" | "Deny" | "HumanApproval";
  priority: number;
  enabled: boolean;
  risk_threshold: number;
};

export type PolicyConfig = {
  risk_weights: {
    alpha: number;
    beta: number;
    gamma: number;
  };
  global_threshold: number;
  rules: PolicyRule[];
};

type ApiResult<T> = { success: boolean; data: T };

export const getPolicyConfig = () => {
  return http.request<ApiResult<PolicyConfig>>("get", "/aegis/policy/config");
};

export const updatePolicyRule = (data?: object) => {
  return http.request<ApiResult<PolicyRule>>("put", "/aegis/policy/rules", {
    data
  });
};

export const getPolicyRules = (params?: object) => {
  return http.request<ApiResult<PolicyRule[]>>("get", "/aegis/policy/rules", {
    params
  });
};
