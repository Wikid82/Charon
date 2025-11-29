import { useAccessLists } from '../hooks/useAccessLists';
import { ExternalLink } from 'lucide-react';

interface AccessListSelectorProps {
  value: number | null;
  onChange: (id: number | null) => void;
}

export default function AccessListSelector({ value, onChange }: AccessListSelectorProps) {
  const { data: accessLists } = useAccessLists();

  const selectedACL = accessLists?.find((acl) => acl.id === value);

  return (
    <div>
      <label className="block text-sm font-medium text-gray-300 mb-2">
        Access Control List
        <span className="text-gray-500 font-normal ml-2">(Optional)</span>
      </label>
      <select
        value={value || 0}
        onChange={(e) => onChange(parseInt(e.target.value) || null)}
        className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
      >
        <option value={0}>No Access Control (Public)</option>
        {accessLists
          ?.filter((acl) => acl.enabled)
          .map((acl) => (
            <option key={acl.id} value={acl.id}>
              {acl.name} ({acl.type.replace('_', ' ')})
            </option>
          ))}
      </select>

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
        <a href="/access-lists" className="text-blue-400 hover:underline">
          Manage lists
        </a>
        {' • '}
        <a
          href="https://wikid82.github.io/charon/docs/security.html#acl-best-practices-by-service-type"
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
