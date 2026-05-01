<script setup lang="ts">
import { ref, onMounted } from "vue";
import LogStream from "@/components/audit/LogStream.vue";
import EvidenceCard from "@/components/audit/EvidenceCard.vue";
import { useAuditStream } from "@/hooks/useAuditStream";

defineOptions({ name: "LogReplayIndex" });

const { events, loadLogs, loading } = useAuditStream();
const selectedEvent = ref<any>(null);
const replayProgress = ref(0);
const isReplaying = ref(false);

onMounted(() => {
  loadLogs();
});

function selectEvent(event: any) {
  selectedEvent.value = event;
}

function startReplay() {
  isReplaying.value = true;
  replayProgress.value = 0;
  const total = events.value.length;
  let i = 0;
  const timer = setInterval(() => {
    if (i < total) {
      replayProgress.value = Math.round(((i + 1) / total) * 100);
      selectedEvent.value = events.value[i];
      i++;
    } else {
      clearInterval(timer);
      isReplaying.value = false;
    }
  }, 800);
}
</script>

<template>
  <div class="log-replay p-4">
    <h1 class="text-2xl font-bold mb-4">Log Replay</h1>

    <el-row :gutter="16">
      <el-col :span="16">
        <el-card shadow="hover">
          <template #header>
            <div class="flex items-center justify-between">
              <span class="font-semibold">Event Stream</span>
              <el-button
                type="primary"
                size="small"
                :loading="isReplaying"
                @click="startReplay"
              >
                {{ isReplaying ? "Replaying..." : "Start Replay" }}
              </el-button>
            </div>
          </template>
          <el-progress
            v-if="isReplaying"
            :percentage="replayProgress"
            :stroke-width="4"
            class="mb-4"
          />
          <LogStream :events="events.map(e => ({
            ...e,
            risk_level: e.risk_score >= 0.8 ? 'critical' : e.risk_score >= 0.6 ? 'high' : e.risk_score >= 0.4 ? 'medium' : 'low'
          }))"           />
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card v-if="selectedEvent" shadow="hover">
          <template #header>
            <span class="font-semibold">Event Detail</span>
          </template>
          <EvidenceCard
            :evidence="{
              request_id: selectedEvent.request_id,
              body_hash: selectedEvent.body_hash,
              method: selectedEvent.method,
              path: selectedEvent.path,
              status: selectedEvent.status,
              duration_ms: selectedEvent.duration_ms,
              agent_id: selectedEvent.agent_id,
              session_id: selectedEvent.session_id
            }"
          />
        </el-card>
        <el-card v-else shadow="hover">
          <el-empty description="Click an event to view details" :image-size="60" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>
