<script setup lang="ts">
import EventNode from "./EventNode.vue";

defineOptions({ name: "AttackTimeline" });

defineProps<{
  chain: {
    chain_id: string;
    events: Array<{
      id: string;
      timestamp: string;
      event_type: string;
      description: string;
      decision: string;
      risk_score: number;
    }>;
    severity: string;
    summary: string;
    start_time: string;
    end_time: string;
  };
}>();
</script>

<template>
  <el-card shadow="hover">
    <template #header>
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="font-semibold">攻击链 / Attack Chain</span>
          <el-tag
            :type="
              chain.severity === 'critical'
                ? 'danger'
                : chain.severity === 'high'
                  ? 'danger'
                  : chain.severity === 'medium'
                    ? 'warning'
                    : 'info'
            "
            size="small"
            effect="dark"
          >
            {{ chain.severity.toUpperCase() }}
          </el-tag>
        </div>
        <code class="text-xs text-gray-400">{{ chain.chain_id }}</code>
      </div>
    </template>
    <div class="mb-3 text-sm text-gray-600 dark:text-gray-300">
      {{ chain.summary }}
    </div>
    <el-timeline>
      <el-timeline-item
        v-for="event in chain.events"
        :key="event.id"
        :timestamp="event.timestamp"
        placement="top"
        :type="
          event.decision === 'Deny' || event.decision === 'Block'
            ? 'danger'
            : event.decision === 'Allow'
              ? 'success'
              : 'warning'
        "
      >
        <EventNode :event="event" />
      </el-timeline-item>
    </el-timeline>
  </el-card>
</template>
