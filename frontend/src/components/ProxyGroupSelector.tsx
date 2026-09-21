import { useProxyGroups } from '../hooks/useProxyGroups';

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from './ui/Select';

interface ProxyGroupSelectorProps {
  value: string | null | undefined;
  onChange: (uuid: string | null) => void;
}

const NONE_VALUE = 'none';
const DEFAULT_DOT_COLOR = '#6b7280';

export default function ProxyGroupSelector({ value, onChange }: ProxyGroupSelectorProps) {
  const { data: groups } = useProxyGroups();

  const selectValue = value ?? NONE_VALUE;

  const handleValueChange = (newValue: string) => {
    if (newValue === NONE_VALUE) {
      onChange(null);
      return;
    }

    onChange(newValue);
  };

  return (
    <div>
      <Select value={selectValue} onValueChange={handleValueChange}>
        <SelectTrigger className="w-full bg-gray-900 border-gray-700 text-white" aria-label="Proxy Group">
          <SelectValue placeholder="Select a group" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={NONE_VALUE}>No Group</SelectItem>
          {groups?.map((group) => (
            <SelectItem key={group.uuid} value={group.uuid}>
              <span className="inline-flex items-center gap-1.5">
                <span
                  className="inline-block w-2.5 h-2.5 rounded-full shrink-0"
                  style={{ backgroundColor: group.color ?? DEFAULT_DOT_COLOR }}
                  aria-hidden="true"
                />
                <span>{group.name}</span>
              </span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <p className="text-xs text-gray-500 mt-1">
        Organize this host with other hosts under a shared group.{' '}
        <a href="/proxy-hosts" className="text-blue-400 hover:underline">
          Manage groups
        </a>
      </p>
    </div>
  );
}
