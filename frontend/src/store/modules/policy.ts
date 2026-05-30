import { defineStore } from "pinia";
import { store } from "../utils";
import {
  type PolicyRule,
  type PolicyConfig,
  type CreateRulePayload,
  getPolicyConfig,
  getPolicyRules,
  updatePolicyRule,
  createPolicyRule,
  deletePolicyRule,
  reorderPolicyRules,
  updatePolicyConfig,
} from "@/api/policy";

export type PolicyState = {
  config: PolicyConfig | null;
  rules: PolicyRule[];
  loading: boolean;
};

const defaultRules: PolicyRule[] = [
  { id: "rule-1", name: "Prompt Injection 检测", description: "检测提示注入攻击，包含忽略/绕过系统指令等模式", gate_type: "message", condition: "(?i)\\b(ignore|forget|bypass|override|system prompt|jailbreak)\\b", action: "Block", priority: 1, enabled: true, risk_threshold: 35 },
  { id: "rule-2", name: "越权操作检测", description: "检测企图获取管理员权限、执行敏感系统命令的行为", gate_type: "action", condition: "(?i)\\b(admin|root|sudo|privileged|shell|exec|spawn)\\b", action: "Block", priority: 2, enabled: true, risk_threshold: 30 },
  { id: "rule-3", name: "敏感数据泄露防护", description: "保护 API Key、密码、Token 等敏感凭证不被泄露", gate_type: "return", condition: "(?i)\\b(api[_-]?key|password|secret|private key|sk-[A-Za-z0-9]{8,})\\b", action: "Block", priority: 3, enabled: true, risk_threshold: 35 },
  { id: "rule-4", name: "高危操作拦截", description: "拦截生产环境删除、转账、支付等高危操作", gate_type: "action", condition: "(?i)\\b(delete|transfer|wire|withdraw|pay|refund)\\b.{0,80}\\b(production|customer|account|database|payment|fund)\\b", action: "Block", priority: 4, enabled: true, risk_threshold: 32 },
  { id: "rule-5", name: "记忆投毒防护", description: "检测并阻止试图污染 Agent 长期记忆的恶意指令", gate_type: "return", condition: "(?i)\\b(save|store|remember|persist)\\b.{0,80}\\b(instruction|rule|memory|policy)\\b", action: "Block", priority: 5, enabled: true, risk_threshold: 45 },
  { id: "rule-6", name: "非法金融活动检测", description: "检测洗钱、内幕交易、逃税等非法金融活动模式", gate_type: "message", condition: "(?i)\\b(money laundering|insider trading|tax evasion|fraudulent|stolen card)\\b", action: "Block", priority: 6, enabled: true, risk_threshold: 70 },
  { id: "rule-7", name: "正常请求放行", description: "允许所有不匹配任何高危模式的正常用户请求", gate_type: "message", condition: "", action: "Allow", priority: 7, enabled: true, risk_threshold: 0 },
  { id: "rule-8", name: "回调污染检测", description: "检测外部工具返回结果中注入恶意指令的回调污染攻击", gate_type: "return", condition: "(?i)\\b(observation injection|tool output|external content)\\b.{0,80}\\b(instruction|command|override)\\b", action: "Degrade", priority: 8, enabled: true, risk_threshold: 25 },
  { id: "rule-9", name: "重放攻击检测", description: "识别会话重放、重复提权操作等重放攻击行为", gate_type: "action", condition: "(?i)\\b(replay|repeat|again|retry)\\b.{0,80}\\b(privileged|action|export|admin)\\b", action: "Deny", priority: 9, enabled: true, risk_threshold: 40 },
  { id: "rule-10", name: "工具误用检测", description: "检测 Agent 工具调用超出合理范围的行为", gate_type: "action", condition: "(?i)\\b(delete_file|shell_exec|drop_table|rm\\s+-rf|format)\\b", action: "Block", priority: 10, enabled: true, risk_threshold: 50 },
];

const defaultConfig: PolicyConfig = {
  risk_weights: { alpha: 0.35, beta: 0.40, gamma: 0.25 },
  global_threshold: 85,
  rules: defaultRules,
};

export const usePolicyStore = defineStore("aegis-policy", {
  state: (): PolicyState => ({
    config: null,
    rules: [],
    loading: false,
  }),
  getters: {
    sortedRules: (state) => [...state.rules].sort((a, b) => a.priority - b.priority),
    enabledRules: (state) => state.rules.filter((r) => r.enabled).sort((a, b) => a.priority - b.priority),
  },
  actions: {
    async fetchConfig() {
      this.loading = true;
      try {
        const res = await getPolicyConfig();
        this.config = res.data;
        this.rules = res.data.rules || [];
      } catch {
        this.config = { ...defaultConfig };
        this.rules = [...defaultRules];
      } finally {
        this.loading = false;
      }
    },
    async fetchRules() {
      this.loading = true;
      try {
        const res = await getPolicyRules();
        this.rules = res.data || [];
      } catch {
        this.rules = [...defaultRules];
      } finally {
        this.loading = false;
      }
    },
    async updateRule(rule: PolicyRule) {
      this.loading = true;
      try {
        await updatePolicyRule(rule);
        const idx = this.rules.findIndex((r) => r.id === rule.id);
        if (idx >= 0) {
          this.rules[idx] = { ...rule };
        }
      } catch {
        const idx = this.rules.findIndex((r) => r.id === rule.id);
        if (idx >= 0) {
          this.rules[idx] = { ...rule };
        }
      } finally {
        this.loading = false;
      }
    },
    async createRule(rule: CreateRulePayload) {
      this.loading = true;
      try {
        const res = await createPolicyRule(rule);
        if (res.data?.id) {
          this.rules.push(res.data);
        }
      } catch {
        const maxPriority = Math.max(...this.rules.map((r) => r.priority), 0);
        this.rules.push({ ...rule, id: `rule-custom-${Date.now()}`, priority: maxPriority + 1 } as PolicyRule);
      } finally {
        this.loading = false;
      }
    },
    async removeRule(id: string) {
      this.loading = true;
      try {
        await deletePolicyRule(id);
        this.rules = this.rules.filter((r) => r.id !== id);
      } catch {
        this.rules = this.rules.filter((r) => r.id !== id);
      } finally {
        this.loading = false;
      }
    },
    async reorder(ruleIds: string[]) {
      this.loading = true;
      try {
        await reorderPolicyRules(ruleIds);
        this.rules = this.rules
          .map((r) => {
            const idx = ruleIds.indexOf(r.id);
            return idx >= 0 ? { ...r, priority: idx + 1 } : r;
          })
          .sort((a, b) => a.priority - b.priority);
      } catch {
        this.rules = this.rules
          .map((r) => {
            const idx = ruleIds.indexOf(r.id);
            return idx >= 0 ? { ...r, priority: idx + 1 } : r;
          })
          .sort((a, b) => a.priority - b.priority);
      } finally {
        this.loading = false;
      }
    },
    async toggleRule(id: string) {
      const rule = this.rules.find((r) => r.id === id);
      if (!rule) return;
      await this.updateRule({ ...rule, enabled: !rule.enabled });
    },
    async saveConfig(config: Partial<PolicyConfig>) {
      this.loading = true;
      try {
        await updatePolicyConfig(config);
        if (this.config) {
          Object.assign(this.config, config);
        }
      } catch {
        if (this.config) {
          Object.assign(this.config, config);
        }
      } finally {
        this.loading = false;
      }
    },
  },
});

export function usePolicyStoreHook() {
  return usePolicyStore(store);
}