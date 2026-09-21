import { useEffect, useRef, useState } from 'react';
import type { AuthSession, InvitationLookup, WeKnoraClient } from '@weknora/api-client';
import { validateLogin, validateRegister, type FieldErrors } from './validation.ts';
import { landingModeForInvite, storePendingInviteToken } from './invite-flow.ts';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';
import { Input } from '@weknora/ui';

const LANGUAGE_OPTIONS: { value: Locale; label: string; shortLabel: string; flag: string }[] = [
  { value: 'zh-CN', label: '简体中文', shortLabel: '中文', flag: '🇨🇳' },
  { value: 'en-US', label: 'English', shortLabel: 'EN', flag: '🇺🇸' },
  { value: 'ru-RU', label: 'Русский', shortLabel: 'RU', flag: '🇷🇺' },
  { value: 'ko-KR', label: '한국어', shortLabel: '한국어', flag: '🇰🇷' },
  { value: 'ja-JP', label: '日本語', shortLabel: '日本語', flag: '🇯🇵' },
];
import weknoraLogo from './assets/weknora.png';
import screenshot1 from './assets/screenshot-1.svg';
import screenshot2 from './assets/screenshot-2.svg';
import screenshot3 from './assets/screenshot-3.svg';
import screenshot4 from './assets/screenshot-4.svg';
import './auth.css'; // 仅保留 nodePulse / lineFlow @keyframes（复杂动画按约定保留 CSS）

export interface LoginPageProps {
  client: WeKnoraClient;
  onAuthenticated?: (session: AuthSession) => void;
  apiBaseUrl?: string;
  initialError?: string;
  initialMode?: 'login' | 'register';
  /** Share-link token from ?token= (Vue Login.vue:773-810). */
  inviteToken?: string;
  /** Called after an already-authenticated user redeems the invite token. */
  onInviteAccepted?: (result: { ok: boolean }) => void;
}

// Vue slides array (Login.vue:385-406)
const SLIDES = [
  { image: screenshot4, titleKey: 'platform.carousel.agenticRagTitle' },
  { image: screenshot2, titleKey: 'platform.carousel.hybridSearchTitle' },
  { image: screenshot3, titleKey: 'platform.carousel.wikiTitle' },
  { image: screenshot1, titleKey: 'platform.carousel.smartDocRetrievalTitle' },
] as const;

const LOCALE_STORAGE_KEY = 'locale';

// 装饰背景节点/连线的定位与延迟（原 auth.css .node-1..12 / .line-1..12）
const AUTH_NODE_PLACEMENT = [
  'top-[15%] left-[20%]', 'top-[25%] left-[35%]', 'top-[20%] left-[55%]', 'top-[30%] left-[75%]',
  'top-[45%] left-[25%]', 'top-[50%] left-[45%]', 'top-[48%] left-[65%]', 'top-[60%] left-[20%]',
  'top-[12%] right-[15%]', 'top-[38%] right-[10%]', 'top-[70%] left-[40%]', 'top-[65%] left-[80%]',
] as const;
const AUTH_NODE_DELAYS = ['0s', '.5s', '1s', '1.5s', '2s', '2.5s', '3s', '.3s', '1.8s', '2.3s', '.8s', '1.3s'] as const;
const AUTH_LINE_DELAYS = ['0s', '.5s', '1s', '.3s', '.8s', '1.3s', '1.8s', '2.3s', '.2s', '.7s', '.9s', '1.5s'] as const;

function readInitialLocale(): Locale {
  const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY);
  return stored && isLocale(stored) ? stored : 'zh-CN';
}

