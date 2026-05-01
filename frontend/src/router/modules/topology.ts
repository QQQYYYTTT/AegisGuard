const Layout = () => import("@/layout/index.vue");

export default {
  path: "/topology",
  name: "Topology",
  component: Layout,
  redirect: "/topology/index",
  meta: {
    icon: "ep:share",
    title: "拓扑与攻击路径",
    rank: 6
  },
  children: [
    {
      path: "/topology/index",
      name: "TopologyIndex",
      component: () => import("@/views/topology/index.vue"),
      meta: {
        title: "拓扑与攻击路径"
      }
    }
  ]
} satisfies RouteConfigsTable;
