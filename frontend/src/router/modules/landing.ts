const Layout = () => import("@/layout/index.vue");

export default {
  path: "/",
  name: "LandingPage",
  component: Layout,
  redirect: "/landing/index",
  meta: {
    icon: "ep:monitor",
    title: "态势感知指挥屏",
    rank: 0
  },
  children: [
    {
      path: "/landing/index",
      name: "LandingIndex",
      component: () => import("@/views/landing/index.vue"),
      meta: {
        title: "态势感知指挥屏"
      }
    }
  ]
} satisfies RouteConfigsTable;