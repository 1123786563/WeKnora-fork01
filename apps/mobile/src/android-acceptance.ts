/**
 * Android 安装包核心工作流验收矩阵（Issue #70）。把验收标准 2「与 iOS 的领域行为一致，平台
 * 差异仅在 Adapter」与验收标准 3「端到端行为通过最高稳定 Interface 验证」编码为可执行守卫：
 * 每个工作流映射到双平台共享的领域 Interface、平台 Adapter seam、仓库内真实存在的本地证据
 * （由 android-parity.test.ts existsSync 强制），以及只在真机/外部凭据环境可验证的残余项
 * （kind 恒 'blocked-env'——绝不计为通过，解锁条件必须显式）。#71 发布证据矩阵直接消费本表。
 */
export const ANDROID_WORKFLOWS = [
  'system-back',
  'background-limits',
  'notifications',
  'file-uri',
  'keystore',
  'microphone',
  'weak-network',
] as const;

export type AndroidWorkflow = (typeof ANDROID_WORKFLOWS)[number];

export interface AndroidWorkflowAcceptance {
  workflow: AndroidWorkflow;
  /** 双平台共享的领域 Interface（packages/mobile-core 深模块 / 导航面）——平台差异绝不在此层。 */
  domainInterface: string;
  /** 平台 Adapter seam（apps/mobile/src/adapters/ 或导航 adapter）——平台差异只允许在这里。 */
  adapterSeam: string;
  /** 仓库内真实存在的本地证据资产（测试/集成冒烟/源级守卫）。 */
  localEvidence: string[];
  /** 真机/外部凭据残余（blocked-env）。 */
  residual: { kind: 'blocked-env'; reason: string; unblock: string };
}

export const ANDROID_ACCEPTANCE_MATRIX: AndroidWorkflowAcceptance[] = [
  {
    workflow: 'system-back',
    domainInterface: 'expo-router Stack 导航 + weknora://oidc 回调（deliverOidcReturn 单次投递 + 已登记回调白名单）',
    adapterSeam: 'apps/mobile/src/app/auth-return.tsx',
    localEvidence: ['apps/mobile/src/app-smoke.test.tsx'],
    residual: {
      kind: 'blocked-env',
      reason: '返回键/手势逐级回退与「返回不复活已登出内容」需要 Android 真机系统行为',
      unblock: 'Release APK 安装于 Android 真机执行返回键/手势路径（android-release.md 验收清单）',
    },
  },
  {
    workflow: 'background-limits',
    domainInterface: 'createForegroundSyncLoop（单飞合并 + 失败包含 + scope 每事件重解析）→ NotificationInbox.page() 权威重投影',
    adapterSeam: 'apps/mobile/src/adapters/app-state.ts',
    localEvidence: ['apps/mobile/src/foreground-sync.test.ts', 'apps/mobile/src/adapters/app-state.test.ts'],
    residual: {
      kind: 'blocked-env',
      reason: 'Doze/电池优化下 AppState 事件与网络可用性的真实组合行为需要真机',
      unblock: 'Android 真机后台/前台往返与省电模式验收轮',
    },
  },
  {
    workflow: 'notifications',
    domainInterface: 'createDeviceRegistry（两步注册/409 重取/接管）+ createNotificationInbox（分页合并/幂等已读/安全深链）',
    adapterSeam: 'apps/mobile/src/adapters/notification-permission.ts',
    localEvidence: [
      'apps/mobile/src/adapters/notification-permission.test.ts',
      'apps/mobile/src/blind-push-integration-smoke.test.ts',
      'apps/mobile/src/device-inbox-integration-smoke.test.ts',
    ],
    residual: {
      kind: 'blocked-env',
      reason: '真实 FCM 通道送达需要推送凭据部署（本地 #67 形态为 disabled provider）；POST_NOTIFICATIONS 系统弹窗需要真机',
      unblock: '有 FCM 凭据的部署 + Android 13+ 真机（android-release.md 验收清单「通知」行）',
    },
  },
  {
    workflow: 'file-uri',
    domainInterface: 'Dictation（audio uri → multipart 上载 → dropIntent 即删原始音频）+ TaskMaterial BlobFetchPort（免凭据签名链接）',
    adapterSeam: 'apps/mobile/src/adapters/dictation-capture.ts',
    localEvidence: [
      'apps/mobile/src/adapters/dictation-capture.test.ts',
      'apps/mobile/src/voice-dictation-integration-smoke.test.ts',
      'apps/mobile/src/material-integration-smoke.test.ts',
    ],
    residual: {
      kind: 'blocked-env',
      reason: '真机上 file:// URI 的 OS 级边界（FileUriExposed）与临时文件实际落盘/删除需要真机',
      unblock: 'Android 真机录音→转写→确认→文件系统核验轮',
    },
  },
  {
    workflow: 'keystore',
    domainInterface: 'ScopedVault（scope 隔离/rotate/revoke）+ CredentialStore/DeploymentRegistry/IntentLog（SecureStore 端口族）',
    adapterSeam: 'apps/mobile/src/adapters/secure-store.ts',
    localEvidence: [
      'apps/mobile/src/adapters/vault-adapters.test.ts',
      'apps/mobile/src/adapters/intent-log.test.ts',
      'apps/mobile/src/adapters/deployment-registry.test.ts',
      'apps/mobile/src/offline-vault-integration-smoke.test.ts',
    ],
    residual: {
      kind: 'blocked-env',
      reason: 'Android Keystore 硬件 backed 密钥与真机 SecureStore 行为（含系统备份排除的实际生效）需要真机',
      unblock: 'Android 真机登录→重启→登出→密钥不可用核验轮',
    },
  },
  {
    workflow: 'microphone',
    domainInterface: 'Dictation 捕获生命周期（begin/finish/cancel + denied fail closed）',
    adapterSeam: 'apps/mobile/src/adapters/dictation-capture.ts',
    localEvidence: [
      'apps/mobile/src/adapters/dictation-capture.test.ts',
      'apps/mobile/src/voice-dictation-integration-smoke.test.ts',
    ],
    residual: {
      kind: 'blocked-env',
      reason: 'RECORD_AUDIO 系统拒权弹窗与真实录音设备行为需要真机（spec：OS audio 属 true external）',
      unblock: 'Android 真机麦克风拒权/授权两路径验收轮（#69/#70 共同口径）',
    },
  },
  {
    workflow: 'weak-network',
    domainInterface: 'createOfflineGate（探测失败=离线 fail closed）+ TaskHandle resync（有界自动重同步上限 2）+ 快照降级渲染',
    adapterSeam: 'apps/mobile/src/adapters/network-status.ts',
    localEvidence: [
      'apps/mobile/src/adapters/network-status.test.ts',
      'apps/mobile/src/offline-vault-integration-smoke.test.ts',
      'apps/mobile/src/task-detail-integration-smoke.test.ts',
    ],
    residual: {
      kind: 'blocked-env',
      reason: '真实丢包/慢速/网络切换下的 SSE 断线与恢复时序需要真机弱网环境',
      unblock: 'Android 真机飞行模式/弱网代理验收轮',
    },
  },
];
