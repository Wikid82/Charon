/**
 * Listen-setting helpers for the external Docker proxy. The server stores one string:
 * '' (automatic), '0.0.0.0' (all networks), a CIDR subnet, or an address. The dialog
 * shows that string as one choice of a small select, so these helpers convert both ways.
 */

export const ALL_NETWORKS_BIND = '0.0.0.0';

export type BindChoiceKind = 'auto' | 'all' | 'network' | 'custom';

export interface BindChoice {
  kind: BindChoiceKind;
  /** The subnet, for the `network` kind. */
  cidr: string;
  /** The typed address or subnet, for the `custom` kind. */
  custom: string;
}

/** Reads the stored listen setting into the choice the dialog shows. */
export function bindChoiceFromSetting(setting: string): BindChoice {
  if (setting === '') return { kind: 'auto', cidr: '', custom: '' };
  if (setting === ALL_NETWORKS_BIND) return { kind: 'all', cidr: '', custom: '' };
  if (setting.includes('/')) return { kind: 'network', cidr: setting, custom: '' };
  return { kind: 'custom', cidr: '', custom: setting };
}

/** Converts the dialog choice back into the value sent to the server. */
export function bindSettingFromChoice(choice: BindChoice): string {
  switch (choice.kind) {
    case 'auto':
      return '';
    case 'all':
      return ALL_NETWORKS_BIND;
    case 'network':
      return choice.cidr;
    case 'custom':
      return choice.custom.trim();
  }
}

/** False when the choice cannot be saved as entered (a custom choice with nothing typed). */
export function isBindChoiceComplete(choice: BindChoice): boolean {
  return choice.kind !== 'custom' || choice.custom.trim() !== '';
}

/**
 * True when a listener address (`127.0.0.1:2375`, `[::1]:2375`, or a bare host) is loopback,
 * meaning other containers and hosts cannot reach it.
 */
export function isLoopbackListener(bindAddress: string): boolean {
  let host = bindAddress;
  if (bindAddress.startsWith('[')) {
    // Bracketed IPv6, optionally followed by a port: [::1]:2375
    const end = bindAddress.indexOf(']');
    host = end > 0 ? bindAddress.slice(1, end) : bindAddress;
  } else if (bindAddress.split(':').length === 2) {
    // One colon means host:port. More colons is a bare IPv6 address.
    host = bindAddress.slice(0, bindAddress.indexOf(':'));
  }
  return host.startsWith('127.') || host === '::1';
}
