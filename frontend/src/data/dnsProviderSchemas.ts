import type { DNSProviderType, DNSProviderTypeInfo } from '../api/dnsProviders'

/**
 * Default provider field schemas.
 * These are fallback definitions; actual field definitions come from the API.
 */
export const defaultProviderSchemas: Record<DNSProviderType, Partial<DNSProviderTypeInfo>> = {
  cloudflare: {
    type: 'cloudflare',
    name: 'Cloudflare',
    fields: [
      {
        name: 'api_token',
        label: 'API Token',
        type: 'password',
        required: true,
        hint: 'Token with Zone:DNS:Edit permissions',
      },
    ],
    documentation_url: 'https://developers.cloudflare.com/api/tokens/',
  },
  route53: {
    type: 'route53',
    name: 'Amazon Route 53',
    fields: [
      {
        name: 'access_key_id',
        label: 'Access Key ID',
        type: 'text',
        required: true,
      },
      {
        name: 'secret_access_key',
        label: 'Secret Access Key',
        type: 'password',
        required: true,
      },
      {
        name: 'region',
        label: 'AWS Region',
        type: 'text',
        required: true,
        default: 'us-east-1',
      },
    ],
    documentation_url: 'https://docs.aws.amazon.com/Route53/',
  },
  digitalocean: {
    type: 'digitalocean',
    name: 'DigitalOcean',
    fields: [
      {
        name: 'auth_token',
        label: 'Auth Token',
        type: 'password',
        required: true,
      },
    ],
    documentation_url: 'https://docs.digitalocean.com/reference/api/',
  },
  googleclouddns: {
    type: 'googleclouddns',
    name: 'Google Cloud DNS',
    fields: [
      {
        name: 'service_account_json',
        label: 'Service Account JSON',
        type: 'password',
        required: true,
        hint: 'Paste the entire JSON file contents',
      },
      {
        name: 'project',
        label: 'Project ID',
        type: 'text',
        required: true,
      },
    ],
    documentation_url: 'https://cloud.google.com/dns/docs',
  },
  namecheap: {
    type: 'namecheap',
    name: 'Namecheap',
    fields: [
      {
        name: 'api_user',
        label: 'API User',
        type: 'text',
        required: true,
      },
      {
        name: 'api_key',
        label: 'API Key',
        type: 'password',
        required: true,
      },
      {
        name: 'client_ip',
        label: 'Client IP',
        type: 'text',
        required: true,
        hint: 'Your whitelisted IP address',
      },
    ],
    documentation_url: 'https://www.namecheap.com/support/api/',
  },
  godaddy: {
    type: 'godaddy',
    name: 'GoDaddy',
    fields: [
      {
        name: 'api_key',
        label: 'API Key',
        type: 'text',
        required: true,
      },
      {
        name: 'api_secret',
        label: 'API Secret',
        type: 'password',
        required: true,
      },
    ],
    documentation_url: 'https://developer.godaddy.com/',
  },
  azure: {
    type: 'azure',
    name: 'Azure DNS',
    fields: [
      {
        name: 'tenant_id',
        label: 'Tenant ID',
        type: 'text',
        required: true,
      },
      {
        name: 'client_id',
        label: 'Client ID',
        type: 'text',
        required: true,
      },
      {
        name: 'client_secret',
        label: 'Client Secret',
        type: 'password',
        required: true,
      },
      {
        name: 'subscription_id',
        label: 'Subscription ID',
        type: 'text',
        required: true,
      },
      {
        name: 'resource_group',
        label: 'Resource Group',
        type: 'text',
        required: true,
      },
    ],
    documentation_url: 'https://learn.microsoft.com/en-us/azure/dns/',
  },
  hetzner: {
    type: 'hetzner',
    name: 'Hetzner',
    fields: [
      {
        name: 'api_key',
        label: 'API Key',
        type: 'password',
        required: true,
      },
    ],
    documentation_url: 'https://dns.hetzner.com/api-docs',
  },
  vultr: {
    type: 'vultr',
    name: 'Vultr',
    fields: [
      {
        name: 'api_key',
        label: 'API Key',
        type: 'password',
        required: true,
      },
    ],
    documentation_url: 'https://www.vultr.com/api/',
  },
  dnsimple: {
    type: 'dnsimple',
    name: 'DNSimple',
    fields: [
      {
        name: 'oauth_token',
        label: 'OAuth Token',
        type: 'password',
        required: true,
      },
      {
        name: 'account_id',
        label: 'Account ID',
        type: 'text',
        required: true,
      },
    ],
    documentation_url: 'https://developer.dnsimple.com/',
  },
  manual: {
    type: 'manual',
    name: 'Manual DNS',
    fields: [],
    documentation_url: 'https://letsencrypt.org/docs/challenge-types/',
  },
  script: {
    type: 'script',
    name: 'Custom Script',
    fields: [
      {
        name: 'script_path',
        label: 'Script Path',
        type: 'text',
        required: true,
        hint: 'Path to custom DNS update script',
      },
    ],
    documentation_url: '',
  },
  webhook: {
    type: 'webhook',
    name: 'Webhook',
    fields: [
      {
        name: 'url',
        label: 'Webhook URL',
        type: 'text',
        required: true,
      },
    ],
    documentation_url: '',
  },
  rfc2136: {
    type: 'rfc2136',
    name: 'RFC2136 (Dynamic DNS)',
    fields: [
      {
        name: 'server',
        label: 'DNS Server',
        type: 'text',
        required: true,
      },
      {
        name: 'key_name',
        label: 'TSIG Key Name',
        type: 'text',
        required: true,
      },
      {
        name: 'key_secret',
        label: 'TSIG Key Secret',
        type: 'password',
        required: true,
      },
    ],
    documentation_url: 'https://tools.ietf.org/html/rfc2136',
  },
}
