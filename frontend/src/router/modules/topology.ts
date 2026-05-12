const Layout = () => import("@/layout/index.vue");

export default {
  path: "/topology",
  name: "Topology",
  component: Layout,
  redirect: "/topology/index",
  meta: {
    icon: "ep:share",
    title: "攻击路径溯源",
    rank: 6
  },
  children: [
    {
      path: "/topology/index",
      name: "TopologyIndex",
      component: () => import("@/views/topology/index.vue"),
      meta: {
        title: "攻击路径溯源"
      }
    }
  ]
} satisfies RouteConfigsTable;
