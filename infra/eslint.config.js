const eslint = require('@eslint/js');
const tseslint = require('typescript-eslint');

module.exports = [
  { ignores: ['**/*.js', 'cdk.out/**', 'node_modules/**'] },
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.ts'],
    ignores: ['cdk.out/**', 'node_modules/**'],
    languageOptions: {
      parserOptions: { project: './tsconfig.json' },
    },
    rules: {
      'no-console': 'warn',
    },
  },
];
