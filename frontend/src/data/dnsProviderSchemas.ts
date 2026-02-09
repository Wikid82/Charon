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
        name: 'create_script',
        label: 'Script Path',
        type: 'text',
        required: true,
        placeholder: '/scripts/dns-challenge.sh',
        hint: 'Path to script that creates DNS TXT records. Receives DOMAIN, TOKEN, and FQDN as environment variables.',
      },
      {
        name: 'delete_script',
        label: 'Delete Record Script',
        type: 'text',
        required: false,
        placeholder: '/path/to/delete-dns.sh',
        hint: 'Path to script that deletes DNS TXT records. If not set, create script is called with DELETE=true.',
      },
      {
        name: 'shell',
        label: 'Shell',
        type: 'text',
        required: false,
        default: '/bin/sh',
        placeholder: '/bin/sh',
        hint: 'Shell to execute scripts with.',
      },
      {
        name: 'env_vars',
        label: 'Environment Variables',
        type: 'textarea',
        required: false,
        placeholder: 'API_KEY=secret\nAPI_URL=https://api.example.com',
        hint: 'Additional environment variables to pass to scripts (one per line, KEY=VALUE format).',
      },
    ],
    documentation_url: '',
  },
  webhook: {
    type: 'webhook',
    name: 'Webhook',
    fields: [
      {
        name: 'create_url',
        label: 'Create Record URL',
        type: 'text',
        required: true,
        placeholder: 'https://api.example.com/dns/create',
        hint: 'HTTP endpoint to call when creating DNS records. Receives JSON payload with domain, token, and fqdn.',
      },
      {
        name: 'delete_url',
        label: 'Delete Record URL',
        type: 'text',
        required: false,
        placeholder: 'https://api.example.com/dns/delete',
        hint: 'HTTP endpoint to call when deleting DNS records. If not set, create URL is called with method DELETE.',
      },
      {
        name: 'auth_type',
        label: 'Authentication Type',
        type: 'select',
        required: false,
        default: 'none',
        options: [
          { value: 'none', label: 'None' },
          { value: 'bearer', label: 'Bearer Token' },
          { value: 'basic', label: 'Basic Auth' },
          { value: 'header', label: 'Custom Header' },
        ],
        hint: 'Authentication method for webhook requests.',
      },
      {
        name: 'auth_value',
        label: 'Auth Value',
        type: 'password',
        required: false,
        placeholder: 'Bearer token, user:pass, or header value',
        hint: 'Authentication value (token, user:password for basic auth, or header value).',
      },
      {
        name: 'insecure_skip_verify',
        label: 'Skip TLS Verification',
        type: 'select',
        required: false,
        default: 'false',
        options: [
          { value: 'false', label: 'No (Recommended)' },
          { value: 'true', label: 'Yes (Insecure)' },
        ],
        hint: 'Skip TLS certificate verification. Only enable for testing with self-signed certificates.',
      },
    ],
    documentation_url: '',
  },
  rfc2136: {
    type: 'rfc2136',
    name: 'RFC2136 (Dynamic DNS)',
    fields: [
      {
        name: 'nameserver',
        label: 'DNS Server',
        type: 'text',
        required: true,
        placeholder: 'ns1.example.com:53',
        hint: 'DNS server address with port (e.g., ns1.example.com:53).',
      },
      {
        name: 'tsig_key_name',
        label: 'TSIG Key Name',
        type: 'text',
        required: true,
        placeholder: 'mykey.example.com.',
        hint: 'TSIG key name (should end with a dot).',
      },
      {
        name: 'tsig_key_secret',
        label: 'TSIG Key Secret',
        type: 'password',
        required: true,
        hint: 'Base64-encoded TSIG key secret.',
      },
      {
        name: 'tsig_key_algorithm',
        label: 'TSIG Algorithm',
        type: 'select',
        required: true,
        default: 'hmac-sha256',
        options: [
          { value: 'hmac-md5', label: 'HMAC-MD5 (Legacy)' },
          { value: 'hmac-sha1', label: 'HMAC-SHA1' },
          { value: 'hmac-sha256', label: 'HMAC-SHA256 (Recommended)' },
          { value: 'hmac-sha512', label: 'HMAC-SHA512' },
        ],
        hint: 'TSIG authentication algorithm. HMAC-SHA256 is recommended.',
      },
      {
        name: 'dns_timeout',
        label: 'DNS Timeout',
        type: 'text',
        required: false,
        default: '10s',
        placeholder: '10s',
        hint: 'Timeout for DNS operations (e.g., 10s, 30s).',
      },
    ],
    documentation_url: 'https://tools.ietf.org/html/rfc2136',
  },
}
