<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import * as echarts from "echarts";
import { useAuditStream } from "@/hooks/useAuditStream";

defineOptions({ name: "TopologyIndex" });

const { attackChains, events, loadChains, loadLogs } = useAuditStream();
const chartRef = ref<HTMLElement>();

onMounted(async () => {
  await Promise.all([loadChains(), loadLogs()]);
  renderGraph();
});

function renderGraph() {
  if (!chartRef.value) return;
  const chart = echarts.init(chartRef.value);

  const nodeMap = new Map<string, number>();
  const nodes: any[] = [];
  const links: any[] = [];

  const eventTypes = [
    "input",
    "detection",
    "authorization",
    "gate",
    "sandbox",
    "block",
    "allow"
  ];
  const typeColors: Record<string, string> = {
    input: "#3b82f6",
    detection: "#ef4444",
    authorization: "#22c55e",
    gate: "#eab308",
    sandbox: "#a855f7",
    block: "#ef4444",
    allow: "#22c55e"
  };

  events.value.forEach(e => {
    if (!nodeMap.has(e.event_type)) {
      nodeMap.set(e.event_type, nodes.length);
      nodes.push({
        name: e.event_type,
        symbolSize: 30 + events.value.filter(x => x.event_type === e.event_type).length * 5,
        itemStyle: { color: typeColors[e.event_type] || "#999" }
      });
    }
  });

  for (let i = 0; i < events.value.length - 1; i++) {
    const src = events.value[i];
    const tgt = events.value[i + 1];
    if (src.event_type !== tgt.event_type) {
      links.push({
        source: src.event_type,
        target: tgt.event_type,
        lineStyle: {
          color: src.decision === "Deny" || src.decision === "Block" ? "#ef4444" : "#94a3b8",
          curveness: 0.2
        }
      });
    }
  }

  chart.setOption({
    tooltip: {},
    series: [
      {
        type: "graph",
        layout: "force",
        roam: true,
        label: { show: true },
        force: { repulsion: 200, edgeLength: 100 },
        data: nodes,
        links: [...new Map(links.map(l => [`${l.source}-${l.target}`, l])).values()],
        lineStyle: { opacity: 0.6 },
        emphasis: { focus: "adjacency", lineStyle: { width: 3 } }
      }
    ]
  });

  window.addEventListener("resize", () => chart.resize());
}

const chainSeverityColor = (s: string) => {
  if (s === "critical") return "danger";
  if (s === "high") return "danger";
  if (s === "medium") return "warning";
  return "info";
};
</script>

<template>
  <div class="topology p-4">
    <h1 class="text-2xl font-bold mb-4">Topology & Attack Paths</h1>

    <el-row :gutter="16">
      <el-col :span="16">
        <el-card shadow="hover">
          <template #header>
            <span class="font-semibold">Event Flow Graph</span>
          </template>
          <div ref="chartRef" style="height: 400px" />
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover">
          <template #header>
            <span class="font-semibold">Attack Chains</span>
          </template>
          <div class="space-y-3">
            <div
              v-for="chain in attackChains"
              :key="chain.chain_id"
              class="p-3 rounded border"
            >
              <div class="flex items-center justify-between mb-2">
                <el-tag :type="chainSeverityColor(chain.severity)" size="small" effect="dark">
                  {{ chain.severity.toUpperCase() }}
                </el-tag>
                <code class="text-xs text-gray-400">{{ chain.chain_id }}</code>
              </div>
              <div class="text-sm mb-2">{{ chain.summary }}</div>
              <div class="text-xs text-gray-500">
                {{ chain.events.length }} events |
                {{ new Date(chain.start_time).toLocaleTimeString() }} →
                {{ new Date(chain.end_time).toLocaleTimeString() }}
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>
