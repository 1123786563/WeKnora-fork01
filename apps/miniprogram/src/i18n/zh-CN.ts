/** Small keyed catalogue; migrate shared terms into @weknora/i18n after review. */
export const zhCN={brand:'WeKnora',retry:'重新加载',loading:'正在加载…',empty:'暂无内容',login:'登录',workspace:'工作空间',home:'工作台',tasks:'任务',knowledge:'知识',me:'我的',send:'发送',cancel:'取消',back:'返回',confirm:'确认',logout:'退出登录',noMock:'此页面使用真实 API，不使用演示数据。',needLogin:'请先登录',network:'检查网络后重试'} as const;
export const t=(key:keyof typeof zhCN):string=>zhCN[key];
