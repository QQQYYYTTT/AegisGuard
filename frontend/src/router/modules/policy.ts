const Layout = () => import("@/layout/index.vue");

export default {
  path: "/policy",
  name: "Policy",
  component: Layout,
  redirect: "/policy/index",
  meta: {
    icon: "ep:document",
    title: "策略规则管理",
    rank: 6
  },
  children: [
    {
      path: "/policy/index",
      name: "PolicyIndex",
      component: () => import("@/views/policy/index.vue"),
      meta: {
        title: "策略规则管理"
      }
    }
  ]
} satisfies RouteConfigsTable;