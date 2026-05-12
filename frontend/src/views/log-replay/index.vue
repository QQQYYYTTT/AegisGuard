<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import LogStream from "@/components/audit/LogStream.vue";
import EvidenceCard from "@/components/audit/EvidenceCard.vue";
import AttackFlowGraph from "@/components/audit/AttackFlowGraph.vue";
import StatCard from "@/components/common/StatCard.vue";
import DecisionBadge from "@/components/common/DecisionBadge.vue";
import RiskBadge from "@/components/common/RiskBadge.vue";
import { useAuditStream } from "@/hooks/useAuditStream";
import type { AuditEvent } from "@/api/audit";

defineOptions({ name: "LogReplayIndex" });

const { events, attackChains, stats, loadLogs, loadChains, loadStats, loading, startPolling, stopPolling } =
  useAuditStream();

const selectedSession = ref<string | null>(null);
const selectedEvent = ref<AuditEvent | null>(null);
const activeTab = ref("timeline");
const logFilter = ref("all");
const showRawLog = ref(false);

const sessionList = computed(() => {
  const sessionMap = new Map<string, {
    sessionId: string;
    events: AuditEvent[];
    startTime: string;
    endTime: string;
    riskLevel: string;
    hasBlock: boolean;
    hasAuth: boolean;
    hasGate: boolean;
    hasSandbox: boolean;
    toolCalls: string[];
  }>();

  events.value.forEach(event => {
    const sessionId = event.session_id || "unknown";
    if (!sessionMap.has(sessionId)) {
      sessionMap.set(sessionId, {
        sessionId,
        events: [],
        startTime: event.timestamp,
        endTime: event.timestamp,
        riskLevel: "low",
        hasBlock: false,
        hasAuth: false,
        hasGate: false,
        hasSandbox: false,
        toolCalls: []
      });
    }
    const session = sessionMap.get(sessionId)!;
    session.events.push(event);
    if (event.timestamp < session.startTime) session.startTime = event.timestamp;
    if (event.timestamp > session.endTime) session.endTime = event.timestamp;
    if (event.decision === "Deny" || event.decision === "Block") session.hasBlock = true;
    if (event.event_type === "authorization") session.hasAuth = true;
    if (event.event_type === "gate") session.hasGate = true;
    if (event.event_type === "sandbox") session.hasSandbox = true;
    if (event.tool_name && !session.toolCalls.includes(event.tool_name)) {
      session.toolCalls.push(event.tool_name);
    }
    const maxRisk = Math.max(...session.events.map(e => e.risk_score || 0));
    session.riskLevel = maxRisk >= 80 ? "critical" : maxRisk >= 60 ? "high" : maxRisk >= 40 ? "medium" : "low";
  });

  return Array.from(sessionMap.values()).sort((a, b) => 
    new Date(b.startTime).getTime() - new Date(a.startTime).getTime()
  );
});

const filteredEvents = computed(() => {
  if (!selectedSession.value) return [];
  const session = sessionList.value.find(s => s.sessionId === selectedSession.value);
  if (!session) return [];

  let filtered = session.events.sort((a, b) => 
    new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
  );

  if (logFilter.value !== "all") {
    filtered = filtered.filter(e => e.event_type === logFilter.value);
  }

  return filtered;
});

const attackSteps = computed(() => {
  if (!selectedSession.value) return [];
  const session = sessionList.value.find(s => s.sessionId === selectedSession.value);
  if (!session) return [];

  return session.events
    .sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime())
    .map((event, index) => ({
      stepNumber: index + 1,
      timestamp: event.timestamp,
      eventType: event.event_type,
      description: event.description,
      decision: event.decision,
      riskScore: event.risk_score || 0,
      riskLevel: (event as any).risk_level || "low",
      toolName: event.tool_name,
      gateType: (event as any).gate_type,
      authMode: (event as any).auth_mode,
      tokenStatus: (event as any).token_status,
      reason: (event as any).reason,
      isAnomaly: event.risk_score >= 60 || event.decision === "Deny" || event.decision === "Block",
      hasAuth: event.event_type === "authorization",
      hasGate: event.event_type === "gate",
      hasSandbox: event.event_type === "sandbox",
      rawEvent: event
    }));
});

