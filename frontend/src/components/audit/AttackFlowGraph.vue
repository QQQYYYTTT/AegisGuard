<script setup lang="ts">
import { ref, onMounted, watch, onUnmounted, computed, nextTick } from "vue";
import * as echarts from "echarts";

interface AttackStep {
  stepNumber: number;
  timestamp: string;
  eventType: string;
  description: string;
  decision: string;
  riskScore: number;
  riskLevel: string;
  toolName?: string;
  gateType?: string;
  authMode?: string;
  tokenStatus?: string;
  reason?: string;
  isAnomaly: boolean;
  hasAuth: boolean;
  hasGate: boolean;
  hasSandbox: boolean;
}

defineOptions({ name: "AttackFlowGraph" });

const props = defineProps<{
  steps: AttackStep[];
}>();

const chartRef = ref<HTMLElement>();
let chart: echarts.ECharts | null = null;
const resizeHandler = () => chart?.resize();

const eventTypeColors: Record<string, string> = {
  input: "#3b82f6",
  detection: "#f59e0b",
  authorization: "#22c55e",
  gate: "#eab308",
  sandbox: "#a855f7",
  block: "#ef4444",
  allow: "#22c55e",
  tool: "#6366f1"
};

const eventTypeLabels: Record<string, string> = {
  input: "输入",
  detection: "检测",
  authorization: "授权",
  gate: "闸门",
  sandbox: "沙箱",
  block: "阻断",
  allow: "放行",
  tool: "工具"
};

