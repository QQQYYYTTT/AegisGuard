const Layout = () => import("@/layout/index.vue");

export default {
  path: "/auth-center",
  name: "AuthCenter",
  component: Layout,
  redirect: "/auth-center/index",
  meta: {
    icon: "ep:lock",
    title: "自动处置中心",
    rank: 3
  },
  children: [
    {
      path: "/auth-center/index",
      name: "AuthCenterIndex",
      component: () => import("@/views/auth-center/index.vue"),
      meta: {
        title: "自动处置中心"
      }
    }
  ]
} satisfies RouteConfigsTable;
