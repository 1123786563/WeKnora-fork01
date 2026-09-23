// Shared-space list page — TDesign 同构迁移（Task 11b，playbook §3 DOM 复刻）。
// 事实源：frontend/src/views/organization/OrganizationList.vue（列表 +
// ListSpaceSidebar organization 模式 + del-org-dialog）与
// OrganizationSettingsModal.vue（壳层 + create 模式 basic/permissions 段，
// ix-orgs-create 扫描态）。样式全部在 orgs.td.css（Vue <style> 平移）。
//
// 留守段（R490 React 端口保留，Tailwind 自持，playbook §4.2 留守例外）：
//  - 设置弹窗编辑模式 sections（members/requests/shares/agents + basic 的
//    邀请成员卡）——不出现在 orgs 三扫描态中；
//  - 加入组织 / 邀请预览弹框（invite-preview）；
//  - 列表加载失败的重试态（React 韧性补充，Vue 模板无对应 UI）。
// 其中的 WkInput/WkSelect/WkSwitch/WkTextarea（@weknora/ui）仅为留守段
// 引用，本迁移未新增旧栈用法。
import { useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, FormEvent, MouseEvent as ReactMouseEvent } from 'react';
import { createPortal } from 'react-dom';
import { readReactPlatformState } from '../platform/legacy-session.ts';
import type { Organization, OrganizationJoinRequest, OrganizationMember, WeKnoraClient } from '@weknora/api-client';
import { formatMessage, isLocale } from '@weknora/i18n';
import { usePreferredLocale } from '../locale.ts';
// S6：@weknora/ui 离栈（T15 硬前置），换 tdesign。
import { Input as TInput, Select as TSelect, Switch as TSwitch, Textarea as TTextarea } from 'tdesign-react';
import { Button, Dialog, Input, Popup, Skeleton, Tag, Textarea, Tooltip } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { clampApplicationNote, inviteJoinMode, requestedRoleOf } from './join.ts';
import { copyText, sharedResourceRow } from './settings-actions.ts';
import { organizationRoleLabel, organizationSettingsNavGroups, organizationSettingsSections } from './summary.ts';
import './orgs.td.css';

/* frontend/src/assets/img/upload.svg —— 空状态插画（162×162，与 kb-list/
   agents 同源文件的内联副本，按页各持一份，Phase 4 归并）。 */
const UPLOAD_SVG_DATA = 'data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMTYyIiBoZWlnaHQ9IjE2MiIgdmlld0JveD0iMCAwIDE2MiAxNjIiIGZpbGw9Im5vbmUiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyI+CjxnIGZpbHRlcj0idXJsKCNmaWx0ZXIwX2RfNjAyMl81MTczMSkiPgo8cGF0aCBkPSJNMzYuODc1IDc4TDIwIDExMS43NVYxMzMuMDQ3QzIwIDE0MC43NiAyNi4yNTI2IDE0Ny4wMTMgMzMuOTY1NSAxNDcuMDEzSDgwLjc1SDEyNy41MzRDMTM1LjI0NyAxNDcuMDEzIDE0MS41IDE0MC43NiAxNDEuNSAxMzMuMDQ3VjExMS43NUwxMjQuNjI1IDc4SDgwLjc1SDM2Ljg3NVoiIGZpbGw9InVybCgjcGFpbnQwX2xpbmVhcl82MDIyXzUxNzMxKSIvPgo8L2c+CjxwYXRoIGQ9Ik0zNy4xMjUgMTExLjM3NVY3Ny42MjVMMjAuMjUgMTExLjM3NUgzNy4xMjVaIiBmaWxsPSJ1cmwoI3BhaW50MV9saW5lYXJfNjAyMl81MTczMSkiLz4KPHBhdGggZD0iTTEyNSAxMTEuNzVWNzhMMTQxLjg3NSAxMTEuNzVIMTI1WiIgZmlsbD0idXJsKCNwYWludDJfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxwYXRoIGQ9Ik03Ny45ODY0IDEwOC42MjdMNjYuMjc0IDkzLjc0MzZDNjUuNDAyOSA5Mi42MzY1IDY2LjE5MTUgOTEuMDEyNSA2Ny42MDAyIDkxLjAxMjVINzIuNjM1QzczLjU2NyA5MS4wMTI1IDc0LjMyOTIgOTAuMjYyNSA3NC4yMDExIDg5LjMzOTRDNzIuNjc1NCA3OC4zNTAzIDU2Ljg4MDUgNTkuNDM1NSAzMy4xMDA3IDUwLjg1NDlDMzIuMTcyOSA1MC41MjAxIDMyLjQwNjcgNDguOTM3NSAzMy4zOTMgNDguOTM3NUgxMjUuMjMyQzEyNi4yMTggNDguOTM3NSAxMjYuNDUyIDUwLjUyMDEgMTI1LjUyNCA1MC44NTQ5QzEwMS43NDQgNTkuNDM1NSA4NS45NDk2IDc4LjM1MDMgODQuNDIzOSA4OS4zMzk0Qzg0LjI5NTcgOTAuMjYyNSA4NS4wNTggOTEuMDEyNSA4NS45OSA5MS4wMTI1SDkxLjAyNDhDOTIuNDMzNSA5MS4wMTI1IDkzLjIyMjEgOTIuNjM2NSA5Mi4zNTEgOTMuNzQzNkw4MC42Mzg2IDEwOC42MjdDNzkuOTYzIDEwOS40ODYgNzguNjYyIDEwOS40ODYgNzcuOTg2NCAxMDguNjI3WiIgZmlsbD0idXJsKCNwYWludDNfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxwYXRoIGZpbGwtcnVsZT0iZXZlbm9kZCIgY2xpcC1ydWxlPSJldmVub2RkIiBkPSJNNjcuNjA0IDExMS4zNzVIMjAuMjVWMTMzLjEzOEMyMC4yNSAxNDAuNTk0IDI2LjI5NDIgMTQ2LjYzOCAzMy43NSAxNDYuNjM4SDEyOC4yNUMxMzUuNzA2IDE0Ni42MzggMTQxLjc1IDE0MC41OTQgMTQxLjc1IDEzMy4xMzhWMTExLjM3NUg5NC4zOTUxQzkzLjU2NDcgMTE4LjAzNCA4Ny44ODM5IDEyMy4xODggODAuOTk5NSAxMjMuMTg4Qzc0LjExNTIgMTIzLjE4OCA2OC40MzQ0IDExOC4wMzQgNjcuNjA0IDExMS4zNzVaIiBmaWxsPSJ1cmwoI3BhaW50NF9saW5lYXJfNjAyMl81MTczMSkiLz4KPHBhdGggZD0iTTQ2LjkzNjYgMTguNTQzNkM0Ni43NDA4IDE4LjE0MTEgNDYuNzEyOSAxNy42Nzc0IDQ2Ljg1OTEgMTcuMjU0NEw0OS4xMTAyIDEwLjczNzVDNDkuODcxIDguNTM1MiA1Mi4yNzI5IDcuMzY2NjEgNTQuNDc1MiA4LjEyNzM0TDY4LjgzMDQgMTMuMDg2MUM3MS4wMzI2IDEzLjg0NjggNzIuMjAxMiAxNi4yNDg4IDcxLjQ0MDUgMTguNDUxMUw2Ny41ODM3IDI5LjYxNjJDNjYuODIyOSAzMS44MTg1IDY0LjQyMSAzMi45ODcxIDYyLjIxODcgMzIuMjI2M0w1Mi41MTE3IDI4Ljg3MzJDNTIuMDg4NyAyOC43MjcxIDUxLjc0MTEgMjguNDE4OSA1MS41NDUzIDI4LjAxNjVMNDYuOTM2NiAxOC41NDM2WiIgZmlsbD0idXJsKCNwYWludDVfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxtYXNrIGlkPSJtYXNrMF82MDIyXzUxNzMxIiBzdHlsZT0ibWFzay10eXBlOmFscGhhIiBtYXNrVW5pdHM9InVzZXJTcGFjZU9uVXNlIiB4PSI0NiIgeT0iNyIgd2lkdGg9IjI2IiBoZWlnaHQ9IjI2Ij4KPHBhdGggZD0iTTQ2LjkzNjYgMTguNTQzNkM0Ni43NDA4IDE4LjE0MTEgNDYuNzEyOSAxNy42Nzc0IDQ2Ljg1OTEgMTcuMjU0NEw0OS4xMTAyIDEwLjczNzVDNDkuODcxIDguNTM1MiA1Mi4yNzI5IDcuMzY2NjEgNTQuNDc1MiA4LjEyNzM0TDY4LjgzMDQgMTMuMDg2MUM3MS4wMzI2IDEzLjg0NjggNzIuMjAxMiAxNi4yNDg4IDcxLjQ0MDUgMTguNDUxMUw2Ny41ODM3IDI5LjYxNjJDNjYuODIyOSAzMS44MTg1IDY0LjQyMSAzMi45ODcxIDYyLjIxODcgMzIuMjI2M0w1Mi41MTE3IDI4Ljg3MzJDNTIuMDg4NyAyOC43MjcxIDUxLjc0MTEgMjguNDE4OSA1MS41NDUzIDI4LjAxNjVMNDYuOTM2NiAxOC41NDM2WiIgZmlsbD0iI0Q5RDlEOSIvPgo8L21hc2s+CjxnIG1hc2s9InVybCgjbWFzazBfNjAyMl81MTczMSkiPgo8cGF0aCBkPSJNNDMuODc2IDI1Ljg5MDFMNDYuNjMwOCAxNy45MTVMNTEuNDE1OSAxOS41NjhDNTMuMTc3NyAyMC4xNzY1IDU0LjExMjYgMjIuMDk4MSA1My41MDQgMjMuODU5OUw1MS44NTExIDI4LjY0NUw0My44NzYgMjUuODkwMVoiIGZpbGw9IiNCNUVDQ0YiLz4KPC9nPgo8cGF0aCBkPSJNODkuNTU3MiAxNi40Mzg1Qzg5LjY2NjUgMTYuMTE4MSA4OS44OTg3IDE1Ljg1NDMgOTAuMjAyNSAxNS43MDUxTDk0Ljg4MzIgMTMuNDA2NkM5Ni40NjQ5IDEyLjYyOTkgOTguMzc2OCAxMy4yODI1IDk5LjE1MzUgMTQuODY0MkwxMDQuMjE3IDI1LjE3NDZDMTA0Ljk5MyAyNi43NTYzIDEwNC4zNDEgMjguNjY4MiAxMDIuNzU5IDI5LjQ0NUw5NC43Mzk4IDMzLjM4MjlDOTMuMTU4MSAzNC4xNTk2IDkxLjI0NjIgMzMuNTA3MSA5MC40Njk1IDMxLjkyNTNMODcuMDQ1OCAyNC45NTM1Qzg2Ljg5NjYgMjQuNjQ5NiA4Ni44NzQyIDI0LjI5OSA4Ni45ODM2IDIzLjk3ODZMODkuNTU3MiAxNi40Mzg1WiIgZmlsbD0idXJsKCNwYWludDZfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxtYXNrIGlkPSJtYXNrMV82MDIyXzUxNzMxIiBzdHlsZT0ibWFzay10eXBlOmFscGhhIiBtYXNrVW5pdHM9InVzZXJTcGFjZU9uVXNlIiB4PSI4NiIgeT0iMTMiIHdpZHRoPSIxOSIgaGVpZ2h0PSIyMSI+CjxwYXRoIGQ9Ik04OS41NTcyIDE2LjQzODVDODkuNjY2NSAxNi4xMTgxIDg5Ljg5ODcgMTUuODU0MyA5MC4yMDI1IDE1LjcwNTFMOTQuODgzMiAxMy40MDY2Qzk2LjQ2NDkgMTIuNjI5OSA5OC4zNzY4IDEzLjI4MjUgOTkuMTUzNSAxNC44NjQyTDEwNC4yMTcgMjUuMTc0NkMxMDQuOTkzIDI2Ljc1NjMgMTA0LjM0MSAyOC42NjgyIDEwMi43NTkgMjkuNDQ1TDk0LjczOTggMzMuMzgyOUM5My4xNTgxIDM0LjE1OTYgOTEuMjQ2MiAzMy41MDcxIDkwLjQ2OTUgMzEuOTI1M0w4Ny4wNDU4IDI0Ljk1MzVDODYuODk2NiAyNC42NDk2IDg2Ljg3NDIgMjQuMjk5IDg2Ljk4MzYgMjMuOTc4Nkw4OS41NTcyIDE2LjQzODVaIiBmaWxsPSIjRDlEOUQ5Ii8+CjwvbWFzaz4KPGcgbWFzaz0idXJsKCNtYXNrMV82MDIyXzUxNzMxKSI+CjxwYXRoIGQ9Ik04NCAxOC43NTFMODkuNzI4IDE1LjkzODJMOTEuNDE1NyAxOS4zNzVDOTIuMDM3MSAyMC42NDAzIDkxLjUxNSAyMi4xNjk5IDkwLjI0OTYgMjIuNzkxM0w4Ni44MTI4IDI0LjQ3OUw4NCAxOC43NTFaIiBmaWxsPSIjRTdFN0U3Ii8+CjwvZz4KPHBhdGggZD0iTTQ2LjM3MzQgNTcuMjI4OUM0Ni4yNTAyIDU3LjYxMjUgNDUuOTc5NiA1Ny45MzE1IDQ1LjYyMTMgNTguMTE1N0w0MC4xMDAxIDYwLjk1MzJDMzguMjM0NCA2MS45MTIxIDM1Ljk0NDUgNjEuMTc2OSAzNC45ODU3IDU5LjMxMTFMMjguNzM1NCA0Ny4xNDk0QzI3Ljc3NjYgNDUuMjgzNiAyOC41MTE4IDQyLjk5MzggMzAuMzc3NSA0Mi4wMzQ5TDM5LjgzNjYgMzcuMTczNkM0MS43MDI0IDM2LjIxNDggNDMuOTkyMiAzNi45NSA0NC45NTExIDM4LjgxNTdMNDkuMTc3NSA0Ny4wMzk1QzQ5LjM2MTcgNDcuMzk3OSA0OS4zOTU5IDQ3LjgxNDcgNDkuMjcyOCA0OC4xOTg0TDQ2LjM3MzQgNTcuMjI4OVoiIGZpbGw9InVybCgjcGFpbnQ3X2xpbmVhcl82MDIyXzUxNzMxKSIvPgo8bWFzayBpZD0ibWFzazJfNjAyMl81MTczMSIgc3R5bGU9Im1hc2stdHlwZTphbHBoYSIgbWFza1VuaXRzPSJ1c2VyU3BhY2VPblVzZSIgeD0iMjgiIHk9IjM2IiB3aWR0aD0iMjIiIGhlaWdodD0iMjYiPgo8cGF0aCBkPSJNNDYuMzczNCA1Ny4yMjg5QzQ2LjI1MDIgNTcuNjEyNSA0NS45Nzk2IDU3LjkzMTUgNDUuNjIxMyA1OC4xMTU3TDQwLjEwMDEgNjAuOTUzMkMzOC4yMzQ0IDYxLjkxMjEgMzUuOTQ0NSA2MS4xNzY5IDM0Ljk4NTcgNTkuMzExMUwyOC43MzU0IDQ3LjE0OTRDMjcuNzc2NiA0NS4yODM2IDI4LjUxMTggNDIuOTkzOCAzMC4zNzc1IDQyLjAzNDlMMzkuODM2NiAzNy4xNzM2QzQxLjcwMjQgMzYuMjE0OCA0My45OTIyIDM2Ljk1IDQ0Ljk1MTEgMzguODE1N0w0OS4xNzc1IDQ3LjAzOTVDNDkuMzYxNyA0Ny4zOTc5IDQ5LjM5NTkgNDcuODE0NyA0OS4yNzI4IDQ4LjE5ODRMNDYuMzczNCA1Ny4yMjg5WiIgZmlsbD0iI0Q5RDlEOSIvPgo8L21hc2s+CjxnIG1hc2s9InVybCgjbWFzazJfNjAyMl81MTczMSkiPgo8cGF0aCBkPSJNNTIuOTM3NSA1NC4zNTU3TDQ2LjE4MSA1Ny44MjgxTDQ0LjA5NzYgNTMuNzc0MkM0My4zMzA1IDUyLjI4MTYgNDMuOTE4NiA1MC40NDk3IDQ1LjQxMTIgNDkuNjgyNkw0OS40NjUxIDQ3LjU5OTJMNTIuOTM3NSA1NC4zNTU3WiIgZmlsbD0iI0U3RTdFNyIvPgo8L2c+CjxwYXRoIGQ9Ik0xMjAuODggMzcuMzg5MkMxMjEuMjAzIDM3LjQ3NTggMTIxLjQ3OSAzNy42ODcyIDEyMS42NDYgMzcuOTc2OUwxMjQuMjIzIDQyLjQzOTlDMTI1LjA5MyA0My45NDgxIDEyNC41NzcgNDUuODc2NiAxMjMuMDY5IDQ2Ljc0NzRMMTEzLjIzOCA1Mi40MjMzQzExMS43MjkgNTMuMjk0IDEwOS44MDEgNTIuNzc3MyAxMDguOTMgNTEuMjY5MUwxMDQuNTE1IDQzLjYyMjhDMTAzLjY0NSA0Mi4xMTQ2IDEwNC4xNjEgNDAuMTg2MSAxMDUuNjcgMzkuMzE1M0wxMTIuMzE3IDM1LjQ3NzNDMTEyLjYwNyAzNS4zMSAxMTIuOTUxIDM1LjI2NDcgMTEzLjI3NCAzNS4zNTEzTDEyMC44OCAzNy4zODkyWiIgZmlsbD0idXJsKCNwYWludDhfbGluZWFyXzYwMjJfNTE3MzEpIi8+CjxtYXNrIGlkPSJtYXNrM182MDIyXzUxNzMxIiBzdHlsZT0ibWFzay10eXBlOmFscGhhIiBtYXNrVW5pdHM9InVzZXJTcGFjZU9uVXNlIiB4PSIxMDQiIHk9IjM1IiB3aWR0aD0iMjEiIGhlaWdodD0iMTgiPgo8cGF0aCBkPSJNMTIwLjg4IDM3LjM4OTJDMTIxLjIwMyAzNy40NzU4IDEyMS40NzkgMzcuNjg3MiAxMjEuNjQ2IDM3Ljk3NjlMMTI0LjIyMyA0Mi40Mzk5QzEyNS4wOTMgNDMuOTQ4MSAxMjQuNTc3IDQ1Ljg3NjYgMTIzLjA2OSA0Ni43NDc0TDExMy4yMzggNTIuNDIzM0MxMTEuNzI5IDUzLjI5NCAxMDkuODAxIDUyLjc3NzMgMTA4LjkzIDUxLjI2OTFMMTA0LjUxNSA0My42MjI4QzEwMy42NDUgNDIuMTE0NiAxMDQuMTYxIDQwLjE4NjEgMTA1LjY3IDM5LjMxNTNMMTEyLjMxNyAzNS40NzczQzExMi42MDcgMzUuMzEgMTEyLjk1MSAzNS4yNjQ3IDExMy4yNzQgMzUuMzUxM0wxMjAuODggMzcuMzg5MloiIGZpbGw9IiNEOUQ5RDkiLz4KPC9tYXNrPgo8ZyBtYXNrPSJ1cmwoI21hc2szXzYwMjJfNTE3MzEpIj4KPHBhdGggZD0iTTExOC4yMzEgMzIuMDYyN0wxMjEuMzg1IDM3LjUyNDRMMTE4LjEwOCAzOS40MTY0QzExNi45MDEgNDAuMTEzIDExNS4zNTggMzkuNjk5NiAxMTQuNjYyIDM4LjQ5M0wxMTIuNzcgMzUuMjE2TDExOC4yMzEgMzIuMDYyN1oiIGZpbGw9IiNCNUVDQ0YiLz4KPC9nPgo8cGF0aCBkPSJNNzMuMzQ4MyA0NS4wOTg0QzczLjM0NzggNDQuODQ2OCA3My40NDczIDQ0LjYwNTMgNzMuNjI0OCA0NC40MjdMNzYuMzYwMyA0MS42ODA1Qzc3LjI4NDcgNDAuNzUyNCA3OC43ODY0IDQwLjc0OTQgNzkuNzE0NSA0MS42NzM4TDg1Ljc2NDMgNDcuNjk5M0M4Ni42OTI0IDQ4LjYyMzcgODYuNjk1NSA1MC4xMjU1IDg1Ljc3MTEgNTEuMDUzNkw4MS4wODQ1IDU1Ljc1OUM4MC4xNjAxIDU2LjY4NzEgNzguNjU4NCA1Ni42OTAxIDc3LjczMDMgNTUuNzY1N0w3My42Mzk0IDUxLjY5MTJDNzMuNDYxMSA1MS41MTM3IDczLjM2MDcgNTEuMjcyNiA3My4zNjAyIDUxLjAyMUw3My4zNDgzIDQ1LjA5ODRaIiBmaWxsPSJ1cmwoI3BhaW50OV9saW5lYXJfNjAyMl81MTczMSkiLz4KPG1hc2sgaWQ9Im1hc2s0XzYwMjJfNTE3MzEiIHN0eWxlPSJtYXNrLXR5cGU6YWxwaGEiIG1hc2tVbml0cz0idXNlclNwYWNlT25Vc2UiIHg9IjczIiB5PSI0MCIgd2lkdGg9IjE0IiBoZWlnaHQ9IjE3Ij4KPHBhdGggZD0iTTczLjM0ODMgNDUuMDk4NEM3My4zNDc4IDQ0Ljg0NjggNzMuNDQ3MyA0NC42MDUzIDczLjYyNDggNDQuNDI3TDc2LjM2MDMgNDEuNjgwNUM3Ny4yODQ3IDQwLjc1MjQgNzguNzg2NCA0MC43NDk0IDc5LjcxNDUgNDEuNjczOEw4NS43NjQzIDQ3LjY5OTNDODYuNjkyNCA0OC42MjM3IDg2LjY5NTUgNTAuMTI1NSA4NS43NzExIDUxLjA1MzZMODEuMDg0NSA1NS43NTlDODAuMTYwMSA1Ni42ODcxIDc4LjY1ODQgNTYuNjkwMSA3Ny43MzAzIDU1Ljc2NTdMNzMuNjM5NCA1MS42OTEyQzczLjQ2MTEgNTEuNTEzNyA3My4zNjA3IDUxLjI3MjYgNzMuMzYwMiA1MS4wMjFMNzMuMzQ4MyA0NS4wOTg0WiIgZmlsbD0iI0Q5RDlEOSIvPgo8L21hc2s+CjxnIG1hc2s9InVybCgjbWFzazRfNjAyMl81MTczMSkiPgo8cGF0aCBkPSJNNzAgNDguMDY2NEw3My4zNDc1IDQ0LjcwNTRMNzUuMzY0MSA0Ni43MTM5Qzc2LjEwNjYgNDcuNDUzNCA3Ni4xMDkgNDguNjU0OCA3NS4zNjk1IDQ5LjM5NzNMNzMuMzYxIDUxLjQxMzlMNzAgNDguMDY2NFoiIGZpbGw9IiMwN0MwNUYiLz4KPC9nPgo8cGF0aCBkPSJNMTA2LjEzOCAxMjAuMTAzQzEwNi4xMzggMTE4Ljk0NiAxMDcuMDc2IDExOC4wMDkgMTA4LjIzMyAxMTguMDA5SDExMy44MTlDMTE0Ljk3NiAxMTguMDA5IDExNS45MTQgMTE4Ljk0NiAxMTUuOTE0IDEyMC4xMDNWMTIwLjEwM0MxMTUuOTE0IDEyMS4yNiAxMTQuOTc2IDEyMi4xOTggMTEzLjgxOSAxMjIuMTk4SDEwOC4yMzNDMTA3LjA3NiAxMjIuMTk4IDEwNi4xMzggMTIxLjI2IDEwNi4xMzggMTIwLjEwM1YxMjAuMTAzWiIgZmlsbD0iIzE0ODVFRSIvPgo8cGF0aCBkPSJNMTIyLjg5NiAxMjAuMTAzQzEyMi44OTYgMTE4Ljk0NiAxMjMuODM0IDExOC4wMDkgMTI0Ljk5MSAxMTguMDA5SDEzMC41NzhDMTMxLjczNCAxMTguMDA5IDEzMi42NzIgMTE4Ljk0NiAxMzIuNjcyIDEyMC4xMDNWMTIwLjEwM0MxMzIuNjcyIDEyMS4yNiAxMzEuNzM0IDEyMi4xOTggMTMwLjU3OCAxMjIuMTk4SDEyNC45OTFDMTIzLjgzNCAxMjIuMTk4IDEyMi44OTYgMTIxLjI2IDEyMi44OTYgMTIwLjEwM1YxMjAuMTAzWiIgZmlsbD0iIzA3QzA1RiIvPgo8cmVjdCB4PSIxMDYuMTM4IiB5PSIxMTcuMzEiIHdpZHRoPSI5Ljc3NTg2IiBoZWlnaHQ9IjQuMTg5NjYiIHJ4PSIyLjA5NDgzIiBmaWxsPSIjNDM5REYxIi8+CjxyZWN0IHg9IjEyMi44OTYiIHk9IjExNy4zMSIgd2lkdGg9IjkuNzc1ODYiIGhlaWdodD0iNC4xODk2NiIgcng9IjIuMDk0ODMiIGZpbGw9IiMzOUNEODAiLz4KPGRlZnM+CjxmaWx0ZXIgaWQ9ImZpbHRlcjBfZF82MDIyXzUxNzMxIiB4PSIxNC40MTM4IiB5PSI3NS4yMDY5IiB3aWR0aD0iMTMyLjY3MiIgaGVpZ2h0PSI4MC4xODU0IiBmaWx0ZXJVbml0cz0idXNlclNwYWNlT25Vc2UiIGNvbG9yLWludGVycG9sYXRpb24tZmlsdGVycz0ic1JHQiI+CjxmZUZsb29kIGZsb29kLW9wYWNpdHk9IjAiIHJlc3VsdD0iQmFja2dyb3VuZEltYWdlRml4Ii8+CjxmZUNvbG9yTWF0cml4IGluPSJTb3VyY2VBbHBoYSIgdHlwZT0ibWF0cml4IiB2YWx1ZXM9IjAgMCAwIDAgMCAwIDAgMCAwIDAgMCAwIDAgMCAwIDAgMCAwIDEyNyAwIiByZXN1bHQ9ImhhcmRBbHBoYSIvPgo8ZmVPZmZzZXQgZHk9IjIuNzkzMSIvPgo8ZmVHYXVzc2lhbkJsdXIgc3RkRGV2aWF0aW9uPSIyLjc5MzEiLz4KPGZlQ29tcG9zaXRlIGluMj0iaGFyZEFscGhhIiBvcGVyYXRvcj0ib3V0Ii8+CjxmZUNvbG9yTWF0cml4IHR5cGU9Im1hdHJpeCIgdmFsdWVzPSIwIDAgMCAwIDAuMTkyNjkxIDAgMCAwIDAgMC4xOTI2OTEgMCAwIDAgMCAwLjE5MjY5MSAwIDAgMCAwLjEgMCIvPgo8ZmVCbGVuZCBtb2RlPSJub3JtYWwiIGluMj0iQmFja2dyb3VuZEltYWdlRml4IiByZXN1bHQ9ImVmZmVjdDFfZHJvcFNoYWRvd182MDIyXzUxNzMxIi8+CjxmZUJsZW5kIG1vZGU9Im5vcm1hbCIgaW49IlNvdXJjZUdyYXBoaWMiIGluMj0iZWZmZWN0MV9kcm9wU2hhZG93XzYwMjJfNTE3MzEiIHJlc3VsdD0ic2hhcGUiLz4KPC9maWx0ZXI+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQwX2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iODAuNzUiIHkxPSI3OCIgeDI9IjgwLjc1IiB5Mj0iMTMyIiBncmFkaWVudFVuaXRzPSJ1c2VyU3BhY2VPblVzZSI+CjxzdG9wIHN0b3AtY29sb3I9IiNFNEY5RUUiLz4KPHN0b3Agb2Zmc2V0PSIxIiBzdG9wLWNvbG9yPSIjOUVERUJEIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQxX2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iMjguNjg3NSIgeTE9Ijc3LjYyNSIgeDI9IjI4LjY4NzUiIHkyPSIxMTEuMzc1IiBncmFkaWVudFVuaXRzPSJ1c2VyU3BhY2VPblVzZSI+CjxzdG9wIHN0b3AtY29sb3I9IiNEQkZBRTkiLz4KPHN0b3Agb2Zmc2V0PSIxIiBzdG9wLWNvbG9yPSIjMkNEODdFIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQyX2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iMTMzLjQzOCIgeTE9Ijc4IiB4Mj0iMTMzLjQzOCIgeTI9IjExMS43NSIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjREJGQUU5Ii8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iIzJDRDg3RSIvPgo8L2xpbmVhckdyYWRpZW50Pgo8bGluZWFyR3JhZGllbnQgaWQ9InBhaW50M19saW5lYXJfNjAyMl81MTczMSIgeDE9Ijc5LjMxMjUiIHkxPSIxMDYuMzEyIiB4Mj0iNzkuMzEyNSIgeTI9IjQ4LjkzNzUiIGdyYWRpZW50VW5pdHM9InVzZXJTcGFjZU9uVXNlIj4KPHN0b3Agc3RvcC1jb2xvcj0iIzgzQzFGQSIvPgo8c3RvcCBvZmZzZXQ9IjEiIHN0b3AtY29sb3I9IiM4M0MxRkEiIHN0b3Atb3BhY2l0eT0iMCIvPgo8L2xpbmVhckdyYWRpZW50Pgo8bGluZWFyR3JhZGllbnQgaWQ9InBhaW50NF9saW5lYXJfNjAyMl81MTczMSIgeDE9IjgxIiB5MT0iMTExLjM3NSIgeDI9IjgxIiB5Mj0iMTQ2LjYzOCIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjRjNGRkY3Ii8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0id2hpdGUiLz4KPC9saW5lYXJHcmFkaWVudD4KPGxpbmVhckdyYWRpZW50IGlkPSJwYWludDVfbGluZWFyXzYwMjJfNTE3MzEiIHgxPSI0Ny4xODE4IiB5MT0iMTYuMzIiIHgyPSI2OS41MTIxIiB5Mj0iMjQuMDMzNiIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjMDdDMDVGIi8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iIzA3QzA1RiIgc3RvcC1vcGFjaXR5PSIwIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQ2X2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iOTAuODczNiIgeTE9IjE1LjM3NTYiIHgyPSI5OC43NDk0IiB5Mj0iMzEuNDEzOSIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjQzVDNUM1Ii8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iI0M1QzVDNSIgc3RvcC1vcGFjaXR5PSIwIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQ3X2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iNDQuODI5NyIgeTE9IjU4LjUyMjUiIHgyPSIzNS4xMDcxIiB5Mj0iMzkuNjA0MyIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjQzVDNUM1Ii8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iI0M1QzVDNSIgc3RvcC1vcGFjaXR5PSIwIi8+CjwvbGluZWFyR3JhZGllbnQ+CjxsaW5lYXJHcmFkaWVudCBpZD0icGFpbnQ4X2xpbmVhcl82MDIyXzUxNzMxIiB4MT0iMTIxLjczMiIgeTE9IjM4LjEyNjIiIHgyPSIxMDUuOTc0IiB5Mj0iNDUuODI0NCIgZ3JhZGllbnRVbml0cz0idXNlclNwYWNlT25Vc2UiPgo8c3RvcCBzdG9wLWNvbG9yPSIjNTRFODlBIi8+CjxzdG9wIG9mZnNldD0iMSIgc3RvcC1jb2xvcj0iIzA3QzA1RiIgc3RvcC1vcGFjaXR5PSIwLjEiLz4KPC9saW5lYXJHcmFkaWVudD4KPGxpbmVhckdyYWRpZW50IGlkPSJwYWludDlfbGluZWFyXzYwMjJfNTE3MzEiIHgxPSI3My42NTYyIiB5MT0iNDQuMzk1NSIgeDI9IjgyLjI0NDYiIHkyPSI1NC4zNzMxIiBncmFkaWVudFVuaXRzPSJ1c2VyU3BhY2VPblVzZSI+CjxzdG9wIHN0b3AtY29sb3I9IiM2RUUxQTUiLz4KPHN0b3Agb2Zmc2V0PSIxIiBzdG9wLWNvbG9yPSIjNkVFMUE1IiBzdG9wLW9wYWNpdHk9IjAiLz4KPC9saW5lYXJHcmFkaWVudD4KPC9kZWZzPgo8L3N2Zz4K';

