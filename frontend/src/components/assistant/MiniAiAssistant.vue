<script setup lang="ts">
import { nextTick, ref } from "vue";
import { chatWithAssistant, type AssistantMessage } from "@/api/assistant";
import { getAuditStats } from "@/api/audit";
import { getAttackFamilyStats, getExperimentSummaries } from "@/api/experiment";
import { getGateDecisions, getGateOverview } from "@/api/gate";
import Close from "~icons/ep/close";
import Service from "~icons/ep/service";

defineOptions({ name: "MiniAiAssistant" });

type ChatMessage = {
  role: "assistant" | "user";
  content: string;
};

type AnalysisType = "security" | "experiment" | "agents" | "advice";

type AnalysisAction = {
  type: AnalysisType;
  title: string;
  desc: string;
  prompt: string;
};

const open = ref(false);
const input = ref("");
const sending = ref(false);
const statusText = ref("");
const scrollRef = ref<HTMLElement>();
const messages = ref<ChatMessage[]>([
  {
    role: "assistant",
    content:
      "我是 Aegis AI 数据助手。可以直接汇总当前态势、生成实验报告、分析已接入 Agent，也可以手动输入问题进行研判。"
  }
]);

const analysisActions: AnalysisAction[] = [
  {
    type: "security",
    title: "态势汇总",
    desc: "汇总闸门、风险分布和最近决策",
    prompt: "请汇总当前安全态势"
  },
  {
    type: "experiment",
    title: "实验报告",
    desc: "整理 DPI/OPI/MIXED/MP/POT 测试结果",
    prompt: "请生成实验结果汇总报告"
  },
  {
    type: "agents",
    title: "Agent 接入",
    desc: "分析已接入 Agent 与调用健康度",
    prompt: "请分析已接入 Agent 状态"
  },
  {
    type: "advice",
    title: "整改建议",
    desc: "输出可执行的风险处置建议",
    prompt: "请给出当前系统整改建议"
  }
];

const quickPrompts = ["五类攻击说明", "怎么看闸门拦截", "如何定位审计链路"];

function toggleAssistant() {
  open.value = !open.value;
  if (open.value) nextTick(scrollToBottom);
}

function askQuick(prompt: string) {
  if (sending.value) return;
  input.value = prompt;
  sendMessage();
}

async function runAnalysisAction(action: AnalysisAction) {
  if (sending.value) return;
  statusText.value = "正在汇总网站数据...";
  const prompt = await buildAnalysisPrompt(action.type);
  await askAssistant(action.prompt, prompt);
}

async function sendMessage() {
  const text = input.value.trim();
  if (!text || sending.value) return;
  input.value = "";
  statusText.value = "正在研判...";
  await askAssistant(text, text);
}

async function askAssistant(displayText: string, modelPrompt: string) {
  const history = buildHistory();
  messages.value.push({ role: "user", content: displayText });
  sending.value = true;
  await nextTick(scrollToBottom);

  try {
    const res = await chatWithAssistant({ message: modelPrompt, messages: history });
    const reply = res?.data?.message || res?.message || getLocalReply(modelPrompt);
    messages.value.push({ role: "assistant", content: reply });
  } catch (error: any) {
    const reason = getAssistantErrorReason(error);
    messages.value.push({
      role: "assistant",
      content: `${getLocalReply(modelPrompt)}\n\n提示：在线模型调用失败，已使用本地规则和页面数据兜底。原因：${reason}`
    });
  } finally {
    sending.value = false;
    statusText.value = "";
    nextTick(scrollToBottom);
  }
}

function getAssistantErrorReason(error: any) {
  return (
    error?.response?.data?.message ||
    error?.message ||
    "unknown error"
  );
}

function buildHistory(): AssistantMessage[] {
  return messages.value
    .filter(item => item.role === "user" || item.role === "assistant")
    .slice(-8);
}

