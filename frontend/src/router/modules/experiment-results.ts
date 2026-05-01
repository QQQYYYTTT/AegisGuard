const Layout = () => import("@/layout/index.vue");

const isDevMode = import.meta.env.VITE_DEV_MODE === "true";

export default isDevMode
  ? {
      path: "/experiment-results",
      name: "ExperimentResults",
      component: Layout,
      redirect: "/experiment-results/index",
      meta: {
        icon: "ep:data-analysis",
        title: "实验结果",
        rank: 10,
        showLink: isDevMode
      },
      children: [
        {
          path: "/experiment-results/index",
          name: "ExperimentResultsIndex",
          component: () => import("@/views/experiment-results/index.vue"),
          meta: {
            title: "实验结果"
          }
        }
      ]
    }
  : null;
