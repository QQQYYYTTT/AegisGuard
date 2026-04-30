const Layout = () => import("@/layout/index.vue");

export default {
  path: "/",
  name: "Home",
  component: Layout,
  redirect: "/dashboard",
  meta: {
    icon: "ep:monitor",
    title: "系统总览",
    rank: 0
  },
  children: [
    {
      path: "/dashboard",
      name: "HomeDashboard",
      component: () => import("@/views/dashboard/index.vue"),
      meta: {
        title: "系统总览",
        showLink: false
      }
    }
  ]
} satisfies RouteConfigsTable;