export function LoginPage({ client, onAuthenticated, apiBaseUrl, initialError, initialMode = 'login', inviteToken = '', onInviteAccepted }: LoginPageProps) {
  const [locale, setLocale] = useState<Locale>(readInitialLocale);
  const t = (key: string, params?: Record<string, string | number>): string => formatMessage(locale, key, params);
  // Vue Login.vue:415 — isRegisterMode always starts false. The only Vue path
  // that loads straight into the register card is a share-link (?token=) in an
  // open deployment (Login.vue:803-808); a bare /register must show the login
  // card, so an initialMode of 'register' is only honoured with a token.
  const [mode, setMode] = useState<'login' | 'register'>(initialMode === 'register' && inviteToken ? 'register' : 'login');
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showLoginPassword, setShowLoginPassword] = useState(false);
  const [confirmPassword, setConfirmPassword] = useState('');
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [state, setState] = useState<'idle' | 'loading' | 'error' | 'success'>(initialError ? 'error' : 'idle');
  const [message, setMessage] = useState(initialError ?? '');
  // Vue Login.vue presents auth outcomes with top-center MessagePlugin toasts
  // (login failure Login.vue:695-697, register success Login.vue:727/742);
  // failures must not render as an inline banner inside the form card.
  const [toast, setToast] = useState<{ tone: 'error' | 'success'; text: string } | null>(null);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const showToast = (tone: 'error' | 'success', text: string) => {
    setToast({ tone, text });
    if (toastTimer.current) clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToast(null), 3000);
  };
  useEffect(() => () => { if (toastTimer.current) clearTimeout(toastTimer.current); }, []);
  const [registrationMode, setRegistrationMode] = useState('self_serve');
  const [registrationLoaded, setRegistrationLoaded] = useState(false);
  const [complexPasswordEnabled, setComplexPasswordEnabled] = useState(false);
  const [oidcEnabled, setOIDCEnabled] = useState(false);
  const [oidcProvider, setOIDCProvider] = useState('');
  const [oidcLoading, setOIDCLoading] = useState(false);
  const [invite, setInvite] = useState<InvitationLookup | null>(null);
  const [inviteError, setInviteError] = useState('');
  const [showLanguageMenu, setShowLanguageMenu] = useState(false);
  const [slideIndex, setSlideIndex] = useState(0);
  const slideTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  // Vue Swiper autoplay { delay: 4000, effect: fade }
  useEffect(() => {
    slideTimer.current = setInterval(() => setSlideIndex((index) => (index + 1) % SLIDES.length), 4000);
    return () => { if (slideTimer.current) clearInterval(slideTimer.current); };
  }, []);

  // Parked auth errors (OIDC bridge etc.) toast once on mount, like the
  // MessagePlugin.error calls in Vue Login.vue:636-647.
  useEffect(() => {
    if (initialError) showToast('error', initialError);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    let active = true;
    void Promise.allSettled([client.auth.registrationConfig(), client.auth.oidcConfig()]).then(([registration, oidc]) => {
      if (!active) return;
      if (registration.status === 'fulfilled') {
        setRegistrationMode(registration.value.registrationMode);
        setComplexPasswordEnabled(registration.value.complexPasswordEnabled);
      }
      setRegistrationLoaded(true);
      if (oidc.status === 'fulfilled') {
        setOIDCEnabled(oidc.value.enabled);
        if (oidc.value.providerDisplayName) setOIDCProvider(oidc.value.providerDisplayName);
      }
    });
    return () => { active = false; };
  }, [client]);

  // Vue Login.vue:773-795 — resolve the invite token before any other flow.
  useEffect(() => {
    if (!inviteToken) return;
    let active = true;
    void client.auth.lookupInvitation(inviteToken)
      .then((lookup) => { if (active) setInvite(lookup); })
      .catch((error: unknown) => { if (active) setInviteError(error instanceof Error ? error.message : t('inviteRegister.invalidBody')); });
    return () => { active = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, inviteToken]);

  // Vue handleClickOutside (Login.vue:531-536)
  useEffect(() => {
    if (!showLanguageMenu) return;
    const close = (event: MouseEvent) => {
      if (!(event.target as HTMLElement).closest('.language-switch')) setShowLanguageMenu(false);
    };
    document.addEventListener('click', close);
    return () => document.removeEventListener('click', close);
  }, [showLanguageMenu]);

  function selectLanguage(next: Locale) {
    setLocale(next);
    window.localStorage.setItem(LOCALE_STORAGE_KEY, next);
    setShowLanguageMenu(false);
    // Vue Login.vue:522-528 — persisting the language confirms it with a
    // MessagePlugin.success(language.languageSaved) toast rendered in the NEW
    // locale (vue-i18n's t is reactive to locale.value, which was just set).
    showToast('success', formatMessage(next, 'language.languageSaved'));
  }

  async function redeemInviteAndEnter(token: string) {
    try {
      await client.auth.acceptInvitationByToken(token);
      onInviteAccepted?.({ ok: true });
    } catch {
      onInviteAccepted?.({ ok: false });
    }
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage('');
    if (mode === 'register') {
      const errors = validateRegister({ username, email, password, confirmPassword }, complexPasswordEnabled);
      setFieldErrors(errors);
      if (Object.keys(errors).length) { setState('error'); return; }
    } else {
      const errors = validateLogin(email, password);
      setFieldErrors(errors);
      if (Object.keys(errors).length) { setState('error'); return; }
    }
    setState('loading');
    try {
      if (mode === 'register' && inviteToken) {
        // Vue Login.vue:716-733 — register-by-invite returns a full session.
        const session = await client.auth.registerByInvite({ token: inviteToken, username, email, password });
        onAuthenticated?.(session);
        setState('idle');
        return;
      }
      if (mode === 'register') {
        await client.auth.register({ username, email, password });
        // Vue Login.vue:744-746 — switch to login and prefill the email.
        setMode('login');
        setUsername(''); setPassword(''); setConfirmPassword('');
        setState('success');
        setMessage(t('auth.registerSuccess'));
        // Vue Login.vue:727/742 — register success is a top toast.
        showToast('success', t('auth.registerSuccess'));
      } else {
        const session = await client.auth.login({ email, password });
        if (inviteToken) {
          // Vue Login.vue:686-691 — persist, redeem token, then enter.
          onAuthenticated?.(session);
          await redeemInviteAndEnter(inviteToken);
          return;
        }
        onAuthenticated?.(session);
        setState('idle');
      }
    } catch (error) {
      setState('error');
      // Vue Login.vue:695-697 — toast the backend message; the localized
      // fallback is auth.loginError (register: auth.registerFailed).
      const text = error instanceof Error ? error.message : mode === 'register' ? t('auth.registerFailed') : t('auth.loginError');
      setMessage(text);
      showToast('error', text);
    }
  }

  async function startOIDC() {
    setOIDCLoading(true);
    setState('idle');
    setMessage('');
    try {
      const base = apiBaseUrl || window.location.origin;
      const redirectURI = new URL('/api/v1/auth/oidc/callback', base).toString();
      const result = await client.auth.oidcUrl(redirectURI);
      // Vue Login.vue:640-643 — the IdP redirect loses the ?token=, park it.
      if (inviteToken) storePendingInviteToken(window.sessionStorage, inviteToken);
      window.location.assign(result.authorizationUrl);
    } catch (error) {
      setState('error');
      // Vue Login.vue:636/647 — the OIDC-entry fallback is auth.oidcLoginFailed
      // (not the generic login-retry copy); a backend message still wins.
      const text = error instanceof Error ? error.message : t('auth.oidcLoginFailed');
      setMessage(text);
      showToast('error', text);
    } finally {
      setOIDCLoading(false);
    }
  }

  const loading = state === 'loading';
  const registrationEnabled = !registrationLoaded || registrationMode !== 'invite_only';
  const inviteMode = inviteToken && invite ? landingModeForInvite(registrationMode) : mode;
  const isRegister = inviteMode === 'register';
  const currentLang = LANGUAGE_OPTIONS.find((option) => option.value === locale) ?? LANGUAGE_OPTIONS[0];
  const fieldError = (field: string) => (fieldErrors[field] ?? []).map((key) => (
    <span key={key} className="text-xs text-[#d54941]">{t(key)}</span>
  ));
  const fieldErrorId = (field: string) => `auth-${field}-error`;
  const hasFieldError = (field: string) => Boolean(fieldErrors[field]?.length);

  // Animated background nodes/lines (Vue Login.vue:3-96)
  const nodeIcons = [
    <g key="a"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" /><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" /></g>,
    <path key="b" d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />,
    <g key="c"><path d="M12 2L2 7l10 5 10-5-10-5z" /><path d="M2 17l10 5 10-5" /><path d="M2 12l10 5 10-5" /></g>,
    <g key="d"><ellipse cx="12" cy="5" rx="9" ry="3" /><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3" /><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5" /></g>,
    <g key="e"><circle cx="11" cy="11" r="8" /><path d="m21 21-4.35-4.35" /></g>,
    <g key="f"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" /><polyline points="3.27 6.96 12 12.01 20.73 6.96" /><line x1="12" y1="22.08" x2="12" y2="12" /></g>,
    <g key="g"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /></g>,
    <g key="h"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M23 21v-2a4 4 0 0 0-3-3.87" /><path d="M16 3.13a4 4 0 0 1 0 7.75" /></g>,
    <path key="i" d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />,
    <g key="j"><circle cx="12" cy="12" r="3" /><path d="M12 1v6m0 6v6M5.64 5.64l4.24 4.24m4.24 4.24l4.24 4.24M1 12h6m6 0h6M5.64 18.36l4.24-4.24m4.24-4.24l4.24-4.24" /></g>,
    <g key="k"><path d="M9 11l3 3L22 4" /><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" /></g>,
    <polygon key="l" points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />,
  ];
  const lines: [number, number, number, number][] = [
    [20, 15, 35, 25], [35, 25, 55, 20], [55, 20, 85, 12], [8, 35, 25, 45], [25, 45, 65, 48], [20, 60, 60, 75],
    [20, 15, 20, 60], [55, 20, 45, 50], [65, 48, 90, 38], [40, 70, 75, 80], [35, 25, 25, 45], [75, 30, 65, 48],
  ];

  return <div className="[--auth-brand:#07C05F] [--auth-text-anti:#ffffff] [--auth-font:-apple-system,'system-ui','Segoe_UI',Roboto,'Helvetica_Neue','PingFang_SC','Hiragino_Sans_GB','Microsoft_YaHei',sans-serif] relative flex w-full min-h-screen overflow-hidden bg-[linear-gradient(225deg,#022c22_0%,#064e3b_15%,#065f46_25%,#047857_38%,#059669_50%,#07C05F_65%,#10B981_78%,#34D399_90%,#6EE7B7_100%)] [font-family:var(--auth-font)] before:pointer-events-none before:absolute before:inset-0 before:content-[''] before:bg-[radial-gradient(circle_at_20%_50%,rgba(255,255,255,0.06)_0%,transparent_50%),radial-gradient(circle_at_80%_50%,rgba(255,255,255,0.04)_0%,transparent_50%)]">
      {toast ? <div data-testid="auth-toast" role="alert" className="fixed left-1/2 top-8 z-[2500] flex -translate-x-1/2 items-center gap-2 whitespace-nowrap rounded-md bg-white px-[18px] py-2.5 text-sm shadow-[0_6px_16px_rgba(0,0,0,0.08),0_0_0_1px_rgba(0,0,0,0.04)]"><span className={toast.tone === 'error' ? 'text-[#d54941]' : 'text-[#0a8f4c]'}>{toast.text}</span></div> : null}
    <div className="pointer-events-none absolute inset-0 z-[1] overflow-hidden" aria-hidden="true">
      {nodeIcons.map((icon, index) => (
        <div key={index} className={`absolute flex h-10 w-10 items-center justify-center rounded-full border-2 border-[rgba(255,255,255,0.3)] bg-[rgba(255,255,255,0.15)] shadow-[0_0_15px_rgba(255,255,255,0.35),0_0_30px_rgba(16,185,129,0.2),inset_0_0_8px_rgba(255,255,255,0.1)] will-change-[transform,opacity] animate-[nodePulse_5s_infinite_ease-in-out] motion-reduce:animate-none motion-reduce:opacity-65 ${AUTH_NODE_PLACEMENT[index]} [animation-delay:${AUTH_NODE_DELAYS[index]}]`}>
          <svg className="h-5 w-5 text-[rgba(255,255,255,0.9)]" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">{icon}</svg>
        </div>
      ))}
      <svg className="absolute inset-0 h-full w-full opacity-[0.35]" viewBox="0 0 100 100" preserveAspectRatio="none">
        {lines.map(([x1, y1, x2, y2], index) => (
          <line key={index} className={`animate-[lineFlow_10s_infinite_linear] motion-reduce:animate-none [stroke:rgba(255,255,255,0.5)] [stroke-width:1.5] [stroke-dasharray:6_3] [stroke-linecap:round] [animation-delay:${AUTH_LINE_DELAYS[index]}]`} x1={x1} y1={y1} x2={x2} y2={y2} />
        ))}
      </svg>
    </div>

    <a href="https://github.com/Tencent/WeKnora" target="_blank" rel="noreferrer" className="fixed left-[50px] top-8 z-[100] cursor-pointer max-[768px]:left-6 max-[768px]:top-5" title={t('common.github')}>
      <img src={weknoraLogo} alt="WeKnora" className="h-auto w-[120px] max-[768px]:w-[90px]" />
    </a>

    <div className="fixed right-7 top-7 z-[100] flex items-center gap-2.5 max-[768px]:right-4 max-[768px]:top-4">
      <a href="https://weknora.weixin.qq.com" target="_blank" rel="noreferrer" className="relative flex cursor-pointer items-center gap-[7px] rounded-[20px] border border-[rgba(255,255,255,0.25)] bg-[rgba(255,255,255,0.2)] px-[15px] py-[9px] text-[13px] font-semibold tracking-[0.2px] text-(--auth-text-anti) no-underline hover:border-[rgba(255,255,255,0.4)] hover:bg-[rgba(255,255,255,0.3)] [&_svg]:shrink-0" title={t('common.website')}>
        <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round"><circle cx="12" cy="12" r="10" /><line x1="2" y1="12" x2="22" y2="12" /><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" /></svg>
        <span className="link-text">{t('common.website')}</span>
      </a>
      <a href="https://github.com/Tencent/WeKnora" target="_blank" rel="noreferrer" className="relative flex cursor-pointer items-center gap-[7px] rounded-[20px] border border-[rgba(255,255,255,0.25)] bg-[rgba(255,255,255,0.2)] px-[15px] py-[9px] text-[13px] font-semibold tracking-[0.2px] text-(--auth-text-anti) no-underline hover:border-[rgba(255,255,255,0.4)] hover:bg-[rgba(255,255,255,0.3)] [&_svg]:shrink-0" title={t('common.info')}>
        <svg width="17" height="17" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z" /></svg>
        <span className="link-text">GitHub</span>
      </a>
      <div className="language-switch relative">
        <button type="button" className="auth-language-control relative flex cursor-pointer items-center gap-[7px] rounded-[20px] border border-[rgba(255,255,255,0.25)] bg-[rgba(255,255,255,0.2)] px-[15px] py-[9px] text-[13px] font-semibold tracking-[0.2px] text-(--auth-text-anti) [font-family:var(--auth-font)] no-underline hover:border-[rgba(255,255,255,0.4)] hover:bg-[rgba(255,255,255,0.3)] [&_svg]:shrink-0" title={currentLang.label} aria-haspopup="menu" aria-expanded={showLanguageMenu} onKeyDown={(event) => { if (event.key === 'Escape') setShowLanguageMenu(false); }} onClick={() => setShowLanguageMenu((visible) => !visible)}>
          <span className="shrink-0 text-base leading-none">{currentLang.flag}</span>
          <span className="link-text">{currentLang.shortLabel}</span>
          <svg className="ml-0.5 shrink-0" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round"><polyline points="6 9 12 15 18 9" /></svg>
        </button>
        {showLanguageMenu ? <div role="menu" className="absolute right-0 top-[calc(100%+8px)] z-[1000] min-w-[160px] overflow-hidden rounded-lg border border-[#e7e7e7] bg-[rgba(255,255,255,0.97)] shadow-[0_4px_16px_rgba(0,0,0,0.12)]">
          {LANGUAGE_OPTIONS.map((option) => (
            <button key={option.value} type="button" role="menuitem" className={`flex w-full cursor-pointer items-center gap-2.5 border-0 bg-transparent px-[14px] py-2.5 text-left text-[13px] text-[#1a1a1a] hover:bg-[#f3f3f3] ${option.value === locale ? 'bg-[#e3f9e9] text-[#04a04c]' : ''}`} onClick={() => selectLanguage(option.value)}>
              <span className="shrink-0 text-base">{option.flag}</span>
              <span className="flex-1">{option.label}</span>
              {option.value === locale ? <span className="shrink-0 text-sm font-bold text-[#07C05F]">✓</span> : null}
            </button>
          ))}
        </div> : null}
      </div>
    </div>

    <div className="relative box-border flex min-w-[680px] flex-[0_0_52%] items-end py-[100px] pl-[50px] pr-[30px] max-[1024px]:min-w-0 max-[1024px]:flex-[0_0_100%] max-[1024px]:pt-[90px] max-[1024px]:px-6 max-[1024px]:pb-5 max-[768px]:pt-20 max-[768px]:px-4 max-[768px]:pb-2.5">
      <div className="relative z-[2] mb-[60px] flex w-full max-w-[600px] flex-col max-[1024px]:mb-6">
        <p className="m-0 mb-2 text-[22px] font-medium leading-[1.4] text-[rgba(255,255,255,0.95)]">{t('platform.subtitle')}</p>
        <p className="m-0 mb-7 text-[15px] leading-[1.5] text-[rgba(255,255,255,0.8)]">{t('platform.description')}</p>
        <div className="mb-10 flex flex-wrap gap-3">
          <span className="inline-block rounded-[20px] bg-[rgba(255,255,255,0.2)] px-5 py-2 text-sm font-medium text-(--auth-text-anti)">{t('platform.rag')}</span>
          <span className="inline-block rounded-[20px] bg-[rgba(255,255,255,0.2)] px-5 py-2 text-sm font-medium text-(--auth-text-anti)">{t('platform.agent')}</span>
          <span className="inline-block rounded-[20px] bg-[rgba(255,255,255,0.2)] px-5 py-2 text-sm font-medium text-(--auth-text-anti)">{t('platform.wiki')}</span>
          <span className="inline-block rounded-[20px] bg-[rgba(255,255,255,0.2)] px-5 py-2 text-sm font-medium text-(--auth-text-anti)">{t('platform.hybridSearch')}</span>
        </div>
        <div className="mt-12 w-full">
          <div className="relative grid w-full overflow-hidden rounded-2xl pb-10 shadow-[0_20px_60px_rgba(0,0,0,0.3)]">
            {SLIDES.map((slide, index) => (
              <div key={slide.titleKey} className={`col-start-1 row-start-1 flex items-center justify-center bg-white opacity-0 transition-opacity duration-[800ms] ease-in-out ${index === slideIndex ? 'opacity-100' : ''}`}>
                <img src={slide.image} alt={t(slide.titleKey)} className="block h-full w-full object-contain" />
              </div>
            ))}
            <div className="absolute bottom-[15px] left-0 right-0 z-10 flex items-center justify-center">
              {SLIDES.map((slide, index) => (
                <button key={slide.titleKey} type="button" aria-label={t(slide.titleKey)}
                  className={`mx-1.5 h-2.5 w-2.5 cursor-pointer rounded-[5px] border-0 bg-[rgba(255,255,255,0.5)] p-0 opacity-100 transition-all duration-300 ${index === slideIndex ? 'w-7 bg-white' : ''}`}
                  onClick={() => setSlideIndex(index)} />
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>

    <div className="relative box-border flex flex-[0_0_48%] items-end justify-center pt-[112px] pr-[50px] pb-[100px] pl-[30px] max-[1024px]:flex-[0_0_100%] max-[1024px]:pt-2.5 max-[1024px]:px-6 max-[1024px]:pb-[60px] max-[768px]:px-4 max-[768px]:pb-12">
      <div className="relative z-[2] mb-[60px] mt-[19px] w-full max-w-[480px]">
        {invite && !isRegister ? <div className="mb-[18px] flex items-center gap-2.5 rounded-[10px] border border-[#b7e8c9] bg-[#f0fbf4] px-4 py-3">
          <div>
            <div className="text-sm font-semibold text-[#1a1a1a]">{t('inviteRegister.bannerTitle', { tenant: invite.tenantName || '' })}</div>
            <div className="mt-0.5 text-[13px] text-[#666]">{t('inviteRegister.bannerHintLogin')}</div>
          </div>
        </div> : null}
        {invite && isRegister ? <div className="mb-[18px] flex items-center gap-2.5 rounded-[10px] border border-[#b7e8c9] bg-[#f0fbf4] px-4 py-3">
          <div>
            <div className="text-sm font-semibold text-[#1a1a1a]">{t('inviteRegister.bannerTitle', { tenant: invite.tenantName || '' })}</div>
            <div className="mt-0.5 text-[13px] text-[#666]">{t('inviteRegister.bannerHint')}</div>
          </div>
        </div> : null}
        {inviteError ? <div className="mb-[18px] flex items-center gap-2.5 rounded-[10px] border border-[#f5c2c2] bg-[#fdf0f0] px-4 py-3 text-[#d54941]">{inviteError}</div> : null}

        {!isRegister ? <div className="box-border w-full rounded-2xl border-0 bg-[rgba(255,255,255,0.97)] p-10 shadow-[0_10px_40px_rgba(0,0,0,0.15)] max-[768px]:p-5">
          <div className="mb-8 text-center">
            <h2 className="m-0 mb-1.5 text-2xl font-semibold leading-[normal] text-[rgba(0,0,0,0.9)]">{t('auth.login')}</h2>
            <p className="m-0 text-[13px] leading-[18px] text-[rgba(0,0,0,0.7)]">{t('auth.subtitle')}</p>
            {registrationEnabled ? <p className="mt-2.5 mb-0 rounded-lg bg-[#e9fbf0] px-3 py-2 text-[12.5px] leading-[1.5] text-[#07C05F]">{t('auth.loginHint')}</p> : null}
          </div>
          <form className="flex flex-col gap-[31px] pt-[11px]" onSubmit={submit} aria-label="Login form">
            <label className="flex flex-col gap-2">
              <span className="text-sm leading-[17px] font-medium text-[#1a1a1a]"><span style={{ color: '#d54941', marginRight: 4 }}>*</span>{t('auth.email')}</span>
              <div className="auth-input-shell flex h-10 w-full items-center rounded-lg border border-[#dcdcdc] bg-white px-3 transition-colors focus-within:border-(--auth-brand) focus-within:shadow-[0_0_0_3px_rgba(7,192,95,0.1)]">
                <Input id="auth-email" aria-invalid={hasFieldError('email')} aria-describedby={hasFieldError('email') ? fieldErrorId('email') : undefined} className="auth-input box-border h-6 w-full rounded-none border-0 bg-transparent p-0 text-[15px] leading-6 text-[rgba(0,0,0,0.9)] outline-none [font-family:var(--auth-font)] disabled:cursor-not-allowed disabled:bg-transparent" value={email} onChange={(event) => setEmail(event.target.value)} type="text" autoComplete="email" disabled={loading} placeholder={t('auth.emailPlaceholder')} />
              </div>
              {hasFieldError('email') ? <span id={fieldErrorId('email')} role="alert">{fieldError('email')}</span> : null}
            </label>
            <label className="flex flex-col gap-2">
              <span className="text-sm leading-[17px] font-medium text-[#1a1a1a]"><span style={{ color: '#d54941', marginRight: 4 }}>*</span>{t('auth.password')}</span>
              <span className="auth-input-shell relative flex h-10 w-full items-center rounded-lg border border-[#dcdcdc] bg-white px-3 transition-colors focus-within:border-(--auth-brand) focus-within:shadow-[0_0_0_3px_rgba(7,192,95,0.1)]"><Input id="auth-password" aria-invalid={hasFieldError('password')} aria-describedby={hasFieldError('password') ? fieldErrorId('password') : undefined} className="auth-input box-border h-6 w-full rounded-none border-0 bg-transparent p-0 pr-8 text-[15px] leading-6 text-[rgba(0,0,0,0.9)] outline-none [font-family:var(--auth-font)] disabled:cursor-not-allowed disabled:bg-transparent" value={password} onChange={(event) => setPassword(event.target.value)} type={showLoginPassword ? 'text' : 'password'} autoComplete="current-password" disabled={loading} placeholder={t('auth.passwordPlaceholder')} /><button type="button" className="absolute right-3 top-1/2 inline-flex h-6 w-6 -translate-y-1/2 cursor-pointer items-center justify-center border-0 bg-transparent p-0 text-[#87909d]" aria-label={t('auth.password')} aria-pressed={showLoginPassword} onClick={() => setShowLoginPassword((visible) => !visible)}>{/* Vue 密码未显示时是斜杠眼（eye-off），显示后才切换成睁眼 */}
{showLoginPassword ? <svg aria-hidden="true" viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M2.5 12s3.4-5.5 9.5-5.5 9.5 5.5 9.5 5.5-3.4 5.5-9.5 5.5S2.5 12 2.5 12Z" /><circle cx="12" cy="12" r="2.5" /></svg> : <svg aria-hidden="true" viewBox="0 0 24 24" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M2.5 12s3.4-5.5 9.5-5.5 9.5 5.5 9.5 5.5-3.4 5.5-9.5 5.5S2.5 12 2.5 12Z" /><circle cx="12" cy="12" r="2.5" /><line x1="4" y1="20" x2="20" y2="4" /></svg>}</button></span>
              {hasFieldError('password') ? <span id={fieldErrorId('password')} role="alert">{fieldError('password')}</span> : null}
            </label>
            <button type="submit" className="mt-[-10px] h-[46px] cursor-pointer rounded-lg border-0 bg-(--auth-brand) text-base font-semibold text-white [font-family:var(--auth-font)] hover:bg-[#06ad55] disabled:cursor-not-allowed disabled:opacity-60" disabled={loading}>{loading ? t('auth.loggingIn') : t('auth.login')}</button>
            {registrationEnabled ? <div>
              {/* Vue .register-cta__divider (Login.vue:1427-1450): centred text
                  over a full-width rule, then the outline CTA. */}
              <div className="relative mb-[-1px] text-center text-[13px] leading-[22px] text-[rgba(0,0,0,0.7)] before:absolute before:inset-x-0 before:top-1/2 before:border-t before:border-[#e7e7e7] before:content-['']">
                <span className="relative z-[1] bg-[rgba(255,255,255,0.97)] px-3">{t('auth.firstTime')}</span>
              </div>
              <button type="button" className="h-[46px] w-full cursor-pointer rounded-lg border border-(--auth-brand) bg-white text-[15px] text-(--auth-brand) [font-family:var(--auth-font)] hover:bg-[#e9fbf0] hover:border-(--auth-brand) hover:text-(--auth-brand) disabled:cursor-not-allowed disabled:opacity-60" disabled={loading} onClick={() => { setMode('register'); setState('idle'); setMessage(''); setFieldErrors({}); }}>{t('auth.createAccount')}</button>
            </div> : null}
            {oidcEnabled ? <div className="mb-3 mt-4 text-center text-[13px] text-[#999]"><span>{t('auth.orContinueWith')}</span></div> : null}
            {oidcEnabled ? <button type="button" className="h-[46px] w-full cursor-pointer rounded-lg border border-[#dcdcdc] bg-white text-[15px] text-[#1a1a1a] [font-family:var(--auth-font)] hover:border-(--auth-brand) hover:text-(--auth-brand) disabled:cursor-not-allowed disabled:opacity-60" disabled={oidcLoading || loading} onClick={() => void startOIDC()}>{oidcLoading ? t('auth.redirectingToOIDC') : oidcProvider ? t('auth.oidcLoginWithProvider', { provider: oidcProvider }) : t('auth.oidcLogin')}</button> : null}
          </form>
          <div className="mt-5 flex flex-col gap-3">
            <div className="flex items-center gap-2.5 text-[13px] leading-[1.4] text-[rgba(0,0,0,0.7)]"><span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[#e9fbf0] text-xs text-(--auth-brand)">✓</span><span>{t('platform.multimodalParsing')}</span></div>
            <div className="flex items-center gap-2.5 text-[13px] leading-[1.4] text-[rgba(0,0,0,0.7)]"><span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[#e9fbf0] text-xs text-(--auth-brand)">✓</span><span>{t('platform.hybridSearchEngine')}</span></div>
            <div className="flex items-center gap-2.5 text-[13px] leading-[1.4] text-[rgba(0,0,0,0.7)]"><span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[#e9fbf0] text-xs text-(--auth-brand)">✓</span><span>{t('platform.ragQandA')}</span></div>
          </div>
        </div> : null}

        {isRegister && (registrationEnabled || invite) ? <div className="box-border w-full rounded-2xl border-0 bg-[rgba(255,255,255,0.97)] p-10 shadow-[0_10px_40px_rgba(0,0,0,0.15)] max-[768px]:p-5">
          <div className="form-header mb-8 text-center">
            <h2 className="m-0 mb-1.5 text-2xl font-semibold leading-[normal] text-[rgba(0,0,0,0.9)]">{t('auth.createAccount')}</h2>
            <p className="m-0 text-[13px] leading-[18px] text-[rgba(0,0,0,0.7)]">{t('auth.registerSubtitle')}</p>
          </div>
          <form className="flex flex-col gap-[18px] pt-[7px]" onSubmit={submit} aria-label="Register form">
            <label className="flex flex-col gap-2">
              <span className="text-sm leading-[17px] font-medium text-[#1a1a1a]"><span style={{ color: '#d54941', marginRight: 4 }}>*</span>{t('auth.username')}</span>
              <div className="auth-input-shell flex h-10 w-full items-center rounded-lg border border-[#dcdcdc] bg-white px-3 transition-colors focus-within:border-(--auth-brand) focus-within:shadow-[0_0_0_3px_rgba(7,192,95,0.1)]"><Input id="auth-username" aria-invalid={hasFieldError('username')} aria-describedby={hasFieldError('username') ? fieldErrorId('username') : undefined} className="auth-input box-border h-6 w-full rounded-none border-0 bg-transparent p-0 text-[15px] leading-6 text-[rgba(0,0,0,0.9)] outline-none [font-family:var(--auth-font)] disabled:cursor-not-allowed disabled:bg-transparent" value={username} onChange={(event) => setUsername(event.target.value)} disabled={loading} placeholder={t('auth.usernamePlaceholder')} /></div>
              {hasFieldError('username') ? <span id={fieldErrorId('username')} role="alert">{fieldError('username')}</span> : null}
            </label>
            <label className="flex flex-col gap-2">
              <span className="text-sm leading-[17px] font-medium text-[#1a1a1a]"><span style={{ color: '#d54941', marginRight: 4 }}>*</span>{t('auth.email')}</span>
              <div className="auth-input-shell flex h-10 w-full items-center rounded-lg border border-[#dcdcdc] bg-white px-3 transition-colors focus-within:border-(--auth-brand) focus-within:shadow-[0_0_0_3px_rgba(7,192,95,0.1)]"><Input id="auth-register-email" aria-invalid={hasFieldError('email')} aria-describedby={hasFieldError('email') ? fieldErrorId('email') : undefined} className="auth-input box-border h-6 w-full rounded-none border-0 bg-transparent p-0 text-[15px] leading-6 text-[rgba(0,0,0,0.9)] outline-none [font-family:var(--auth-font)] disabled:cursor-not-allowed disabled:bg-transparent" value={email} onChange={(event) => setEmail(event.target.value)} type="text" autoComplete="email" disabled={loading} placeholder={t('auth.emailPlaceholder')} /></div>
              {hasFieldError('email') ? <span id={fieldErrorId('email')} role="alert">{fieldError('email')}</span> : null}
            </label>
            <label className="flex flex-col gap-2">
              <span className="text-sm leading-[17px] font-medium text-[#1a1a1a]"><span style={{ color: '#d54941', marginRight: 4 }}>*</span>{t('auth.password')}</span>
              <div className="auth-input-shell flex h-10 w-full items-center rounded-lg border border-[#dcdcdc] bg-white px-3 transition-colors focus-within:border-(--auth-brand) focus-within:shadow-[0_0_0_3px_rgba(7,192,95,0.1)]"><Input className="auth-input box-border h-6 w-full rounded-none border-0 bg-transparent p-0 text-[15px] leading-6 text-[rgba(0,0,0,0.9)] outline-none [font-family:var(--auth-font)] disabled:cursor-not-allowed disabled:bg-transparent" value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="new-password" disabled={loading} placeholder={t('auth.passwordPlaceholder')} /></div>
              {fieldError('password')}
            </label>
            <label className="flex flex-col gap-2">
              <span className="text-sm leading-[17px] font-medium text-[#1a1a1a]"><span style={{ color: '#d54941', marginRight: 4 }}>*</span>{t('auth.confirmPassword')}</span>
              <div className="auth-input-shell flex h-10 w-full items-center rounded-lg border border-[#dcdcdc] bg-white px-3 transition-colors focus-within:border-(--auth-brand) focus-within:shadow-[0_0_0_3px_rgba(7,192,95,0.1)]"><Input className="auth-input box-border h-6 w-full rounded-none border-0 bg-transparent p-0 text-[15px] leading-6 text-[rgba(0,0,0,0.9)] outline-none [font-family:var(--auth-font)] disabled:cursor-not-allowed disabled:bg-transparent" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} type="password" autoComplete="new-password" disabled={loading} placeholder={t('auth.confirmPasswordPlaceholder')} /></div>
              {fieldError('confirmPassword')}
            </label>
            <button type="submit" className="mt-[19px] h-[46px] cursor-pointer rounded-lg border-0 bg-(--auth-brand) text-base font-semibold text-white [font-family:var(--auth-font)] hover:bg-[#06ad55] disabled:cursor-not-allowed disabled:opacity-60" disabled={loading}>{loading ? t('auth.registering') : t('auth.register')}</button>
            {/* Vue shows the 已有账户？返回登录 footer unconditionally, also on
                the share-link invite register form. Vue .form-footer carries a
                bottom rule (Login.vue:1575-1582) separating the features list. */}
            <div className="mt-4 border-b border-[#e7e7e7] pb-4 text-center text-sm leading-[1.4] text-[rgba(0,0,0,0.7)]">
              <span>{t('auth.haveAccount')}</span>
              <a href="#" className="ml-1 cursor-pointer text-sm font-medium text-(--auth-brand) no-underline hover:underline" onClick={(event) => { event.preventDefault(); setMode('login'); setState('idle'); setMessage(''); setFieldErrors({}); }}>{t('auth.backToLogin')}</a>
            </div>
          </form>
          <div className="mt-5 flex flex-col gap-3">
            <div className="flex items-center gap-2.5 text-[13px] leading-[1.4] text-[rgba(0,0,0,0.7)]"><span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[#e9fbf0] text-xs text-(--auth-brand)">✓</span><span>{t('platform.independentTenant')}</span></div>
            <div className="flex items-center gap-2.5 text-[13px] leading-[1.4] text-[rgba(0,0,0,0.7)]"><span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[#e9fbf0] text-xs text-(--auth-brand)">✓</span><span>{t('platform.fullApiAccess')}</span></div>
            <div className="flex items-center gap-2.5 text-[13px] leading-[1.4] text-[rgba(0,0,0,0.7)]"><span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[#e9fbf0] text-xs text-(--auth-brand)">✓</span><span>{t('platform.knowledgeBaseManagement')}</span></div>
          </div>
        </div> : null}
      </div>
    </div>
  </div>;
}
