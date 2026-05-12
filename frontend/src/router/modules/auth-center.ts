const Layout = () => import("@/layout/index.vue");

export default {
  path: "/auth-center",
  name: "AuthCenter",
  component: Layout,
  redirect: "/auth-center/index",
  meta: {
    icon: "ep:lock",
<<<<<<< main
    title: "自动处置中心",
=======
    title: "风险告警中心",
>>>>>>> main
    rank: 3
  },
  children: [
    {
      path: "/auth-center/index",
      name: "AuthCenterIndex",
      component: () => import("@/views/auth-center/index.vue"),
      meta: {
<<<<<<< main
        title: "自动处置中心"
=======
        title: "风险告警中心"
>>>>>>> main
      }
    }
  ]
} satisfies RouteConfigsTable;
