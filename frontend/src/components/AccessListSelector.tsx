import { ExternalLink } from 'lucide-react';

import { useAccessLists } from '../hooks/useAccessLists';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from './ui/Select';

interface AccessListSelectorProps {
  value: number | string | null;
  onChange: (id: number | string | null) => void;
}

function resolveAccessListToken(
  value: number | string | null | undefined,
  accessLists?: Array<{ id?: number | string; uuid?: string }>
): string {
  if (value === null || value === undefined) {
    return 'none';
  }

  if (typeof value === 'number') {
    return `id:${value}`;
  }

  const trimmed = value.trim();
  if (trimmed === '') {
    return 'none';
  }

  if (trimmed.startsWith('id:')) {
    return trimmed;
  }

  if (trimmed.startsWith('uuid:')) {
    const uuid = trimmed.slice(5);
    const matchingACL = accessLists?.find((acl) => acl.uuid === uuid);
    const matchingToken = matchingACL ? getOptionToken(matchingACL) : null;
    return matchingToken ?? trimmed;
  }

  if (/^\d+$/.test(trimmed)) {
    const parsed = Number.parseInt(trimmed, 10);
    return `id:${parsed}`;
  }

  const matchingACL = accessLists?.find((acl) => acl.uuid === trimmed);
  const matchingToken = matchingACL ? getOptionToken(matchingACL) : null;
  return matchingToken ?? `uuid:${trimmed}`;
}

function getOptionToken(acl: { id?: number | string; uuid?: string }): string | null {
  if (typeof acl.id === 'number' && Number.isFinite(acl.id)) {
    return `id:${acl.id}`;
  }

  if (typeof acl.id === 'string') {
    const trimmed = acl.id.trim();
    if (trimmed !== '' && /^\d+$/.test(trimmed)) {
      const parsed = Number.parseInt(trimmed, 10);
      if (!Number.isNaN(parsed)) {
        return `id:${parsed}`;
      }
    }
  }

  if (acl.uuid) {
    return `uuid:${acl.uuid}`;
  }

  return null;
}

export default function AccessListSelector({ value, onChange }: AccessListSelectorProps) {
  const { data: accessLists } = useAccessLists();

  const selectedToken = resolveAccessListToken(value, accessLists);
  const selectedACL = accessLists?.find((acl) => getOptionToken(acl) === selectedToken);

  // Keep select value stable for both numeric-ID and UUID-only payload shapes.
  const selectValue = selectedToken;

  const handleValueChange = (newValue: string) => {
    if (newValue === 'none') {
      onChange(null);
      return;
    }

    if (newValue.startsWith('id:')) {
      const numericId = Number.parseInt(newValue.slice(3), 10);
      if (!Number.isNaN(numericId)) {
        onChange(numericId);
      }
      return;
    }

    if (newValue.startsWith('uuid:')) {
      const selectedUUID = newValue.slice(5);
      const matchingACL = accessLists?.find((acl) => acl.uuid === selectedUUID);
      const matchingToken = matchingACL ? getOptionToken(matchingACL) : null;

      if (matchingToken?.startsWith('id:')) {
        const numericId = Number.parseInt(matchingToken.slice(3), 10);
        if (!Number.isNaN(numericId)) {
          onChange(numericId);
          return;
        }
      }

      onChange(selectedUUID);
      return;
    }

    if (/^\d+$/.test(newValue)) {
      const numericId = Number.parseInt(newValue, 10);
      onChange(numericId);
      return;
    }

    onChange(newValue);
  };

  return (
    <div>
      <label className="block text-sm font-medium text-gray-300 mb-2">
        Access Control List
        <span className="text-gray-500 font-normal ml-2">(Optional)</span>
      </label>
      <Select
        value={selectValue}
        onValueChange={handleValueChange}
      >
        <SelectTrigger className="w-full bg-gray-900 border-gray-700 text-white" aria-label="Access Control List">
          <SelectValue placeholder="Select an ACL" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="none">No Access Control (Public)</SelectItem>
          {accessLists
            ?.filter((acl) => acl.enabled)
            .map((acl) => {
              const optionToken = getOptionToken(acl);
              if (!optionToken) {
                return null;
              }

              return (
                <SelectItem key={optionToken} value={optionToken}>
                  {acl.name} ({acl.type.replace('_', ' ')})
                </SelectItem>
              );
            })}
        </SelectContent>
      </Select>

      {selectedACL && (
        <div className="mt-2 p-3 bg-gray-800 border border-gray-700 rounded-lg">
          <div className="flex items-center gap-2 mb-1">
            <span className="text-sm font-medium text-gray-200">{selectedACL.name}</span>
            <span className="px-2 py-0.5 text-xs bg-gray-700 border border-gray-600 rounded">
              {selectedACL.type.replace('_', ' ')}
            </span>
          </div>
          {selectedACL.description && (
            <p className="text-xs text-gray-400 mb-2">{selectedACL.description}</p>
          )}
          {selectedACL.local_network_only && (
            <div className="text-xs text-blue-400">
              🏠 Local Network Only (RFC1918)
            </div>
          )}
          {selectedACL.type.startsWith('geo_') && selectedACL.country_codes && (
            <div className="text-xs text-gray-400">
              🌍 Countries: {selectedACL.country_codes}
            </div>
          )}
        </div>
      )}

      <p className="text-xs text-gray-500 mt-1">
        Restrict access based on IP address, CIDR ranges, or geographic location.{' '}
        <a href="/security/access-lists" className="text-blue-400 hover:underline">
          Manage lists
        </a>
        {' • '}
        <a
          href="https://wikid82.github.io/charon/security#acl-best-practices-by-service-type"
          target="_blank"
          rel="noopener noreferrer"
          className="text-blue-400 hover:underline inline-flex items-center gap-1"
        >
          <ExternalLink className="inline h-3 w-3" />
          Best Practices
        </a>
      </p>
    </div>
  );
}
