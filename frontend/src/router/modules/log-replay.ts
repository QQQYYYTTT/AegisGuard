const Layout = () => import("@/layout/index.vue");

export default {
  path: "/log-replay",
  name: "LogReplay",
  component: Layout,
  redirect: "/log-replay/index",
  meta: {
    icon: "ep:timer",
    title: "攻击日志回放",
    rank: 7
  },
  children: [
    {
      path: "/log-replay/index",
      name: "LogReplayIndex",
      component: () => import("@/views/log-replay/index.vue"),
      meta: {
        title: "攻击日志回放"
      }
    }
  ]
} satisfies RouteConfigsTable;
