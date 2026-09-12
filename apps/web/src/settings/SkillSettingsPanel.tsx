import type { SkillConfiguration, WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { SkillOperations } from '../configuration/ConfigurationOperations.tsx';

type Props = { client: WeKnoraClient; role: 'viewer' | 'admin' | 'owner' | 'system-admin'; initialSkills?: readonly SkillConfiguration[] };

export function SkillSettingsPanel({ client, role, initialSkills }: Props) {
  const canEdit = role === 'admin' || role === 'owner';
  if (!canEdit) {
    return <Card data-testid="skill-settings"><h3>Skills</h3><p className="wk-muted">Installed skills are managed by workspace administrators.</p>{initialSkills && initialSkills.length > 0 ? <ul className="wk-list">{initialSkills.map((skill) => <li key={skill.id}><strong>{skill.name}</strong><span>{skill.description ?? 'No description returned.'}</span></li>)}</ul> : <Status>No skills configured.</Status>}</Card>;
  }
  return <section data-testid="skill-settings"><SkillOperations client={client} /></section>;
}
