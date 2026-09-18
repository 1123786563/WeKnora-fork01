// Mounted `/new` route integration tests (W10 final13 follow-up).
//
// Unlike the adapter-level tests in `sources/utils/productSessionEntry.test.ts`,
// these cases render the real `NewSessionScreen` from the `(app)/new` route and
// drive its production success path: submit -> `machineSpawnNewSession` ->
// refresh -> durable session read -> product router destination. The
// missing-metadata case asserts the production fail-closed behavior: no
// navigation and the real unavailable alert.
//
// Native RN/Expo surfaces (react-native, expo-router, glass, keyboard, assets)
// are stubbed at the module boundary; the product navigation chain
// (`useNavigateToProductSession` -> `navigateToProductSession` ->
// `sessionHref`) and the creation seam (`completeProductSessionCreation`) run
// as real code so the asserted `router.push` destination is the one production
// would issue.

import * as React from 'react';
// @ts-expect-error react-test-renderer ships no type declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { createRequire } from 'node:module';
import { beforeEach, describe, expect, it, vi } from 'vitest';

// The route module evaluates `require('@/assets/images/...')` at import time.
// Under vitest those calls fall through to Node's require (the `@/` alias
// resolves nowhere there), so intercept just that specifier family on the
// loader before the route module is imported dynamically below. Everything
// else still loads through the original loader.
const nodeRequire = createRequire(import.meta.url);
type NodeModuleWithLoad = typeof import('node:module') & {
    // Internal loader used by every CommonJS `require`; not part of the
    // public typings, hence the structural extension below.
    _load: (request: unknown, parent: unknown, isMain: boolean) => unknown;
};
const NodeModule = nodeRequire('module') as NodeModuleWithLoad;
const originalModuleLoad = NodeModule._load;
NodeModule._load = ((request: unknown, parent: unknown, isMain: boolean) => {
    if (typeof request === 'string' && request.startsWith('@/assets/')) {
        return `asset:${request}`;
    }
    return originalModuleLoad(request, parent, isMain);
}) as NodeModuleWithLoad['_load'];

const mocks = vi.hoisted(() => {
    const events: string[] = [];
    // The durable session store the route reads after refreshing. The getter
    // records every read so ordering can be asserted.
    const sessionRecords: Record<string, unknown> = {};
    const storageState = {
        currentViewingSessionId: null as string | null,
        get sessions(): Record<string, unknown> {
            events.push('storage.sessions-read');
            return sessionRecords;
        },
    };
    // Minimal stand-in for the MMKV-backed zustand draft store: callable as a
    // selector hook, with `getState()` for the send handler's live read.
    const draftState: Record<string, unknown> = {};
    const setDraft = (key: string) => (value: unknown) => {
        draftState[key] = value;
    };
    const draftStore = (selector?: (state: Record<string, unknown>) => unknown) => (
        selector ? selector(draftState) : draftState
    );
    draftStore.getState = () => draftState;
    const resetDraft = () => {
        Object.assign(draftState, {
            input: 'Plan the launch',
            attachments: [] as unknown[],
            selectedMachineId: 'machine-1',
            selectedPath: null,
            agentType: 'claude',
            permissionMode: null,
            modelMode: null,
            effortLevel: null,
            sessionType: 'simple',
            worktreeKey: null,
            setMachineId: setDraft('selectedMachineId'),
            renameMachineId: setDraft('selectedMachineId'),
            setPath: setDraft('selectedPath'),
            setAgentType: setDraft('agentType'),
            setPermissionMode: setDraft('permissionMode'),
            setModelMode: setDraft('modelMode'),
            setEffortLevel: setDraft('effortLevel'),
            setSessionType: setDraft('sessionType'),
            setWorktreeKey: setDraft('worktreeKey'),
            setInput: setDraft('input'),
            setAttachments: setDraft('attachments'),
        });
    };
    resetDraft();
    return {
        events,
        sessionRecords,
        storageState,
        draftState,
        draftStore,
        resetDraft,
        machines: [] as Array<Record<string, unknown>>,
        sessionsList: [] as Array<unknown>,
        settings: {} as Record<string, unknown>,
        localSettings: { zenMode: false } as Record<string, unknown>,
        routerPush: vi.fn(),
        refreshSessions: vi.fn(),
        sendMessage: vi.fn(),
        machineSpawnNewSession: vi.fn(),
        sessionSetAgentModes: vi.fn(),
        alert: vi.fn(),
        confirm: vi.fn(),
        completeCreation: vi.fn(),
    };
});

