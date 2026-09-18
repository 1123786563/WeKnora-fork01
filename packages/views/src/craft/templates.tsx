// CFT-S01-T009 (P03): the templates entry — versioned goal prompts per kind.
// Using a template ONLY fills the create form (goal + kind); it never
// authorizes, executes or pre-selects any knowledge the user did not choose.
// A kind outside the server gate disables its card WITH the reason.
import React from 'react';
import type { CraftSessionKind } from '@weknora/contracts';
import { Button } from '@weknora/ui';
import { craftKindDisabledReason, type CraftViewCapabilities } from '@weknora/domain/craft/capabilities';
import './craft.css';

export interface CraftTemplate {
  id: string;
  kind: CraftSessionKind;
  title: string;
  goal: string;
}

export const CRAFT_STARTER_TEMPLATES: readonly CraftTemplate[] = [
  { id: 'tpl-web-report', kind: 'web', title: '数据报告网页', goal: '基于上传的数据生成可筛选的网页报告' },
  { id: 'tpl-web-tool', kind: 'web', title: '前端小工具页', goal: '制作一个可交互的实用前端小工具页面' },
  { id: 'tpl-doc-brief', kind: 'document', title: '结构化文档', goal: '把资料整理为带目录和引用的结构化文档' },
  { id: 'tpl-sheet-analysis', kind: 'spreadsheet', title: '分析表格', goal: '生成含公式重算的数据分析表格' },
  { id: 'tpl-slides-review', kind: 'slides', title: '汇报演示稿', goal: '把结论整理为逐页演示稿' },
];

export interface CraftTemplatesProps {
  templates?: readonly CraftTemplate[];
  /** Server gate projection; null keeps every card enabled (legacy). */
  capabilities?: CraftViewCapabilities | null;
  onUse(template: CraftTemplate): void;
}

export function CraftTemplates(props: CraftTemplatesProps) {
  const templates = props.templates ?? CRAFT_STARTER_TEMPLATES;
  return (
    <main className="wk-craft wk-craft-page" aria-label="模板">
      <header className="wk-craft-head">
        <div>
          <h1>模板</h1>
          <p className="wk-craft-muted">模板只填充目标与类型；不会自动执行、授权或选择知识库</p>
        </div>
      </header>
      <ul className="wk-craft-list">
        {templates.map((template) => {
          const reason = props.capabilities ? craftKindDisabledReason(template.kind, props.capabilities) : null;
          return (
            <li key={template.id}>
              <span className="wk-craft-item-title">{template.title}</span>
              <span className="wk-craft-kind">{template.kind}</span>
              <span className="wk-craft-item-meta">{template.goal}</span>
              <Button
                type="button"
                size="small"
                disabled={reason !== null}
                title={reason ?? undefined}
                onClick={() => props.onUse(template)}
              >
                填入创建器
              </Button>
              {reason !== null ? <span className="wk-craft-muted">{reason}</span> : null}
            </li>
          );
        })}
      </ul>
    </main>
  );
}
