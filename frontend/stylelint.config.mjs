export default {
  customSyntax: 'postcss-scss',
  rules: { 'color-no-hex': true, 'declaration-no-important': true, 'block-no-empty': true },
  overrides: [
    { files: ['src/styles/_tokens.scss'], rules: { 'color-no-hex': null } },
    { files: ['src/styles.scss'], rules: { 'declaration-no-important': null } },
  ],
};
