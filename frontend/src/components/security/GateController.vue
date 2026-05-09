<script setup lang="ts">
import DecisionBadge from "@/components/common/DecisionBadge.vue";
import RiskBadge from "@/components/common/RiskBadge.vue";

defineOptions({ name: "GateController" });

defineProps<{
  decisions: Array<{
    request_id: string;
    gate_type: string;
    decision: string;
    risk_score: number;
    risk_level: string;
    reason: string;
    matched_rules: string[];
    timestamp: string;
    token_status?: string;
    auth_mode?: string;
    unauthorized_allow?: boolean;
  }>;
}>();

function scoreColor(score: number) {
  if (score >= 80) return "#ff4d4f";
  if (score >= 60) return "#f56c6c";
  if (score >= 40) return "#e6a23c";
  return "#67c23a";
}
</script>

<template>
  <div class="space-y-3">
    <div
      v-for="d in decisions"
      :key="d.request_id"
      class="p-3 border rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
    >
      <div class="flex items-center justify-between mb-2">
        <div class="flex items-center gap-2">
          <DecisionBadge :decision="d.decision" />
          <RiskBadge :level="d.risk_level as any" />
          <el-tag size="small" type="info">{{ d.gate_type }}</el-tag>
        </div>
        <span class="text-xs text-gray-400">{{ d.timestamp }}</span>
      </div>

      <div class="text-sm mb-2">{{ d.reason }}</div>

      <div class="flex items-center gap-4 flex-wrap">
        <div class="flex items-center gap-1">
          <span class="text-xs text-gray-500">风险分数 / Risk Score:</span>
          <el-progress
            :percentage="d.risk_score"
            :stroke-width="6"
            :color="scoreColor(d.risk_score)"
            style="width: 100px"
          />
        </div>

        <div v-if="d.matched_rules.length" class="flex flex-wrap gap-1">
          <el-tag
            v-for="r in d.matched_rules"
            :key="r"
            size="small"
            type="warning"
            effect="plain"
          >
            {{ r }}
          </el-tag>
        </div>
      </div>

      <div v-if="d.auth_mode || d.token_status || d.unauthorized_allow" class="mt-2 flex flex-wrap gap-2">
        <el-tag v-if="d.auth_mode" size="small" type="info">模式 / Mode: {{ d.auth_mode }}</el-tag>
        <el-tag v-if="d.token_status" size="small" type="warning">Token: {{ d.token_status }}</el-tag>
        <el-tag v-if="d.unauthorized_allow" size="small" type="danger">
          未授权放行 / Unauthorized Allow
        </el-tag>
      </div>
    </div>
  </div>
</template>
