const Layout = () => import("@/layout/index.vue");

export default {
  path: "/audit-trace",
  name: "AuditTrace",
  component: Layout,
  redirect: "/audit-trace/index",
  meta: {
    icon: "ep:document-checked",
<<<<<<< main
    title: "攻击路径溯源",
    rank: 8
=======
    title: "审计追踪",
    rank: 8,
    showLink: false
>>>>>>> main
  },
  children: [
    {
      path: "/audit-trace/index",
      name: "AuditTraceIndex",
      component: () => import("@/views/audit-trace/index.vue"),
      meta: {
        title: "攻击路径溯源"
      }
    }
  ]
} satisfies RouteConfigsTable;
