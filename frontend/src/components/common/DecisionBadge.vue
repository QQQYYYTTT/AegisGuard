<script setup lang="ts">
defineOptions({ name: "DecisionBadge" });

defineProps<{
  decision: string;
}>();

const decisionMap: Record<
  string,
  { label: string; type: string; color: string }
> = {
  Allow: {
    label: "Allow",
    type: "success",
    color: "var(--aegis-success, #67c23a)"
  },
  Deny: { label: "Deny", type: "danger", color: "var(--aegis-danger, #f56c6c)" },
  Block: {
    label: "Block",
    type: "danger",
    color: "var(--aegis-danger, #f56c6c)"
  },
  Degrade: {
    label: "Degrade",
    type: "warning",
    color: "var(--aegis-warning, #e6a23c)"
  },
  HumanApproval: {
    label: "Human Approval",
    type: "warning",
    color: "var(--aegis-warning, #e6a23c)"
  }
};
</script>

<template>
  <el-tag
    :type="(decisionMap[decision]?.type as any) || 'info'"
    effect="plain"
    size="small"
  >
    {{ decisionMap[decision]?.label || decision }}
  </el-tag>
</template>