const toolCallRecords = computed(() => {
  if (!selectedSession.value) return [];
  const session = sessionList.value.find(s => s.sessionId === selectedSession.value);
  if (!session) return [];

  return session.events
    .filter(e => e.tool_name)
    .sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime())
    .map(event => ({
      timestamp: event.timestamp,
      toolName: event.tool_name,
      agentId: event.agent_id,
      decision: event.decision,
      riskScore: event.risk_score || 0,
      description: event.description,
      duration: (event as any).duration_ms
    }));
});

const sessionStats = computed(() => {
  if (!selectedSession.value) return null;
  const session = sessionList.value.find(s => s.sessionId === selectedSession.value);
  if (!session) return null;

  const totalEvents = session.events.length;
  const blockedEvents = session.events.filter(e => e.decision === "Deny" || e.decision === "Block").length;
  const allowedEvents = session.events.filter(e => e.decision === "Allow").length;
  const authEvents = session.events.filter(e => e.event_type === "authorization").length;
  const gateEvents = session.events.filter(e => e.event_type === "gate").length;
  const sandboxEvents = session.events.filter(e => e.event_type === "sandbox").length;
  const toolCalls = session.toolCalls.length;
  const maxRisk = Math.max(...session.events.map(e => e.risk_score || 0));
  const startTime = new Date(session.startTime).getTime();
  const endTime = new Date(session.endTime).getTime();
  const duration = (!isNaN(startTime) && !isNaN(endTime)) ? endTime - startTime : 0;

  return {
    totalEvents,
    blockedEvents,
    allowedEvents,
    authEvents,
    gateEvents,
    sandboxEvents,
    toolCalls,
    maxRisk,
    duration,
    riskLevel: session.riskLevel
  };
});

function selectSession(sessionId: string) {
  selectedSession.value = sessionId;
  selectedEvent.value = null;
  activeTab.value = "timeline";
}

function selectEvent(event: AuditEvent) {
  selectedEvent.value = event;
  showRawLog.value = false;
}

function toggleRawLog() {
  showRawLog.value = !showRawLog.value;
}

function getRiskLevelColor(level: string) {
  const colors: Record<string, string> = {
    critical: "#ff4d4f",
    high: "#f56c6c",
    medium: "#e6a23c",
    low: "#67c23a"
  };
  return colors[level] || "#909399";
}

onMounted(() => {
  Promise.all([loadLogs(), loadChains(), loadStats()]);
  startPolling(5000);
});

onUnmounted(() => {
  stopPolling();
});
</script>

