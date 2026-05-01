const Layout = () => import("@/layout/index.vue");

export default {
  path: "/gate-control",
  name: "GateControl",
  component: Layout,
  redirect: "/gate-control/index",
  meta: {
    icon: "ep:shield",
    title: "阻断决策中心",
    rank: 4
  },
  children: [
    {
      path: "/gate-control/index",
      name: "GateControlIndex",
      component: () => import("@/views/gate-control/index.vue"),
      meta: {
        title: "决策总览"
      }
    },
    {
      path: "/gate-control/message",
      name: "MessageGate",
      component: () => import("@/views/gate-control/index.vue"),
      meta: {
        title: "Message Gate",
        showLink: false
      }
    },
    {
      path: "/gate-control/action",
      name: "ActionGate",
      component: () => import("@/views/gate-control/index.vue"),
      meta: {
        title: "Action Gate",
        showLink: false
      }
    },
    {
      path: "/gate-control/return",
      name: "ReturnGate",
      component: () => import("@/views/gate-control/index.vue"),
      meta: {
        title: "Return Gate",
        showLink: false
      }
    }
  ]
} satisfies RouteConfigsTable;
