import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';
import { VoicePanel } from './VoicePanel';
import { createVoiceControls, createRealtimeVoiceSession, type RealtimeVoicePort, type VoiceSessionAdmission } from './realtime';

vi.mock('react-native', async () => {
  const ReactModule = await import('react');
  const host = (name: string) => (props: any) => ReactModule.createElement(name, props, props.children);
  return { Pressable: host('Pressable'), Text: host('Text'), TextInput: host('TextInput'), View: host('View') };
});

function recorderPort(log: string[]): RealtimeVoicePort {
  return {
    async connect() { log.push('port.connect'); },
    async close() { log.push('port.close'); },
    mute(value) { log.push(`mute:${value}`); },
    stopAudio() { log.push('stopAudio'); },
  };
}

function connectedSession(log: string[]) {
  const session = createRealtimeVoiceSession({
    port: recorderPort(log),
    admit: async (): Promise<VoiceSessionAdmission> => ({ id: 'vs_1', grant: { token: 'tok-1', expiresAt: '2031-01-01T00:00:00Z' }, tokenIssued: true }),
    release: async () => undefined,
  });
  return { session, connect: () => session.begin() };
}

describe('VoicePanel', () => {
  it('renders the three separated audio controls while connected and keeps every path single-purpose', async () => {
    const log: string[] = [];
    const commands: string[] = [];
    const { session, connect } = connectedSession(log);
    const controls = createVoiceControls({
      stopAudio: () => { log.push('stopAudio'); },
      closeVoice: async () => { commands.push('close'); await session.end(); },
      cancelRun: async () => { commands.push('cancel'); },
    });
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(VoicePanel, { session, controls, canCancel: true, runActive: true })); });
    await act(async () => { await connect(); });
    // Mute is a media-surface toggle only.
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '静音' }).props.onPress(); });
    expect(log).toEqual(['port.connect', 'mute:true']);
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '取消静音' }).props.onPress(); });
    expect(log).toEqual(['port.connect', 'mute:true', 'mute:false']);
    // Stop playback is the barge-in: audio only, no close, no cancel.
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '停止播放' }).props.onPress(); });
    expect(commands).toEqual([]);
    // Ending the voice connection closes the provider; the run survives.
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '结束语音' }).props.onPress(); });
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
    expect(commands).toEqual(['close']);
    expect(session.state()).toBe('ended');
    await act(async () => renderer!.unmount());
  });

  it('keeps cancel an explicit product command with its own button, gated by capability and an active run', async () => {
    const log: string[] = [];
    const commands: string[] = [];
    const { session, connect } = connectedSession(log);
    const controls = createVoiceControls({
      stopAudio: () => undefined,
      closeVoice: async () => { commands.push('close'); },
      cancelRun: async () => { commands.push('cancel'); },
    });
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(VoicePanel, { session, controls, canCancel: true, runActive: true })); });
    await act(async () => { await connect(); });
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '取消任务' }).props.onPress(); });
    expect(commands).toEqual(['cancel']);
    await act(async () => renderer!.unmount());

    // W37-gated (canCancel false) or no active run: the button never mounts.
    const gated = connectedSession([]);
    let gatedRenderer: ReturnType<typeof create>;
    await act(async () => { gatedRenderer = create(React.createElement(VoicePanel, { session: gated.session, controls, canCancel: false, runActive: true })); });
    expect(gatedRenderer!.root.findAllByProps({ accessibilityLabel: '取消任务' })).toHaveLength(0);
    await act(async () => gatedRenderer!.unmount());

    const idleRun = connectedSession([]);
    let idleRenderer: ReturnType<typeof create>;
    await act(async () => { idleRenderer = create(React.createElement(VoicePanel, { session: idleRun.session, controls, canCancel: true, runActive: false })); });
    expect(idleRenderer!.root.findAllByProps({ accessibilityLabel: '取消任务' })).toHaveLength(0);
    await act(async () => idleRenderer!.unmount());
  });

  it('offers an explicit connect entry while disconnected and surfaces typed failures', async () => {
    const log: string[] = [];
    let admissions = 0;
    const session = createRealtimeVoiceSession({
      port: recorderPort(log),
      admit: async () => {
        admissions += 1;
        if (admissions === 1) throw new Error('ADMISSION_HTTP_503');
        return { id: 'vs_1', grant: { token: 'tok-1', expiresAt: '2031-01-01T00:00:00Z' }, tokenIssued: true };
      },
      release: async () => undefined,
    });
    const controls = createVoiceControls({ stopAudio: () => undefined, closeVoice: async () => undefined, cancelRun: async () => undefined });
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(VoicePanel, { session, controls, canCancel: false, runActive: false })); });
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '接通语音' }).props.onPress(); });
    expect(session.state()).toBe('failed');
    expect(renderer!.root.findByProps({ accessibilityLabel: 'voice-failure' }).props.children).toBe('语音准入被拒绝');
    // The user explicitly retries; there is no automatic loop in between.
    await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '接通语音' }).props.onPress(); });
    expect(session.state()).toBe('connected');
    await act(async () => renderer!.unmount());
  });
});
