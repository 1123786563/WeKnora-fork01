export type AuthTranslator = (key: string, values?: Record<string, string | number>) => string;
const messages: Record<string, string> = {
  'auth.login': '登录', 'auth.createAccount': '创建账户', 'auth.subtitle': '欢迎使用 WeKnora', 'auth.email': '邮箱', 'auth.password': '密码',
  'auth.emailPlaceholder': '请输入邮箱', 'auth.passwordPlaceholder': '请输入密码', 'auth.loggingIn': '登录中...', 'auth.register': '注册', 'auth.registering': '注册中...',
  'auth.username': '用户名', 'auth.confirmPassword': '确认密码', 'auth.usernamePlaceholder': '请输入用户名', 'auth.confirmPasswordPlaceholder': '请再次输入密码',
  'auth.emailRequired': '请输入邮箱', 'auth.emailInvalid': '请输入有效的邮箱地址', 'auth.passwordRequired': '请输入密码', 'auth.passwordMinLength': '密码至少需要 8 个字符', 'auth.passwordMaxLength': '密码不能超过 32 个字符', 'auth.passwordComplexity': '密码需包含大小写字母、数字和特殊字符',
  'auth.usernameRequired': '请输入用户名', 'auth.usernameMinLength': '用户名至少需要 2 个字符', 'auth.usernameMaxLength': '用户名不能超过 20 个字符', 'auth.usernameInvalid': '用户名只能包含字母、数字、下划线或中文', 'auth.confirmPasswordRequired': '请确认密码', 'auth.passwordMismatch': '两次输入的密码不一致',
  'auth.loginError': '登录失败，请检查邮箱或密码', 'auth.registerSuccess': '注册成功，请登录', 'auth.registerFailed': '注册失败', 'auth.registerSubtitle': '创建账户并开始使用 WeKnora', 'auth.firstTime': '首次使用？', 'auth.backToLogin': '返回登录', 'auth.loginHint': '登录以继续使用；首次使用请在下方创建账户。', 'auth.oidcLogin': '使用单点登录', 'auth.oidcFailed': '单点登录失败',
  'auth.workspaceTitle': '创建或加入空间', 'auth.workspaceInviteOnlyTitle': '加入空间', 'auth.workspaceDescription': '创建一个空间来开始协作。', 'auth.workspaceInviteOnlyDescription': '当前部署仅支持通过邀请加入空间。', 'auth.loadingPolicy': '正在加载空间策略...', 'auth.policyLoadFailed': '空间策略加载失败', 'auth.retry': '重试', 'auth.createWorkspace': '创建空间', 'auth.viewInvitations': '查看邀请', 'auth.inviteOnlyNotice': '你没有创建空间的权限，请使用邀请加入。', 'auth.workspaceHelp': '你也可以稍后从空间设置中管理成员。', 'auth.inviteOnlyHelp': '请联系空间管理员获取邀请。', 'auth.logout': '退出登录', 'auth.inviteBanner': '你已被邀请加入 {tenant}', 'auth.invalidInvite': '邀请链接无效或已过期', 'auth.joined': '已加入空间',
};
export const defaultAuthTranslator: AuthTranslator = (key, values) => (messages[key] ?? key).replace(/\{(\w+)\}/g, (_, name) => String(values?.[name] ?? `{${name}}`));
