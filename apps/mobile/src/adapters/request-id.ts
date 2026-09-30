/**
 * 一次用户意图的 request_id（持久幂等关联键，NOT a password, credential,
 * signature or auth token——与 miniprogram core/intent.ts:25 同一纪律）。
 * 不可碰撞是正确性要求（碰撞 = 两个意图被服务端当成同一个幂等键），因此
 * 优先使用平台 CSPRNG；Math.random 仅为最后兜底。
 */
export function createNativeRequestId(): () => string {
  return (): string => {
    if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
    // Hermes 无 globalThis.crypto：expo-crypto 的 CSPRNG 是主路径（B3-F27）。
    try {
      const expoCrypto = require('expo-crypto') as { randomUUID?: () => string };
      if (typeof expoCrypto.randomUUID === 'function') return expoCrypto.randomUUID();
    } catch { /* expo-crypto 未安装（测试 stub 环境）：落既有兜底链 */ }
    const bytes = new Uint8Array(16);
    if (typeof globalThis.crypto?.getRandomValues === 'function') {
      globalThis.crypto.getRandomValues(bytes);
    } else {
      // Math.random 兜底强度低于 CSPRNG 但可用性优先（mobile-core 的 fail-closed
      // 纪律会阻断整个提交流）；expo-crypto 是缺省安装项。
      for (let index = 0; index < bytes.length; index += 1) bytes[index] = Math.floor(Math.random() * 256);
    }
    bytes[6] = (bytes[6]! & 0x0f) | 0x40;
    bytes[8] = (bytes[8]! & 0x3f) | 0x80;
    const hex = [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('');
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  };
}