const productSessionMetadata = {
    spaceId: 'space-1',
    agentId: 'agent-1',
    targetId: 'target-1',
    workspaceRef: 'workspace-1',
    resourceUserId: 'user-1',
    resourceTenantId: 'tenant-1',
    runId: 'run-1',
};

vi.mock('react-native', async () => {
    const ReactModule = await import('react');
    const host = (name: string) => {
        const Component = (props: {
            children?: unknown;
            [key: string]: unknown;
        }) => {
            const { children, ...rest } = props ?? {};
            const rendered = typeof children === 'function'
                ? children({ pressed: false })
                : children;
            return ReactModule.createElement(name, rest, rendered as React.ReactNode);
        };
        Component.displayName = name;
        return Component;
    };
    const animatedValue = class {
        value: number;
        constructor(initial: number) {
            this.value = initial;
        }
    };
    const animation = (value: { value: number }, config: { toValue: number }) => ({
        start: (done?: (result: unknown) => void) => {
            value.value = config.toValue;
            done?.({ finished: true });
        },
        stop: () => undefined,
    });
    return {
        Platform: {
            OS: 'ios',
            select: (options: Record<string, unknown>) => options.ios ?? options.default,
        },
        View: host('View'),
        Text: host('Text'),
        Pressable: host('Pressable'),
        TextInput: host('TextInput'),
        ScrollView: host('ScrollView'),
        TouchableWithoutFeedback: host('TouchableWithoutFeedback'),
        ActivityIndicator: host('ActivityIndicator'),
        Modal: host('Modal'),
        Image: host('Image'),
        Keyboard: {
            addListener: () => ({ remove: () => undefined }),
            dismiss: () => undefined,
            isVisible: () => false,
        },
        LayoutAnimation: {
            configureNext: () => undefined,
            Presets: { easeInEaseOut: {} },
        },
        useWindowDimensions: () => ({ width: 390, height: 844, scale: 2, fontScale: 1 }),
        Animated: {
            Value: animatedValue,
            View: host('AnimatedView'),
            Text: host('AnimatedText'),
            parallel: (animations: unknown[]) => ({
                start: (done?: (result: unknown) => void) => done?.({ finished: true }),
                stop: () => undefined,
            }),
            timing: animation,
            spring: animation,
        },
    };
});

vi.mock('react-native-unistyles', () => {
    const theme = {
        colors: {
            text: '#111111',
            textSecondary: '#666666',
            background: '#FFFFFF',
            surfaceHighest: '#F4F4F5',
            divider: '#E4E4E7',
            header: { background: '#FFFFFF', tint: '#111111' },
            input: { background: '#F4F4F5' },
            groupped: { background: '#FFFFFF', chevron: '#C7C7CC' },
            button: {
                primary: { tint: '#FFFFFF', background: '#111111', disabled: '#9CA3AF' },
                secondary: { tint: '#EEEEEE' },
            },
            status: { disconnected: '#CC0000', connected: '#00AA00' },
            glass: {
                overlayTint: '#CCCCCC',
                overlay: '#EEEEEE',
                backgroundStrong: '#DDDDDD',
                backgroundSubtle: '#EEEEF0',
                border: '#D4D4D8',
                highlight: '#FFFFFF',
                shadow: '#00000055',
            },
        },
    };
    return {
        StyleSheet: {
            create: (defs: unknown) => (typeof defs === 'function'
                ? (defs as (t: unknown) => unknown)(theme)
                : defs),
        },
        useUnistyles: () => ({ theme }),
    };
});

vi.mock('expo-router', () => ({
    useRouter: () => ({
        push: mocks.routerPush,
        replace: vi.fn(),
        back: vi.fn(),
        prefetch: vi.fn(),
        navigate: vi.fn(),
        setParams: vi.fn(),
        canDismiss: () => false,
    }),
    useLocalSearchParams: () => ({} as Record<string, string | undefined>),
    useNavigation: () => ({ setOptions: vi.fn() }),
}));

