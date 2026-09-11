import { describe, expect, test } from 'vitest';
import { createAuthState } from '@weknora/domain/mobile';
import { AUTH_RETURN_REDIRECT, buildCallbackUrl, evaluateAuthReturn } from './auth-return-status';

describe('evaluateAuthReturn', () => {
  test('accepts a state this app started and consumes it exactly once', () => {
    const state = createAuthState();
    const outcome = evaluateAuthReturn({ state, code: 'c' }, AUTH_RETURN_REDIRECT);
    expect(outcome).toEqual({ status: 'verified', state });
    expect(evaluateAuthReturn({ state, code: 'c' }, AUTH_RETURN_REDIRECT)).toEqual({ status: 'unknown_state' });
  });

  test('rejects unsolicited states this app never issued', () => {
    expect(evaluateAuthReturn({ state: 'forged', code: 'c' }, AUTH_RETURN_REDIRECT)).toEqual({ status: 'unknown_state' });
  });

  test('rejects links missing the state parameter', () => {
    expect(evaluateAuthReturn({ code: 'c' }, AUTH_RETURN_REDIRECT)).toEqual({ status: 'missing_state' });
  });

  test('rejects callback URLs carrying bearer tokens', () => {
    const state = createAuthState();
    expect(evaluateAuthReturn({ state, access_token: 'secret' }, AUTH_RETURN_REDIRECT)).toEqual({ status: 'rejected' });
    const fragmentState = createAuthState();
    expect(evaluateAuthReturn({ state: fragmentState, token: 'secret' }, AUTH_RETURN_REDIRECT)).toEqual({ status: 'rejected' });
  });

  test('callback URL keeps only the first value of repeated params', () => {
    expect(buildCallbackUrl({ state: ['a', 'b'], code: 'c' }, AUTH_RETURN_REDIRECT)).toBe(
      `${AUTH_RETURN_REDIRECT}?state=a&code=c`,
    );
  });
});