<template>
  <div class="log-replay p-4">
    <h1 class="text-2xl font-bold mb-4">攻击日志回放 / Attack Log Replay</h1>

    <el-row :gutter="16" class="mb-4">
      <el-col :span="6">
        <StatCard title="总会话数 / Total Sessions" :value="sessionList.length" color="var(--aegis-primary)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="拦截会话 / Blocked Sessions" :value="sessionList.filter(s => s.hasBlock).length" color="var(--aegis-danger)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="总事件数 / Total Events" :value="stats?.total_events || 0" color="var(--aegis-info)" />
      </el-col>
      <el-col :span="6">
        <StatCard title="攻击链数 / Attack Chains" :value="stats?.attack_chains || 0" color="var(--aegis-warning)" />
      </el-col>
    </el-row>

    <el-row :gutter="16">
      <el-col :span="6">
        <el-card shadow="hover" class="mb-4">
          <template #header>
            <span class="font-semibold">会话列表 / Sessions</span>
          </template>
          <div v-loading="loading" class="space-y-2 max-h-[calc(100vh-320px)] overflow-y-auto">
            <div
              v-for="session in sessionList"
              :key="session.sessionId"
              class="p-3 border rounded cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
              :class="{ 'border-blue-500 bg-blue-50 dark:bg-blue-900': selectedSession === session.sessionId }"
              @click="selectSession(session.sessionId)"
            >
              <div class="flex items-center justify-between mb-2">
                <span class="font-medium text-sm truncate flex-1 mr-2">{{ session.sessionId }}</span>
                <el-tag
                  size="small"
                  :type="session.riskLevel === 'critical' ? 'danger' : session.riskLevel === 'high' ? 'danger' : session.riskLevel === 'medium' ? 'warning' : 'info'"
                  effect="dark"
                >
                  {{ session.riskLevel.toUpperCase() }}
                </el-tag>
              </div>
              <div class="text-xs text-gray-500 mb-1">
                {{ session.events.length }} 事件 | {{ new Date(session.startTime).toLocaleTimeString() }}
              </div>
              <div class="flex flex-wrap gap-1">
                <el-tag v-if="session.hasBlock" size="small" type="danger" effect="plain">拦截</el-tag>
                <el-tag v-if="session.hasAuth" size="small" type="success" effect="plain">授权</el-tag>
                <el-tag v-if="session.hasGate" size="small" type="warning" effect="plain">闸门</el-tag>
                <el-tag v-if="session.hasSandbox" size="small" type="info" effect="plain">沙箱</el-tag>
              </div>
            </div>
            <el-empty v-if="!sessionList.length && !loading" description="暂无会话数据" :image-size="80" />
          </div>
        </el-card>
      </el-col>

      <el-col :span="18">
        <el-card shadow="hover" class="mb-4" v-if="selectedSession && sessionStats">
          <template #header>
            <div class="flex items-center justify-between">
              <span class="font-semibold">会话详情 / Session Details</span>
              <el-tag size="small" type="info">{{ selectedSession }}</el-tag>
            </div>
          </template>

          <el-row :gutter="16" class="mb-4">
            <el-col :span="4">
              <StatCard title="事件总数" :value="sessionStats.totalEvents" size="small" />
            </el-col>
            <el-col :span="4">
              <StatCard title="拦截数" :value="sessionStats.blockedEvents" size="small" color="var(--aegis-danger)" />
            </el-col>
            <el-col :span="4">
              <StatCard title="授权事件" :value="sessionStats.authEvents" size="small" color="var(--aegis-success)" />
            </el-col>
            <el-col :span="4">
              <StatCard title="闸门事件" :value="sessionStats.gateEvents" size="small" color="var(--aegis-warning)" />
            </el-col>
            <el-col :span="4">
              <StatCard title="沙箱事件" :value="sessionStats.sandboxEvents" size="small" color="var(--aegis-info)" />
            </el-col>
            <el-col :span="4">
              <StatCard title="工具调用" :value="sessionStats.toolCalls" size="small" />
            </el-col>
          </el-row>

          <el-tabs v-model="activeTab">
            <el-tab-pane label="时间轴 / Timeline" name="timeline" />
            <el-tab-pane label="攻击步骤 / Attack Steps" name="steps" />
            <el-tab-pane label="工具调用 / Tool Calls" name="tools" />
            <el-tab-pane label="原始日志 / Raw Logs" name="raw" />
          </el-tabs>

          <div v-show="activeTab === 'timeline'">
            <div class="mb-3 flex items-center gap-2">
              <span class="text-sm">筛选：</span>
              <el-radio-group v-model="logFilter" size="small">
                <el-radio-button label="all">全部</el-radio-button>
                <el-radio-button label="input">输入</el-radio-button>
                <el-radio-button label="detection">检测</el-radio-button>
                <el-radio-button label="authorization">授权</el-radio-button>
                <el-radio-button label="gate">闸门</el-radio-button>
                <el-radio-button label="sandbox">沙箱</el-radio-button>
                <el-radio-button label="block">拦截</el-radio-button>
              </el-radio-group>
            </div>

            <el-timeline>
              <el-timeline-item
                v-for="event in filteredEvents"
                :key="event.id"
                :timestamp="new Date(event.timestamp).toLocaleString()"
                placement="top"
                :type="event.decision === 'Deny' || event.decision === 'Block' ? 'danger' : event.decision === 'Allow' ? 'success' : 'warning'"
                :color="event.risk_score >= 60 ? '#ff4d4f' : undefined"
              >
                <div
                  class="p-3 border rounded hover:bg-gray-50 dark:hover:bg-gray-800 cursor-pointer transition-colors"
                  :class="{ 'border-red-500 bg-red-50 dark:bg-red-900': event.risk_score >= 60 || event.decision === 'Deny' || event.decision === 'Block' }"
                  @click="selectEvent(event)"
                >
                  <div class="flex items-center justify-between mb-2">
                    <div class="flex items-center gap-2">
                      <el-tag size="small" type="info">{{ event.event_type }}</el-tag>
                      <DecisionBadge :decision="event.decision" />
                      <RiskBadge :level="((event as any).risk_level || 'low') as any" />
                      <el-tag v-if="event.tool_name" size="small" effect="plain">{{ event.tool_name }}</el-tag>
                    </div>
                    <span class="text-xs text-gray-400">风险分: {{ event.risk_score || 0 }}</span>
                  </div>
                  <div class="text-sm">{{ event.description }}</div>
                  <div v-if="(event as any).gate_type || (event as any).auth_mode" class="mt-2 flex gap-2">
                    <el-tag v-if="(event as any).gate_type" size="small" type="warning">闸门: {{ (event as any).gate_type }}</el-tag>
                    <el-tag v-if="(event as any).auth_mode" size="small" type="success">授权: {{ (event as any).auth_mode }}</el-tag>
                    <el-tag v-if="(event as any).token_status" size="small" type="info">Token: {{ (event as any).token_status }}</el-tag>
                  </div>
                </div>
              </el-timeline-item>
            </el-timeline>
          </div>

          <div v-if="activeTab === 'steps'">
            <AttackFlowGraph :steps="attackSteps" />
          </div>

          <div v-show="activeTab === 'tools'">
            <el-table :data="toolCallRecords" stripe size="small">
              <el-table-column prop="timestamp" label="时间 / Time" width="180">
                <template #default="{ row }">
                  {{ new Date(row.timestamp).toLocaleString() }}
                </template>
              </el-table-column>
              <el-table-column prop="toolName" label="工具名称 / Tool" width="150">
                <template #default="{ row }">
                  <el-tag size="small">{{ row.toolName }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="agentId" label="Agent" width="120" />
              <el-table-column prop="decision" label="决策 / Decision" width="100">
                <template #default="{ row }">
                  <DecisionBadge :decision="row.decision" />
                </template>
              </el-table-column>
              <el-table-column prop="riskScore" label="风险分 / Risk" width="90" />
              <el-table-column prop="duration" label="耗时 / Duration" width="100">
                <template #default="{ row }">
                  {{ row.duration ? `${row.duration}ms` : '-' }}
                </template>
              </el-table-column>
              <el-table-column prop="description" label="描述 / Description" min-width="200" show-overflow-tooltip />
            </el-table>
          </div>

          <div v-show="activeTab === 'raw'">
            <el-table :data="filteredEvents" stripe size="small" max-height="600">
              <el-table-column prop="id" label="事件 ID" width="100" />
              <el-table-column prop="timestamp" label="时间" width="180">
                <template #default="{ row }">
                  {{ new Date(row.timestamp).toLocaleString() }}
                </template>
              </el-table-column>
              <el-table-column prop="event_type" label="类型" width="100" />
              <el-table-column prop="method" label="方法" width="80" />
              <el-table-column prop="path" label="路径" width="150" />
              <el-table-column prop="decision" label="决策" width="80" />
              <el-table-column prop="risk_score" label="风险分" width="80" />
              <el-table-column prop="agent_id" label="Agent" width="120" />
              <el-table-column prop="session_id" label="会话 ID" width="120" />
              <el-table-column prop="tool_name" label="工具" width="120" />
              <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
            </el-table>
          </div>
        </el-card>

        <el-card shadow="hover" v-if="selectedEvent">
          <template #header>
            <div class="flex items-center justify-between">
              <span class="font-semibold">事件详情 / Event Detail</span>
              <el-button size="small" @click="toggleRawLog">
                {{ showRawLog ? '隐藏原始数据' : '查看原始数据' }}
              </el-button>
            </div>
          </template>
          <EvidenceCard :evidence="selectedEvent as any" />
          <el-collapse v-if="showRawLog" class="mt-4">
            <el-collapse-item title="原始 JSON 数据">
              <pre class="text-xs bg-gray-100 dark:bg-gray-800 p-3 rounded overflow-auto max-h-96">{{ JSON.stringify(selectedEvent, null, 2) }}</pre>
            </el-collapse-item>
          </el-collapse>
        </el-card>

        <el-empty v-if="!selectedSession" description="请选择一个会话查看日志回放" :image-size="120" />
      </el-col>
    </el-row>
  </div>
</template>
