import { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Button,
  Input,
  Textarea,
} from './ui';
import { useCreateProxyGroup, useUpdateProxyGroup } from '../hooks/useProxyGroups';

import type { ProxyGroup } from '../api/proxyGroups';

const COLOR_PRESETS = [
  '#6366f1',
  '#ef4444',
  '#f59e0b',
  '#10b981',
  '#3b82f6',
  '#8b5cf6',
  '#ec4899',
  '#6b7280',
];

interface ProxyGroupFormProps {
  open: boolean;
  onClose: () => void;
  group?: ProxyGroup;
}

export function ProxyGroupForm({ open, onClose, group }: ProxyGroupFormProps) {
  const { t } = useTranslation();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [color, setColor] = useState(COLOR_PRESETS[0]);

  const createGroup = useCreateProxyGroup();
  const updateGroup = useUpdateProxyGroup();

  const isEdit = !!group;
  const isPending = createGroup.isPending || updateGroup.isPending;

  useEffect(() => {
    if (open) {
      setName(group?.name ?? '');
      setDescription(group?.description ?? '');
      setColor(group?.color ?? COLOR_PRESETS[0]);
    }
  }, [open, group]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    const payload = {
      name: name.trim(),
      description: description.trim() || undefined,
      color,
    };

    try {
      if (isEdit) {
        await updateGroup.mutateAsync({ uuid: group.uuid, data: payload });
      } else {
        await createGroup.mutateAsync(payload);
      }
      onClose();
    } catch {
      // errors handled by hook toast
    }
  };

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{isEdit ? t('proxyGroups.editGroup') : t('proxyGroups.createGroup')}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? t('proxyGroups.editGroupDescription')
              : t('proxyGroups.createGroupDescription')}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label
              htmlFor="group-name"
              className="block text-sm font-medium text-content-primary mb-1.5"
            >
              {t('proxyGroups.groupName')}{' '}
              <span aria-hidden="true" className="text-error">
                *
              </span>
            </label>
            <Input
              id="group-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t('proxyGroups.groupNamePlaceholder')}
              required
              aria-required="true"
            />
          </div>

          <div>
            <label
              htmlFor="group-description"
              className="block text-sm font-medium text-content-primary mb-1.5"
            >
              {t('proxyGroups.groupDescription')}
            </label>
            <Textarea
              id="group-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t('proxyGroups.groupDescriptionPlaceholder')}
              rows={2}
            />
          </div>

          <div>
            <span
              id="color-label"
              className="block text-sm font-medium text-content-primary mb-1.5"
            >
              {t('proxyGroups.groupColor')}
            </span>
            <div
              className="flex flex-wrap items-center gap-2"
              role="group"
              aria-labelledby="color-label"
            >
              {COLOR_PRESETS.map((preset) => (
                <button
                  key={preset}
                  type="button"
                  aria-label={t('proxyGroups.colorPreset', { color: preset })}
                  aria-pressed={color === preset}
                  className="w-7 h-7 rounded-full border-2 transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
                  style={{
                    backgroundColor: preset,
                    borderColor: color === preset ? 'white' : 'transparent',
                    boxShadow: color === preset ? `0 0 0 2px ${preset}` : undefined,
                  }}
                  onClick={() => setColor(preset)}
                />
              ))}
              <input
                type="color"
                value={color}
                onChange={(e) => setColor(e.target.value)}
                aria-label={t('proxyGroups.customColor')}
                className="w-7 h-7 rounded cursor-pointer border border-border bg-transparent"
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose} disabled={isPending}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={!name.trim() || isPending} isLoading={isPending}>
              {t('common.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
