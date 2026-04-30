const Layout = () => import("@/layout/index.vue");

export default {
  path: "/audit-trace",
  name: "AuditTrace",
  component: Layout,
  redirect: "/audit-trace/index",
  meta: {
    icon: "ep:document-checked",
    title: "审计追踪",
    rank: 8
  },
  children: [
    {
      path: "/audit-trace/index",
      name: "AuditTraceIndex",
      component: () => import("@/views/audit-trace/index.vue"),
      meta: {
        title: "审计追踪"
      }
    }
  ]
} satisfies RouteConfigsTable;
