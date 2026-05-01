<script setup lang="ts">
defineOptions({ name: "RiskBadge" });

defineProps<{
  level: "low" | "medium" | "high" | "critical";
}>();

const levelMap = {
  low: { label: "Low", color: "var(--risk-low, #67c23a)", type: "success" },
  medium: {
    label: "Medium",
    color: "var(--risk-medium, #e6a23c)",
    type: "warning"
  },
  high: { label: "High", color: "var(--risk-high, #f56c6c)", type: "danger" },
  critical: {
    label: "Critical",
    color: "var(--risk-critical, #ff4d4f)",
    type: "danger"
  }
};
</script>

<template>
  <el-tag
    :type="(levelMap[level]?.type as any) || 'info'"
    effect="dark"
    size="small"
    round
  >
    {{ levelMap[level]?.label || level }}
  </el-tag>
</template>
