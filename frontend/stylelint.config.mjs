export default {
  customSyntax: 'postcss-scss',
  plugins: ['./tooling/design-token-rule.mjs'],
  rules: {
    'color-no-hex': true,
    'declaration-no-important': true,
    'block-no-empty': true,
    'espacigo/design-tokens': true,
  },
  overrides: [
    {
      files: ['src/styles/_tokens.scss'],
      rules: { 'color-no-hex': null, 'espacigo/design-tokens': null },
    },
    {
      files: ['src/styles/_metrics.scss', 'src/styles/_breakpoints.scss'],
      rules: { 'espacigo/design-tokens': null },
    },
    { files: ['src/styles.scss'], rules: { 'declaration-no-important': null } },
  ],
};