/* ---- 留守段共享 Tailwind recipes（R490 编辑模式/加入弹框 internals） ---- */
const ORG_BTN = 'box-border inline-flex min-h-[32px] cursor-pointer items-center justify-center gap-0 rounded-[3px] border px-4 py-0 font-[inherit] text-[14px] font-medium [transition:all_.2s_ease] disabled:cursor-not-allowed disabled:opacity-55';
const ORG_BTN_PRIMARY = ORG_BTN + ' border-0 bg-accent text-white shadow-[0_2px_8px_rgba(7,192,95,0.25)] hover:shadow-[0_4px_14px_rgba(7,192,95,0.35)]';
const ORG_BTN_OUTLINE = ORG_BTN + ' border-[rgba(7,192,95,0.5)] bg-surface text-accent hover:border-accent hover:bg-accent-wash';
const ORG_BTN_NEUTRAL = ORG_BTN + ' border-[#e7e7ea] bg-surface text-[rgba(23,26,29,0.92)] hover:border-[#c9c9cf]';
const ORG_FIELD = 'box-border w-full rounded-[6px] border border-[#e7e7ea] bg-surface px-[10px] py-[6px] font-[inherit] text-[14px]! text-[rgba(23,26,29,0.92)] focus:border-accent focus:outline-none';
const ORG_MEMBER_ROW = 'flex items-center justify-between gap-[12px] border-b border-[#e7e7ea] py-[10px] last:border-b-0';
const ORG_MEMBER_COPY = 'flex min-w-0 flex-col gap-[2px]';
const ORG_ROW_ACTIONS = 'flex shrink-0 items-center gap-[8px]';
const ORG_FORM_ITEM = 'mb-[16px]';
const ORG_FORM_LABEL = 'mb-[4px] block text-[14px] font-medium text-[rgba(23,26,29,0.92)]';
const ORG_FORM_DESC = 'm-0 mb-[10px] text-[13px] leading-[1.5] text-[rgba(23,26,29,0.6)]';
const ORG_SECTION_TITLE = 'm-0 mb-[4px] text-[15px] font-semibold text-[rgba(23,26,29,0.92)]';
const ORG_SECTION_DESC = 'm-0 mb-[16px] text-[12px] leading-[1.5] text-[rgba(23,26,29,0.6)]';
const ORG_EMPTY_INLINE = 'py-[18px] text-[13px] text-[rgba(23,26,29,0.4)]';
const ORG_AVATAR_EMOJIS = ['🚀', '📁', '👥', '🏢', '💡', '📚', '🌟', '🔧', '📌', '🎯', '📂', '🔒', '🌐', '⚡', '🎨', '📊', '🤝', '💼', '📧', '🏠', '🔑', '📈', '✨', '📋', '🌍', '💬', '🔔', '📦', '🎉', '🌈'];
const ORG_PERMISSION_ITEMS: Record<'admin' | 'editor' | 'viewer', Array<[string, boolean]>> = {
  admin: [['organization.editor.adminPerm1', true], ['organization.editor.adminPerm2', true], ['organization.editor.adminPerm3', true], ['organization.editor.adminPerm4', true], ['organization.editor.useSharedAgentsPerm', true]],
  editor: [['organization.editor.editorPerm1', true], ['organization.editor.editorPerm2', true], ['organization.editor.useSharedAgentsPerm', true], ['organization.editor.shareKBPerm', true], ['organization.editor.editorPerm3', false]],
  viewer: [['organization.editor.viewerPerm1', true], ['organization.editor.useSharedAgentsPerm', true], ['organization.editor.shareKBPerm', false], ['organization.editor.viewerPerm2', false], ['organization.editor.viewerPerm3', false]],
};
/* R487 K1 — compact role matrix behind the members-section info trigger,
 * ported from Vue OrganizationSettingsModal orgRoleMatrix (L1089-1111). */
const ORG_ROLE_MATRIX: Array<{ role: 'admin' | 'editor' | 'viewer'; perms: Array<[string, boolean]> }> = [
  { role: 'admin', perms: [['organization.editor.viewerPerm1', true], ['organization.editor.editorPerm1', true], ['organization.editor.useSharedAgentsPerm', true], ['organization.editor.shareKBPerm', true], ['organization.editor.adminPerm1', true]] },
  { role: 'editor', perms: [['organization.editor.viewerPerm1', true], ['organization.editor.editorPerm1', true], ['organization.editor.useSharedAgentsPerm', true], ['organization.editor.shareKBPerm', true], ['organization.editor.adminPerm1', false]] },
  { role: 'viewer', perms: [['organization.editor.viewerPerm1', true], ['organization.editor.editorPerm1', false], ['organization.editor.useSharedAgentsPerm', true], ['organization.editor.shareKBPerm', false], ['organization.editor.adminPerm1', false]] },
];
/* Invite-code validity options, Vue inviteValidityOptions (L1279-1284). */
const ORG_INVITE_VALIDITY_OPTIONS: Array<[number, string]> = [
  [1, 'organization.settings.validity1Day'],
  [7, 'organization.settings.validity7Days'],
  [30, 'organization.settings.validity30Days'],
  [0, 'organization.settings.validityNever'],
];
const ORG_MODAL_OVERLAY = 'fixed inset-0 z-[2000] flex items-center justify-center bg-[rgba(0,0,0,0.5)] p-[20px] backdrop-blur-[4px]';
const ORG_CLOSE_BTN = 'absolute right-[16px] top-[16px] z-[10] flex h-[32px] w-[32px] cursor-pointer items-center justify-center rounded-[8px] border-0 bg-transparent text-[rgba(23,26,29,0.6)] hover:bg-[#f3f3f5] hover:text-[rgba(23,26,29,0.92)]';
const FEATURE_BADGE_BASE = 'box-border inline-flex h-[20px] cursor-default items-center justify-center gap-[3px] rounded-[5px] px-[5px] text-[11px] font-medium [transition:background_.2s_ease]';
const FEATURE_BADGE_TONES: Record<string, string> = {
  'stat-member': 'bg-[rgba(100,116,139,0.08)] text-[rgba(23,26,29,0.6)] hover:bg-[rgba(100,116,139,0.12)]',
  'stat-kb': 'bg-accent-wash text-accent hover:bg-accent-soft',
  'stat-agent': 'bg-[rgba(124,77,255,0.08)] text-[#7c4dff] hover:bg-[rgba(124,77,255,0.12)]',
};
const RELATION_ROLE_TAG = 'inline-flex h-[22px] items-center gap-[4px] rounded-[6px] px-[6px] text-[12px] font-medium';
const RELATION_ROLE_TAG_TONES: Record<string, string> = {
  owner: 'bg-[rgba(124,77,255,0.1)] text-accent',
  admin: 'bg-accent-soft text-accent',
  editor: 'bg-accent-wash text-accent',
};
const ORG_TAG = 'inline-flex rounded-[4px] px-[8px] text-[12px] leading-[20px]';
const ORG_TAG_TONES: Record<string, string> = {
  warning: 'bg-[rgba(250,173,20,0.12)] text-[#faad14]',
  success: 'bg-accent-wash text-accent',
};

function t(locale: string, key: string, values?: Record<string, string | number>): string {
  return formatMessage(isLocale(locale) ? locale : 'en-US', key, values);
}

/* Invite-code remaining-validity note, Vue remainingValidityText (L1286-1294):
 * no expiry → 永不过期, past expiry → 已过期, otherwise 剩余 {n} 天. */
function remainingValidityText(locale: string, expiresAt: string | null): string {
  if (!expiresAt) return t(locale, 'organization.settings.remainingValidityNever');
  const exp = new Date(expiresAt).getTime();
  if (Number.isNaN(exp) || exp <= Date.now()) return t(locale, 'organization.settings.remainingValidityExpired');
  const days = Math.ceil((exp - Date.now()) / (24 * 60 * 60 * 1000));
  return t(locale, 'organization.settings.remainingValidity', { n: days });
}

/* R488 D-B5 — Vue formatDate (OrganizationSettingsModal.vue:1798-1805):
 * local YYYY-MM-DD for the 加入时间 column. */
function formatDateYmd(dateStr: string): string {
  if (!dateStr) return '';
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return '';
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return year + '-' + month + '-' + day;
}

/* R488 D-B5 — static role tag tones mirroring Vue getRoleTheme (L1807-1814):
 * admin=primary (accent), editor=warning (amber), viewer=default (grey). */
const MEMBER_ROLE_TAG = 'inline-flex h-[22px] items-center rounded-[4px] px-[6px] text-[12px] font-medium';
const MEMBER_ROLE_TAG_TONES: Record<string, string> = {
  admin: 'bg-accent-soft text-accent',
  editor: 'bg-[rgba(250,173,20,0.12)] text-[#faad14]',
  viewer: 'bg-[rgba(100,116,139,0.08)] text-[rgba(23,26,29,0.6)]',
};
/* Table recipes for the members table (Vue members-table-shell, t-table
 * medium: 13px rows on #f9f9fc zebra-free header). */
const MEMBER_TABLE = 'w-full border-collapse text-[13px]';
const MEMBER_TH = 'border-b border-[#e7e7ea] bg-[#f9f9f9] px-[12px] py-[10px] text-left text-[12px] font-semibold whitespace-nowrap text-[rgba(23,26,29,0.6)]';
const MEMBER_TD = 'border-b border-[#e7e7ea] px-[12px] py-[10px] align-middle text-[rgba(23,26,29,0.92)]';

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }
function strOf(value: unknown): string { return typeof value === 'string' ? value : ''; }
function numOf(value: unknown): number { return typeof value === 'number' ? value : 0; }
function boolOf(value: unknown): boolean { return value === true; }
function shortId(value: unknown): string {
  const id = strOf(value);
  return id.length > 12 ? id.slice(0, 8) + '…' + id.slice(-4) : id;
}

type SpaceSelection = 'all' | 'created' | 'joined';
type OrgSectionKey = 'created' | 'joined';
/** Tenant membership role, mirroring scopeRuntime.role() / Vue authStore. */
export type OrganizationSpaceRole = 'owner' | 'admin' | 'contributor' | 'viewer';

