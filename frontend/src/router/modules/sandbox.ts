const Layout = () => import("@/layout/index.vue");

export default {
  path: "/sandbox",
  name: "Sandbox",
  component: Layout,
  redirect: "/sandbox/index",
  meta: {
    icon: "ep:box",
    title: "记忆沙箱",
    rank: 5,
    showLink: false
  },
  children: [
    {
      path: "/sandbox/index",
      name: "SandboxIndex",
      component: () => import("@/views/sandbox/index.vue"),
      meta: {
        title: "记忆沙箱"
      }
    }
  ]
} satisfies RouteConfigsTable;
