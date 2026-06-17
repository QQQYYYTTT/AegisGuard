<script setup lang="ts">
import { ref, onMounted, watch, onUnmounted, nextTick } from "vue";
import * as echarts from "echarts";
import type { AttackChain, AuditEvent } from "@/api/audit";

defineOptions({ name: "AttackChainGraph" });

const props = defineProps<{
  chain: AttackChain;
}>();

const chartRef = ref<HTMLElement>();
let chart: echarts.ECharts | null = null;
const resizeHandler = () => chart?.resize();

const eventTypeColors: Record<string, string> = {
  input: "#3b82f6",        // 蓝色 - 输入
  detection: "#f59e0b",    // 橙色 - 检测
  authorization: "#22c55e", // 绿色 - 授权
  gate: "#eab308",         // 黄色 - 闸门
  sandbox: "#a855f7",      // 紫色 - 沙箱
  block: "#ef4444",        // 红色 - 阻断
  allow: "#22c55e"         // 绿色 - 允许
};

const eventTypeLabels: Record<string, string> = {
  input: "输入检测",
  detection: "风险检测",
  authorization: "授权校验",
  gate: "安全闸门",
  sandbox: "沙箱隔离",
  block: "阻断",
  allow: "放行"
};

function buildGraphData(events: AuditEvent[]) {
  // 按时间排序
  const sortedEvents = [...events].sort(
    (a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
  );

  const nodes = sortedEvents.map((event, index) => {
    // 限制节点大小在合理范围内
    const riskScore = Math.max(0, Math.min(100, event.risk_score || 0));
    const symbolSize = Math.max(30, Math.min(60, 30 + riskScore / 5));
    
    return {
      id: event.id,
      name: `${eventTypeLabels[event.event_type] || event.event_type}\n${riskScore}分`,
      value: riskScore,
      symbol: "circle",
      symbolSize,
      itemStyle: {
        color: eventTypeColors[event.event_type] || "#999",
        borderColor: event.decision === "Block" || event.decision === "Deny" ? "#ff4d7d" : "#06172e",
        borderWidth: event.decision === "Block" || event.decision === "Deny" ? 3 : 1
      },
      label: {
        show: true,
        fontSize: 10,
        color: "#e8f8ff",
        fontWeight: 700,
        formatter: eventTypeLabels[event.event_type] || event.event_type
      },
      // 保存原始数据用于 tooltip
      rawData: event
    };
  });

  const links = [];
  for (let i = 0; i < sortedEvents.length - 1; i++) {
    const current = sortedEvents[i];
    const next = sortedEvents[i + 1];
    const isBlocked = current.decision === "Block" || current.decision === "Deny";
    
    links.push({
      source: current.id,
      target: next.id,
      lineStyle: {
        color: isBlocked ? "#ef4444" : "#94a3b8",
        width: isBlocked ? 3 : 1,
        type: isBlocked ? "solid" : "solid"
      },
      label: {
        show: isBlocked,
        formatter: "阻断",
        fontSize: 10,
        color: "#ef4444"
      }
    });
  }

  return { nodes, links };
}

function initChart() {
  if (!chartRef.value || !props.chain) return;
  
  if (!chart) {
    chart = echarts.init(chartRef.value, undefined, { renderer: 'canvas' });
  }
  
  renderChart();
}

function renderChart() {
  if (!chart || !props.chain) return;

  const { nodes, links } = buildGraphData(props.chain.events);
  const nodeCount = nodes.length;

  const option: echarts.EChartsOption = {
    tooltip: {
      trigger: "item",
      formatter: (params: any) => {
        if (params.dataType === "node") {
          const event = params.data.rawData as AuditEvent;
          return `
            <div style="padding: 8px;">
              <div style="font-weight: bold; margin-bottom: 4px;">${eventTypeLabels[event.event_type] || event.event_type}</div>
              <div>风险分数: <span style="color: ${(event.risk_score || 0) >= 60 ? '#ef4444' : '#22c55e'}">${event.risk_score || 0}</span></div>
              <div>决策: <span style="color: ${event.decision === 'Allow' ? '#22c55e' : '#ef4444'}">${event.decision}</span></div>
              <div>时间: ${new Date(event.timestamp).toLocaleTimeString()}</div>
              <div style="margin-top: 4px; color: #b7dfff; max-width: 200px; white-space: normal;">${event.description || ''}</div>
            </div>
          `;
        }
        return "";
      }
    },
    animationDuration: 1500,
    animationEasingUpdate: "quinticInOut",
    grid: {
      left: 50,
      right: 50,
      top: 50,
      bottom: 50
    },
    series: [
      {
        type: "graph",
        layout: "force",
        data: nodes,
        links: links,
        roam: true,
        draggable: true,
        focusNodeAdjacency: true,
        // 限制节点在画布范围内
        center: ["50%", "50%"],
        // 根据节点数量调整力导向参数
        force: {
          repulsion: nodeCount <= 10 ? 300 : nodeCount <= 20 ? 400 : 500,
          edgeLength: [60, 120],
          gravity: 0.3,
          layoutAnimation: true
        },
        // 缩放和平移限制
        scaleLimit: {
          min: 0.5,
          max: 3
        },
        emphasis: {
          focus: "adjacency",
          lineStyle: {
            width: 4
          }
        },
        lineStyle: {
          curveness: 0.2,
          opacity: 0.7
        },
        edgeSymbol: ["none", "arrow"],
        edgeSymbolSize: [0, 10],
        // 根据节点数量调整初始缩放
        zoom: nodeCount <= 10 ? 1 : nodeCount <= 20 ? 0.8 : 0.6
      }
    ]
  };

  chart.setOption(option, true);
}

// 监听数据变化
watch(() => props.chain, () => {
  nextTick(() => {
    initChart();
  });
}, { deep: true });

// 监听容器大小变化
let resizeObserver: ResizeObserver | null = null;

onMounted(() => {
  nextTick(() => {
    initChart();
    
    // 使用 ResizeObserver 监听容器大小变化，只调用 resize 不重新渲染
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
  <div class="attack-chain-graph">
    <div class="graph-header">
      <div class="legend">
        <span class="legend-item">
          <span class="dot" style="background: #3b82f6;"></span>输入
        </span>
        <span class="legend-item">
          <span class="dot" style="background: #f59e0b;"></span>检测
        </span>
        <span class="legend-item">
          <span class="dot" style="background: #22c55e;"></span>授权
        </span>
        <span class="legend-item">
          <span class="dot" style="background: #eab308;"></span>闸门
        </span>
        <span class="legend-item">
          <span class="dot" style="background: #a855f7;"></span>沙箱
        </span>
        <span class="legend-item">
          <span class="dot" style="background: #ef4444;"></span>阻断
        </span>
      </div>
    </div>
    <div ref="chartRef" class="chart-container"></div>
    <div class="graph-footer">
      <el-alert type="info" :closable="false" show-icon>
        <template #title>
          <span class="text-xs">节点大小表示风险分数，红色边框表示阻断事件，可拖拽节点调整布局</span>
        </template>
      </el-alert>
    </div>
  </div>
</template>

<style scoped>
.attack-chain-graph {
  width: 100%;
}

.graph-header {
  margin-bottom: 12px;
}

.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 12px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.chart-container {
  width: 100% !important;
  min-width: 100%;
  height: 450px;
  min-height: 400px;
  border: 1px solid rgba(78, 192, 255, 0.18);
  border-radius: 8px;
  overflow: hidden;
  background:
    linear-gradient(rgb(0 212 255 / 4%) 1px, transparent 1px),
    linear-gradient(90deg, rgb(0 212 255 / 4%) 1px, transparent 1px),
    rgba(5, 18, 38, 0.82);
  background-size: 24px 24px;
}

.graph-footer {
  margin-top: 12px;
}
</style>
