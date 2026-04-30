<script setup lang="ts">
import DecisionBadge from "@/components/common/DecisionBadge.vue";

defineOptions({ name: "EventNode" });

defineProps<{
  event: {
    id: string;
    event_type: string;
    description: string;
    decision: string;
    risk_score: number;
  };
}>();

const typeColors: Record<string, string> = {
  input: "bg-blue-100 text-blue-700",
  detection: "bg-red-100 text-red-700",
  authorization: "bg-green-100 text-green-700",
  gate: "bg-yellow-100 text-yellow-700",
  sandbox: "bg-purple-100 text-purple-700",
  block: "bg-red-100 text-red-700",
  allow: "bg-green-100 text-green-700"
};
</script>

<template>
  <div class="p-2 rounded border">
    <div class="flex items-center gap-2 mb-1">
      <span
        class="px-2 py-0.5 rounded text-xs font-medium"
        :class="typeColors[event.event_type] || 'bg-gray-100 text-gray-700'"
      >
        {{ event.event_type }}
      </span>
      <DecisionBadge :decision="event.decision" />
      <span class="text-xs text-gray-400">Score: {{ event.risk_score }}</span>
    </div>
    <div class="text-sm">{{ event.description }}</div>
  </div>
</template>