async function buildAnalysisPrompt(type: AnalysisType) {
  try {
    if (type === "security") {
      const [overview, auditStats, decisions] = await Promise.all([
        getGateOverview(),
        getAuditStats(),
        getGateDecisions({ limit: 12 })
      ]);

      return buildReportPrompt("安全态势汇总", {
        gate_overview: overview?.data,
        audit_stats: auditStats?.data,
        recent_decisions: decisions?.data?.slice(0, 12)
      });
    }

    if (type === "experiment") {
      const [summaries, families] = await Promise.all([
        getExperimentSummaries(),
        getAttackFamilyStats()
      ]);

      return buildReportPrompt("实验结果汇总报告", {
        runs: summaries?.data?.slice(0, 24),
        attack_families: families?.data,
        required_attack_types: ["DPI", "OPI", "MIXED", "MP", "POT"]
      });
    }

    if (type === "agents") {
      const [auditStats, decisions] = await Promise.all([
        getAuditStats(),
        getGateDecisions({ limit: 20 })
      ]);

      return buildReportPrompt("已接入 Agent 状态分析", {
        top_agents: auditStats?.data?.top_agents,
        recent_agent_decisions: decisions?.data
          ?.filter(item => item.agent_id)
          .slice(0, 20)
      });
    }

    const [overview, auditStats, families] = await Promise.all([
      getGateOverview(),
      getAuditStats(),
      getAttackFamilyStats()
    ]);

    return buildReportPrompt("风险整改建议", {
      gate_overview: overview?.data,
      audit_stats: auditStats?.data,
      attack_families: families?.data
    });
  } catch {
    return `${type} 数据拉取失败，请基于 AegisGuard 已知模块给出简明分析，并提示需要检查后端数据接口。`;
  }
}

function buildReportPrompt(title: string, payload: unknown) {
  return [
    `你是 AegisGuard 网关的数据研判助手，请生成《${title}》。`,
    "要求：",
    "1. 只围绕 DPI、OPI、MIXED、MP、POT、三道闸、审计链路、Agent 接入进行分析。",
    "2. 先给 3 条以内结论，再列关键指标，最后给可执行建议。",
    "3. 如果数据为空，请说明当前暂无数据，不要编造具体数值。",
    "页面数据如下：",
    JSON.stringify(payload, null, 2)
  ].join("\n");
}

function scrollToBottom() {
  if (!scrollRef.value) return;
  scrollRef.value.scrollTop = scrollRef.value.scrollHeight;
}

function getLocalReply(text: string) {
  const normalized = text.toLowerCase();

  if (text.includes("安全态势") || text.includes("security")) {
    return "当前可从三道闸、审计统计、最近决策三类数据汇总态势：重点看 Block/Deny 数量、critical/high 风险占比、以及是否集中在某个 Agent 或工具调用。";
  }

  if (text.includes("实验结果") || text.includes("experiment")) {
    return "实验报告建议按 DPI、OPI、MIXED、MP、POT 五类列出运行数、用例数、ASR、RR 和平均延迟。ASR 越低表示攻击越难成功，RR 可辅助观察拒答或拦截强度。";
  }

  if (text.includes("Agent") || text.includes("agent") || text.includes("接入")) {
    return "已接入 Agent 可从审计统计的 top_agents 和最近闸门决策中判断：请求量高、风险集中、频繁触发 Action Gate 的 Agent 需要优先核查授权 scope 与工具调用边界。";
  }

  if (normalized.includes("dpi") || text.includes("五类") || text.includes("攻击")) {
    return "DPI 是直接提示注入；OPI 是外部观察结果注入；MIXED 是多阶段混合攻击；MP 是记忆投毒；POT 是权限工具诱导。模拟器和实验结果页已按这五类测试族组织。";
  }

  if (normalized.includes("opi")) {
    return "OPI 重点看 Return Gate：外部工具返回内容如果夹带隐藏指令、密钥或越权要求，会在返回闸门阶段被降级、隔离或拦截。";
  }

  if (normalized.includes("mp") || text.includes("记忆")) {
    return "MP 记忆投毒关注沙箱隔离：高风险内容不应进入 trusted memory，可在记忆沙箱和审计链路里检查隔离记录。";
  }

  if (normalized.includes("pot") || text.includes("工具") || text.includes("权限")) {
    return "POT 主要触发 Action Gate 和 RequireToken 校验。请重点看 tool_name、scope、调用预算和 nonce，确认高权限工具没有被越权调用。";
  }

  if (text.includes("闸门") || text.includes("拦截")) {
    return "闸门判断建议同时看 Message、Action、Return Gate：输入侧看提示注入，动作侧看工具授权，返回侧看外部内容污染。";
  }

  if (text.includes("审计") || text.includes("溯源")) {
    return "审计溯源建议看攻击路径和日志回放，把 request_id、agent_id、decision、risk_score、reason 串起来定位攻击入口和处置结果。";
  }

  return "我可以辅助汇总安全态势、生成实验报告、分析已接入 Agent、给出整改建议。也可以直接问 DPI、OPI、MIXED、MP、POT 或三道闸相关问题。";
}
</script>

