<script setup lang="ts">
import DecisionBadge from "@/components/common/DecisionBadge.vue";

defineOptions({ name: "StructuredSummary" });

defineProps<{
  transfers: Array<{
    id: string;
    from: string;
    to: string;
    fields: string[];
    summary: string;
    sm3_hash: string;
    approved: boolean;
    timestamp: string;
  }>;
}>();
</script>

<template>
  <el-card shadow="hover">
    <template #header>
      <span class="font-semibold">Transfer Flow Summary</span>
    </template>
    <div class="space-y-3">
      <div
        v-for="t in transfers"
        :key="t.id"
        class="p-3 rounded border"
        :class="
          t.approved
            ? 'border-green-200 bg-green-50 dark:border-green-800 dark:bg-green-900/20'
            : 'border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-900/20'
        "
      >
        <div class="flex items-center justify-between mb-2">
          <div class="flex items-center gap-2">
            <el-tag size="small" type="info">{{ t.from }}</el-tag>
            <span class="text-gray-400">→</span>
            <el-tag size="small" type="info">{{ t.to }}</el-tag>
            <DecisionBadge :decision="t.approved ? 'Allow' : 'Block'" />
          </div>
          <span class="text-xs text-gray-400">{{ t.timestamp }}</span>
        </div>
        <div class="text-sm mb-1">{{ t.summary }}</div>
        <div class="flex items-center gap-4 text-xs text-gray-500">
          <span>Fields: {{ t.fields.join(", ") }}</span>
          <span>Hash: {{ t.sm3_hash }}</span>
        </div>
      </div>
    </div>
  </el-card>
</template>