// R017 (?scope=): deep link + URL sync use the KB-list convention (App.tsx
// readScopeFromUrl/writeScopeToUrl). Values all|created|joined match the Vue
// spaceSelection union; 'all' removes the param so the canonical URL stays
// /platform/organizations.
function readScopeFromUrl(): SpaceSelection {
  const value = new URLSearchParams(window.location.search).get('scope');
  return value === 'created' || value === 'joined' ? value : 'all';
}

function writeScopeToUrl(selection: SpaceSelection): void {
  const url = new URL(window.location.href);
  if (selection === 'all') url.searchParams.delete('scope');
  else url.searchParams.set('scope', selection);
  window.history.replaceState({}, document.title, url.pathname + url.search + url.hash);
}

// shouldShowOrgRelationTag ported from frontend/src/utils/card-list-badge.ts:
// suppress the corner role tag when a section header already communicates it.
function shouldShowOrgRelationTag(opts: { selection: SpaceSelection; isOwner: boolean; myRole: string }): boolean {
  if (opts.selection === 'created') return false;
  if (opts.selection === 'joined' && !opts.myRole) return false;
  if (opts.selection === 'all' && opts.isOwner) return false;
  if (opts.selection === 'all' && !opts.isOwner && !opts.myRole) return false;
  return true;
}

// canRequestUpgradeForOrg ported from Vue OrganizationSettingsModal.vue
// canRequestUpgrade: the role-upgrade request is offered only inside edit
// mode, to members whose space role is below admin, while the tenant role is
// admin+ (below that the modal shows the read-only tenant-role hint instead).
export function canRequestUpgradeForOrg(opts: { mode: 'create' | 'edit'; myRole: string; tenantAdmin: boolean }): boolean {
  return opts.mode === 'edit' && opts.myRole !== '' && opts.myRole !== 'admin' && opts.tenantAdmin;
}

// upgradeRoleOptionsForRole ported from Vue OrganizationSettingsModal.vue
// upgradeRoleOptions: only roles above the current space role are selectable.
export function upgradeRoleOptionsForRole(myRole: string): Array<'editor' | 'admin'> {
  if (myRole === 'viewer') return ['editor', 'admin'];
  if (myRole === 'editor') return ['admin'];
  return [];
}

/* ---- 内联图片资源（frontend/src/assets/img，与 kb-list/agents 同源副本） ---- */

const CIRCLE_PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAMAAABg3Am1AAAACXBIWXMAACE4AAAhOAFFljFgAAAABGdBTUEAALGPC/xhBQAAAPZQTFRFAAAA91BQ+lBQ9FBQ/1BQ/FBQ+FBQ/FJS+lBQ91BQ/FFR/FBQ+VBQ+FBQ+lNT+FBQ+1JS+VBQ+lBQ+FBQ/FBQ+VBQ91BQ+lNT+1FR+1BQ+FBQ+VFR+VBQ+lFR+VFR91BQ+lFR+VFR+FFR+1JS+1FR+VFR+VBQ+lBQ+FBQ+VFR+FFR+1FR+lFR+lBQ+lFR+lBQ+lFR+VFR+VBQ+lFR+lJS+lFR+VFR+lFR+VFR+lFR+VFR+lFR+VFR+lFR+VFR+lFR+lFR+lFR+lFR+lFR+lFR+lFR+lFR+lFR+lFR+VFR+lFR+lFR+lFR+lFR+lFR+lFR+lFR+lFR8WpGUQAAAFF0Uk5TAAQFBQcJCRISEhcXFxcZGRwcJCQqKysxMjIyOTk8PEBDQ0NGR0dHS0tOYGp/f4CAg4ODh42VlZycn5+mp62ur7O0tbi/wc7V2Njc3+bt8fT4NJ1opQAAAAFiS0dEUg1gLZAAAAFuSURBVHjaxdXJUsJAFIXhAwYNIjIGEQQBB1AQFQWR2TDEgQTu+7+MVUmFSpoMDRu+/V91e3MahxGKJCUpGQmCi1B9ncwXirKYjxu3AnyclD40slD7BREeLmSVGMthGm7OP1fkYN2Ow1H2i1xM89gWqGjkSisHwKr8kIfvMhh5jTxpWdjEp+RDjsGqQ3ZPQIPserDIrP2DVQ4b4RH5BySLMBWXPIFagmlAPAF1AYOg8gWaAMMd8QVUhaHJG7xAFxzzBpOQHpzNeYP5qR6kFrzBIqkHksIbKNLOwc4n7ffo0IQ3GAeha9GWWbM5oy0NGGrE6QYGQSMu6mY4u8SlD1NJJcavovwRY1mASZSJ8SBJj8QYitjIsaOaAa7ZiU3DoucftGEVY456q9ffmUFOwCbrN5VX+48x/9yz8lNyIV/CUaKzJgerXgxuMqMlMVQ5Bw/h4kAlC61bEuFDuH8eGx/7pFUTwOUompKkVPQYB/EPlK2oyxaXjlIAAAAASUVORK5CYII=';
const MORE_PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACAAAAAgBAMAAACBVGfHAAAAD1BMVEUAAAAwMTMwMDMwMjIwMTPbLw9bAAAABHRSTlMA3llYOk1BewAAABxJREFUKM9jGGnAUAiJAAERRwSBXUBRCIkYYQAAnNMDYY7Uun8AAAAASUVORK5CYII=';
const ORG_GREEN_SVG = 'data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjAiIGhlaWdodD0iMjAiIHZpZXdCb3g9IjAgMCAyMCAyMCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KICA8cGF0aCBkPSJNMTAgMTBDOC44IDcuNSA3LjggMy44IDQuOCAzLjhDMi4yIDMuOCAwLjggNi44IDAuOCAxMEMwLjggMTMuMiAyLjIgMTYuMiA0LjggMTYuMkM3LjggMTYuMiA4LjggMTIuNSAxMCAxMEMxMS4yIDcuNSAxMi41IDUuNSAxNC41IDUuNUMxNi41IDUuNSAxOCA3LjUgMTggMTBDMTggMTIuNSAxNi41IDE0LjUgMTQuNSAxNC41QzEyLjUgMTQuNSAxMS4yIDEyLjUgMTAgMTBaIiBzdHJva2U9IiMwN0MwNUYiIHN0cm9rZS13aWR0aD0iMS41IiBzdHJva2UtbGluZWNhcD0icm91bmQiIHN0cm9rZS1saW5lam9pbj0icm91bmQiIGZpbGw9Im5vbmUiLz4KPC9zdmc+Cg==';
const AGENT_GREEN_SVG = 'data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjAiIGhlaWdodD0iMjAiIHZpZXdCb3g9IjAgMCAyMCAyMCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KICA8cGF0aCBkPSJNMTAgM0wxMC44IDYuMkMxMC45IDYuNyAxMS4zIDcuMSAxMS44IDcuMkwxNSA4TDExLjggOC44QzExLjMgOC45IDEwLjkgOS4zIDEwLjggOS44TDEwIDEzTDkuMiA5LjhDOS4xIDkuMyA4LjcgOC45IDguMiA4LjhMNSA4TDguMiA3LjJDOC43IDcuMSA5LjEgNi43IDkuMiA2LjJMMTAgM1oiIGZpbGw9IiMwN0MwNUYiIHN0cm9rZT0iIzA3QzA1RiIgc3Ryb2tlLXdpZHRoPSIwLjgiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIgc3Ryb2tlLWxpbmVqb2luPSJyb3VuZCIvPgogIDxwYXRoIGQ9Ik0xNS41IDRMMTUuOCA1LjJDMTUuODUgNS40NSAxNi4wNSA1LjY1IDE2LjMgNS43TDE3LjUgNkwxNi4zIDYuM0MxNi4wNSA2LjM1IDE1Ljg1IDYuNTUgMTUuOCA2LjhMMTUuNSA4TDE1LjIgNi44QzE1LjE1IDYuNTUgMTQuOTUgNi4zNSAxNC43IDYuM0wxMy41IDZMMTQuNyA1LjdDMTQuOTUgNS42NSAxNS4xNSA1LjQ1IDE1LjIgNS4yTDE1LjUgNFoiIGZpbGw9IiMwN0MwNUYiIHN0cm9rZT0iIzA3QzA1RiIgc3Ryb2tlLXdpZHRoPSIwLjYiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIgc3Ryb2tlLWxpbmVqb2luPSJyb3VuZCIvPgogIDxwYXRoIGQ9Ik00LjUgMTNMNC44IDE0LjJDNC44NSAxNC40NSA1LjA1IDE0LjY1IDUuMyAxNC43TDYuNSAxNUw1LjMgMTUuM0M1LjA1IDE1LjM1IDQuODUgMTUuNTUgNC44IDE1LjhMNC41IDE3TDQuMiAxNS44QzQuMTUgMTUuNTUgMy45NSAxNS4zNSAzLjcgMTUuM0wyLjUgMTVMMy43IDE0LjdDMy45NSAxNC42NSA0LjE1IDE0LjQ1IDQuMiAxNC4yTDQuNSAxM1oiIGZpbGw9IiMwN0MwNUYiIHN0cm9rZT0iIzA3QzA1RiIgc3Ryb2tlLXdpZHRoPSIwLjYiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIgc3Ryb2tlLWxpbmVqb2luPSJyb3VuZCIvPgo8L3N2Zz4K';

