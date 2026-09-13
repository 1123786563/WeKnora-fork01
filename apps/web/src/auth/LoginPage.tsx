import { useEffect, useRef, useState } from 'react';
import type { AuthSession, InvitationLookup, WeKnoraClient } from '@weknora/api-client';
import { validateLogin, validateRegister, type FieldErrors } from './validation.ts';
import { landingModeForInvite, storePendingInviteToken } from './invite-flow.ts';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';

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
import './auth.css';

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

function readInitialLocale(): Locale {
  const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY);
  return stored && isLocale(stored) ? stored : 'zh-CN';
}

export function LoginPage({ client, onAuthenticated, apiBaseUrl, initialError, initialMode = 'login', inviteToken = '', onInviteAccepted }: LoginPageProps) {
  const [locale, setLocale] = useState<Locale>(readInitialLocale);
  const t = (key: string, params?: Record<string, string | number>): string => formatMessage(locale, key, params);
  const [mode, setMode] = useState<'login' | 'register'>(initialMode);
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
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
      setMessage(error instanceof Error ? error.message : t('auth.loginErrorRetry'));
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
    <span key={key} className="field-error">{t(key)}</span>
  ));

  // Animated background nodes/lines (Vue Login.vue:3-96)
  const nodeIcons = [
    <g key="a"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" /><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" /></g>,
    <path key="b" d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />,
    <g key="c"><path d="M12 2L2 7l10 5 10-5-10-5z" /><path d="M2 17l10 5 10-5" /><path d="M2 12l10 5 10-5" /></g>,
    <g key="d"><circle cx="11" cy="11" r="8" /><path d="m21 21-4.35-4.35" /></g>,
    <g key="e"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" /><polyline points="3.27 6.96 12 12.01 20.73 6.96" /><line x1="12" y1="22.08" x2="12" y2="12" /></g>,
    <g key="f"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /></g>,
    <g key="g"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M23 21v-2a4 4 0 0 0-3-3.87" /><path d="M16 3.13a4 4 0 0 1 0 7.75" /></g>,
    <path key="i" d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />,
    <g key="i"><circle cx="12" cy="12" r="3" /><path d="M12 1v6m0 6v6M5.64 5.64l4.24 4.24m4.24 4.24l4.24 4.24M1 12h6m6 0h6M5.64 18.36l4.24-4.24m4.24-4.24l4.24-4.24" /></g>,
    <g key="j"><path d="M9 11l3 3L22 4" /><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" /></g>,
    <polygon key="k" points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />,
  ];
  const lines: [number, number, number, number][] = [
    [20, 15, 35, 25], [35, 25, 55, 20], [55, 20, 85, 12], [8, 35, 25, 45], [25, 45, 65, 48], [20, 60, 60, 75],
    [20, 15, 20, 60], [55, 20, 45, 50], [65, 48, 90, 38], [40, 70, 75, 80], [35, 25, 25, 45], [75, 30, 65, 48],
  ];

  return <div className="auth-layout">
      {toast ? <div className={`auth-toast ${toast.tone}`} role="alert">{toast.text}</div> : null}
    <div className="animated-bg" aria-hidden="true">
      {nodeIcons.map((icon, index) => (
        <div key={index} className={`knowledge-node node-${index + 1}`}>
          <svg className="node-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">{icon}</svg>
        </div>
      ))}
      <svg className="knowledge-lines" viewBox="0 0 100 100" preserveAspectRatio="none">
        {lines.map(([x1, y1, x2, y2], index) => (
          <line key={index} className={`connection-line line-${index + 1}`} x1={x1} y1={y1} x2={x2} y2={y2} />
        ))}
      </svg>
    </div>

    <a href="https://github.com/Tencent/WeKnora" target="_blank" rel="noreferrer" className="header-logo" title={t('common.github')}>
      <img src={weknoraLogo} alt="WeKnora" className="logo-image" />
    </a>

    <div className="header-links">
      <a href="https://weknora.weixin.qq.com" target="_blank" rel="noreferrer" className="header-link" title={t('common.website')}>
        <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><circle cx="12" cy="12" r="10" /><line x1="2" y1="12" x2="22" y2="12" /><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" /></svg>
        <span className="link-text">{t('common.website')}</span>
      </a>
      <a href="https://github.com/Tencent/WeKnora" target="_blank" rel="noreferrer" className="header-link" title={t('common.info')}>
        <svg width="17" height="17" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z" /></svg>
        <span className="link-text">GitHub</span>
      </a>
      <div className="language-switch">
        <button type="button" className="header-link" title={currentLang.label} onClick={() => setShowLanguageMenu((visible) => !visible)}>
          <span className="lang-flag-icon">{currentLang.flag}</span>
          <span className="link-text">{currentLang.shortLabel}</span>
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><polyline points="6 9 12 15 18 9" /></svg>
        </button>
        {showLanguageMenu ? <div className="language-dropdown">
          {LANGUAGE_OPTIONS.map((option) => (
            <div key={option.value} className={`language-option${option.value === locale ? ' active' : ''}`} onClick={() => selectLanguage(option.value)}>
              <span className="lang-flag">{option.flag}</span>
              <span className="lang-label">{option.label}</span>
              {option.value === locale ? <span className="check-icon">✓</span> : null}
            </div>
          ))}
        </div> : null}
      </div>
    </div>

    <div className="showcase-section">
      <div className="showcase-content">
        <p className="showcase-subtitle">{t('platform.subtitle')}</p>
        <p className="showcase-description">{t('platform.description')}</p>
        <div className="feature-tags">
          <span className="tag">{t('platform.rag')}</span>
          <span className="tag">{t('platform.agent')}</span>
          <span className="tag">{t('platform.wiki')}</span>
          <span className="tag">{t('platform.hybridSearch')}</span>
        </div>
        <div className="carousel-container">
          <div className="screenshot-swiper">
            {SLIDES.map((slide, index) => (
              <div key={slide.titleKey} className={`slide-content${index === slideIndex ? ' active' : ''}`}>
                <img src={slide.image} alt={t(slide.titleKey)} className="slide-image" />
              </div>
            ))}
            <div className="swiper-pagination">
              {SLIDES.map((slide, index) => (
                <button key={slide.titleKey} type="button" aria-label={t(slide.titleKey)}
                  className={`swiper-pagination-bullet${index === slideIndex ? ' active' : ''}`}
                  onClick={() => setSlideIndex(index)} />
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>

    <div className="form-section">
      <div className="form-panel">
        {invite && !isRegister ? <div className="invite-banner">
          <div>
            <div className="invite-banner__title">{t('inviteRegister.bannerTitle', { tenant: invite.tenantName || '' })}</div>
            <div className="invite-banner__hint">{t('inviteRegister.bannerHintLogin')}</div>
          </div>
        </div> : null}
        {invite && isRegister ? <div className="invite-banner">
          <div>
            <div className="invite-banner__title">{t('inviteRegister.bannerTitle', { tenant: invite.tenantName || '' })}</div>
            <div className="invite-banner__hint">{t('inviteRegister.bannerHint')}</div>
          </div>
        </div> : null}
        {inviteError ? <div className="invite-banner invite-banner--error">{inviteError}</div> : null}

        {!isRegister ? <div className="form-card">
          <div className="form-header">
            <h2 className="form-title">{t('auth.login')}</h2>
            <p className="form-welcome">{t('auth.subtitle')}</p>
            {registrationEnabled ? <p className="form-hint">{t('auth.loginHint')}</p> : null}
          </div>
          <form className="auth-form" onSubmit={submit} aria-label="Login form">
            <label className="auth-field">
              <span><span style={{ color: '#d54941', marginRight: 4 }}>*</span>{t('auth.email')}</span>
              <input value={email} onChange={(event) => setEmail(event.target.value)} type="text" autoComplete="email" disabled={loading} placeholder={t('auth.emailPlaceholder')} />
              {fieldError('email')}
            </label>
            <label className="auth-field">
              <span><span style={{ color: '#d54941', marginRight: 4 }}>*</span>{t('auth.password')}</span>
              <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="current-password" disabled={loading} placeholder={t('auth.passwordPlaceholder')} />
              {fieldError('password')}
            </label>
            <button type="submit" className="submit-button" disabled={loading}>{loading ? t('auth.loggingIn') : t('auth.login')}</button>
            {registrationEnabled ? <div className="register-cta">
              <div className="register-cta__divider"><span>{t('auth.firstTime')}</span></div>
              <button type="button" className="outline-button" disabled={loading} onClick={() => { setMode('register'); setState('idle'); setMessage(''); setFieldErrors({}); }}>{t('auth.createAccount')}</button>
            </div> : null}
            {oidcEnabled ? <div className="oidc-divider"><span>{t('auth.orContinueWith')}</span></div> : null}
            {oidcEnabled ? <button type="button" className="outline-button" disabled={oidcLoading || loading} onClick={() => void startOIDC()}>{oidcLoading ? t('auth.redirectingToOIDC') : oidcProvider ? t('auth.oidcLoginWithProvider', { provider: oidcProvider }) : t('auth.oidcLogin')}</button> : null}
          </form>
          <div className="login-features">
            <div className="feature-item"><span className="feature-icon">✓</span><span>{t('platform.multimodalParsing')}</span></div>
            <div className="feature-item"><span className="feature-icon">✓</span><span>{t('platform.hybridSearchEngine')}</span></div>
            <div className="feature-item"><span className="feature-icon">✓</span><span>{t('platform.ragQandA')}</span></div>
          </div>
        </div> : null}

        {isRegister && (registrationEnabled || invite) ? <div className="form-card">
          <div className="form-header">
            <h2 className="form-title">{t('auth.createAccount')}</h2>
            <p className="form-subtitle">{t('auth.registerSubtitle')}</p>
          </div>
          <form className="auth-form" onSubmit={submit} aria-label="Register form">
            <label className="auth-field">
              <span>{t('auth.username')}</span>
              <input value={username} onChange={(event) => setUsername(event.target.value)} disabled={loading} placeholder={t('auth.usernamePlaceholder')} />
              {fieldError('username')}
            </label>
            <label className="auth-field">
              <span>{t('auth.email')}</span>
              <input value={email} onChange={(event) => setEmail(event.target.value)} type="text" autoComplete="email" disabled={loading} placeholder={t('auth.emailPlaceholder')} />
              {fieldError('email')}
            </label>
            <label className="auth-field">
              <span>{t('auth.password')}</span>
              <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="new-password" disabled={loading} placeholder={t('auth.passwordPlaceholder')} />
              {fieldError('password')}
            </label>
            <label className="auth-field">
              <span>{t('auth.confirmPassword')}</span>
              <input value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} type="password" autoComplete="new-password" disabled={loading} placeholder={t('auth.confirmPasswordPlaceholder')} />
              {fieldError('confirmPassword')}
            </label>
            <button type="submit" className="submit-button" disabled={loading}>{loading ? t('auth.registering') : t('auth.register')}</button>
            {!inviteToken ? <div className="form-footer">
              <span>{t('auth.haveAccount')}</span>
              <button type="button" className="link-button" onClick={() => { setMode('login'); setState('idle'); setMessage(''); setFieldErrors({}); }}>{t('auth.backToLogin')}</button>
            </div> : null}
          </form>
          <div className="login-features">
            <div className="feature-item"><span className="feature-icon">✓</span><span>{t('platform.independentTenant')}</span></div>
            <div className="feature-item"><span className="feature-icon">✓</span><span>{t('platform.fullApiAccess')}</span></div>
            <div className="feature-item"><span className="feature-icon">✓</span><span>{t('platform.knowledgeBaseManagement')}</span></div>
          </div>
        </div> : null}
      </div>
    </div>
  </div>;
}
