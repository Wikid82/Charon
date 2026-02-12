import { useAccessLists } from '../hooks/useAccessLists';
import { ExternalLink } from 'lucide-react';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from './ui/Select';

interface AccessListSelectorProps {
  value: number | null;
  onChange: (id: number | null) => void;
}

export default function AccessListSelector({ value, onChange }: AccessListSelectorProps) {
  const { data: accessLists } = useAccessLists();

  const selectedACL = accessLists?.find((acl) => acl.id === value);

  // Convert between component's string-based value and the prop's number|null
  const selectValue = value === null || value === undefined ? 'none' : String(value);

  const handleValueChange = (newValue: string) => {
    if (newValue === 'none') {
      onChange(null);
    } else {
      const numericId = parseInt(newValue, 10);
      if (!isNaN(numericId)) {
        onChange(numericId);
      }
    }
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
            .map((acl) => (
              <SelectItem key={acl.id} value={String(acl.id)}>
                {acl.name} ({acl.type.replace('_', ' ')})
              </SelectItem>
            ))}
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
