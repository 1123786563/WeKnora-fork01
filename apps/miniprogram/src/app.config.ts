export default defineAppConfig({
  "pages": [
    "pages/home/index",
    "pages/tasks/index",
    "pages/knowledge/index",
    "pages/me/index"
  ],
  "subPackages": [
    {
      "root": "subpackages/auth",
      "pages": [
        "login/index",
        "workspace/index"
      ]
    },
    {
      "root": "subpackages/agent",
      "pages": [
        "list/index",
        "detail/index"
      ]
    },
    {
      "root": "subpackages/chat",
      "pages": [
        "conversation/index"
      ]
    },
    {
      "root": "subpackages/execution",
      "pages": [
        "detail/index",
        "approval/index",
        "artifact/index"
      ]
    },
    {
      "root": "subpackages/knowledge",
      "pages": [
        "detail/index",
        "upload/index",
        "document/index"
      ]
    },
    {
      "root": "subpackages/account",
      "pages": [
        "usage/index",
        "checkout/index",
        "order/index",
        "invitations/index",
        "states/index"
      ]
    }
  ],
  "window": {
    "navigationStyle": "custom",
    "backgroundColor": "#F5F7F2",
    "backgroundTextStyle": "dark"
  },
  // 不启用 lazyCodeLoading:requiredComponents——Taro 全局自定义组件（comp）在该模式下
  // 基础库按需注入失败（Component is not found in path "wx://not-found"），全部页面白屏。
  "tabBar": {
    "color": "#657469",
    "selectedColor": "#155B49",
    "backgroundColor": "#FFFFFF",
    "borderStyle": "white",
    "list": [
      {
        "pagePath": "pages/home/index",
        "text": "工作台",
        "iconPath": "assets/tab-spark.png",
        "selectedIconPath": "assets/tab-spark-active.png"
      },
      {
        "pagePath": "pages/tasks/index",
        "text": "任务",
        "iconPath": "assets/tab-tasks.png",
        "selectedIconPath": "assets/tab-tasks-active.png"
      },
      {
        "pagePath": "pages/knowledge/index",
        "text": "知识",
        "iconPath": "assets/tab-book.png",
        "selectedIconPath": "assets/tab-book-active.png"
      },
      {
        "pagePath": "pages/me/index",
        "text": "我的",
        "iconPath": "assets/tab-user.png",
        "selectedIconPath": "assets/tab-user-active.png"
      }
    ]
  }
});