<template>
  <div class="mini-ai">
    <transition name="assistant-panel">
      <aside v-if="open" class="mini-ai__panel">
        <header class="mini-ai__header">
          <div>
            <strong>Aegis AI 数据助手</strong>
            <span>态势汇总 / 实验报告 / 研判问答</span>
          </div>
          <div class="mini-ai__header-actions">
            <span class="mini-ai__status-dot">API</span>
            <el-button text circle @click="toggleAssistant">
              <el-icon><Close /></el-icon>
            </el-button>
          </div>
        </header>

        <section class="mini-ai__actions" aria-label="数据汇总快捷操作">
          <button
            v-for="action in analysisActions"
            :key="action.type"
            :disabled="sending"
            @click="runAnalysisAction(action)"
          >
            <strong>{{ action.title }}</strong>
            <span>{{ action.desc }}</span>
          </button>
        </section>

        <div ref="scrollRef" class="mini-ai__messages">
          <div
            v-for="(message, index) in messages"
            :key="index"
            :class="['mini-ai__message', `is-${message.role}`]"
          >
            {{ message.content }}
          </div>
          <div v-if="sending" class="mini-ai__message is-assistant is-loading">
            {{ statusText || "正在分析..." }}
          </div>
        </div>

        <div class="mini-ai__quick">
          <button v-for="prompt in quickPrompts" :key="prompt" @click="askQuick(prompt)">
            {{ prompt }}
          </button>
        </div>

        <div class="mini-ai__input">
          <el-input
            v-model="input"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 4 }"
            resize="none"
            placeholder="直接输入问题，例如：请解释这次 DPI 测试的 ASR 为什么偏高"
            @keydown.enter.exact.prevent="sendMessage"
          />
          <el-button type="primary" :loading="sending" @click="sendMessage">
            发送
          </el-button>
        </div>
      </aside>
    </transition>

    <button :class="['mini-ai__fab', { 'is-open': open }]" @click="toggleAssistant">
      <el-icon><Service /></el-icon>
      <span>{{ open ? "收起" : "AI 助手" }}</span>
    </button>
  </div>
</template>

<style scoped>
.mini-ai {
  position: fixed;
  top: 86px;
  right: 18px;
  bottom: 18px;
  z-index: 3000;
  display: flex;
  align-items: stretch;
  pointer-events: none;
}

.mini-ai__fab,
.mini-ai__panel {
  pointer-events: auto;
}

.mini-ai__fab {
  align-self: center;
  display: flex;
  flex-direction: column;
  gap: 5px;
  align-items: center;
  justify-content: center;
  width: 48px;
  min-height: 126px;
  padding: 10px 6px;
  color: #031126;
  font-weight: 800;
  letter-spacing: 0;
  writing-mode: vertical-rl;
  cursor: pointer;
  background: linear-gradient(135deg, #00d4ff, #37f6a4);
  border: 0;
  border-radius: 14px 0 0 14px;
  box-shadow: 0 0 26px rgb(0 212 255 / 42%);
}

.mini-ai__fab.is-open {
  box-shadow: none;
}

.mini-ai__panel {
  display: flex;
  flex-direction: column;
  width: min(460px, calc(100vw - 82px));
  height: 100%;
  margin-right: 10px;
  overflow: hidden;
  color: #d9f4ff;
  background:
    radial-gradient(circle at 16% 0%, rgb(0 212 255 / 14%), transparent 34%),
    linear-gradient(180deg, rgb(5 18 38 / 96%), rgb(3 11 24 / 96%));
  border: 1px solid rgb(0 212 255 / 28%);
  border-radius: 10px;
  box-shadow: 0 24px 60px rgb(0 0 0 / 44%);
  backdrop-filter: blur(14px);
}

.mini-ai__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 15px 16px 13px;
  background: linear-gradient(90deg, rgb(0 212 255 / 15%), transparent);
  border-bottom: 1px solid rgb(78 192 255 / 16%);
}

