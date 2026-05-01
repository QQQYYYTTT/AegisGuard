<script setup lang="ts">
import DecisionBadge from "@/components/common/DecisionBadge.vue";
import RiskBadge from "@/components/common/RiskBadge.vue";

defineOptions({ name: "LogStream" });

defineProps<{
  events: Array<{
    id: string;
    timestamp: string;
    event_type: string;
    description: string;
    decision: string;
    risk_score: number;
    risk_level: string;
    agent_id: string;
    tool_name: string;
  }>;
}>();
</script>

<template>
  <div class="log-stream space-y-2 max-h-96 overflow-y-auto">
    <div
      v-for="event in events"
      :key="event.id"
      class="flex items-start gap-3 p-2 rounded border text-sm hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
    >
      <span class="text-xs text-gray-400 whitespace-nowrap mt-0.5">
        {{ new Date(event.timestamp).toLocaleTimeString() }}
      </span>
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2 mb-1">
          <DecisionBadge :decision="event.decision" />
          <RiskBadge :level="(event.risk_level as any)" />
          <span v-if="event.tool_name" class="text-xs text-gray-500">
            tool:{{ event.tool_name }}
          </span>
        </div>
        <div class="truncate">{{ event.description }}</div>
      </div>
      <span class="text-xs text-gray-400 whitespace-nowrap">
        {{ event.agent_id }}
      </span>
    </div>
  </div>
</template>
