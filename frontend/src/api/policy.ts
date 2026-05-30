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

export type CreateRulePayload = {
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

type ApiResult<T> = { code: number; data: T; message?: string };

export const getPolicyConfig = () => {
  return http.request<ApiResult<PolicyConfig>>("get", "/aegis/policy/config");
};

export const getPolicyRules = (params?: object) => {
  return http.request<ApiResult<PolicyRule[]>>("get", "/aegis/policy/rules", {
    params
  });
};

export const createPolicyRule = (data: CreateRulePayload) => {
  return http.request<ApiResult<PolicyRule>>("post", "/aegis/policy/rules", {
    data
  });
};

export const updatePolicyRule = (data: PolicyRule) => {
  return http.request<ApiResult<PolicyRule>>("put", "/aegis/policy/rules", {
    data
  });
};

export const deletePolicyRule = (id: string) => {
  return http.request<ApiResult<null>>("delete", `/aegis/policy/rules/${id}`);
};

export const reorderPolicyRules = (ruleIds: string[]) => {
  return http.request<ApiResult<null>>("put", "/aegis/policy/rules/reorder", {
    data: { rule_ids: ruleIds }
  });
};

export const updatePolicyConfig = (data: Partial<PolicyConfig>) => {
  return http.request<ApiResult<null>>("put", "/aegis/policy/config", {
    data
  });
};