vi.mock('expo-glass-effect', async () => {
    const ReactModule = await import('react');
    return {
        GlassView: (props: { children?: React.ReactNode }) => ReactModule.createElement('GlassView', props),
    };
});

vi.mock('@expo/vector-icons', async () => {
    const ReactModule = await import('react');
    const icon = (name: string) => (props: { children?: React.ReactNode }) => ReactModule.createElement(name, props);
    return { Ionicons: icon('Ionicons'), Octicons: icon('Octicons'), MaterialCommunityIcons: icon('MaterialCommunityIcons') };
});

vi.mock('react-native-safe-area-context', () => ({
    useSafeAreaInsets: () => ({ top: 47, bottom: 34, left: 0, right: 0 }),
}));

vi.mock('react-native-keyboard-controller', async () => {
    const ReactModule = await import('react');
    const host = (name: string) => (props: { children?: React.ReactNode }) => ReactModule.createElement(name, props);
    return { KeyboardAvoidingView: host('KeyboardAvoidingView'), KeyboardStickyView: host('KeyboardStickyView') };
});

vi.mock('expo-constants', () => ({ default: { statusBarHeight: 20, expoConfig: {} } }));

vi.mock('expo-crypto', () => ({ randomUUID: () => 'spawn-request-uuid' }));

// Real product/session logic (node-safe, alias-free dependency graphs):
vi.mock('@/utils/productSessionEntry', async () => {
    const actual = await vi.importActual<typeof import('../../../utils/productSessionEntry')>(
        '../../../utils/productSessionEntry',
    );
    // Spy on the creation seam itself: the production `/new` success path must
    // route through `completeProductSessionCreation`, not an inline copy.
    mocks.completeCreation.mockImplementation((input: Parameters<typeof actual.completeProductSessionCreation>[0]) => (
        actual.completeProductSessionCreation(input)
    ));
    return { ...actual, completeProductSessionCreation: mocks.completeCreation };
});
vi.mock('@/hooks/useNavigateToSession', async () => await import('../../../hooks/useNavigateToSession'));
vi.mock('@/utils/machineUtils', async () => await import('../../../utils/machineUtils'));
vi.mock('@/utils/newSessionModeSelection', async () => await import('../../../utils/newSessionModeSelection'));
vi.mock('@/utils/newSessionSidebarLayout', async () => await import('../../../utils/newSessionSidebarLayout'));
vi.mock('@/utils/newSessionPickerItems', async () => await import('../../../utils/newSessionPickerItems'));
vi.mock('@/utils/newSessionPickerInteraction', async () => await import('../../../utils/newSessionPickerInteraction'));
vi.mock('@/utils/time', async () => await import('../../../utils/time'));
vi.mock('@/sync/spawnRequestId', async () => await import('../../../sync/spawnRequestId'));

// Storage/sync/ops boundaries:
vi.mock('@/sync/storage', () => ({
    useAllMachines: () => mocks.machines,
    useSessions: () => mocks.sessionsList,
    useSetting: (key: string) => mocks.settings[key],
    useLocalSetting: (key: string) => mocks.localSettings[key],
    storage: { getState: () => mocks.storageState },
}));
vi.mock('@/sync/sync', () => ({
    sync: {
        refreshSessions: mocks.refreshSessions,
        sendMessage: mocks.sendMessage,
        preloadSession: vi.fn(),
    },
}));
vi.mock('@/sync/ops', () => ({
    machineSpawnNewSession: mocks.machineSpawnNewSession,
    sessionSetAgentModes: mocks.sessionSetAgentModes,
}));
vi.mock('@/sync/agentDefaults', () => ({
    getCodeAgentDefaults: () => ({ permissionMode: 'auto', modelMode: 'default', effortLevel: null }),
    resolveAgentDefaultConfig: (
        overrides: Record<string, unknown>,
        agentType: string,
    ) => overrides[agentType] ?? { permissionMode: 'auto', modelMode: 'default', effortLevel: null },
}));
vi.mock('@/components/modelModeOptions', () => ({
    filterPermissionModesForCli: (modes: Array<{ key: string }>) => modes,
    getHardcodedPermissionModes: () => [
        { key: 'auto', name: 'Auto' },
        { key: 'default', name: 'Default' },
    ],
    getHardcodedModelModes: () => [
        { key: 'default', name: 'Default' },
        { key: 'opus', name: 'Opus' },
    ],
    getEffortLevelsForModel: () => [{ key: 'medium', name: 'Medium' }],
    getSupportsWorktree: (agentType: string) => agentType !== 'openclaw',
    includeConfiguredModel: (
        _flavor: string,
        models: Array<{ key: string; name: string }>,
    ) => models,
}));

