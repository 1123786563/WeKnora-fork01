import { ContractError } from '../index.ts';

export interface AgentAdoptionVariant {
  id: string;
  adoption_id: string;
  release_id: string;
  name: string;
  state: 'draft' | 'mapped' | 'tested' | 'published';
  local_agent_id?: string;
  local_agent_version_id?: string;
  missing_capabilities: string[];
  created_at: string;
  updated_at: string;
  [key: string]: unknown;
}

export interface AgentAdoption {
  id: string;
  listing_id: string;
  accepted_release_id: string;
  state: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  variants: AgentAdoptionVariant[];
  [key: string]: unknown;
}

export interface AvailableAgent {
  agent_id: string;
  variant_id: string;
  adoption_id: string;
  release_id: string;
  name: string;
  description: string;
  is_builtin: boolean;
  capability: { state: 'supported' | 'unavailable' | 'forbidden'; reason: string };
  [key: string]: unknown;
}

const VARIANT_STATES = ['draft', 'mapped', 'tested', 'published'];
const CAPABILITY_STATES = ['supported', 'unavailable', 'forbidden'];

function object(value: unknown, path: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ContractError(path, 'expected an object');
  return value as Record<string, unknown>;
}

function requiredString(row: Record<string, unknown>, key: string, path: string): string {
  const value = row[key];
  if (typeof value !== 'string' || value.trim() === '') throw new ContractError(`${path}.${key}`, 'expected a non-empty string');
  return value;
}

function requiredStringArray(row: Record<string, unknown>, key: string, path: string): string[] {
  const value = row[key];
  if (!Array.isArray(value) || value.some((item) => typeof item !== 'string')) throw new ContractError(`${path}.${key}`, 'expected an array of strings');
  return value as string[];
}

function enumValue<T extends string>(row: Record<string, unknown>, key: string, allowed: readonly T[], path: string): T {
  const value = row[key];
  if (typeof value !== 'string' || !(allowed as readonly string[]).includes(value)) {
    throw new ContractError(`${path}.${key}`, `expected one of ${allowed.join(', ')}`);
  }
  return value as T;
}

function envelopeData(value: unknown): unknown {
  const envelope = object(value, '');
  if (envelope.success !== true) throw new ContractError('success', 'expected true');
  if (!Object.prototype.hasOwnProperty.call(envelope, 'data')) throw new ContractError('data', 'is required');
  return envelope.data;
}

function optionalString(row: Record<string, unknown>, key: string): string | undefined {
  const value = row[key];
  return typeof value === 'string' && value !== '' ? value : undefined;
}

function parseVariantRow(value: unknown, path: string): AgentAdoptionVariant {
  const row = object(value, path);
  const variant: AgentAdoptionVariant = {
    ...row,
    id: requiredString(row, 'id', path),
    adoption_id: requiredString(row, 'adoption_id', path),
    release_id: requiredString(row, 'release_id', path),
    name: requiredString(row, 'name', path),
    state: enumValue(row, 'state', VARIANT_STATES, path),
    missing_capabilities: requiredStringArray(row, 'missing_capabilities', path),
    created_at: requiredString(row, 'created_at', path),
    updated_at: requiredString(row, 'updated_at', path),
  };
  const localAgentId = optionalString(row, 'local_agent_id');
  if (localAgentId !== undefined) variant.local_agent_id = localAgentId;
  const localVersionId = optionalString(row, 'local_agent_version_id');
  if (localVersionId !== undefined) variant.local_agent_version_id = localVersionId;
  return variant;
}

function parseAdoptionRow(value: unknown, path: string): AgentAdoption {
  const row = object(value, path);
  const variants = row.variants;
  if (!Array.isArray(variants)) throw new ContractError(`${path}.variants`, 'expected an array');
  return {
    ...row,
    id: requiredString(row, 'id', path),
    listing_id: requiredString(row, 'listing_id', path),
    accepted_release_id: requiredString(row, 'accepted_release_id', path),
    state: requiredString(row, 'state', path),
    created_by: typeof row.created_by === 'string' ? row.created_by : '',
    created_at: requiredString(row, 'created_at', path),
    updated_at: requiredString(row, 'updated_at', path),
    variants: variants.map((variant, index) => parseVariantRow(variant, `${path}.variants[${index}]`)),
  } as AgentAdoption;
}

export function parseAdoptionResponse(value: unknown): AgentAdoption {
  return parseAdoptionRow(envelopeData(value), 'data');
}

export function parseAdoptionListResponse(value: unknown): AgentAdoption[] {
  const data = envelopeData(value);
  if (!Array.isArray(data)) throw new ContractError('data', 'expected an array');
  return data.map((row, index) => parseAdoptionRow(row, `data[${index}]`));
}

export function parseVariantResponse(value: unknown): AgentAdoptionVariant {
  return parseVariantRow(envelopeData(value), 'data');
}

export function parseAvailableAgentListResponse(value: unknown): AvailableAgent[] {
  const data = envelopeData(value);
  if (!Array.isArray(data)) throw new ContractError('data', 'expected an array');
  return data.map((row, index) => {
    const path = `data[${index}]`;
    const recordRow = object(row, path);
    const capability = object(recordRow.capability, `${path}.capability`);
    return {
      ...recordRow,
      agent_id: requiredString(recordRow, 'agent_id', path),
      variant_id: requiredString(recordRow, 'variant_id', path),
      adoption_id: requiredString(recordRow, 'adoption_id', path),
      release_id: requiredString(recordRow, 'release_id', path),
      name: requiredString(recordRow, 'name', path),
      description: typeof recordRow.description === 'string' ? recordRow.description : '',
      is_builtin: recordRow.is_builtin === true,
      capability: {
        state: enumValue(capability, 'state', CAPABILITY_STATES, `${path}.capability`),
        reason: typeof capability.reason === 'string' ? capability.reason : '',
      },
    } as AvailableAgent;
  });
}
