export interface ValueStore { read(key: string): unknown; write(key: string, value: unknown): void; remove(key: string): void; keys?(): string[] }
let sequence=0;
/** Unique intent correlation, NOT a password, credential, signature or auth token. */
export function requestId(): string { sequence+=1; return `mini-${Date.now().toString(36)}-${sequence.toString(36)}-${Math.random().toString(36).slice(2,14)}`; }