// Machine pairing: a faithful in-memory twin of `sync/machineChoices` for the
// single-CLI-machine shape these tests mount with.
vi.mock('@/sync/machineChoices', () => ({
    getMachineName: (machine: { metadata?: { displayName?: string; host?: string } }) => (
        machine?.metadata?.displayName || machine?.metadata?.host || 'Unknown machine'
    ),
    collectMachineChoices: (machines: Array<Record<string, any>>) => machines.map((machine) => {
        const rig = machine.metadata?.machineKind === 'rig';
        return {
            id: machine.id,
            name: machine.metadata?.displayName || machine.metadata?.host || 'Unknown machine',
            machineIds: [machine.id],
            happyMachine: rig ? null : machine,
            rigMachine: rig ? machine : null,
            online: machine.active === true,
            activeAt: machine.activeAt ?? 0,
        };
    }),
    findMachineChoice: (
        choices: Array<{ machineIds: string[] }>,
        machineId: string | null | undefined,
    ) => (machineId ? choices.find((choice) => choice.machineIds.includes(machineId)) ?? null : null),
    machineChoiceAgentAvailable: (
        choice: { happyMachine: any; rigMachine: any } | null,
        agent: string,
    ) => {
        if (!choice) return false;
        if (agent === 'rig') return choice.rigMachine !== null;
        const happy = choice.happyMachine;
        if (!happy) return false;
        const availability = happy.metadata?.cliAvailability;
        return !availability || availability[agent] !== false;
    },
    machineChoiceAgentVisible: (
        choice: { happyMachine: any; rigMachine: any } | null,
        agent: string,
    ) => (agent !== 'agy' && agent !== 'rig') || (choice !== null && (
        agent === 'rig' ? choice.rigMachine !== null : true
    )),
    resolveChoiceAgent: (
        _choice: unknown,
        agent: string,
    ) => agent,
    resolveAgentMachine: (
        choice: { happyMachine: any; rigMachine: any } | null,
        agent: string,
    ) => (choice ? (agent === 'rig' ? choice.rigMachine : choice.happyMachine) : null),
    resolveWorktreeCreationMachine: (
        choice: { happyMachine: any; rigMachine: any } | null,
        agent: string,
        agentSupportsWorktrees: boolean,
    ) => {
        if (!choice) return null;
        if (agent === 'rig' && choice.happyMachine?.active) return choice.happyMachine;
        if (!agentSupportsWorktrees) return null;
        const machine = agent === 'rig' ? choice.rigMachine : choice.happyMachine;
        return machine?.active ? machine : null;
    },
}));

vi.mock('@/sync/agentSessionPlaces', () => ({
    collectSessionPlaces: () => [],
    collectSessionWorkspaces: () => [],
}));
vi.mock('@/sync/rigSessionCreation', () => ({
    buildRigSpawnConfiguration: () => ({}),
    getRigMachineSessionCreation: () => null,
    resolveRigPendingRetryDelayMs: (published: number | null, fallback: number | null) => (
        published ?? fallback ?? 250
    ),
    isRigMachine: () => false,
    findConnectedRigMachine: () => null,
}));
vi.mock('@/sync/happyAgentSpawn', () => ({ resolveHappyAgentSpawnTarget: () => null }));

