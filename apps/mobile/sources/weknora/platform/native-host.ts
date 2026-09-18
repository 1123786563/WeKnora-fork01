import * as SQLite from 'expo-sqlite';
import * as SecureStore from 'expo-secure-store';
import sodium from '@/encryption/libsodium.lib';
import {
  createExpoSQLiteExecutionDriver,
  createExecutionStorage,
  createSecureStoreAeadCipher,
  type ExecutionStorage,
  type ExpoSQLiteDatabase,
} from './execution-storage.ts';
import type { ExecutionScope } from '@weknora/domain/mobile';

/**
 * 原生持久组合根（MX-012，关闭 G06 适配面）。
 * 真实 expo-sqlite（openDatabaseSync）→ 官方 void 事务 API 适配；真实 SecureStore AEAD 密钥
 * （single-flight：密钥初始化只发生一次，并发首次加密不重复生成）；libsodium XChaCha20-Poly1305。
 * 本模块是组合根（native-only）：node 测试经 execution-storage 的注入接口覆盖行为；
 * 真机 SQLite 运行证据归 MX-034（native-e2e）。
 */

export interface NativeExecutionHandle {
  storage: ExecutionStorage;
  close(): void;
}

export function openNativeExecutionDatabase(name = 'weknora-executions.db'): SQLite.SQLiteDatabase {
  return SQLite.openDatabaseSync(name);
}

/** 官方 SQLiteDatabase → 存储 driver 需要的最小面（类型层面即官方 API，无伪造返回值）。 */
export function adaptExpoSQLite(db: SQLite.SQLiteDatabase): ExpoSQLiteDatabase {
  return {
    withTransactionAsync: (work) => db.withTransactionAsync(work),
    // 存储层参数均为 string/number（见 execution-storage 的调用点），此处收敛为官方绑定值类型
    runAsync: (sql, ...params) => db.runAsync(sql, ...(params as SQLiteBindParams)),
    getFirstAsync: async <T extends Record<string, unknown>>(sql: string, ...params: unknown[]) => (await db.getFirstAsync(sql, ...(params as SQLiteBindParams))) as T | null,
    getAllAsync: async <T extends Record<string, unknown>>(sql: string, ...params: unknown[]) => (await db.getAllAsync(sql, ...(params as SQLiteBindParams))) as T[],
  };
}

type SQLiteBindParams = SQLite.SQLiteBindValue[];

const aeadBox = {
  randomBytes: (size: number) => sodium.randombytes_buf(size),
  encrypt: (message: Uint8Array, aad: Uint8Array, nonce: Uint8Array, key: Uint8Array) =>
    sodium.crypto_aead_xchacha20poly1305_ietf_encrypt(message, aad, null, nonce, key),
  decrypt: (ciphertext: Uint8Array, aad: Uint8Array, nonce: Uint8Array, key: Uint8Array) =>
    sodium.crypto_aead_xchacha20poly1305_ietf_decrypt(null, ciphertext, aad, nonce, key),
};

/** AEAD 密钥 single-flight：并发首次加密共享同一次初始化。 */
let cipherPromise: ReturnType<typeof createSecureStoreAeadCipher> | undefined;

export function nativeExecutionCipher() {
  cipherPromise ??= createSecureStoreAeadCipher(
    {
      getItemAsync: (key) => SecureStore.getItemAsync(key),
      setItemAsync: (key, value) => SecureStore.setItemAsync(key, value),
    },
    aeadBox,
  );
  return cipherPromise;
}

export function openNativeExecutionStorage(scope: ExecutionScope, db = openNativeExecutionDatabase()): NativeExecutionHandle {
  const driver = createExpoSQLiteExecutionDriver(adaptExpoSQLite(db));
  const storage = createExecutionStorage(driver, nativeExecutionCipher(), scope);
  return {
    storage,
    close: () => db.closeSync(),
  };
}