const sortedSteps = computed(() => {
  return [...props.steps].sort(
    (a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
  );
});

function buildFlowData(steps: AttackStep[]) {
  const nodes = steps.map((step, index) => ({
    id: `step-${step.stepNumber}`,
    name: `步骤${step.stepNumber}`,
    value: step.riskScore,
    symbol: "circle",
    symbolSize: 45 + step.riskScore / 4,
    itemStyle: {
      color: step.isAnomaly ? "#ef4444" : (eventTypeColors[step.eventType] || "#999"),
      borderColor: step.decision === "Block" || step.decision === "Deny" ? "#7f1d1d" : "#fff",
      borderWidth: step.decision === "Block" || step.decision === "Deny" ? 4 : 2,
      shadowBlur: step.isAnomaly ? 15 : 0,
      shadowColor: step.isAnomaly ? "#ef4444" : "transparent"
    },
    label: {
      show: true,
      fontSize: 11,
      position: index % 2 === 0 ? "top" : "bottom",
      distance: 10,
      formatter: `{name|步骤 ${step.stepNumber}}\n{type|${eventTypeLabels[step.eventType] || step.eventType}}`,
      rich: {
        name: {
          fontSize: 12,
          fontWeight: "bold",
          color: "#333"
        },
        type: {
          fontSize: 10,
          color: "#666"
        }
      }
    },
    rawData: step
  }));

  const links = [];
  for (let i = 0; i < steps.length - 1; i++) {
    const current = steps[i];
    const next = steps[i + 1];
    const isBlocked = current.decision === "Block" || current.decision === "Deny";
    
    links.push({
      source: `step-${current.stepNumber}`,
      target: `step-${next.stepNumber}`,
      lineStyle: {
        color: isBlocked ? "#ef4444" : "#3b82f6",
        width: isBlocked ? 4 : 2,
        type: isBlocked ? "dashed" : "solid",
        curveness: 0.2
      },
      label: {
        show: isBlocked,
        formatter: "已阻断",
        fontSize: 10,
        color: "#ef4444",
        backgroundColor: "#fef2f2",
        padding: [2, 6],
        borderRadius: 4
      },
      symbol: ["none", "arrow"],
      symbolSize: 12
    });
  }

  return { nodes, links };
}

function initChart() {
  if (!chartRef.value) return;
  
  if (!chart) {
    chart = echarts.init(chartRef.value, undefined, { renderer: 'canvas' });
  }
  
  renderChart();
}

function renderChart() {
  if (!chart || !props.steps.length) return;

  const { nodes, links } = buildFlowData(sortedSteps.value);

  const option: echarts.EChartsOption = {
    tooltip: {
      trigger: "item",
      formatter: (params: any) => {
        if (params.dataType === "node") {
          const step = params.data.rawData as AttackStep;
          const features = [];
          if (step.hasAuth) features.push("✓ 授权校验");
          if (step.hasGate) features.push("⚡ 闸门触发");
          if (step.hasSandbox) features.push("🔒 沙箱隔离");
          if (step.toolName) features.push(`🔧 工具: ${step.toolName}`);

          return `
            <div style="padding: 8px; max-width: 280px;">
              <div style="font-weight: bold; margin-bottom: 8px; font-size: 14px;">
                步骤 ${step.stepNumber}: ${eventTypeLabels[step.eventType] || step.eventType}
              </div>
              <div style="margin-bottom: 4px;">
                风险分数: <span style="color: ${step.riskScore >= 60 ? '#ef4444' : '#22c55e'}; font-weight: bold;">${step.riskScore}</span>
                <span style="color: #999; margin-left: 8px;">(${step.riskLevel})</span>
              </div>
              <div style="margin-bottom: 4px;">
                决策: <span style="color: ${step.decision === 'Allow' ? '#22c55e' : '#ef4444'}; font-weight: bold;">${step.decision}</span>
              </div>
              <div style="margin-bottom: 8px; color: #666;">${step.description}</div>
              ${features.length ? `<div style="border-top: 1px solid #eee; padding-top: 8px; margin-top: 8px;">${features.map(f => `<div style="font-size: 12px; margin-bottom: 2px;">${f}</div>`).join('')}</div>` : ''}
              ${step.reason ? `<div style="border-top: 1px solid #eee; padding-top: 8px; margin-top: 8px; color: #ef4444; font-size: 12px;">原因: ${step.reason}</div>` : ''}
            </div>
          `;
        }
        return "";
      }
    },
    animationDuration: 800,
    animationEasing: "cubicOut",
    series: [
      {
        type: "graph",
        layout: "force",
        data: nodes,
        links: links,
        roam: true,
        draggable: true,
        focusNodeAdjacency: true,
        force: {
          repulsion: 400,
          edgeLength: [80, 150],
          gravity: 0.3,
          layoutAnimation: true
        },
        scaleLimit: {
          min: 0.5,
          max: 3
        },
        emphasis: {
          focus: "adjacency",
          lineStyle: {
            width: 5
          }
        },
        lineStyle: {
          opacity: 0.8,
          curveness: 0.1
        },
        edgeSymbol: ["none", "arrow"],
        edgeSymbolSize: [0, 10],
        zoom: 0.8
      }
    ]
  };

  chart.setOption(option, true);
}

watch(() => props.steps, () => {
  nextTick(() => {
    initChart();
  });
}, { deep: true });

let resizeObserver: ResizeObserver | null = null;

onMounted(() => {
  nextTick(() => {
    initChart();
    
    if (chartRef.value && window.ResizeObserver) {
      resizeObserver = new ResizeObserver(() => {
        chart?.resize();
      });
      resizeObserver.observe(chartRef.value);
    }
  });
  
  window.addEventListener("resize", resizeHandler);
});

onUnmounted(() => {
  if (resizeObserver && chartRef.value) {
    resizeObserver.unobserve(chartRef.value);
    resizeObserver = null;
  }
  chart?.dispose();
  chart = null;
  window.removeEventListener("resize", resizeHandler);
});
</script>

<template>
  <div class="attack-flow-graph">
    <div v-if="!steps.length" class="empty-state">
      <el-empty description="暂无攻击步骤数据" :image-size="80" />
    </div>
    <template v-else>
      <div class="flow-header">
        <div class="flow-stats">
          <el-tag size="small" type="info">共 {{ steps.length }} 个步骤</el-tag>
          <el-tag size="small" type="danger" v-if="steps.some(s => s.isAnomaly)">
            {{ steps.filter(s => s.isAnomaly).length }} 个异常
          </el-tag>
          <el-tag size="small" type="success" v-if="steps.some(s => s.decision === 'Allow')">
            {{ steps.filter(s => s.decision === 'Allow').length }} 个放行
          </el-tag>
        </div>
      </div>
      <div ref="chartRef" class="chart-container"></div>
      <div class="flow-footer">
        <div class="flow-legend">
          <span class="legend-item">
            <span class="line normal"></span>正常流程
          </span>
          <span class="legend-item">
            <span class="line blocked"></span>阻断流程
          </span>
          <span class="legend-item">
            <span class="node anomaly"></span>异常节点
          </span>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.attack-flow-graph {
  width: 100%;
}

.empty-state {
  padding: 40px 0;
}

.flow-header {
  margin-bottom: 12px;
}

.flow-stats {
  display: flex;
  gap: 8px;
}

.chart-container {
  width: 100% !important;
  min-width: 100%;
  height: 350px;
  min-height: 350px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: linear-gradient(to right, #f8fafc 0%, #ffffff 50%, #f8fafc 100%);
}

.flow-footer {
  margin-top: 12px;
}

.flow-legend {
  display: flex;
  gap: 20px;
  font-size: 12px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.line {
  width: 30px;
  height: 3px;
  border-radius: 2px;
}

.line.normal {
  background: #3b82f6;
}

.line.blocked {
  background: repeating-linear-gradient(
    to right,
    #ef4444 0px,
    #ef4444 5px,
    transparent 5px,
    transparent 10px
  );
  height: 3px;
}

.node {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.node.anomaly {
  background: #ef4444;
  box-shadow: 0 0 8px #ef4444;
}
</style>