vi.mock('@/hooks/useNewSessionDraft', () => ({ useNewSessionDraft: mocks.draftStore }));
vi.mock('@/hooks/useWorktrees', () => ({
    useWorktrees: () => ({ worktrees: [], refresh: () => undefined }),
}));
vi.mock('@/track', () => ({ trackSessionSwitched: () => undefined }));
vi.mock('@/utils/perfLog', () => ({ perfMark: () => undefined }));
vi.mock('@/utils/platform', () => ({ isRunningOnMac: () => false }));
vi.mock('@/utils/responsive', () => ({ useHeaderHeight: () => 64 }));
vi.mock('@/utils/sessionUtils', () => ({
    formatPathRelativeToHome: (path: string | null | undefined) => path ?? '~',
    formatLastSeen: () => 'just now',
}));
vi.mock('@/utils/pathUtils', () => ({
    resolveAbsolutePath: (path: string, homeDir?: string) => (
        path?.startsWith('~') ? `${homeDir ?? '/home'}${path.slice(1)}` : (path ?? '')
    ),
}));
vi.mock('@/utils/worktree', () => ({
    createWorktree: vi.fn(async () => ({ success: false, error: 'not available in tests' })),
}));

vi.mock('@/modal', () => ({
    Modal: { alert: mocks.alert, confirm: mocks.confirm },
}));
vi.mock('@/text', () => ({ t: (key: string) => key }));
vi.mock('@/constants/Typography', () => ({
    Typography: { default: () => ({ fontSize: 15, lineHeight: 20 }) },
}));
vi.mock('@/components/layout', () => ({ layout: { maxWidth: 720, gap: 16 } }));
vi.mock('@/components/glassInteractionPolicy', async () => await import('../../../components/glassInteractionPolicy'));

vi.mock('@/components/MultiTextInput', async () => {
    const ReactModule = await import('react');
    return {
        MULTI_TEXT_INPUT_LINE_HEIGHT: 20,
        MultiTextInput: ReactModule.forwardRef(function MultiTextInput(props: Record<string, unknown>) {
            return ReactModule.createElement('MultiTextInput', props);
        }),
    };
});
vi.mock('@/components/MobileGlass', async () => {
    const ReactModule = await import('react');
    return {
        MobileGlassSurface: (props: { children?: React.ReactNode }) => ReactModule.createElement('MobileGlassSurface', props),
    };
});
vi.mock('@/components/BubblePressable', async () => {
    const ReactModule = await import('react');
    return {
        BubblePressable: (props: { children?: React.ReactNode }) => ReactModule.createElement('BubblePressable', props),
    };
});
vi.mock('@/components/navigation/Header', async () => {
    const ReactModule = await import('react');
    return {
        Header: (props: { children?: React.ReactNode }) => ReactModule.createElement('Header', props),
    };
});
vi.mock('@/components/navigation/headerMetrics', () => ({ MOBILE_GLASS_HEADER_HEIGHT: 56 }));
vi.mock('@/components/AnimatedOverlay', async () => {
    const ReactModule = await import('react');
    const host = (name: string) => (props: { children?: React.ReactNode }) => ReactModule.createElement(name, props);
    return {
        AnimatedClickAwayBackdrop: host('AnimatedClickAwayBackdrop'),
        AnimatedPopup: host('AnimatedPopup'),
        LocalBlurHalo: host('LocalBlurHalo'),
    };
});

// Imported dynamically so the asset-require interception above is installed
// before the route module evaluates its `agentIcons` map.
const { NewSessionScreen } = await import('./index');

function onlineMachine() {
    return {
        id: 'machine-1',
        active: true,
        activeAt: 1_700_000_000_000,
        metadata: { homeDir: '/Users/dev', displayName: 'Dev Laptop' },
    };
}

async function mountScreen() {
    let renderer!: { root: any; unmount: () => unknown };
    await act(async () => {
        renderer = create(React.createElement(NewSessionScreen)) as unknown as typeof renderer;
    });
    return renderer;
}

async function pressSend(renderer: { root: any }) {
    const sendButton = renderer.root.findByProps({ accessibilityLabel: 'Send' });
    await act(async () => {
        await sendButton.props.onPress();
    });
}

