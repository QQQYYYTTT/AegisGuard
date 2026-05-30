const Layout = () => import("@/layout/index.vue");

export default {
  path: "/simulator",
  name: "Simulator",
  component: Layout,
  redirect: "/simulator/index",
  meta: {
    icon: "ep:video-play",
    title: "运行时模拟器",
    rank: 2,
    showLink: true
  },
  children: [
    {
      path: "/simulator/index",
      name: "SimulatorIndex",
      component: () => import("@/views/simulator/index.vue"),
      meta: {
        title: "运行时模拟器"
      }
    }
  ]
} satisfies RouteConfigsTable;
