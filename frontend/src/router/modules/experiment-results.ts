const Layout = () => import("@/layout/index.vue");

export default {
  path: "/experiment-results",
  name: "ExperimentResults",
  component: Layout,
  redirect: "/experiment-results/index",
  meta: {
    icon: "ep:data-analysis",
    title: "安全态势分析",
    rank: 10
  },
  children: [
    {
      path: "/experiment-results/index",
      name: "ExperimentResultsIndex",
      component: () => import("@/views/experiment-results/index.vue"),
      meta: {
        title: "安全态势分析"
      }
    }
  ]
} satisfies RouteConfigsTable;