beforeEach(() => {
    vi.clearAllMocks();
    mocks.events.length = 0;
    for (const key of Object.keys(mocks.sessionRecords)) delete mocks.sessionRecords[key];
    mocks.resetDraft();
    mocks.machines.length = 0;
    mocks.machines.push(onlineMachine());
    mocks.settings.agentInputEnterToSend = true;
    mocks.settings.agentDefaultOverrides = {};
    mocks.settings.fileDiffsSidebar = false;
    mocks.refreshSessions.mockImplementation(async () => {
        mocks.events.push('refreshSessions');
    });
    mocks.routerPush.mockImplementation((href: string) => {
        mocks.events.push('router.push');
        return href;
    });
    mocks.machineSpawnNewSession.mockResolvedValue({ type: 'success', sessionId: 'session-1' });
});

describe('mounted NewSessionScreen production route', () => {
    it('completes a successful spawn through refresh, durable read, and the product router destination', async () => {
        mocks.sessionRecords['session-1'] = {
            id: 'session-1',
            metadata: { productSession: productSessionMetadata },
        };

        const renderer = await mountScreen();
        await pressSend(renderer);

        // The spawn request left the mounted screen with the resolved machine/agent.
        expect(mocks.machineSpawnNewSession).toHaveBeenCalledTimes(1);
        expect(mocks.machineSpawnNewSession).toHaveBeenCalledWith(expect.objectContaining({
            machineId: 'machine-1',
            agent: 'claude',
            directory: '/Users/dev',
        }));

        // The production success path runs the shared creation seam, not an
        // inline copy of it.
        expect(mocks.completeCreation).toHaveBeenCalledTimes(1);
        expect(mocks.completeCreation).toHaveBeenCalledWith(expect.objectContaining({
            sessionId: 'session-1',
        }));

        // Order: refresh -> durable session read -> product router push.
        const refreshedAt = mocks.events.indexOf('refreshSessions');
        const readAt = mocks.events.indexOf('storage.sessions-read');
        const pushedAt = mocks.events.indexOf('router.push');
        expect(refreshedAt).toBeGreaterThanOrEqual(0);
        expect(readAt).toBeGreaterThan(refreshedAt);
        expect(pushedAt).toBeGreaterThan(readAt);

        // The actual product router destination carries the complete durable
        // product metadata for the created session.
        expect(mocks.routerPush).toHaveBeenCalledTimes(1);
        expect(mocks.routerPush).toHaveBeenCalledWith(
            '/session/session-1?spaceId=space-1&agentId=agent-1&targetId=target-1'
            + '&workspaceRef=workspace-1&resourceUserId=user-1&resourceTenantId=tenant-1'
            + '&resourceSessionId=session-1&runId=run-1',
        );

        // Production side effects between refresh and navigation still run.
        expect(mocks.sessionSetAgentModes).toHaveBeenCalledWith('session-1', {
            permissionMode: 'auto',
            modelMode: 'default',
            effortLevel: 'medium',
        });
        expect(mocks.sendMessage).toHaveBeenCalledWith('session-1', 'Plan the launch', {
            source: 'new_session',
            attachments: [],
        });
        expect(mocks.alert).not.toHaveBeenCalled();
        expect(mocks.draftState.input).toBe('');

        await act(async () => {
            renderer.unmount();
        });
    });

    it('fails closed without navigating and shows the production unavailable alert when durable metadata is missing', async () => {
        // The spawned session persisted without product metadata.
        mocks.sessionRecords['session-2'] = { id: 'session-2', metadata: {} };
        mocks.machineSpawnNewSession.mockResolvedValue({ type: 'success', sessionId: 'session-2' });

        const renderer = await mountScreen();
        await pressSend(renderer);

        expect(mocks.completeCreation).toHaveBeenCalledWith(expect.objectContaining({
            sessionId: 'session-2',
        }));
        expect(mocks.events).toContain('refreshSessions');
        expect(mocks.events).toContain('storage.sessions-read');

        // No navigation happened, and the production unavailable UI was reached.
        expect(mocks.routerPush).not.toHaveBeenCalled();
        expect(mocks.alert).toHaveBeenCalledTimes(1);
        expect(mocks.alert).toHaveBeenCalledWith(
            'common.error',
            'The new product session is not ready yet. Please retry after synchronization.',
        );

        await act(async () => {
            renderer.unmount();
        });
    });
});
