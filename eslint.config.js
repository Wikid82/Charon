import frontendConfig from './frontend/eslint.config.js';

export default [
  {
    ignores: [
      'backend/**/*',
      'data/**/*',
      'scripts/**/*',
      'tools/**/*',
      'docs/**/*',
      '.venv/**/*',
      'node_modules/**/*',
      'dist/**/*',
      '*.md',
      '*.yml',
      '*.yaml',
      '*.json',
      '*.toml',
      '*.sh',
      'Dockerfile*',
      '.git/**/*',
      '.github/**/*'
    ]
  },
  // Apply frontend config to frontend files only
  ...frontendConfig.map(config => ({
    ...config,
    files: config.files ? config.files.map(pattern => `frontend/${pattern}`) : ['frontend/**/*.{ts,tsx,js,jsx}']
  }))
];