/* ---- 留守段小图标（IconGlyph 手绘集，仅编辑模式邀请卡 / 加入弹框使用） ---- */
function IconGlyph(props: { d: string; size?: number; viewBox?: string; fill?: boolean; className?: string }) {
  return (
    <svg width={props.size ?? 16} height={props.size ?? 16} viewBox={props.viewBox ?? '0 0 16 16'} fill={props.fill ? 'currentColor' : 'none'} xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className={props.className}>
      <path d={props.d} stroke={props.fill ? 'none' : 'currentColor'} strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
const IconUser = ({ size = 12 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true"><path d="M16.5 7.5a4.5 4.5 0 1 1-9 0 4.5 4.5 0 0 1 9 0ZM20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2h16Z" stroke="currentColor" strokeWidth="2" strokeLinecap="square" /></svg>
);
const IconUsergroupAdd = ({ size = 16 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true"><path d="M9 4a4 4 0 1 0 0 8 6 6 0 0 0-6 6v3m11-6h-2a4 4 0 0 0-4 4v2h6m5-13a4 4 0 1 1-8 0 4 4 0 0 1 8 0ZM20 15v3m0 0v3m0-3h-3m3 0h3" stroke="currentColor" strokeWidth="2" strokeLinecap="square" /></svg>
);
const IconChevron = ({ size = 14, direction }: { size?: number; direction: 'down' | 'right' }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">{direction === 'down'
    ? <path d="M17.5 9.5 12 15 6.5 9.5" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />
    : <path d="M9.5 17.5 15 12 9.5 6.5" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />}</svg>
);
const IconClose = ({ size = 20 }: { size?: number }) => (<IconGlyph size={size} d="M4 4l8 8M12 4l-8 8" viewBox="0 0 16 16" />);
const IconBack = ({ size = 18 }: { size?: number }) => (<IconGlyph size={size} d="M10 3 5 8l5 5" />);
const IconSearch = ({ size = 14 }: { size?: number }) => (<IconGlyph size={size} d="M7 12A5 5 0 1 0 7 2a5 5 0 0 0 0 10Zm6.5 1.5L10.4 10.4" />);
/* R488 D-B4.2 — icon glyphs for the Vue t-icon file-copy / refresh invite-card
 * actions (OrganizationSettingsModal.vue:125/:131). */
const IconCopy = ({ size = 15 }: { size?: number }) => (<IconGlyph size={size} d="M5.5 5.5V4a1 1 0 0 1 1-1H12a1 1 0 0 1 1 1v5.5a1 1 0 0 1-1 1h-1.5M4 6h5.5a1 1 0 0 1 1 1V12a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1Z" />);
const IconRefresh = ({ size = 15 }: { size?: number }) => (<IconGlyph size={size} d="M13.2 8a5.2 5.2 0 1 1-1.6-3.8M13.4 2.6v2.8h-2.8" />);
const IconCheckCircle = ({ size = 18 }: { size?: number }) => (<IconGlyph size={size} d="M8 14A6 6 0 1 0 8 2a6 6 0 0 0 0 12Zm-2.6-6.2L7.5 9.9l3.2-3.8" />);
const IconInfoCircle = ({ size = 20 }: { size?: number }) => (<IconGlyph size={size} d="M8 14A6 6 0 1 0 8 2a6 6 0 0 0 0 12ZM8 7.4V11M8 5.2v.2" />);
/* Vue navItems icon per section key (OrganizationSettingsModal.vue L993-1010). */
const ORG_NAV_ICON_BY_KEY: Record<string, string> = {
  basic: 'info-circle',
  permissions: 'user-safety',
  members: 'user',
  joinRequests: 'user-add',
  sharedKb: 'folder-open',
  sharedAgents: 'control-platform',
};

/* SpaceAvatar —— frontend/src/components/SpaceAvatar.vue 1:1（类名/结构照搬，
   尺寸样式由 orgs.td.css §2 承载）。 */
const AVATAR_GRADIENTS: Array<[string, string]> = [
  ['#07c05f', '#059669'], ['#11998e', '#38ef7d'], ['#43e97b', '#38f9d7'], ['#02aab0', '#00cdac'],
  ['#36d1dc', '#5b86e5'], ['#4facfe', '#00f2fe'], ['#667eea', '#764ba2'], ['#4776e6', '#8e54e9'],
  ['#56ab2f', '#a8e063'], ['#00b09b', '#96c93d'], ['#5ee7df', '#b490ca'], ['#614385', '#516395'],
];
function avatarHash(name: string): number {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = ((hash << 5) - hash) + name.charCodeAt(i);
    hash = hash & hash;
  }
  return Math.abs(hash);
}
function SpaceAvatar(props: { name: string; avatar?: unknown; size?: 'small' | 'medium' | 'large'; className?: string }) {
  const size = props.size ?? 'medium';
  const avatarText = strOf(props.avatar).trim();
  const isEmoji = avatarText.startsWith('emoji:') && avatarText.length > 6;
  const name = props.name?.trim() ?? '';
  const firstChar = name ? name.charAt(0) : '?';
  const letter = /[a-zA-Z]/.test(firstChar) ? firstChar.toUpperCase() : firstChar;
  const gradient = AVATAR_GRADIENTS[avatarHash(name) % AVATAR_GRADIENTS.length];
  const background = isEmoji
    ? 'linear-gradient(135deg, #f1f5f9 0%, #e2e8f0 100%)'
    : 'linear-gradient(135deg, ' + gradient[0] + ' 0%, ' + gradient[1] + ' 100%)';
  return (
    <div
      className={'space-avatar' + (size === 'small' ? ' space-avatar-small' : size === 'large' ? ' space-avatar-large' : '') + (isEmoji ? ' space-avatar-emoji' : '') + (props.className ? ' ' + props.className : '')}
      style={{ background }}
    >
      {isEmoji ? (
        <span className="space-avatar-emoji-char">{avatarText.slice(6).trim()}</span>
      ) : (
        <>
          <svg className="space-avatar-decoration" viewBox="0 0 56 40" preserveAspectRatio="xMaxYMax meet" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
            <circle cx="10" cy="12" r="4" stroke="currentColor" strokeWidth="1.5" fill="none" opacity="0.5" />
            <circle cx="28" cy="8" r="5" stroke="currentColor" strokeWidth="1.8" fill="none" opacity="0.7" />
            <circle cx="46" cy="14" r="4" stroke="currentColor" strokeWidth="1.5" fill="none" opacity="0.5" />
            <path d="M14 13 L24 10 M32 10 L42 13" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" opacity="0.4" />
            <circle cx="28" cy="28" r="6" stroke="currentColor" strokeWidth="1.2" fill="none" opacity="0.35" />
            <path d="M28 14 L28 22 M20 18 L26 24 M36 18 L30 24" stroke="currentColor" strokeWidth="1" strokeLinecap="round" opacity="0.3" />
          </svg>
          <span className="space-avatar-letter" style={{ textShadow: '0 1px 2px ' + gradient[1] + '80, 0 0 8px ' + gradient[0] + '30' }}>{letter}</span>
        </>
      )}
    </div>
  );
}

/* 留守段（加入弹框预览态）的统计徽标 —— Vue feature-badge DOM 由迁移后的
   列表卡片自持，此组件仅供 invite-preview 预览态复用。 */
function FeatureBadge(props: { tone: 'stat-member' | 'stat-kb' | 'stat-agent'; title: string; count: number }) {
  return (
    <div className={FEATURE_BADGE_BASE + ' ' + FEATURE_BADGE_TONES[props.tone]} title={props.title}>
      {props.tone === 'stat-member' ? <IconUser /> : props.tone === 'stat-kb' ? (
        <svg width={14} height={14} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true"><path d="M2 3.5h7L11 6h11v14H2V3.5Z" stroke="currentColor" strokeWidth="2" /></svg>
      ) : (
        <svg width={14} height={14} viewBox="0 0 20 20" fill="currentColor" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" className="shrink-0">
          <path d="M10 3l.8 3.2c.1.5.5.9 1 1L15 8l-3.2.8c-.5.1-.9.5-1 1L10 13l-.8-3.2c-.1-.5-.5-.9-1-1L5 8l3.2-.8c.5-.1.9-.5 1-1L10 3Z" />
        </svg>
      )}
      <span>{props.count}</span>
    </div>
  );
}

type ToastState = { tone: 'success' | 'error' | 'warning'; text: string } | null;

/* ---- ListSpaceSidebar（organization 模式）—— ListSpaceSidebar.vue 1:1 --------
   与 kb-list.td.css §5 / KnowledgeBasesPage.tsx 同源（按页各持一份）。
   organization 模式 collapsed 条带：全部 / 我创建的 / 我加入的 三条目；
   expanded 面板：同样三条目 + 计数徽标；右缘 resize-handle 拖拽 snap。 */
const ORG_RAIL_COLLAPSED_WIDTH = 56;
const ORG_RAIL_EXPANDED_WIDTH = 208;
const ORG_RAIL_SNAP_THRESHOLD = 120;
const ORG_RAIL_MAX_DRAG_WIDTH = ORG_RAIL_EXPANDED_WIDTH + 20;
const ORG_RAIL_STORAGE_KEY = 'sidebar-collapsed-list-expanded';

const ORG_RAIL_ITEMS: Array<{ key: 'all' | 'created' | 'joined'; icon: string; labelKey: string }> = [
  { key: 'all', icon: 'layers', labelKey: 'listSpaceSidebar.all' },
  { key: 'created', icon: 'usergroup-add', labelKey: 'organization.createdByMe' },
  { key: 'joined', icon: 'usergroup', labelKey: 'organization.joinedByMe' },
];

function OrgListSpaceSidebar({ t, selection, counts, onSelect }: { t: (key: string) => string; selection: 'all' | 'created' | 'joined'; counts: { all: number; created: number; joined: number }; onSelect: (key: 'all' | 'created' | 'joined') => void }) {
  const [expanded, setExpanded] = useState(() => {
    try { return window.localStorage.getItem(ORG_RAIL_STORAGE_KEY) === 'true'; } catch { return false; }
  });
  const [dragging, setDragging] = useState(false);
  const [dragWidth, setDragWidth] = useState<number | null>(null);
  const drag = useRef<{ startX: number; startWidth: number; width: number } | null>(null);

  const tooltipText = (name: string, count: number) => `${name} (${count})`;

  const onDragStart = (event: ReactMouseEvent<HTMLElement>) => {
    event.preventDefault();
    const startWidth = expanded ? ORG_RAIL_EXPANDED_WIDTH : ORG_RAIL_COLLAPSED_WIDTH;
    drag.current = { startX: event.clientX, startWidth, width: startWidth };
    setDragWidth(startWidth);
    setDragging(true);
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  };

  useEffect(() => {
    if (!dragging) return;
    const onMove = (event: MouseEvent) => {
      const state = drag.current;
      if (!state) return;
      state.width = Math.max(ORG_RAIL_COLLAPSED_WIDTH, Math.min(ORG_RAIL_MAX_DRAG_WIDTH, state.startWidth + (event.clientX - state.startX)));
      setDragWidth(state.width);
    };
    const onUp = () => {
      const width = drag.current?.width ?? ORG_RAIL_COLLAPSED_WIDTH;
      drag.current = null;
      const shouldExpand = width >= ORG_RAIL_SNAP_THRESHOLD;
      setExpanded(shouldExpand);
      try { window.localStorage.setItem(ORG_RAIL_STORAGE_KEY, String(shouldExpand)); } catch { /* storage unavailable */ }
      setDragging(false);
      setDragWidth(null);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
    return () => {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
  }, [dragging]);

  return (
    <div
      className={`list-space-sidebar${expanded ? ' expanded' : ''}${dragging ? ' dragging' : ''}`}
      style={dragging && dragWidth !== null ? { width: `${dragWidth}px` } : undefined}
    >
      {!expanded ? (
        <div className="icon-strip">
          {ORG_RAIL_ITEMS.map((item) => (
            <Tooltip key={item.key} content={tooltipText(t(item.labelKey), counts[item.key])} placement="right" showArrow={false}>
              <div
                className={`icon-item-labeled${selection === item.key ? ' active' : ''}`}
                data-space-key={item.key}
                onClick={() => onSelect(item.key)}
              >
                <TIcon name={item.icon} size="16px" />
                <span className="icon-label">{t(item.labelKey)}</span>
              </div>
            </Tooltip>
          ))}
        </div>
      ) : (
        <nav className="expanded-panel">
          {ORG_RAIL_ITEMS.map((item) => (
            <div key={item.key} className={`sidebar-item${selection === item.key ? ' active' : ''}`} data-space-key={item.key} onClick={() => onSelect(item.key)}>
              <div className="item-left">
                <TIcon name={item.icon} className="item-icon" />
                <span className="item-label">{t(item.labelKey)}</span>
              </div>
              <span className="item-count">{counts[item.key]}</span>
            </div>
          ))}
        </nav>
      )}
      <div className="resize-handle" aria-hidden="true" onMouseDown={onDragStart}>
        <div className="resize-handle-line" />
      </div>
    </div>
  );
}

type DetailFeedKey = 'members' | 'requests' | 'shares' | 'agents';
type DetailFeedState = { status: 'idle' | 'loading' | 'ready' | 'error'; message?: string };
const idleDetailFeeds: Record<DetailFeedKey, DetailFeedState> = {
  members: { status: 'idle' }, requests: { status: 'idle' }, shares: { status: 'idle' }, agents: { status: 'idle' },
};

export function OrganizationsPage({ client, inviteCode, role }: { client: WeKnoraClient; inviteCode?: string; role?: OrganizationSpaceRole }) {
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [loading, setLoading] = useState(true);
  const [listError, setListError] = useState('');
  const [toast, setToast] = useState<ToastState>(null);
  const [selection, setSelectionState] = useState<SpaceSelection>(readScopeFromUrl);
  const [collapsedSections, setCollapsedSections] = useState<Set<OrgSectionKey>>(new Set());
  const [moreMenuOrgId, setMoreMenuOrgId] = useState<string | null>(null);

  // Settings / create modal (Vue OrganizationSettingsModal parity).
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [settingsMode, setSettingsMode] = useState<'create' | 'edit'>('edit');
  const [settingsOrg, setSettingsOrg] = useState<Organization | null>(null);
  const [settingsSection, setSettingsSection] = useState('basic');
  const [formName, setFormName] = useState('');
  const [formDescription, setFormDescription] = useState('');
  const [formAvatar, setFormAvatar] = useState('');
  const [avatarPickerOpen, setAvatarPickerOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [members, setMembers] = useState<OrganizationMember[]>([]);
  const [memberSearchQuery, setMemberSearchQuery] = useState('');
  const [requests, setRequests] = useState<OrganizationJoinRequest[]>([]);
  const [sharedResources, setSharedResources] = useState<Array<Record<string, unknown>>>([]);
  const [sharedAgents, setSharedAgents] = useState<Array<Record<string, unknown>>>([]);
  const [detailFeeds, setDetailFeeds] = useState<Record<DetailFeedKey, DetailFeedState>>(idleDetailFeeds);
  const [memberInviteQuery, setMemberInviteQuery] = useState('');
  const [memberInviteCandidates, setMemberInviteCandidates] = useState<Array<Record<string, unknown>>>([]);
  const [memberInviteRole, setMemberInviteRole] = useState<'admin' | 'editor' | 'viewer'>('viewer');
  const [memberInviteLoading, setMemberInviteLoading] = useState(false);
  const [memberInviteSaving, setMemberInviteSaving] = useState<string | null>(null);
  // R487 K1 — Vue invite-member card state (OrganizationSettingsModal):
  // invite code + expiry, the two org switches, validity and member limit.
  const [settingsInviteCode, setSettingsInviteCode] = useState('');
  const [inviteCodeExpiresAt, setInviteCodeExpiresAt] = useState<string | null>(null);
  const [refreshingCode, setRefreshingCode] = useState(false);
  const [formRequireApproval, setFormRequireApproval] = useState(false);
  const [formSearchable, setFormSearchable] = useState(false);
  const [formValidityDays, setFormValidityDays] = useState(7);
  const [formMemberLimit, setFormMemberLimit] = useState<number | ''>(50);
  const [permissionsPopupOpen, setPermissionsPopupOpen] = useState(false);
  // R488 D-B5 — Vue renders the「我」badge via authStore.currentUserId
  // (OrganizationSettingsModal.vue:467); the auth/me call already runs for
  // canManageOrg, so the user id rides along.
  const [currentUserId, setCurrentUserId] = useState('');
  // R488 D-B5 — Vue's add-member entry is a popup behind an icon button
  // (:408-446), not a resident form.
  const [addMemberPopupOpen, setAddMemberPopupOpen] = useState(false);
  const [selectedInviteTenant, setSelectedInviteTenant] = useState<Record<string, unknown> | null>(null);
  const [validityPopupOpen, setValidityPopupOpen] = useState(false);
  const [upgradeRole, setUpgradeRole] = useState<'admin' | 'editor' | 'viewer'>('editor');
  const [upgradeNote, setUpgradeNote] = useState('');

  // Join modal (Vue invite-preview modal parity).
  const [joinOpen, setJoinOpen] = useState(false);
  const [joinStep, setJoinStep] = useState<'invite' | 'search'>('invite');
  const [joinInputCode, setJoinInputCode] = useState('');
  const [joinCode, setJoinCode] = useState('');
  const [joinPreview, setJoinPreview] = useState<Record<string, unknown> | null>(null);
  const [joinPreviewLoading, setJoinPreviewLoading] = useState(false);
  const [joinPreviewError, setJoinPreviewError] = useState('');
  const [joining, setJoining] = useState(false);
  const [requestRole, setRequestRole] = useState<'admin' | 'editor' | 'viewer'>('viewer');
  const [requestNote, setRequestNote] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [searchItems, setSearchItems] = useState<Array<Record<string, unknown>>>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [activeInviteCode, setActiveInviteCode] = useState(inviteCode);
  const [confirmState, setConfirmState] = useState<{ kind: 'leave' | 'delete'; org: Organization } | null>(null);
  const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Guards the settings modal's org-detail refresh: a late response for a
  // closed (or switched) modal must not resurrect stale settingsOrg state.
  const settingsRequestId = useRef('');
  const locale = usePreferredLocale();
  const organizationsApi = client.identity.organizations;

  function showToast(tone: 'success' | 'error' | 'warning', text: string) { setToast({ tone, text }); }

  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(null), 3000);
    return () => clearTimeout(timer);
  }, [toast]);

  // R017 RBAC (Vue OrganizationList.vue:499-504): every write affordance on
  // this page (创建/加入/删除) requires the current TENANT role ≥ admin —
  // canManageOrg = hasRole('admin') || canAccessAllTenants. An explicit role
  // prop (scopeRuntime.role() wiring, same pattern as SettingsPage) wins;
  // otherwise the page resolves it from auth/me: the active-tenant membership
  // role (selected tenant, falling back to the home tenant like Vue's
  // currentTenantRole) plus the can_access_all_tenants superuser flag. UI
  // rendering only — the server route guard remains the real boundary.
  const [resolvedCanManage, setResolvedCanManage] = useState<boolean | null>(null);
  useEffect(() => {
    if (role) setResolvedCanManage(role === 'admin' || role === 'owner');
    // R490 C1: auth/me must still run when the role prop short-circuits the
    // canManage resolution — the app router always passes scopeRuntime.role()
    // (never null), and the「我」member badge keys off the user id that call
    // carries (Vue authStore.currentUserId, OrganizationSettingsModal.vue:467).
    // Skipping it left currentUserId '' and hid the badge in production while
    // role-less test mounts kept seeing it.
    let active = true;
    void client.auth?.me?.().then((me) => {
      if (!active) return;
      const record = me.user as Record<string, unknown>;
      // R488 D-B5: the「我」member badge keys off the current user id
      // (Vue authStore.currentUserId, OrganizationSettingsModal.vue:467).
      const userId = record.id;
      if (typeof userId === 'string' || typeof userId === 'number') setCurrentUserId(String(userId));
      if (role) return;
      const selected = readReactPlatformState(window.localStorage)?.tenantId ?? null;
      const homeTenant = me.tenant && me.tenant.id !== null && me.tenant.id !== undefined ? String(me.tenant.id) : '';
      const tenantId = selected ?? homeTenant;
      let currentRole = '';
      for (const item of me.memberships ?? []) {
        if (!item || typeof item !== 'object') continue;
        const row = item as Record<string, unknown>;
        const id = row.tenant_id ?? row.tenantId;
        if (tenantId && String(id) === tenantId && typeof row.role === 'string') { currentRole = row.role; break; }
      }
      setResolvedCanManage(currentRole === 'admin' || currentRole === 'owner' || record.can_access_all_tenants === true);
    }).catch(() => {
      // Identity unavailable (embedded/test mounts): keep the legacy
      // permissive UI; the server still rejects unauthorized writes.
      if (active && !role) setResolvedCanManage(false);
    });
    return () => { active = false; };
  }, [client, role]);
  const canManageOrg = resolvedCanManage ?? false;
  const writeGuardTitle = t(locale, 'organization.rbac.needTenantAdminTip');

  // Vue ListSpaceSidebar v-model="spaceSelection": rail clicks mirror the
  // selection into ?scope= so shell sub-filter and deep links stay in sync.
  const setSelection = (next: SpaceSelection) => {
    setSelectionState(next);
    writeScopeToUrl(next);
  };

  function clearInviteFromUrl() {
    const url = new URL(window.location.href);
    url.searchParams.delete('invite_code');
    window.history.replaceState({}, document.title, url.pathname + url.search + url.hash);
  }

  useEffect(() => { setActiveInviteCode(inviteCode); }, [inviteCode]);

  // An incoming invite code (URL ?invite_code=…) opens the join modal and
  // previews the target organization, mirroring the Vue onMounted behavior.
  useEffect(() => {
    if (!activeInviteCode) return;
    let active = true;
    setJoinOpen(true);
    setJoinCode(activeInviteCode);
    setJoinPreview(null);
    setJoinPreviewError('');
    setJoinPreviewLoading(true);
    void organizationsApi.preview(activeInviteCode).then((preview) => { if (active) setJoinPreview(preview); })
      .catch((reason) => { if (active) setJoinPreviewError(errorText(reason, t(locale, 'organization.invite.previewFailed'))); })
      .finally(() => { if (active) setJoinPreviewLoading(false); });
    return () => { active = false; };
  }, [activeInviteCode, organizationsApi]);

  async function load() {
    setLoading(true);
    setListError('');
    try {
      const result = await organizationsApi.list();
      setOrganizations(result.items);
      if (settingsOrg) {
        const next = result.items.find((item) => item.id === settingsOrg.id);
        if (next) setSettingsOrg(next);
      }
    } catch (reason) { setListError(errorText(reason, t(locale, 'common.error'))); }
    finally { setLoading(false); }
  }
  useEffect(() => { void load(); /* eslint-disable-line react-hooks/exhaustive-deps */ }, [client]);

  async function loadOrganizationDetail(id: string) {
    setDetailFeeds({ members: { status: 'loading' }, requests: { status: 'loading' }, shares: { status: 'loading' }, agents: { status: 'loading' } });
    const [membersResult, requestsResult, sharesResult, agentsResult] = await Promise.allSettled([
      organizationsApi.members.list(id),
      organizationsApi.joinRequests.list(id),
      organizationsApi.knowledgeBaseShares.listForOrganization(id),
      organizationsApi.agentShares.listForOrganization(id),
    ]);
    const feedState = (result: PromiseSettledResult<unknown>, fallback: string): DetailFeedState => result.status === 'fulfilled'
      ? { status: 'ready' } : { status: 'error', message: errorText(result.reason, fallback) };
    setDetailFeeds({
      members: feedState(membersResult, t(locale, 'organization.memberRemoveFailed')),
      requests: feedState(requestsResult, t(locale, 'organization.settings.reviewFailed')),
      shares: feedState(sharesResult, t(locale, 'organization.settings.removeShareFailed')),
      agents: feedState(agentsResult, t(locale, 'organization.settings.removeShareFailed')),
    });
    if (membersResult.status === 'fulfilled') setMembers(membersResult.value.items);
    if (requestsResult.status === 'fulfilled') setRequests(requestsResult.value.items);
    if (sharesResult.status === 'fulfilled') setSharedResources(sharesResult.value.items);
    if (agentsResult.status === 'fulfilled') setSharedAgents(agentsResult.value.items);
  }

  function openCreateModal() {
    setSettingsMode('create'); setSettingsOrg(null); setSettingsSection('basic');
    setFormName(''); setFormDescription('');
    setFormAvatar(''); setAvatarPickerOpen(false);
    setSettingsOpen(true);
  }

  function openSettingsModal(org: Organization) {
    setSettingsMode('edit'); setSettingsOrg(org); setSettingsSection('basic');
    setFormName(org.name); setFormDescription(strOf(org.description)); setFormAvatar(strOf(org.avatar));
    setMembers([]); setRequests([]); setSharedResources([]); setSharedAgents([]); setDetailFeeds(idleDetailFeeds);
    setMemberSearchQuery(''); setMemberInviteQuery(''); setMemberInviteCandidates([]); setMemberInviteRole('viewer');
    setPermissionsPopupOpen(false);
    // Vue formData defaults (L901-909) until the org detail lands.
    setSettingsInviteCode(''); setInviteCodeExpiresAt(null); setRefreshingCode(false);
    setFormRequireApproval(false); setFormSearchable(false); setFormValidityDays(7); setFormMemberLimit(50);
    setSettingsOpen(true);
    void loadOrganizationDetail(org.id);
    // Vue OrganizationSettingsModal fetchOrgDetail: the org detail endpoint
    // (GET /organizations/:id) carries has_pending_upgrade and the
    // authoritative my_role; the list row alone cannot gate the upgrade form.
    // It also seeds the whole 邀请成员 card (invite code + expiry, switches,
    // validity, member limit — Vue L1312-1324).
    settingsRequestId.current = org.id;
    void organizationsApi.get(org.id).then((detail) => {
      if (settingsRequestId.current !== org.id) return;
      setSettingsOrg(detail);
      const record = detail as Record<string, unknown>;
      setFormAvatar(strOf(record.avatar));
      setFormRequireApproval(boolOf(record.require_approval));
      setFormSearchable(boolOf(record.searchable));
      const validity = record.invite_code_validity_days;
      setFormValidityDays(typeof validity === 'number' ? validity : 7);
      const limit = record.member_limit;
      setFormMemberLimit(typeof limit === 'number' && limit >= 0 ? limit : 50);
      setSettingsInviteCode(strOf(record.invite_code));
      const expiresAt = record.invite_code_expires_at;
      setInviteCodeExpiresAt(typeof expiresAt === 'string' ? expiresAt : null);
    }).catch(() => { /* keep the list row, like Vue's caught fetchOrgDetail */ });
  }

  function closeSettings() { settingsRequestId.current = ''; setSettingsOpen(false); setSettingsOrg(null); }

  function openJoinModal() {
    setJoinOpen(true); setJoinStep('invite'); setJoinInputCode(''); setJoinPreview(null);
    setJoinPreviewError(''); setRequestRole('viewer'); setRequestNote('');
    setSearchQuery(''); setSearchItems([]);
  }

  function closeJoin() {
    setJoinOpen(false); setJoinPreview(null); setJoinInputCode(''); setJoinCode(''); setJoinPreviewError('');
    setJoinStep('invite'); setSearchQuery(''); setSearchItems([]); setRequestNote(''); setRequestRole('viewer');
    setActiveInviteCode(undefined);
    clearInviteFromUrl();
  }

  async function doPreviewFromInput() {
    const code = joinInputCode.trim();
    if (!code) { showToast('error', t(locale, 'organization.inviteCodeRequired')); return; }
    setJoinCode(code); setJoinPreviewError(''); setJoinPreview(null); setJoinPreviewLoading(true);
    try { setJoinPreview(await organizationsApi.preview(code)); }
    // Vue OrganizationList.vue:879 — messageless preview failures fall back to
    // previewFailed, not invalidCode (which is only the server's own copy).
    catch (reason) { setJoinPreviewError(errorText(reason, t(locale, 'organization.invite.previewFailed'))); }
    finally { setJoinPreviewLoading(false); }
  }

  function runSearch(query: string) {
    setSearchLoading(true);
    void organizationsApi.search(query, 20).then((result) => setSearchItems(result.items))
      .catch(() => setSearchItems([]))
      .finally(() => setSearchLoading(false));
  }

  function onSearchQueryChange(value: string) {
    setSearchQuery(value);
    if (searchTimer.current) clearTimeout(searchTimer.current);
    searchTimer.current = setTimeout(() => runSearch(value.trim()), 300);
  }

  function openSearchTab() { setJoinStep('search'); runSearch(searchQuery.trim()); }

  function previewSearchableOrg(row: Record<string, unknown>) {
    setJoinPreview(row);
    setRequestRole('viewer'); setRequestNote('');
  }

  const previewJoinMode = inviteJoinMode(joinPreview);
  const previewIsAlreadyMember = boolOf(joinPreview?.is_already_member);

  async function confirmJoin() {
    if (!joinPreview || previewIsAlreadyMember) return;
    setJoining(true);
    try {
      if (!joinCode) {
        // Joined from the search tab: no invite code, join by organization id.
        await organizationsApi.joinById(strOf(joinPreview.id), previewJoinMode === 'request' ? { message: clampApplicationNote(requestNote) || undefined, role: requestRole } : {});
      } else if (previewJoinMode === 'request') {
        await organizationsApi.submitJoinRequest({ invite_code: joinCode, role: requestRole, ...(requestNote.trim() ? { message: clampApplicationNote(requestNote) } : {}) });
      } else {
        await organizationsApi.join({ invite_code: joinCode });
      }
      setJoinOpen(false); setJoinPreview(null); setJoinCode(''); setActiveInviteCode(undefined);
      clearInviteFromUrl();
      await load();
      showToast('success', t(locale, previewJoinMode === 'request' ? 'organization.invite.requestSubmitted' : 'organization.invite.joinSuccess'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.invite.joinFailed'))); }
    finally { setJoining(false); }
  }

  function viewOrganizationFromPreview() {
    if (!joinPreview || !strOf(joinPreview.id)) return;
    const org = {
      id: strOf(joinPreview.id),
      name: strOf(joinPreview.name),
      description: strOf(joinPreview.description),
      avatar: strOf(joinPreview.avatar),
      member_count: numOf(joinPreview.member_count),
      share_count: numOf(joinPreview.share_count),
      agent_share_count: numOf(joinPreview.agent_share_count),
      is_owner: false,
      my_role: 'viewer',
    } as unknown as Organization;
    setSettingsMode('edit'); setSettingsOrg(org); setSettingsSection('basic');
    setFormName(org.name); setFormDescription(strOf(org.description)); setFormAvatar(strOf(org.avatar));
    setSettingsInviteCode(''); setInviteCodeExpiresAt(null); setMembers([]); setRequests([]); setSharedResources([]); setSettingsOpen(true);
    void loadOrganizationDetail(org.id);
  }

  async function submitCreate(event?: React.FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!canManageOrg) return;
    if (!formName.trim()) {
      setSettingsSection('basic');
      showToast('warning', t(locale, 'organization.nameRequired'));
      return;
    }
    setSaving(true);
    try {
      await organizationsApi.create({ name: formName.trim(), description: formDescription.trim(), ...(formAvatar ? { avatar: formAvatar } : {}) });
      setSettingsOpen(false); setFormName(''); setFormDescription('');
      setFormAvatar(''); setAvatarPickerOpen(false);
      await load();
      showToast('success', t(locale, 'organization.createSuccess'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.createFailed'))); }
    finally { setSaving(false); }
  }

  async function submitBasic(event?: React.FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!settingsOrg || !settingsCanManage) return;
    if (!formName.trim()) {
      setSettingsSection('basic');
      showToast('warning', t(locale, 'organization.nameRequired'));
      return;
    }
    setSaving(true);
    try {
      // Vue handleSave (L1520-1528) submits the whole basic form: name,
      // description, avatar plus the invite-card fields.
      await organizationsApi.update(settingsOrg.id, {
        name: formName.trim(),
        description: formDescription.trim(),
        ...(formAvatar ? { avatar: formAvatar } : {}),
        require_approval: formRequireApproval,
        searchable: formSearchable,
        invite_code_validity_days: formValidityDays,
        member_limit: formMemberLimit === '' ? 0 : formMemberLimit,
      });
      await load();
      showToast('success', t(locale, 'common.saveSuccess'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.roleUpdateFailed'))); }
    finally { setSaving(false); }
  }

  async function updateMemberRole(member: OrganizationMember, nextRole: 'admin' | 'editor' | 'viewer') {
    if (!settingsOrg || !settingsCanManage || member.tenant_id === settingsOrg.owner_tenant_id || member.user_id === settingsOrg.owner_id) return;
    try {
      await organizationsApi.members.updateRole(settingsOrg.id, member.tenant_id, { role: nextRole });
      await loadOrganizationDetail(settingsOrg.id);
      showToast('success', t(locale, 'organization.roleUpdated'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.roleUpdateFailed'))); }
  }

  async function removeMember(member: OrganizationMember) {
    if (!settingsOrg || !settingsCanManage || member.tenant_id === settingsOrg.owner_tenant_id || member.user_id === settingsOrg.owner_id) return;
    if (!window.confirm(t(locale, 'organization.detail.removeMemberConfirm', { name: member.tenant_name ?? member.username }))) return;
    try {
      await organizationsApi.members.remove(settingsOrg.id, member.tenant_id);
      await loadOrganizationDetail(settingsOrg.id);
      showToast('success', t(locale, 'organization.memberRemoved'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.memberRemoveFailed'))); }
  }

  async function reviewRequest(request: OrganizationJoinRequest, approved: boolean) {
    if (!settingsOrg || !settingsCanManage) return;
    try {
      await organizationsApi.joinRequests.review(settingsOrg.id, request.id, { approved, role: requestedRoleOf(request) });
      await loadOrganizationDetail(settingsOrg.id);
      showToast('success', t(locale, approved ? 'organization.settings.approveSuccess' : 'organization.settings.rejectSuccess'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.settings.reviewFailed'))); }
  }

  async function confirmLeaveOrDelete() {
    if (!confirmState) return;
    const { kind, org } = confirmState;
    try {
      if (kind === 'leave') {
        await organizationsApi.leave(org.id);
        showToast('success', t(locale, 'organization.leaveSuccess'));
      } else {
        if (!canManageOrg || !isOwnerOf(org)) return;
        await organizationsApi.remove(org.id);
        showToast('success', t(locale, 'organization.deleteSuccess'));
      }
      setConfirmState(null);
      setSettingsOpen(false); setSettingsOrg(null);
      await load();
    } catch (reason) {
      setConfirmState(null);
      showToast('error', errorText(reason, t(locale, kind === 'leave' ? 'organization.leaveFailed' : 'organization.deleteFailed')));
    }
  }

  // R487 K1 — Vue invite-card handlers. The switches and the validity select
  // save IMMEDIATELY (handleValidityChange L1701 / handleApprovalToggle L1720 /
  // handleSearchableToggle L1741), rolling the optimistic UI change back when
  // the server rejects the patch.
  async function saveInvitePatch(patch: Record<string, unknown>): Promise<boolean> {
    if (!settingsOrg || !settingsCanManage) return false;
    try {
      await organizationsApi.update(settingsOrg.id, patch);
      showToast('success', t(locale, 'common.saveSuccess'));
      return true;
    } catch (reason) {
      showToast('error', errorText(reason, t(locale, 'organization.roleUpdateFailed')));
      return false;
    }
  }

  async function handleValidityChange(value: number) {
    const previous = formValidityDays;
    setFormValidityDays(value);
    if (!await saveInvitePatch({ invite_code_validity_days: value })) setFormValidityDays(previous);
  }

  async function handleApprovalToggle(value: boolean) {
    const previous = formRequireApproval;
    setFormRequireApproval(value);
    if (!await saveInvitePatch({ require_approval: value })) setFormRequireApproval(previous);
  }

  async function handleSearchableToggle(value: boolean) {
    const previous = formSearchable;
    setFormSearchable(value);
    if (!await saveInvitePatch({ searchable: value })) setFormSearchable(previous);
  }

  // Vue refreshInviteCode (L1682-1699): regenerate through the invite-code
  // endpoint, then refetch the org detail so the expiry note tracks the new
  // code (only the expiry is taken from the refetch — the generated code is
  // always the freshest value).
  async function refreshInviteCode() {
    if (!settingsOrg || !settingsCanManage) return;
    setRefreshingCode(true);
    try {
      const { inviteCode: code } = await organizationsApi.generateInviteCode(settingsOrg.id);
      setSettingsInviteCode(code);
      showToast('success', t(locale, 'organization.inviteCodeRefreshed'));
      const detail = await organizationsApi.get(settingsOrg.id);
      if (settingsRequestId.current === settingsOrg.id) {
        setSettingsOrg(detail);
        const expiresAt = (detail as Record<string, unknown>).invite_code_expires_at;
        setInviteCodeExpiresAt(typeof expiresAt === 'string' ? expiresAt : null);
      }
    } catch (reason) {
      showToast('error', errorText(reason, t(locale, 'organization.inviteCodeRefreshFailed')));
    } finally {
      setRefreshingCode(false);
    }
  }

  async function unshareKnowledgeBase(row: ReturnType<typeof sharedResourceRow>) {
    if (!settingsOrg || !settingsCanManage) return;
    if (!window.confirm(t(locale, 'organization.settings.removeShareConfirm', { name: row.name }))) return;
    try {
      await organizationsApi.knowledgeBaseShares.remove(row.knowledgeBaseId, row.shareId);
      await loadOrganizationDetail(settingsOrg.id);
      showToast('success', t(locale, 'organization.settings.removeShareSuccess'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.settings.removeShareFailed'))); }
  }

  async function searchMemberInviteCandidates(query: string) {
    setMemberInviteQuery(query);
    if (!settingsOrg || query.trim().length < 2) { setMemberInviteCandidates([]); setSelectedInviteTenant(null); return; }
    setMemberInviteLoading(true);
    try {
      const rows = await organizationsApi.searchTenantsForInvite(settingsOrg.id, query.trim(), 10);
      setMemberInviteCandidates(rows as unknown as Array<Record<string, unknown>>);
    } catch (reason) {
      setMemberInviteCandidates([]);
      showToast('error', errorText(reason, t(locale, 'organization.addMember.failed')));
    } finally { setMemberInviteLoading(false); }
  }

  async function inviteMember(candidate: Record<string, unknown>) {
    if (!settingsOrg || !settingsCanManage) return;
    const tenantId = typeof candidate.tenant_id === 'number' ? candidate.tenant_id : Number(candidate.tenant_id);
    if (!Number.isSafeInteger(tenantId)) return;
    const candidateId = String(candidate.tenant_id);
    setMemberInviteSaving(candidateId);
    try {
      await organizationsApi.inviteMember(settingsOrg.id, { tenant_id: tenantId, representative_user_id: strOf(candidate.representative_user_id), role: memberInviteRole });
      setMemberInviteCandidates((rows) => rows.filter((row) => String(row.tenant_id) !== candidateId));
      // Vue handleAddMember (L1652-1656): close the popup, reset the dialog
      // (selectedTenantId null, role back to viewer, results cleared) and
      // refetch the member list — loadOrganizationDetail reloads all feeds.
      setAddMemberPopupOpen(false);
      setSelectedInviteTenant(null);
      setMemberInviteRole('viewer');
      setMemberInviteQuery('');
      setMemberInviteCandidates([]);
      await loadOrganizationDetail(settingsOrg.id);
      showToast('success', t(locale, 'organization.addMember.success'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.addMember.failed'))); }
    finally { setMemberInviteSaving(null); }
  }

  async function unshareAgent(agent: Record<string, unknown>) {
    if (!settingsOrg || !settingsCanManage) return;
    const agentId = strOf(agent.agent_id);
    const shareId = strOf(agent.id) || strOf(agent.share_id);
    if (!agentId || !shareId || !window.confirm(t(locale, 'organization.settings.removeShareConfirm', { name: strOf(agent.agent_name) || strOf(agent.name) || agentId }))) return;
    try {
      await organizationsApi.agentShares.remove(agentId, shareId);
      await loadOrganizationDetail(settingsOrg.id);
      showToast('success', t(locale, 'organization.settings.removeShareSuccess'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.settings.removeShareFailed'))); }
  }

  async function submitUpgradeRequest(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!settingsOrg) return;
    // Vue disables the upgrade entry while hasPendingUpgrade; the inline form
    // mirrors that by refusing to submit a second request.
    if (settingsOrg.has_pending_upgrade === true) return;
    try {
      await organizationsApi.requestRoleUpgrade(settingsOrg.id, { requested_role: upgradeRole, ...(upgradeNote.trim() ? { message: clampApplicationNote(upgradeNote) } : {}) });
      setUpgradeNote('');
      // Vue handleSubmitUpgrade: hasPendingUpgrade.value = true (plus the
      // store patch) so the entry stays disabled without a refetch.
      setSettingsOrg((current) => (current ? { ...current, has_pending_upgrade: true } : current));
      showToast('success', t(locale, 'organization.upgrade.submitSuccess'));
    } catch (reason) { showToast('error', errorText(reason, t(locale, 'organization.upgrade.submitFailed'))); }
  }

  const isOwnerOf = (org: Organization) => boolOf(org.is_owner);
  const sectionOf = (org: Organization): OrgSectionKey => (isOwnerOf(org) ? 'created' : 'joined');
  const createdCount = organizations.filter(isOwnerOf).length;
  const joinedCount = organizations.length - createdCount;
  const ordered = useMemo(() => {
    if (selection === 'created') return organizations.filter(isOwnerOf);
    if (selection === 'joined') return organizations.filter((org) => !isOwnerOf(org));
    // 全部 view: owners first, matching the Vue grouping headers.
    return [...organizations].sort((a, b) => {
      if (isOwnerOf(a) === isOwnerOf(b)) return 0;
      return isOwnerOf(a) ? -1 : 1;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [organizations, selection]);

  const toggleSection = (key: OrgSectionKey) => {
    setCollapsedSections((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key); else next.add(key);
      return next;
    });
  };

  /* Vue OrganizationList.vue:53-74 —— 分组标题（我创建的/我加入的）。 */
  function sectionHeader(key: OrgSectionKey) {
    const collapsed = collapsedSections.has(key);
    return (
      <div key={'header-' + key} className="org-section-header" role="button" tabIndex={0}
        onClick={() => toggleSection(key)}
        onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); toggleSection(key); } }}>
        {/* Vue OrganizationList.vue:57 — created uses the single-person `user`
            glyph; joined uses `usergroup`. */}
        <TIcon name={key === 'created' ? 'user' : 'usergroup'} size="14px" />
        <span>{t(locale, key === 'created' ? 'organization.createdByMe' : 'organization.joinedByMe')}</span>
        <span className="org-section-count">{key === 'created' ? createdCount : joinedCount}</span>
        <TIcon className="org-section-toggle" name={collapsed ? 'chevron-right' : 'chevron-down'} size="14px" />
      </div>
    );
  }

  const cardRows: React.ReactNode[] = [];
  ordered.forEach((org, index) => {
    const owner = isOwnerOf(org);
    /* Vue 标题插位：created 仅 index===0 的 owner 卡前；joined 在首张非 owner
       卡（或 owner→非 owner 过渡处）前（OrganizationList.vue:53/64）。 */
    if (selection === 'all' && owner && index === 0) cardRows.push(sectionHeader('created'));
    if (selection === 'all' && !owner && (index === 0 || isOwnerOf(ordered[index - 1]))) cardRows.push(sectionHeader('joined'));
    const role = strOf(org.my_role);
    const memberCount = numOf(org.member_count);
    const shareCount = numOf(org.share_count);
    const agentShareCount = numOf(org.agent_share_count);
    const pendingCount = numOf(org.pending_join_request_count);
    const description = strOf(org.description);
    const showRelation = shouldShowOrgRelationTag({ selection, isOwner: owner, myRole: role });
    const relationClass = owner ? 'owner' : role;
    /* Vue v-show="!isOrgRowHidden(org)" —— 折叠保留 DOM 仅 display:none。 */
    const rowHidden = selection === 'all' && collapsedSections.has(sectionOf(org));
    cardRows.push(
      <div key={org.id || index} className={'org-card' + (owner ? '' : ' joined-org')} style={rowHidden ? { display: 'none' } : undefined}
        onClick={() => openSettingsModal(org)}>
        {/* 装饰：协作网络感图形（Vue :77-90 逐 path 复刻）。 */}
        <div className="card-decoration">
          <svg className="card-deco-svg" width="56" height="40" viewBox="0 0 56 40" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
            <circle cx="10" cy="12" r="4" stroke="currentColor" strokeWidth="1.5" fill="none" opacity="0.5" />
            <circle cx="28" cy="8" r="5" stroke="currentColor" strokeWidth="1.8" fill="none" opacity="0.7" />
            <circle cx="46" cy="14" r="4" stroke="currentColor" strokeWidth="1.5" fill="none" opacity="0.5" />
            <path d="M14 13 L24 10 M32 10 L42 13" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" opacity="0.4" />
            <circle cx="28" cy="28" r="6" stroke="currentColor" strokeWidth="1.2" fill="none" opacity="0.35" />
            <path d="M28 14 L28 22 M20 18 L26 24 M36 18 L30 24" stroke="currentColor" strokeWidth="1" strokeLinecap="round" opacity="0.3" />
          </svg>
        </div>
        <div className="card-header">
          <div className="card-header-left">
            <div className="org-avatar"><SpaceAvatar name={org.name} avatar={org.avatar} size="small" /></div>
            <div className="card-title-block"><span className="card-title" title={org.name}>{org.name}</span></div>
          </div>
          {/* Vue t-popup v-model + card-more-popup（OrganizationList.vue:102-125）。 */}
          <Popup
            visible={moreMenuOrgId === org.id}
            trigger="click"
            destroyOnClose
            placement="bottom-right"
            overlayClassName="card-more-popup"
            onVisibleChange={(visible: boolean) => { if (!visible && moreMenuOrgId === org.id) setMoreMenuOrgId(null); }}
            content={(
              <div className="popup-menu" onClick={(event) => event.stopPropagation()}>
                <div className="popup-menu-item" onClick={(event) => { event.stopPropagation(); setMoreMenuOrgId(null); openSettingsModal(org); }}>
                  <TIcon className="menu-icon" name="setting" />
                  <span>{t(locale, 'organization.settings.editTitle')}</span>
                </div>
                {!owner ? (
                  <div className="popup-menu-item delete" onClick={(event) => { event.stopPropagation(); setMoreMenuOrgId(null); setConfirmState({ kind: 'leave', org }); }}>
                    <TIcon className="menu-icon" name="logout" />
                    <span>{t(locale, 'organization.leave')}</span>
                  </div>
                ) : canManageOrg ? (
                  // Vue: v-if="org.is_owner && canManageOrg" — deleting an
                  // owned space also requires the tenant admin+ role.
                  <div className="popup-menu-item delete" onClick={(event) => { event.stopPropagation(); setMoreMenuOrgId(null); setConfirmState({ kind: 'delete', org }); }}>
                    <TIcon className="menu-icon" name="delete" />
                    <span>{t(locale, 'common.delete')}</span>
                  </div>
                ) : null}
              </div>
            )}
          >
            <div className={'more-wrap' + (moreMenuOrgId === org.id ? ' active-more' : '')} onClick={(event) => { event.stopPropagation(); setMoreMenuOrgId(moreMenuOrgId === org.id ? null : org.id); }}>
              <img className="more-icon" src={MORE_PNG} alt="" />
            </div>
          </Popup>
        </div>
        <div className="card-content">
          <div className="card-description">{description || t(locale, 'organization.noDescription')}</div>
        </div>
        <div className="card-bottom">
          <div className="bottom-left">
            <div className="feature-badges">
              <Tooltip content={t(locale, 'organization.memberCount')} placement="top">
                <div className="feature-badge stat-member">
                  <TIcon name="user" size="14px" />
                  <span className="badge-count">{memberCount}</span>
                </div>
              </Tooltip>
              <Tooltip content={t(locale, 'organization.invite.knowledgeBases')} placement="top">
                <div className="feature-badge stat-kb">
                  <TIcon name="folder" size="14px" />
                  <span className="badge-count">{shareCount}</span>
                </div>
              </Tooltip>
              <Tooltip content={t(locale, 'organization.invite.agents')} placement="top">
                <div className="feature-badge stat-agent">
                  <img src={AGENT_GREEN_SVG} className="stat-agent-icon" alt="" aria-hidden="true" />
                  <span className="badge-count">{agentShareCount}</span>
                </div>
              </Tooltip>
            </div>
            {pendingCount > 0 ? (
              <Tooltip content={t(locale, 'organization.settings.pendingJoinRequestsBadge')} placement="top">
                <span className="pending-requests-badge">{pendingCount} {t(locale, 'organization.settings.pendingReview')}</span>
              </Tooltip>
            ) : null}
          </div>
          {showRelation ? (
            <div className="bottom-right">
              <div className={'relation-role-tag ' + relationClass}>
                <TIcon name={owner ? 'usergroup-add' : 'usergroup'} size="14px" />
                <span>{owner ? t(locale, 'organization.owner') : role ? t(locale, 'organization.role.' + role) : t(locale, 'organization.joinedByMe')}</span>
              </div>
            </div>
          ) : null}
        </div>
      </div>
    );
  });

  const emptyTitle = selection === 'created' ? t(locale, 'organization.emptyCreated') : selection === 'joined' ? t(locale, 'organization.emptyJoined') : t(locale, 'organization.empty');
  const emptyDesc = selection === 'created' ? t(locale, 'organization.emptyCreatedDesc') : selection === 'joined' ? t(locale, 'organization.emptyJoinedDesc') : t(locale, 'organization.emptyDesc');

  const roleOptions: Array<['admin' | 'editor' | 'viewer', string]> = [['viewer', 'organization.role.viewer'], ['editor', 'organization.role.editor'], ['admin', 'organization.role.admin']];
  const previewName = strOf(joinPreview?.name);
  const previewDescription = strOf(joinPreview?.description);
  const searchRowFull = (row: Record<string, unknown>) => numOf(row.member_limit) > 0 && numOf(row.member_count) >= numOf(row.member_limit);
  // Vue OrganizationSettingsModal.isAdmin requires both organization-level
  // admin/owner membership and tenant-level admin access. The list-level
  // canManageOrg check alone must not make an editor/viewer's settings form
  // writable after they open a card.
  const settingsOrgAdmin = settingsMode === 'create' || Boolean(settingsOrg && (settingsOrg.is_owner === true || settingsOrg.my_role === 'admin'));
  const settingsCanManage = canManageOrg && settingsOrgAdmin;
  const showSettingsRoleHint = settingsMode === 'edit' && settingsOrgAdmin && !canManageOrg;
  // Vue OrganizationSettingsModal canRequestUpgrade/upgradeRoleOptions: only
  // space members below admin with a tenant-admin+ role may request a role
  // upgrade, and only roles above their current space role are selectable.
  const canRequestUpgrade = canRequestUpgradeForOrg({ mode: settingsMode, myRole: strOf(settingsOrg?.my_role), tenantAdmin: canManageOrg });
  const upgradeChoices = upgradeRoleOptionsForRole(strOf(settingsOrg?.my_role));
  // Vue OrganizationSettingsModal hasPendingUpgrade/current-role bar: the org
  // detail endpoint reports an in-flight upgrade request (entry disabled,
  // organization.upgrade.pending title) and the popup always tags the current
  // space role (organization.upgrade.currentRole + organization.role.{my_role},
  // falling back to viewer like Vue's `orgInfo?.my_role || 'viewer'`).
  const upgradeCurrentRole = strOf(settingsOrg?.my_role) || 'viewer';
  const hasPendingUpgrade = boolOf(settingsOrg?.has_pending_upgrade);
  // R487 K1 — nav labels follow the Vue navItems keys (L993-1026): members
  // reads organization.manageMembers (成员管理), shared KB/agents read the same
  // keys the Vue modal uses; the standalone invite entry is gone.
  const settingsNavLabels: Record<string, string> = {
    basic: 'organization.editor.navBasic',
    permissions: 'organization.editor.navPermissions',
    members: 'organization.manageMembers',
    // Vue OrganizationSettingsModal.vue:1008 labels the nav entry with
    // t('organization.settings.joinRequests') (加入申请); 待审核申请 stays
    // reserved for the inner list title (Vue :513).
    requests: 'organization.settings.joinRequests',
    shares: 'organization.share.sharedKnowledgeBase',
    agents: 'organization.settings.sharedAgents',
  };
  const settingsNavItems: Array<[string, string]> = organizationSettingsSections(settingsMode, settingsCanManage)
    .map((key) => [key, settingsNavLabels[key]] as [string, string]);
  // Vue nav badges (L30-33): join requests badge only while pending > 0;
  // shared KB/agent totals always badge (nav-badge-count variant).
  const pendingJoinRequestCount = requests.filter((request) => request.status === 'pending').length;
  const settingsNavBadgeAlways = new Set(['shares', 'agents']);
  const settingsNavBadges: Record<string, number> = {
    requests: pendingJoinRequestCount,
    shares: sharedResources.length,
    agents: sharedAgents.length,
  };
  // Vue inviteLink computed (L1274-1277): origin + /join?code=.
  const settingsInviteLink = settingsInviteCode ? window.location.origin + '/join?code=' + settingsInviteCode : '';
  const normalizedMemberSearchQuery = memberSearchQuery.trim().toLocaleLowerCase();
  const filteredMembers = normalizedMemberSearchQuery
    ? members.filter((member) => [member.tenant_name, member.username, member.email].some((value) => strOf(value).toLocaleLowerCase().includes(normalizedMemberSearchQuery)))
    : members;
  // R488 D-B5 — member-row helpers ported from Vue OrganizationSettingsModal
  // (L1246-1272): the workspace name is the primary label (members are
  // workspaces after Plan 3), the representative username is the secondary
  // line, and owner identification is tenant-keyed with a user-id fallback.
  const memberPrimaryLabelOf = (member: OrganizationMember): string => member.tenant_name || member.username || 'tenant#' + String(member.tenant_id);
  const memberSecondaryLabelOf = (member: OrganizationMember): string => (member.tenant_name && member.username) ? member.username : '';
  const isOwnerMemberOf = (member: OrganizationMember): boolean => {
    const ownerTenantId = numOf(settingsOrg?.owner_tenant_id);
    if (ownerTenantId > 0) return member.tenant_id === ownerTenantId;
    return member.user_id === strOf(settingsOrg?.owner_id);
  };
  const feedStatus = (key: DetailFeedKey, fallback: string) => {
    const state = detailFeeds[key];
    if (state.status === 'loading') return <p className={ORG_EMPTY_INLINE}>{t(locale, 'common.loading')}</p>;
    if (state.status === 'error') return <div className="flex flex-col items-start gap-[8px] rounded-[8px] bg-[rgba(213,73,65,0.08)] px-[12px] py-[10px] text-[13px] text-[#d54941]" role="alert"><span>{state.message || fallback}</span><button type="button" className={ORG_BTN_OUTLINE} onClick={() => { if (settingsOrg) void loadOrganizationDetail(settingsOrg.id); }}>{t(locale, 'common.retry')}</button></div>;
    return null;
  };

  /* OrganizationList.vue 模板 1:1（playbook §3）：
     .org-list-container > ListSpaceSidebar(organization) + .org-list-content。 */
  return (
    <div className="org-list-container">
      <OrgListSpaceSidebar
        t={(key: string) => t(locale, key)}
        selection={selection}
        counts={{ all: organizations.length, created: createdCount, joined: joinedCount }}
        onSelect={setSelection}
      />
      <div className="org-list-content">
        <div className="header" style={{ '--wails-draggable': 'drag' } as CSSProperties}>
          <div className="header-title" style={{ '--wails-draggable': 'drag' } as CSSProperties}>
            <div className="title-row" style={{ '--wails-draggable': 'drag' } as CSSProperties}>
              <h2 style={{ '--wails-draggable': 'drag' } as CSSProperties}>{t(locale, 'organization.title')}</h2>
              {/* Vue header-actions（OrganizationList.vue:10-24）：t-tooltip +
                  纯图标 t-button，禁用态 tooltip 切 rbac 提示。 */}
              <div className="header-actions" style={{ '--wails-draggable': 'no-drag' } as CSSProperties}>
                <Tooltip content={canManageOrg ? t(locale, 'organization.joinOrg') : writeGuardTitle} placement="bottom">
                  <Button
                    variant="text"
                    theme="default"
                    size="small"
                    className="header-action-btn"
                    style={{ '--wails-draggable': 'no-drag' } as CSSProperties}
                    disabled={!canManageOrg}
                    onClick={openJoinModal}
                    icon={<TIcon name="enter" size="16px" />}
                  />
                </Tooltip>
                <Tooltip content={canManageOrg ? t(locale, 'organization.createOrg') : writeGuardTitle} placement="bottom">
                  <Button
                    variant="text"
                    theme="default"
                    size="small"
                    className="header-action-btn"
                    style={{ '--wails-draggable': 'no-drag' } as CSSProperties}
                    disabled={!canManageOrg}
                    onClick={openCreateModal}
                    icon={<img src={ORG_GREEN_SVG} className="org-create-icon" alt="" aria-hidden="true" />}
                  />
                </Tooltip>
              </div>
            </div>
            <p className="header-subtitle" style={{ '--wails-draggable': 'drag' } as CSSProperties}>{t(locale, 'organization.subtitle')}</p>
          </div>
        </div>
        <div className="org-list-main">
          {/* 骨架屏占位（Vue :30-46：t-skeleton rowCol 三段）。 */}
          {loading && ordered.length === 0 ? (
            <div className="org-card-wrap">
              {[0, 1, 2, 3].map((n) => (
                <div key={'skel-' + n} className="org-card org-card-skeleton">
                  <div className="card-header">
                    <Skeleton animation="gradient" rowCol={[[{ width: '36px', height: '36px', type: 'circle' }, { width: '50%', height: '20px' }]]} />
                  </div>
                  <div style={{ flex: 1, marginTop: '12px' }}>
                    <Skeleton animation="gradient" rowCol={[{ width: '100%', height: '14px' }, { width: '70%', height: '14px' }]} />
                  </div>
                  <div style={{ marginTop: 'auto' }}>
                    <Skeleton animation="gradient" rowCol={[[{ width: '60px', height: '22px', type: 'rect' }, { width: '60px', height: '22px', type: 'rect' }]]} />
                  </div>
                </div>
              ))}
            </div>
          ) : listError ? (
            /* React 韧性补充（Vue 模板无列表失败态）：重试入口。 */
            <div className="flex flex-col items-center justify-center px-[20px] py-[60px] text-center" role="alert">
              <IconInfoCircle size={24} />
              <p className="m-0 mt-[12px] text-[14px] text-[#d54941]">{listError}</p>
              <button type="button" className={ORG_BTN_OUTLINE + ' mt-[16px]'} onClick={() => void load()}>{t(locale, 'common.retry')}</button>
            </div>
          ) : ordered.length === 0 && !loading ? (
            /* 空状态（Vue :177-198）。 */
            <div className="empty-state">
              <img className="empty-img" src={UPLOAD_SVG_DATA} alt="" />
              <span className="empty-txt">{emptyTitle}</span>
              <span className="empty-desc">{emptyDesc}</span>
              <div className="empty-state-actions">
                <Tooltip content={writeGuardTitle} placement="top" disabled={canManageOrg}>
                  <Button theme="default" variant="outline" className="org-join-btn" disabled={!canManageOrg} onClick={openJoinModal} icon={<TIcon name="enter" />}>
                    {t(locale, 'organization.joinOrg')}
                  </Button>
                </Tooltip>
                <Tooltip content={writeGuardTitle} placement="top" disabled={canManageOrg}>
                  <Button className="org-create-btn" disabled={!canManageOrg} onClick={openCreateModal} icon={<img src={ORG_GREEN_SVG} className="org-create-icon" alt="" aria-hidden="true" />}>
                    {t(locale, 'organization.createOrg')}
                  </Button>
                </Tooltip>
              </div>
            </div>
          ) : ordered.length > 0 ? (
            <div className="org-card-wrap">{cardRows}</div>
          ) : null}
        </div>
      </div>

      {/* 创建/编辑设置弹窗 —— OrganizationSettingsModal.vue Teleport body →
          createPortal；壳层 + create 模式 basic/permissions 段为 Vue DOM 1:1
          （ix-orgs-create 扫描态），编辑模式各 section 为 R490 留守段（.section
          v-show 容器 + Tailwind 自持 internals）。 */}
      {settingsOpen ? createPortal(
        <div className="settings-overlay" onClick={(event) => { if (event.target === event.currentTarget) closeSettings(); }} role="dialog" aria-label={t(locale, settingsMode === 'create' ? 'organization.createOrg' : 'organization.settings.editTitle')}>
          <div className="settings-modal">
            <button type="button" className="close-btn" aria-label={t(locale, 'common.close')} onClick={closeSettings}>
              <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
                <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
              </svg>
            </button>
            <div className="settings-container">
              <div className="settings-sidebar">
                <div className="sidebar-header">
                  <h2 className="sidebar-title">{t(locale, settingsMode === 'create' ? 'organization.createOrg' : 'organization.settings.editTitle')}</h2>
                </div>
                <div className="settings-nav">
                  {/* Vue navGroups（L1028-1058）：nav-group-title 与 nav-item 同级
                      平铺；badge 可见性 mirrors Vue L30-33。 */}
                  {organizationSettingsNavGroups(settingsMode, settingsCanManage).flatMap((group, groupIndex) => [
                    <div key={group.key + '-title'} className="nav-group-title" style={groupIndex === 0 ? { paddingTop: '2px' } : undefined}>{t(locale, group.titleKey)}</div>,
                    ...group.items.filter((key) => settingsNavLabels[key]).map((key) => {
                      const badge = settingsNavBadges[key];
                      const showBadge = badge !== undefined && (settingsNavBadgeAlways.has(key) || badge > 0);
                      return (
                        <div key={key} className={'nav-item' + (settingsSection === key ? ' active' : '')} onClick={() => setSettingsSection(key)}>
                          <TIcon name={ORG_NAV_ICON_BY_KEY[key] ?? 'info-circle'} className="nav-icon" />
                          <span className="nav-label">{t(locale, settingsNavLabels[key])}</span>
                          {showBadge ? <span data-nav-badge className={'nav-badge' + (settingsNavBadgeAlways.has(key) ? ' nav-badge-count' : '')}>{badge}</span> : null}
                        </div>
                      );
                    }),
                  ])}
                </div>
              </div>
              <div className="settings-content">
                {/* React 韧性补充（<720px 时侧栏不可见时的等价 section 切换），
                    Vue 无对应物；桌面扫描态 display:none 零像素影响。 */}
                <div className="hidden border-b border-[#e7e7ea] px-4 pb-3 pt-4 max-[720px]:block">
                  <TSelect
                    className="organization-settings-section-selector min-h-[34px]"
                    value={settingsSection}
                    onChange={(value) => setSettingsSection(String(value))}
                  options={settingsNavItems.map(([key, labelKey]) => ({ value: key, label: t(locale, labelKey) }))} />
                </div>
                <div className="content-wrapper">
                  {showSettingsRoleHint ? (
                    <div className="tenant-role-hint">
                      <TIcon name="info-circle" size="16px" />
                      <span>{writeGuardTitle}</span>
                    </div>
                  ) : null}
                  {settingsMode === 'create' ? (
                    <>
                      {/* 基本信息（Vue :48-226，v-show 语义保留 DOM）。 */}
                      <div className="section" style={{ display: settingsSection === 'basic' ? undefined : 'none' }}>
                        <div className="section-header">
                          <h2>{t(locale, 'organization.editor.basicTitle')}</h2>
                          <p className="section-description">{t(locale, 'organization.editor.basicDesc')}</p>
                        </div>
                        {/* React 保留：form 包装仅为 create 提交契约（Vue 走 footer
                            handleSave），display 中性无像素影响。 */}
                        <form id="organization-create-form" onSubmit={submitCreate}>
                        <div className="settings-group">
                          <div className="setting-row">
                            <div className="setting-info">
                              <label htmlFor="organization-name">{t(locale, 'organization.name')}{' '}<span className="required">*</span></label>
                              <p className="desc">{t(locale, 'organization.editor.nameTip')}</p>
                            </div>
                            <div className="setting-control">
                              <div className="name-input-wrapper">
                                {/* 头像 emoji 弹层（Vue :63-86 t-popup）。 */}
                                <Popup
                                  visible={avatarPickerOpen}
                                  trigger="click"
                                  placement="bottom-left"
                                  disabled={!settingsCanManage}
                                  overlayClassName="avatar-emoji-popover"
                                  onVisibleChange={(visible: boolean) => setAvatarPickerOpen(visible)}
                                  content={(
                                    <div className="avatar-popover-content" onClick={(event) => event.stopPropagation()}>
                                      <p className="avatar-popover-title">{t(locale, 'organization.avatarPickerHint')}</p>
                                      <div className="avatar-emoji-grid">
                                        {ORG_AVATAR_EMOJIS.map((emoji) => (
                                          <button key={emoji} type="button" className={'avatar-emoji-btn' + (formAvatar === 'emoji:' + emoji ? ' is-selected' : '')} onClick={() => { setFormAvatar('emoji:' + emoji); setAvatarPickerOpen(false); }}>
                                            {emoji}
                                          </button>
                                        ))}
                                      </div>
                                      {formAvatar ? (
                                        <Button variant="text" size="small" className="avatar-clear-btn" onClick={() => { setFormAvatar(''); setAvatarPickerOpen(false); }}>
                                          {t(locale, 'organization.avatarClear')}
                                        </Button>
                                      ) : null}
                                    </div>
                                  )}
                                >
                                  <div className="avatar-trigger-wrap">
                                    <SpaceAvatar name={formName || '?'} avatar={formAvatar} size="medium" />
                                    {settingsCanManage ? <span className="avatar-change-hint">{t(locale, 'organization.avatar')}</span> : null}
                                  </div>
                                </Popup>
                                <Input name="organization-name" className="name-input" value={formName} onChange={(value: string) => setFormName(value)} placeholder={t(locale, 'organization.namePlaceholder')} />
                              </div>
                            </div>
                          </div>
                          <div className="setting-row">
                            <div className="setting-info">
                              <label>{t(locale, 'organization.description')}</label>
                              <p className="desc">{t(locale, 'organization.editor.descriptionTip')}</p>
                            </div>
                            <div className="setting-control">
                              {/* count render-prop：单模板字符串 child＝单文本节点整串 shaping，
                                  复刻 vue-next 计数器 DOM（台账 #13 处置；默认渲染是
                                  "0"+"/500" 两文本节点，跨节点 kern 丢失致字形相位差）。 */}
                              <Textarea name="organization-description" value={formDescription} onChange={(value: string) => setFormDescription(value)} placeholder={t(locale, 'organization.descriptionPlaceholder')} autosize={{ minRows: 3, maxRows: 6 }} maxlength={500} count={({ count, maxLength }) => <span className="t-textarea__limit">{`${count}/${maxLength}`}</span>} />
                            </div>
                          </div>
                        </div>
                        </form>
                      </div>
                      {/* 创建空间 - 权限说明（Vue :228-295）。 */}
                      <div className="section" style={{ display: settingsSection === 'permissions' ? undefined : 'none' }}>
                        <div className="section-header">
                          <h2>{t(locale, 'organization.editor.permissionsTitle')}</h2>
                          <p className="section-description">{t(locale, 'organization.editor.permissionsDesc')}</p>
                        </div>
                        <div className="permissions-info">
                          {(['admin', 'editor', 'viewer'] as const).map((roleKey) => (
                            <div key={roleKey} className="permission-card">
                              <div className="permission-header">
                                <div className={'permission-icon ' + roleKey}>
                                  <TIcon name={roleKey === 'admin' ? 'user-safety' : roleKey === 'editor' ? 'edit' : 'browse'} />
                                </div>
                                <div className="permission-title">
                                  <span className="role-name">{t(locale, 'organization.role.' + roleKey)}</span>
                                  <Tag size="small" theme={roleKey === 'admin' ? 'primary' : roleKey === 'editor' ? 'warning' : 'default'}>{t(locale, roleKey === 'admin' ? 'organization.editor.fullAccess' : roleKey === 'editor' ? 'organization.editor.editAccess' : 'organization.editor.viewAccess')}</Tag>
                                </div>
                              </div>
                              <ul className="permission-list">
                                {ORG_PERMISSION_ITEMS[roleKey].map(([permKey, allowed]) => (
                                  <li key={permKey}><TIcon name={allowed ? 'check' : 'close'} className={allowed ? 'check-icon' : 'close-icon'} />{t(locale, permKey)}</li>
                                ))}
                              </ul>
                            </div>
                          ))}
                        </div>
                        <div className="info-notice">
                          <TIcon name="info-circle" />
                          <span>{t(locale, 'organization.editor.ownerNote')}</span>
                        </div>
                      </div>
                    </>
                  ) : settingsSection === 'basic' ? (
                    <>
                      <form onSubmit={submitBasic}>
                        <h2 className={ORG_SECTION_TITLE}>{t(locale, 'organization.editor.basicTitle')}</h2>
                        <p className={ORG_SECTION_DESC}>{t(locale, 'organization.editor.basicDesc')}</p>
                        {/* Vue name-input-wrapper (L62-89): the avatar emoji
                            picker sits on the name row in edit mode too. */}
                        <div className={ORG_FORM_ITEM}>
                          <label className={ORG_FORM_LABEL} htmlFor="organization-name">{t(locale, 'organization.name')} *</label>
                          {/* R488 D-B4.1 — Vue keeps the field hints in edit mode
                              too (setting-info .desc, Vue :59). */}
                          <p className={ORG_FORM_DESC + ' mb-[6px]'}>{t(locale, 'organization.editor.nameTip')}</p>
                          <div className="flex min-w-0 items-center gap-3">
                            <div className="relative flex shrink-0 flex-col items-center gap-1">
                              <button type="button" className="cursor-pointer rounded-lg border-0 bg-transparent p-0 disabled:cursor-not-allowed disabled:opacity-55" aria-label={t(locale, 'organization.avatarPickerHint')} onClick={() => setAvatarPickerOpen((open) => !open)} disabled={!settingsCanManage}><SpaceAvatar name={formName || '?'} avatar={formAvatar} size="medium" /></button>
                              {settingsCanManage ? <span className="text-[12px] text-[rgba(23,26,29,0.4)]">{t(locale, 'organization.avatar')}</span> : null}
                              {avatarPickerOpen && settingsCanManage ? <div className="absolute left-0 top-[64px] z-20 grid w-[220px] grid-cols-6 gap-1 rounded-lg border border-[#e7e7ea] bg-surface p-2 shadow-[0_8px_24px_rgba(0,0,0,0.12)]">{ORG_AVATAR_EMOJIS.map((emoji) => <button type="button" key={emoji} className="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-base hover:bg-[#f3f3f5]" aria-label={emoji} onClick={() => { setFormAvatar('emoji:' + emoji); setAvatarPickerOpen(false); }}>{emoji}</button>)}{formAvatar ? <button type="button" className="col-span-6 border-0 bg-transparent py-1 text-xs text-muted hover:bg-[#f3f3f5]" onClick={() => { setFormAvatar(''); setAvatarPickerOpen(false); }}>{t(locale, 'organization.avatarClear')}</button> : null}</div> : null}
                            </div>
                            <input id="organization-name" name="organization-name" className={ORG_FIELD + ' min-h-[34px] min-w-0 flex-1'} value={formName} onChange={(event) => setFormName(event.target.value)} required disabled={!settingsCanManage} />
                          </div>
                        </div>
                        <div className={ORG_FORM_ITEM}>
                          <label className={ORG_FORM_LABEL} htmlFor="organization-description">{t(locale, 'organization.description')}</label>
                          {/* R488 D-B4.1 — the description hint survives edit mode
                              like Vue (setting-info .desc, Vue :97). */}
                          <p className={ORG_FORM_DESC + ' mb-[6px]'}>{t(locale, 'organization.editor.descriptionTip')}</p>
                          {/* Vue t-textarea :maxlength="500" (L102) — the edit
                              mode shows the same 0/500 counter as create. */}
                          <div className="min-w-0">
                            <TTextarea id="organization-description" name="organization-description" className={ORG_FIELD + ' min-h-[72px] resize-y'} rows={3} maxLength={500} value={formDescription} onChange={(value) => setFormDescription(String(value))} disabled={!settingsCanManage} />
                            <p className="m-0 mt-1 text-right text-[12px] text-[rgba(23,26,29,0.4)]">{formDescription.length}/500</p>
                          </div>
                        </div>
                      </form>
                      {/* 邀请成员 card — Vue v-if="isAdmin && orgId" (L107-220):
                          all six invite control groups live INSIDE basic, not
                          behind a standalone nav item. */}
                      {settingsCanManage && settingsOrg ? (
                        <div className="mt-[8px] border-t border-dashed border-[#e7e7ea] pt-[16px]">
                          <h3 className={ORG_SECTION_TITLE}>{t(locale, 'organization.settings.inviteMembers')}</h3>
                          <p className={ORG_SECTION_DESC}>{t(locale, 'organization.settings.inviteMembersDesc')}</p>
                          <div className="flex flex-col rounded-[8px] border border-[#e7e7ea] bg-[#f9f9f9]">
                            {/* ① 邀请码：值 + 复制 + 刷新 + 剩余有效期 */}
                            <div className="flex flex-col gap-[6px] border-b border-[#e7e7ea] px-[12px] py-[10px]">
                              <strong className="text-[13px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.inviteCode')}</strong>
                              <div className="flex min-w-0 items-center gap-[8px]">
                                <code className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap rounded-[6px] border border-[#e7e7ea] bg-surface px-[10px] py-[6px] text-[13px] text-[rgba(23,26,29,0.92)]">{settingsInviteCode || '—'}</code>
                                {/* R488 D-B4.2 — Vue t-button variant=text icon-only with a tooltip (L123-127). */}
                                <button type="button" className="box-border inline-flex h-[30px] w-[30px] shrink-0 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent p-0 text-[rgba(23,26,29,0.6)] hover:bg-[#f3f3f5] hover:text-accent disabled:cursor-not-allowed disabled:opacity-55" aria-label={t(locale, 'common.copy')} title={t(locale, 'common.copy')} disabled={!settingsInviteCode} onClick={() => { void copyText(settingsInviteCode).then((copied) => { if (copied) showToast('success', t(locale, 'common.copied')); }); }}><IconCopy size={15} /></button>
                                <button type="button" className="box-border inline-flex h-[30px] w-[30px] shrink-0 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent p-0 text-[rgba(23,26,29,0.6)] hover:bg-[#f3f3f5] hover:text-accent disabled:cursor-not-allowed disabled:opacity-55" aria-label={t(locale, 'organization.refreshInviteCode')} title={t(locale, 'organization.refreshInviteCode')} disabled={refreshingCode} onClick={() => void refreshInviteCode()}><span className={refreshingCode ? 'inline-flex animate-[orgSpin_1s_linear_infinite]' : 'inline-flex'}><IconRefresh size={15} /></span></button>
                              </div>
                              {settingsInviteCode ? <p className="m-0 text-[12px] text-[rgba(23,26,29,0.6)]">{remainingValidityText(locale, inviteCodeExpiresAt)}</p> : null}
                            </div>
                            {/* ② 邀请链接有效期：立即保存 */}
                            <div className="flex flex-col gap-[6px] border-b border-[#e7e7ea] px-[12px] py-[10px]">
                              <strong className="text-[13px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.settings.inviteLinkValidity')}</strong>
                              <p className="m-0 text-[12px] leading-[1.5] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.settings.inviteLinkValidityDesc')}</p>
                              {/* R488 D-B4.3 — Vue t-select renders only the
                                  selected label until opened; a native select
                                  leaks every option into the DOM text. */}
                              <div className="relative w-[220px] max-w-full">
                                <button type="button" className={ORG_FIELD + ' flex min-h-[30px] cursor-pointer items-center justify-between gap-[6px] text-left'} aria-label={t(locale, 'organization.settings.inviteLinkValidity')} aria-expanded={validityPopupOpen} aria-haspopup="listbox" onClick={() => setValidityPopupOpen((open) => !open)}>
                                  {t(locale, (ORG_INVITE_VALIDITY_OPTIONS.find(([value]) => value === formValidityDays) ?? ORG_INVITE_VALIDITY_OPTIONS[1])[1])}
                                  <IconChevron size={14} direction="down" />
                                </button>
                                {validityPopupOpen ? (
                                  <div role="listbox" aria-label={t(locale, 'organization.settings.inviteLinkValidity')} className="absolute left-0 top-[34px] z-30 w-full overflow-hidden rounded-[6px] border border-[#e7e7ea] bg-surface py-[4px] shadow-[0_8px_24px_rgba(0,0,0,0.12)]">
                                    {ORG_INVITE_VALIDITY_OPTIONS.map(([value, labelKey]) => (
                                      <button type="button" key={value} role="option" aria-selected={value === formValidityDays} className={'block w-full cursor-pointer border-0 bg-transparent px-[10px] py-[6px] text-left text-[13px] text-[rgba(23,26,29,0.92)] hover:bg-[#f3f3f5] ' + (value === formValidityDays ? 'font-semibold text-accent' : '')} onClick={() => { setValidityPopupOpen(false); void handleValidityChange(value); }}>{t(locale, labelKey)}</button>
                                    ))}
                                  </div>
                                ) : null}
                              </div>
                            </div>
                            {/* ③ 邀请链接：/join?code= + 复制 */}
                            <div className="flex flex-col gap-[6px] border-b border-[#e7e7ea] px-[12px] py-[10px]">
                              <strong className="text-[13px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.settings.inviteLink')}</strong>
                              <div className="flex min-w-0 items-center gap-[8px]">
                                <code className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap rounded-[6px] border border-[#e7e7ea] bg-surface px-[10px] py-[6px] text-[13px] text-[rgba(23,26,29,0.92)] [overflow-wrap:anywhere]">{settingsInviteLink || '—'}</code>
                                {/* R488 D-B4.2 — icon-only copy like Vue (L164-168). */}
                                <button type="button" className="box-border inline-flex h-[30px] w-[30px] shrink-0 cursor-pointer items-center justify-center rounded-[3px] border-0 bg-transparent p-0 text-[rgba(23,26,29,0.6)] hover:bg-[#f3f3f5] hover:text-accent disabled:cursor-not-allowed disabled:opacity-55" aria-label={t(locale, 'common.copy')} title={t(locale, 'common.copy')} disabled={!settingsInviteLink} onClick={() => { void copyText(settingsInviteLink).then((copied) => { if (copied) showToast('success', t(locale, 'common.copied')); }); }}><IconCopy size={15} /></button>
                              </div>
                            </div>
                            {/* ④ 需要审核：立即保存 */}
                            <div className="flex items-start justify-between gap-[12px] border-b border-[#e7e7ea] px-[12px] py-[10px]">
                              <div className="flex min-w-0 flex-col gap-[2px]">
                                <strong className="text-[13px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.settings.requireApproval')}</strong>
                                <span className="text-[12px] leading-[1.5] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.settings.requireApprovalDesc')}</span>
                              </div>
                              <TSwitch aria-label={t(locale, 'organization.settings.requireApproval')} value={formRequireApproval} onChange={(value) => void handleApprovalToggle(Boolean(value))} />
                            </div>
                            {/* ⑤ 开放可被搜索：立即保存 */}
                            <div className="flex items-start justify-between gap-[12px] border-b border-[#e7e7ea] px-[12px] py-[10px]">
                              <div className="flex min-w-0 flex-col gap-[2px]">
                                <strong className="text-[13px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.settings.searchable')}</strong>
                                <span className="text-[12px] leading-[1.5] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.settings.searchableDesc')}</span>
                              </div>
                              <TSwitch aria-label={t(locale, 'organization.settings.searchable')} value={formSearchable} onChange={(value) => void handleSearchableToggle(Boolean(value))} />
                            </div>
                            {/* ⑥ 成员数量上限：0-10000 + 当前成员数 hint */}
                            <div className="flex flex-col gap-[6px] px-[12px] py-[10px]">
                              <strong className="text-[13px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.settings.memberLimit')}</strong>
                              <p className="m-0 text-[12px] leading-[1.5] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.settings.memberLimitDesc')}</p>
                              <div className="flex items-center gap-[10px]">
                                {/* R488 D-B4.3 — Vue t-input-number theme="normal"
                                    carries no stepper column; the ▲▼ glyphs were
                                    scrape noise. Clamping keeps the 0-10000
                                    contract (submitBasic payload unchanged). */}
                                <TInput type="number" aria-label={t(locale, 'organization.settings.memberLimit')} className={ORG_FIELD + ' min-h-[30px] w-[140px]'} value={formMemberLimit === '' ? '' : String(formMemberLimit)} placeholder={t(locale, 'organization.settings.memberLimitPlaceholder')} onChange={(value) => { const raw = String(value); if (raw === '') { setFormMemberLimit(''); return; } const next = Math.min(10000, Math.max(0, Math.round(Number(raw)))); if (Number.isSafeInteger(next)) setFormMemberLimit(next); }} />
                                <span className="text-[12px] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.settings.memberLimitHint', { count: numOf(settingsOrg?.member_count) })}</span>
                              </div>
                            </div>
                          </div>
                        </div>
                      ) : null}
                    </>
                  ) : settingsSection === 'members' ? (
                    <>
                      {/* Vue members section header (L299-340): h2 reads
                          organization.manageMembers (成员管理) with the
                          permission-matrix info trigger beside it; the shared
                          共享空间成员 wording stays on the INNER list title. */}
                      <div className="mb-[16px] flex flex-wrap items-start justify-between gap-[12px]">
                        <div>
                          <div className="flex items-center gap-[8px]">
                            <h2 className={ORG_SECTION_TITLE + ' mb-0'}>{t(locale, 'organization.manageMembers')}</h2>
                            <span className="relative inline-flex">
                              <button type="button" className="flex h-[22px] w-[22px] cursor-pointer items-center justify-center rounded-full border-0 bg-transparent p-0 text-[rgba(23,26,29,0.45)] hover:bg-[#f3f3f5] hover:text-accent" aria-label={t(locale, 'organization.editor.permissionsTitle')} title={t(locale, 'organization.settings.permissionsIconHint')} aria-expanded={permissionsPopupOpen} onClick={() => setPermissionsPopupOpen((open) => !open)}><IconInfoCircle size={16} /></button>
                              {permissionsPopupOpen ? (
                                <div className="absolute left-[26px] top-0 z-30 w-[min(520px,calc(100vw-24px))] overflow-hidden rounded-[8px] border border-[#e7e7ea] bg-surface shadow-[0_8px_24px_rgba(0,0,0,0.12)]">
                                  <div className="border-b border-[#e7e7ea] px-[14px] py-[10px]">
                                    <div className="text-[14px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.editor.permissionsTitle')}</div>
                                    <div className="text-[12px] leading-[1.5] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.editor.permissionsDesc')}</div>
                                  </div>
                                  <div className="grid grid-cols-1 gap-[8px] p-[10px] min-[560px]:grid-cols-3">
                                    {ORG_ROLE_MATRIX.map(({ role, perms }) => (
                                      <div key={role} className={'rounded-[8px] border px-[10px] py-[8px] ' + (strOf(settingsOrg?.my_role) === role ? 'border-accent bg-accent-wash' : 'border-[#e7e7ea] bg-[#f9f9f9]')}>
                                        <div className="mb-[6px] flex items-center gap-[4px] text-[13px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.role.' + role)}{strOf(settingsOrg?.my_role) === role ? <span className="rounded-[4px] bg-accent-soft px-[4px] text-[11px] font-medium text-accent">{t(locale, 'common.me')}</span> : null}</div>
                                        <div className="flex flex-col gap-[3px]">
                                          {perms.map(([permKey, allowed]) => (
                                            <span key={permKey} className={'text-[12px] leading-[1.45] ' + (allowed ? 'text-[rgba(23,26,29,0.82)]' : 'text-[rgba(23,26,29,0.4)]')}><span className="mr-[4px]" aria-hidden="true">{allowed ? '✓' : '✗'}</span>{t(locale, permKey)}</span>
                                          ))}
                                        </div>
                                      </div>
                                    ))}
                                  </div>
                                </div>
                              ) : null}
                            </span>
                          </div>
                          <p className={ORG_SECTION_DESC}>{t(locale, 'organization.settings.membersDesc')}</p>
                        </div>
                      </div>
                      {/* Vue members-list-header (L343-448): the INNER list keeps
                          the 共享空间成员 wording plus a live count badge on the
                          left; the right side carries the member search and —
                          for managing admins — the add-member icon button with
                          its popup (R488 D-B5: no resident form anymore). */}
                      <div className="mb-[8px] flex flex-wrap items-center justify-between gap-[12px]">
                        <div className="flex items-center gap-[8px]">
                          <span className="text-[14px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.members.listTitle')}</span>
                          <span className="inline-flex min-w-[24px] items-center justify-center rounded-full bg-accent-wash px-[7px] py-[2px] text-[12px] font-medium text-accent" aria-label={t(locale, 'organization.members.listTitle') + ' count'}>{filteredMembers.length}</span>
                        </div>
                        <div className="flex items-center gap-[8px]">
                          {detailFeeds.members.status === 'ready' && members.length > 0 ? <TInput className={ORG_FIELD + ' min-h-[30px] w-[min(100%,200px)]'} aria-label={t(locale, 'organization.members.listTitle')} placeholder={t(locale, 'organization.members.searchPlaceholder')} value={memberSearchQuery} onChange={(value) => setMemberSearchQuery(String(value))} /> : null}
                          {settingsCanManage ? (
                            <div className="relative">
                              <button type="button" className="box-border inline-flex h-[30px] w-[30px] cursor-pointer items-center justify-center rounded-[3px] border border-[rgba(7,192,95,0.5)] bg-surface text-accent [transition:all_.2s_ease] hover:border-accent hover:bg-accent-wash" aria-label={t(locale, 'organization.addMember.button')} title={t(locale, 'organization.addMember.button')} aria-expanded={addMemberPopupOpen} onClick={() => { setAddMemberPopupOpen((open) => !open); setSelectedInviteTenant(null); }}><IconUsergroupAdd size={16} /></button>
                              {addMemberPopupOpen ? (
                                <div className="absolute right-0 top-[36px] z-30 w-[min(340px,calc(100vw-48px))] rounded-[8px] border border-[#e7e7ea] bg-surface p-[14px] text-left shadow-[0_8px_24px_rgba(0,0,0,0.12)]">
                                  <div className="mb-[4px] text-[14px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.addMember.dialogTitle')}</div>
                                  <p className="m-0 mb-[12px] text-[12px] leading-[1.5] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.addMember.tipTenant')}</p>
                                  <div className="mb-[12px]">
                                    <label className={ORG_FORM_LABEL + ' mb-[4px]'}>{t(locale, 'organization.addMember.searchTenant')}</label>
                                    <TInput className={ORG_FIELD + ' min-h-[30px]'} aria-label={t(locale, 'organization.addMember.searchTenant')} value={memberInviteQuery} onChange={(value) => void searchMemberInviteCandidates(String(value))} placeholder={t(locale, 'organization.addMember.searchTenantPlaceholder')} />
                                    <p className="m-0 mt-[4px] text-[12px] leading-[1.5] text-[rgba(23,26,29,0.4)]">{t(locale, 'organization.addMember.searchTenantHint')}</p>
                                    {memberInviteLoading ? <p className={ORG_EMPTY_INLINE}>{t(locale, 'common.loading')}</p> : memberInviteCandidates.map((candidate) => {
                                      const candidateId = String(candidate.tenant_id);
                                      const selected = selectedInviteTenant != null && String(selectedInviteTenant.tenant_id) === candidateId;
                                      return (
                                        <button type="button" key={candidateId} className={'mb-[6px] flex w-full cursor-pointer flex-col items-start gap-[2px] rounded-[6px] border px-[10px] py-[8px] text-left last:mb-0 ' + (selected ? 'border-accent bg-accent-wash' : 'border-[#e7e7ea] bg-surface hover:border-[#c9c9cf]')} aria-pressed={selected} onClick={() => setSelectedInviteTenant(candidate)}>
                                          <strong className="text-[13px] font-semibold text-[rgba(23,26,29,0.92)]">{strOf(candidate.tenant_name) || ('tenant#' + candidateId)}</strong>
                                          <span className="text-[12px] text-[rgba(23,26,29,0.6)]">{strOf(candidate.representative_username) || strOf(candidate.representative_email)}</span>
                                        </button>
                                      );
                                    })}
                                  </div>
                                  <div className="mb-[12px]">
                                    <label className={ORG_FORM_LABEL + ' mb-[4px]'}>{t(locale, 'organization.addMember.selectRole')}</label>
                                    <TSelect className={ORG_FIELD + ' min-h-[30px]'} aria-label={t(locale, 'organization.addMember.selectRole')} value={memberInviteRole} options={roleOptions.map(([value, labelKey]) => ({ value, label: t(locale, labelKey) }))} onChange={(value) => setMemberInviteRole(String(value) as 'admin' | 'editor' | 'viewer')} />
                                  </div>
                                  <div className="flex items-center justify-end gap-[8px]">
                                    <button type="button" className={ORG_BTN_OUTLINE + ' min-h-[30px] px-[12px] text-[13px]'} onClick={() => setAddMemberPopupOpen(false)}>{t(locale, 'common.cancel')}</button>
                                    <button type="button" className={ORG_BTN_PRIMARY + ' min-h-[30px] px-[12px] text-[13px]'} disabled={selectedInviteTenant == null || memberInviteSaving != null} onClick={() => { if (selectedInviteTenant) void inviteMember(selectedInviteTenant); }}>{t(locale, 'organization.addMember.confirmBtn')}</button>
                                  </div>
                                </div>
                              ) : null}
                            </div>
                          ) : null}
                        </div>
                      </div>
                      {feedStatus('members', t(locale, 'organization.memberRemoveFailed'))}
                      {detailFeeds.members.status === 'ready' && members.length === 0 ? <p className={ORG_EMPTY_INLINE}>{t(locale, 'organization.noMembers')}</p> : null}
                      {detailFeeds.members.status === 'ready' && members.length > 0 && filteredMembers.length === 0 ? <p className={ORG_EMPTY_INLINE}>{t(locale, 'organization.members.emptySearch').replace('{q}', memberSearchQuery.trim())}</p> : null}
                      {detailFeeds.members.status === 'ready' && filteredMembers.length > 0 ? (
                        /* R488 D-B5 — the member list is a table mirroring Vue
                         * memberColumns (L1124-1134): 成员/角色/加入时间 plus
                         * 操作 for managing admins; the owner row renders
                         * static role/创建者/我 badges (L462-497). */
                        <div className="overflow-hidden rounded-[8px] border border-[#e7e7ea]">
                          <table className={MEMBER_TABLE}>
                            <thead>
                              <tr>
                                <th className={MEMBER_TH}>{t(locale, 'organization.members.columns.member')}</th>
                                <th className={MEMBER_TH + ' w-[132px]'}>{t(locale, 'organization.members.columns.role')}</th>
                                <th className={MEMBER_TH + ' w-[154px]'}>{t(locale, 'organization.members.columns.joinedAt')}</th>
                                {settingsCanManage ? <th className={MEMBER_TH + ' w-[96px]'}>{t(locale, 'organization.members.columns.operations')}</th> : null}
                              </tr>
                            </thead>
                            <tbody>
                              {filteredMembers.map((member) => {
                                const memberIsOwner = isOwnerMemberOf(member);
                                const secondary = memberSecondaryLabelOf(member);
                                return (
                                  <tr key={member.id}>
                                    <td className={MEMBER_TD}>
                                      <div className="flex min-w-0 flex-col gap-[2px]">
                                        <span className="flex min-w-0 items-center gap-[6px] text-[14px] font-medium text-[rgba(23,26,29,0.92)]">
                                          <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{memberPrimaryLabelOf(member)}</span>
                                          {memberIsOwner ? <span className="inline-flex h-[16px] shrink-0 items-center rounded-[3px] bg-accent-wash px-[5px] text-[10px] font-medium text-accent">{t(locale, 'organization.owner')}</span> : null}
                                          {member.user_id === currentUserId ? <span className="inline-flex h-[16px] shrink-0 items-center rounded-[3px] bg-accent px-[5px] text-[10px] font-medium text-white">{t(locale, 'common.me')}</span> : null}
                                        </span>
                                        {secondary ? <span className="text-[12px] leading-[1.35] text-[rgba(23,26,29,0.6)]">{secondary}</span> : null}
                                      </div>
                                    </td>
                                    <td className={MEMBER_TD}>
                                      {settingsCanManage && !memberIsOwner ? (
                                        <TSelect className={'org-member-role-sel ' + ORG_FIELD + ' min-h-[28px] w-[116px]!'} value={member.role} options={roleOptions.map(([value, labelKey]) => ({ value, label: t(locale, labelKey) }))} onChange={(value) => void updateMemberRole(member, String(value) as 'admin' | 'editor' | 'viewer')} />
                                      ) : (
                                        <span className={MEMBER_ROLE_TAG + ' ' + (MEMBER_ROLE_TAG_TONES[member.role] ?? MEMBER_ROLE_TAG_TONES.viewer)}>{t(locale, 'organization.role.' + member.role)}</span>
                                      )}
                                    </td>
                                    <td className={MEMBER_TD + ' text-[13px] text-[rgba(23,26,29,0.82)]'}>{formatDateYmd(strOf(member.joined_at))}</td>
                                    {settingsCanManage ? <td className={MEMBER_TD}>{memberIsOwner ? null : <button type="button" className={ORG_BTN_NEUTRAL + ' min-h-[28px] px-[10px] text-[12px]'} onClick={() => void removeMember(member)}>{t(locale, 'common.remove')}</button>}</td> : null}
                                  </tr>
                                );
                              })}
                            </tbody>
                          </table>
                        </div>
                      ) : null}
                      {/* R487 K1 — the upgrade entry lives in the members
                          section (Vue :357-390 popup on the members header),
                          not in basic. */}
                      {canRequestUpgrade ? <form onSubmit={submitUpgradeRequest} style={{ marginTop: '24px', borderTop: '1px dashed #e7e7ea', paddingTop: '16px' }}>
                        <h3 className={ORG_SECTION_TITLE}>{t(locale, 'organization.upgrade.requestUpgrade')}</h3>
                        <div className="mb-[16px] flex items-center gap-[8px]">
                          <span className="text-[13px] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.upgrade.currentRole')}</span>
                          <span className={RELATION_ROLE_TAG + ' ' + (RELATION_ROLE_TAG_TONES[upgradeCurrentRole] ?? 'bg-[rgba(107,114,128,0.08)] text-[rgba(23,26,29,0.6)]')}>{t(locale, 'organization.role.' + upgradeCurrentRole)}</span>
                        </div>
                        <div className={ORG_FORM_ITEM}>
                          <label className={ORG_FORM_LABEL} htmlFor="upgrade-role">{t(locale, 'organization.upgrade.selectRole')}</label>
                          <TSelect className={ORG_FIELD + ' min-h-[34px]'} value={upgradeRole} options={upgradeChoices.map((value) => ({ value, label: t(locale, 'organization.role.' + value) }))} onChange={(value) => setUpgradeRole(String(value) as 'admin' | 'editor' | 'viewer')} />
                        </div>
                        <div className={ORG_FORM_ITEM}>
                          <label className={ORG_FORM_LABEL} htmlFor="upgrade-note">{t(locale, 'organization.upgrade.reason')}</label>
                          <TTextarea id="upgrade-note" className={ORG_FIELD + ' min-h-[72px] resize-y'} rows={2} maxLength={500} value={upgradeNote} onChange={(value) => setUpgradeNote(clampApplicationNote(String(value)))} placeholder={t(locale, 'organization.upgrade.reasonPlaceholder')} />
                        </div>
                        <button type="submit" className={ORG_BTN_OUTLINE} disabled={hasPendingUpgrade} title={hasPendingUpgrade ? t(locale, 'organization.upgrade.pending') : undefined} aria-label={hasPendingUpgrade ? t(locale, 'organization.upgrade.pending') : undefined}>{t(locale, 'organization.upgrade.submitBtn')}</button>
                      </form> : null}
                    </>
                  ) : settingsSection === 'requests' ? (
                    <>
                      {/* Vue OrganizationSettingsModal.vue:505-516 — the section
                          heading is 加入申请 (+ description); 待审核申请 with a
                          live count is the inner list title below it. */}
                      <h2 className={ORG_SECTION_TITLE}>{t(locale, 'organization.settings.joinRequests')}</h2>
                      <p className={ORG_SECTION_DESC}>{t(locale, 'organization.settings.joinRequestsDesc')}</p>
                      <div className="mb-[8px] flex items-center gap-[8px]">
                        <span className="text-[14px] font-semibold text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.joinRequests.listTitle')}</span>
                        <span className="inline-flex min-w-[24px] items-center justify-center rounded-full bg-accent-wash px-[7px] py-[2px] text-[12px] font-medium text-accent" aria-label={t(locale, 'organization.joinRequests.listTitle') + ' count'}>{requests.filter((request) => request.status === 'pending').length}</span>
                      </div>
                      {feedStatus('requests', t(locale, 'organization.settings.reviewFailed'))}
                      {detailFeeds.requests.status === 'ready' && requests.filter((request) => request.status === 'pending').length === 0 ? <p className={ORG_EMPTY_INLINE}>{t(locale, 'organization.settings.noPendingRequests')}</p> : null}
                      {detailFeeds.requests.status === 'ready' ? requests.filter((request) => request.status === 'pending').map((request) => (
                        <div key={request.id} className={ORG_MEMBER_ROW}>
                          <div className={ORG_MEMBER_COPY}>
                            <strong className="text-[14px] font-semibold text-[rgba(23,26,29,0.92)]">{request.username}</strong>
                            <span className="text-[12px] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.joinRequests.columns.requestedRole')}: {t(locale, 'organization.role.' + request.requested_role)}</span>
                          </div>
                          <div className={ORG_ROW_ACTIONS}>
                            {settingsCanManage ? <><button type="button" className={ORG_BTN_OUTLINE} onClick={() => void reviewRequest(request, true)}>{t(locale, 'organization.settings.approve')}</button><button type="button" className={ORG_BTN_NEUTRAL} onClick={() => void reviewRequest(request, false)}>{t(locale, 'organization.settings.reject')}</button></> : null}
                          </div>
                        </div>
                      )) : null}
                    </>
                  ) : settingsSection === 'shares' ? (
                    <>
                      {/* Vue shares header (J3 #7): h2 reads
                          organization.share.sharedKnowledgeBase with the
                          organization.settings.sharedDesc description. */}
                      <h2 className={ORG_SECTION_TITLE}>{t(locale, 'organization.share.sharedKnowledgeBase')}</h2>
                      <p className={ORG_SECTION_DESC}>{t(locale, 'organization.settings.sharedDesc')}</p>
                      {feedStatus('shares', t(locale, 'organization.settings.removeShareFailed'))}
                      {detailFeeds.shares.status === 'ready' && sharedResources.length === 0 ? <p className={ORG_EMPTY_INLINE}>{t(locale, 'organization.settings.noSharedKB')}</p> : null}
                      {detailFeeds.shares.status === 'ready' ? sharedResources.map((resource, index) => {
                        const row = sharedResourceRow(resource);
                        return (
                          <div key={row.shareId || index} className={ORG_MEMBER_ROW}>
                            <div className={ORG_MEMBER_COPY}>
                              <strong className="text-[14px] font-semibold text-[rgba(23,26,29,0.92)]">{row.name}</strong>
                              <span className="text-[12px] text-[rgba(23,26,29,0.6)]">{row.permission || t(locale, 'organization.sharedResources.columns.permission')}</span>
                            </div>
                            {row.canUnshare && settingsCanManage ? (
                              <div className={ORG_ROW_ACTIONS}>
                                <button type="button" className={ORG_BTN_NEUTRAL} onClick={() => void unshareKnowledgeBase(row)}>{t(locale, 'organization.share.unshareAction')}</button>
                              </div>
                            ) : null}
                          </div>
                        );
                      }) : null}
                    </>
                  ) : settingsSection === 'agents' ? (
                    <>
                      <h2 className={ORG_SECTION_TITLE}>{t(locale, 'organization.sharedResources.agentListTitle')}</h2>
                      <p className={ORG_SECTION_DESC}>{t(locale, 'organization.settings.sharedAgentsDesc')}</p>
                      {feedStatus('agents', t(locale, 'organization.settings.removeShareFailed'))}
                      {detailFeeds.agents.status === 'ready' && sharedAgents.length === 0 ? <p className={ORG_EMPTY_INLINE}>{t(locale, 'organization.settings.noSharedAgents')}</p> : null}
                      {detailFeeds.agents.status === 'ready' ? sharedAgents.map((agent, index) => <div key={strOf(agent.id) || index} className={ORG_MEMBER_ROW}><div className={ORG_MEMBER_COPY}><strong className="text-[14px] font-semibold text-[rgba(23,26,29,0.92)]">{strOf(agent.agent_name) || strOf(agent.name) || strOf(agent.agent_id)}</strong><span className="text-[12px] text-[rgba(23,26,29,0.6)]">{strOf(agent.permission) || t(locale, 'organization.sharedResources.columns.permission')}</span></div>{settingsCanManage && strOf(agent.agent_id) && (strOf(agent.id) || strOf(agent.share_id)) ? <button type="button" className={ORG_BTN_NEUTRAL} onClick={() => void unshareAgent(agent)}>{t(locale, 'organization.share.unshareAction')}</button> : null}</div>) : null}
                    </>
                  ) : null}
                </div>
                {/* 底部操作按钮（Vue :806-811 settings-footer）。 */}
                <div className="settings-footer">
                  <Button variant="outline" onClick={closeSettings}>{t(locale, 'common.cancel')}</Button>
                  {settingsCanManage ? (
                    <Button theme="primary" loading={saving} onClick={() => { void (settingsMode === 'create' ? submitCreate() : submitBasic()); }}>
                      {t(locale, settingsMode === 'create' ? 'common.create' : 'common.save')}
                    </Button>
                  ) : null}
                </div>
              </div>
            </div>
          </div>
        </div>,
        document.body,
      ) : null}

      {/* Join modal (invite code / search / preview). */}
      {joinOpen ? (
        <div className={ORG_MODAL_OVERLAY} onClick={closeJoin}>
          <div className={'relative box-border flex w-full flex-col overflow-hidden rounded-[12px] bg-surface shadow-[0_8px_32px_rgba(0,0,0,0.12)] max-h-[90vh] ' + (!joinPreview && !joinPreviewLoading && joinStep === 'search' ? 'max-w-[560px]' : 'max-w-[480px]')} role="dialog" aria-label={t(locale, joinPreview ? 'organization.invite.previewTitle' : 'organization.joinOrg')} onClick={(event) => event.stopPropagation()}>
            <button type="button" className={ORG_CLOSE_BTN} aria-label={t(locale, 'common.close')} onClick={closeJoin}><IconClose /></button>
            <div className="flex shrink-0 items-center justify-between gap-[12px] border-b border-[#e7e7ea] py-[16px] pl-[20px] pr-[48px]">
              {joinPreview && !joinCode ? (
                <button type="button" className="flex h-[32px] w-[32px] shrink-0 cursor-pointer items-center justify-center rounded-[8px] border-0 bg-transparent text-[rgba(23,26,29,0.6)] hover:bg-[#f3f3f5] hover:text-accent" aria-label={t(locale, 'organization.join.backToSearch')} onClick={() => { setJoinPreview(null); setJoinStep('search'); }}><IconBack /></button>
              ) : null}
              <h2 className="m-0 min-w-0 flex-1 text-[16px] font-semibold leading-[1.4] text-[rgba(23,26,29,0.92)]">{joinPreview ? t(locale, 'organization.invite.previewTitle') : t(locale, 'organization.joinOrg')}</h2>
            </div>
            <div className="max-h-[calc(90vh-120px)] min-h-0 overflow-x-hidden overflow-y-auto px-[24px] pt-[20px]">
              {joinPreviewLoading ? (
                <div className={ORG_EMPTY_INLINE} style={{ textAlign: 'center', padding: '48px 0' }}>{t(locale, 'organization.invite.loading')}</div>
              ) : joinPreview ? (
                <>
                  <div className="flex flex-col items-center pt-[8px] pb-[20px] text-center">
                    <SpaceAvatar name={previewName} avatar={joinPreview.avatar} size="large" className="mb-[12px]" />
                    <h3 className="mx-0 mt-0 mb-[6px] max-w-full truncate text-[18px] font-semibold leading-[1.35] text-[rgba(23,26,29,0.92)]">{previewName}</h3>
                    <p className="mx-0 mt-0 mb-[14px] line-clamp-2 max-w-[360px] text-[13px] leading-[1.5] text-[rgba(23,26,29,0.6)]">{previewDescription || t(locale, 'organization.noDescription')}</p>
                    <div className="flex items-center justify-center gap-[4px] mb-[12px]">
                      <FeatureBadge tone="stat-member" title={t(locale, 'organization.memberCount')} count={numOf(joinPreview.member_count)} />
                      <FeatureBadge tone="stat-kb" title={t(locale, 'organization.invite.knowledgeBases')} count={numOf(joinPreview.share_count)} />
                      <FeatureBadge tone="stat-agent" title={t(locale, 'organization.invite.agents')} count={numOf(joinPreview.agent_share_count)} />
                    </div>
                    <button type="button" className="inline-flex max-w-full items-center gap-[6px] rounded-full border-0 bg-[#f3f3f5] px-[10px] py-[4px] font-[inherit] text-[12px] text-[rgba(23,26,29,0.4)] hover:bg-accent-wash hover:text-accent" aria-label={t(locale, 'organization.join.spaceId')} onClick={() => { void copyText(strOf(joinPreview.id)).then((copied) => { if (copied) showToast('success', t(locale, 'common.copied')); }); }}>
                      <span>{t(locale, 'organization.join.spaceId')}</span><code className="font-mono text-[11px] text-[rgba(23,26,29,0.6)]">{shortId(joinPreview.id)}</code><span aria-hidden="true">⧉</span>
                    </button>
                  </div>
                  {previewIsAlreadyMember ? (
                    <div className="flex items-center justify-center gap-[8px] pt-[12px] pb-[4px] text-[14px] font-medium text-accent"><IconCheckCircle /><span>{t(locale, 'organization.invite.alreadyMember')}</span></div>
                  ) : (
                    <div style={{ borderTop: '1px solid #e7e7ea', paddingTop: '16px' }}>
                      <div className="flex min-h-[28px] items-center justify-between gap-[12px]">
                        <span className="text-[14px] font-medium text-[rgba(23,26,29,0.92)]">{t(locale, 'organization.invite.approvalLabel')}</span>
                        <span className={ORG_TAG + ' ' + ORG_TAG_TONES[previewJoinMode === 'request' ? 'warning' : 'success']}>{t(locale, previewJoinMode === 'request' ? 'organization.invite.needApproval' : 'organization.invite.noApproval')}</span>
                      </div>
                      {previewJoinMode === 'request' ? (
                        <>
                          <p className="mx-0 mb-0 mt-[8px] text-[13px] leading-[1.5] text-[#faad14]">{t(locale, 'organization.invite.requireApprovalTip')}</p>
                          <div className="mt-[14px] flex flex-col gap-[12px] border-t border-dashed border-[#e7e7ea] pt-[14px]">
                            <div className={ORG_FORM_ITEM} style={{ marginBottom: '0' }}>
                              <label className={ORG_FORM_LABEL} htmlFor="join-request-role">{t(locale, 'organization.invite.requestRole')}</label>
                              <TSelect className={ORG_FIELD + ' min-h-[34px]'} aria-label={t(locale, 'organization.invite.requestRole')} value={requestRole} options={roleOptions.map(([value, labelKey]) => ({ value, label: t(locale, labelKey) }))} onChange={(value) => setRequestRole(String(value) as 'admin' | 'editor' | 'viewer')} />
                            </div>
                            <div className={ORG_FORM_ITEM} style={{ marginBottom: '0' }}>
                              <label className={ORG_FORM_LABEL} htmlFor="join-request-note">{t(locale, 'organization.invite.applicationNote')}</label>
                              <TTextarea id="join-request-note" className={ORG_FIELD + ' min-h-[72px] resize-y'} rows={2} maxLength={500} value={requestNote} onChange={(value) => setRequestNote(clampApplicationNote(String(value)))} placeholder={t(locale, 'organization.invite.messagePlaceholder')} />
                            </div>
                          </div>
                        </>
                      ) : (
                        <p className="mx-0 mb-0 mt-[8px] text-[13px] leading-[1.5] text-[rgba(23,26,29,0.6)]">{t(locale, 'organization.invite.defaultRoleAfterJoin', { role: t(locale, 'organization.role.viewer') })}</p>
                      )}
                    </div>
                  )}
                </>
              ) : (
                <>
                  <div className="mb-[20px] flex flex-wrap gap-[8px]">
                    <button type="button" className={'cursor-pointer rounded-[6px] border-0 px-[14px] py-[6px] font-[inherit] text-[13px] leading-[1.4] ' + (joinStep === 'invite' ? 'bg-accent-soft font-medium text-accent' : 'bg-[#f3f3f5] text-[rgba(23,26,29,0.6)] hover:text-accent')} onClick={() => setJoinStep('invite')}>{t(locale, 'organization.join.byInviteCode')}</button>
                    <button type="button" className={'cursor-pointer rounded-[6px] border-0 px-[14px] py-[6px] font-[inherit] text-[13px] leading-[1.4] ' + (joinStep === 'search' ? 'bg-accent-soft font-medium text-accent' : 'bg-[#f3f3f5] text-[rgba(23,26,29,0.6)] hover:text-accent')} onClick={openSearchTab}>{t(locale, 'organization.join.searchSpaces')}</button>
                  </div>
                  {joinStep === 'invite' ? (
                    <>
                      {joinPreviewError ? <div className="mb-[12px] flex items-center gap-[8px] rounded-[8px] bg-[rgba(213,73,65,0.08)] px-[12px] py-[10px] text-[13px] text-[#d54941]"><IconInfoCircle size={20} /><span>{joinPreviewError}</span></div> : null}
                      <div className={ORG_FORM_ITEM}>
                        <label className={ORG_FORM_LABEL} htmlFor="join-code">{t(locale, 'organization.inviteCode')}</label>
                        <p className={ORG_FORM_DESC}>{t(locale, 'organization.invite.inputDesc')}</p>
                        <TInput name="join-code" className={ORG_FIELD + ' min-h-[34px]'} value={joinInputCode} maxlength={32} placeholder={t(locale, 'organization.inviteCodePlaceholder')} onChange={(value) => setJoinInputCode(String(value))} onKeydown={(_, context) => { if (context.e.key === 'Enter') void doPreviewFromInput(); }} />
                        <p className="m-0 mt-[8px] text-[12px] leading-[1.45] text-[rgba(23,26,29,0.4)]">{t(locale, 'organization.editor.inviteCodeTip')}</p>
                      </div>
                    </>
                  ) : (
                    <>
                      <div className={ORG_FORM_ITEM}>
                        <label className={ORG_FORM_LABEL} htmlFor="join-search">{t(locale, 'organization.join.searchSpaces')}</label>
                        <p className={ORG_FORM_DESC}>{t(locale, 'organization.join.searchSpacesDesc')}</p>
                        <div style={{ position: 'relative' }}>
                          <TInput className={ORG_FIELD + ' min-h-[34px]'} value={searchQuery} placeholder={t(locale, 'organization.join.searchSpacesPlaceholder')} onChange={(value) => onSearchQueryChange(String(value))} onKeydown={(_, context) => { if (context.e.key === 'Enter') runSearch(searchQuery.trim()); }} />
                          <span style={{ position: 'absolute', right: '10px', top: '8px', color: 'rgba(23, 26, 29, 0.4)' }}><IconSearch /></span>
                        </div>
                      </div>
                      <div className="mb-[16px] flex max-h-[320px] min-h-[143px] flex-col overflow-y-auto rounded-[10px] border border-[#e7e7ea] bg-surface">
                        {searchLoading ? (
                          <div className={ORG_EMPTY_INLINE} style={{ textAlign: 'center' }}>{t(locale, 'common.loading')}</div>
                        ) : searchItems.length === 0 ? (
                          <div className="flex min-h-0 flex-col items-center justify-center gap-[4px] px-[12px] py-[18px] text-center">
                            <div className="text-[13px] leading-[22px] text-[rgba(23,26,29,0.6)]">{t(locale, 'common.noData') === 'common.noData' ? '暂无数据' : t(locale, 'common.noData')}</div>
                            <div className="text-[13px] leading-[22px] text-[rgba(23,26,29,0.4)]">{t(locale, searchQuery ? 'organization.join.noSearchResult' : 'organization.join.noSearchableSpaces')}</div>
                          </div>
                        ) : searchItems.map((row) => (
                          <div key={strOf(row.id)} className="flex cursor-pointer items-center justify-between gap-[12px] border-b border-[#e7e7ea] px-[14px] py-[12px] last:border-b-0 hover:bg-[#f3f3f5]" onClick={() => { if (!searchRowFull(row)) previewSearchableOrg(row); }}>
                            <div className="flex min-w-0 flex-1 items-center gap-[10px]">
                              <SpaceAvatar name={strOf(row.name)} avatar={row.avatar} size="small" />
                              <div className="flex min-w-0 flex-col gap-[2px]">
                                <span className="truncate text-[14px] font-medium text-[rgba(23,26,29,0.92)]" title={strOf(row.name)}>{strOf(row.name)}</span>
                                <span className="truncate text-[12px] text-[rgba(23,26,29,0.6)]">{strOf(row.description) || t(locale, 'organization.noDescription')}</span>
                              </div>
                            </div>
                            <div className="flex shrink-0 items-center gap-[8px]">
                              <span style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', fontSize: '12px', color: 'rgba(23, 26, 29, 0.6)' }}><IconUser size={12} />{numOf(row.member_limit) > 0 ? numOf(row.member_count) + '/' + numOf(row.member_limit) : numOf(row.member_count)}</span>
                              {row.require_approval === true ? <span className={ORG_TAG + ' ' + ORG_TAG_TONES.warning}>{t(locale, 'organization.invite.needApproval')}</span> : null}
                              {searchRowFull(row) ? <span className={ORG_TAG}>{t(locale, 'organization.join.memberLimitReached')}</span> : <button type="button" className={ORG_BTN_OUTLINE} style={{ minHeight: '26px', fontSize: '12px', padding: '0 10px' }} onClick={(event) => { event.stopPropagation(); previewSearchableOrg(row); }}>{t(locale, 'organization.invite.previewAction')}</button>}
                            </div>
                          </div>
                        ))}
                      </div>
                    </>
                  )}
                </>
              )}
            </div>
            <div className="flex shrink-0 justify-end gap-[12px] border-t border-[#e7e7ea] px-[24px] pt-[16px] pb-[20px]">
              {joinPreview ? (
                <>
                  <button type="button" className={ORG_BTN_NEUTRAL + ' !px-[15px]'} onClick={() => { setJoinPreview(null); if (!joinCode) setJoinStep('search'); }}>{!joinCode ? t(locale, 'organization.join.backToSearch') : t(locale, 'common.cancel')}</button>
                  {!previewIsAlreadyMember ? (
                    <button type="button" className={ORG_BTN_PRIMARY + ' !px-[15px]'} disabled={joining} onClick={() => void confirmJoin()}>{previewJoinMode === 'request' ? t(locale, 'organization.invite.submitRequest') : t(locale, 'organization.invite.primaryJoin')}</button>
                  ) : <button type="button" className={ORG_BTN_PRIMARY + ' !px-[15px]'} onClick={viewOrganizationFromPreview}>{t(locale, 'organization.invite.viewOrganization')}</button>}
                </>
              ) : joinStep === 'invite' ? (
                <>
                  <button type="button" className={ORG_BTN_NEUTRAL + ' !px-[15px]'} onClick={closeJoin}>{t(locale, 'common.cancel')}</button>
                  <button type="button" className={ORG_BTN_PRIMARY + ' !px-[15px]'} disabled={joinPreviewLoading} onClick={() => void doPreviewFromInput()}>{t(locale, 'organization.invite.previewAction')}</button>
                </>
              ) : (
                <button type="button" className={ORG_BTN_NEUTRAL + ' !px-[15px]'} onClick={closeJoin}>{t(locale, 'common.cancel')}</button>
              )}
            </div>
          </div>
        </div>
      ) : null}

      {/* 删除/退出确认弹窗（Vue :207-240 t-dialog del-org-dialog + circle-wrap，
          与 kb-list del-knowledge-dialog 同款先例）。 */}
      {confirmState ? <Dialog
        visible
        dialogClassName="del-org-dialog"
        closeBtn={false}
        cancelBtn={null}
        confirmBtn={null}
        onClose={() => setConfirmState(null)}
      >
        <div className="circle-wrap">
          <div className="dialog-header">
            <img className="circle-img" src={CIRCLE_PNG} alt="" />
            <span className="circle-title">{t(locale, confirmState?.kind === 'delete' ? 'organization.deleteConfirmTitle' : 'organization.leaveConfirmTitle')}</span>
          </div>
          <span className="del-circle-txt">
            {t(locale, confirmState?.kind === 'delete' ? 'organization.deleteConfirmMessage' : 'organization.leaveConfirmMessage', { name: confirmState?.org.name ?? '' })}
          </span>
          <div className="circle-btn">
            <span className="circle-btn-txt" onClick={() => setConfirmState(null)}>{t(locale, 'common.cancel')}</span>
            <span className="circle-btn-txt confirm" onClick={() => void confirmLeaveOrDelete()}>{confirmState?.kind === 'delete' ? t(locale, 'common.delete') : t(locale, 'organization.leave')}</span>
          </div>
        </div>
</Dialog> : null}

      {toast ? <div className={'fixed left-1/2 top-[24px] z-[3000] flex items-center rounded-[8px] px-[18px] py-[10px] shadow-[0_6px_20px_rgba(0,0,0,0.18)] box-border max-w-[420px] bg-[rgba(23,26,29,0.86)] -translate-x-1/2 ' + (toast.tone === 'success' ? 'text-[#7bf2b6]' : toast.tone === 'warning' ? 'text-[#faad14]' : 'text-[#ffb4ae]')} role="status">{toast.text}</div> : null}
    </div>
  );
}
