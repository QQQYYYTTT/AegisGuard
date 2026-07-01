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
  lastError: string | null;
};

export const usePolicyStore = defineStore("aegis-policy", {
  state: (): PolicyState => ({
    config: null,
    rules: [],
    loading: false,
    lastError: null
  }),
  getters: {
    sortedRules: (state) => [...state.rules].sort((a, b) => a.priority - b.priority),
    enabledRules: (state) => state.rules.filter((r) => r.enabled).sort((a, b) => a.priority - b.priority),
  },
  actions: {
    async fetchConfig() {
      this.loading = true;
      this.lastError = null;
      try {
        const res = await getPolicyConfig();
        if (res.code !== 0 || !res.data) {
          throw new Error(res.message || "获取策略配置失败");
        }
        this.config = res.data;
        this.rules = res.data.rules || [];
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "获取策略配置失败";
        this.lastError = message;
        this.config = null;
        this.rules = [];
      } finally {
        this.loading = false;
      }
    },
    async fetchRules() {
      this.loading = true;
      this.lastError = null;
      try {
        const res = await getPolicyRules();
        if (res.code !== 0) {
          throw new Error(res.message || "获取策略规则失败");
        }
        this.rules = res.data || [];
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "获取策略规则失败";
        this.lastError = message;
        this.rules = [];
      } finally {
        this.loading = false;
      }
    },
    async updateRule(rule: PolicyRule) {
      this.loading = true;
      this.lastError = null;
      try {
        const res = await updatePolicyRule(rule);
        if (res.code !== 0) {
          throw new Error(res.message || "更新规则失败");
        }
        const idx = this.rules.findIndex((r) => r.id === rule.id);
        if (idx >= 0) {
          this.rules[idx] = { ...rule };
        }
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "更新规则失败";
        this.lastError = message;
        throw error;
      } finally {
        this.loading = false;
      }
    },
    async createRule(rule: CreateRulePayload) {
      this.loading = true;
      this.lastError = null;
      try {
        const res = await createPolicyRule(rule);
        if (res.code !== 0) {
          throw new Error(res.message || "创建规则失败");
        }
        if (res.data?.id) {
          this.rules.push(res.data);
        }
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "创建规则失败";
        this.lastError = message;
        throw error;
      } finally {
        this.loading = false;
      }
    },
    async removeRule(id: string) {
      this.loading = true;
      this.lastError = null;
      try {
        const res = await deletePolicyRule(id);
        if (res.code !== 0) {
          throw new Error(res.message || "删除规则失败");
        }
        this.rules = this.rules.filter((r) => r.id !== id);
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "删除规则失败";
        this.lastError = message;
        throw error;
      } finally {
        this.loading = false;
      }
    },
    async reorder(ruleIds: string[]) {
      this.loading = true;
      this.lastError = null;
      try {
        const res = await reorderPolicyRules(ruleIds);
        if (res.code !== 0) {
          throw new Error(res.message || "规则排序失败");
        }
        this.rules = this.rules
          .map((r) => {
            const idx = ruleIds.indexOf(r.id);
            return idx >= 0 ? { ...r, priority: idx + 1 } : r;
          })
          .sort((a, b) => a.priority - b.priority);
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "规则排序失败";
        this.lastError = message;
        throw error;
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
      this.lastError = null;
      try {
        const res = await updatePolicyConfig(config);
        if (res.code !== 0) {
          throw new Error(res.message || "保存配置失败");
        }
        if (this.config) {
          Object.assign(this.config, config);
        }
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "保存配置失败";
        this.lastError = message;
        throw error;
      } finally {
        this.loading = false;
      }
    },
  },
});

export function usePolicyStoreHook() {
  return usePolicyStore(store);
}