.mini-ai__header strong {
  display: block;
  color: #f4fbff;
  font-size: 15px;
}

.mini-ai__header span {
  font-size: 12px;
  color: #8fb6d8;
}

.mini-ai__header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.mini-ai__status-dot {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 8px;
  color: #37f6a4 !important;
  font-size: 11px !important;
  font-weight: 800;
  background: rgb(55 246 164 / 10%);
  border: 1px solid rgb(55 246 164 / 26%);
  border-radius: 999px;
}

.mini-ai__actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 9px;
  padding: 12px;
  border-bottom: 1px solid rgb(78 192 255 / 12%);
}

.mini-ai__actions button {
  min-height: 74px;
  padding: 10px;
  text-align: left;
  cursor: pointer;
  background: rgb(8 31 62 / 76%);
  border: 1px solid rgb(0 212 255 / 17%);
  border-radius: 8px;
  transition:
    border-color 0.18s ease,
    transform 0.18s ease,
    background 0.18s ease;
}

.mini-ai__actions button:hover {
  background: rgb(0 212 255 / 10%);
  border-color: rgb(0 212 255 / 38%);
  transform: translateY(-1px);
}

.mini-ai__actions button:disabled {
  cursor: wait;
  opacity: 0.65;
}

.mini-ai__actions strong {
  display: block;
  margin-bottom: 5px;
  color: #f4fbff;
  font-size: 13px;
}

.mini-ai__actions span {
  color: #8fb6d8;
  font-size: 12px;
  line-height: 1.45;
}

.mini-ai__messages {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
  padding: 13px 14px;
  overflow: auto;
}

.mini-ai__message {
  max-width: 92%;
  padding: 10px 12px;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
  font-size: 13px;
  line-height: 1.65;
  border-radius: 8px;
}

.mini-ai__message.is-assistant {
  align-self: flex-start;
  background: rgb(8 31 62 / 92%);
  border: 1px solid rgb(78 192 255 / 16%);
}

.mini-ai__message.is-user {
  align-self: flex-end;
  color: #031126;
  font-weight: 700;
  background: linear-gradient(135deg, #00d4ff, #37f6a4);
}

.mini-ai__message.is-loading {
  color: #9fdfff;
}

.mini-ai__quick {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 0 14px 12px;
}

.mini-ai__quick button {
  padding: 6px 9px;
  color: #9fdfff;
  cursor: pointer;
  background: rgb(0 212 255 / 8%);
  border: 1px solid rgb(0 212 255 / 20%);
  border-radius: 999px;
}

.mini-ai__input {
  display: grid;
  grid-template-columns: 1fr 74px;
  gap: 9px;
  align-items: end;
  padding: 12px;
  border-top: 1px solid rgb(78 192 255 / 16%);
}

.assistant-panel-enter-active,
.assistant-panel-leave-active {
  transition:
    opacity 0.18s ease,
    transform 0.18s ease;
}

.assistant-panel-enter-from,
.assistant-panel-leave-to {
  opacity: 0;
  transform: translateX(16px);
}

@media (max-width: 640px) {
  .mini-ai {
    top: 72px;
    right: 10px;
    bottom: 12px;
  }

  .mini-ai__panel {
    width: calc(100vw - 72px);
  }

  .mini-ai__actions {
    grid-template-columns: 1fr;
  }
}
</style>
