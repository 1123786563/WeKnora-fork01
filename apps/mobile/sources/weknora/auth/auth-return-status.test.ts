import { describe, expect, test } from 'vitest';
import { createAuthState } from '@weknora/domain/mobile';
import { AUTH_RETURN_REDIRECT, buildCallbackUrl, buildNativeExchangeInput, evaluateAuthReturn } from './auth-return-status';

describe('evaluateAuthReturn', () => {
  test('builds a POST exchange payload and rejects URL credentials', () => {
    expect(buildNativeExchangeInput({ code: 'c', state: 's' }, AUTH_RETURN_REDIRECT, 'v')).toEqual({ code: 'c', state: 's', redirect_uri: AUTH_RETURN_REDIRECT, code_verifier: 'v' });
    expect(buildNativeExchangeInput({ code: 'c', state: 's', access_token: 'bearer' }, AUTH_RETURN_REDIRECT, 'v')).toBeNull();
  });
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

  test('callback URL rejects repeated parameters instead of dropping values', () => {
    expect(buildCallbackUrl({ state: ['a', 'b'], code: 'c' }, AUTH_RETURN_REDIRECT)).toBeNull();
  });
});
