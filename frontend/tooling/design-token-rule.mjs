import fs from 'node:fs';
import stylelint from 'stylelint';
const ruleName = 'espacigo/design-tokens';
const messages = stylelint.utils.ruleMessages(ruleName, {
  expected: (property) => `Use a reviewed metric token for ${property}; literals belong in styles/_metrics.scss`,
  breakpoint: 'Use the shared breakpoint mixin; viewport thresholds belong in styles/_breakpoints.scss',
});
const metricNames = new Set([...fs.readFileSync(new URL('../src/styles/_metrics.scss', import.meta.url), 'utf8').matchAll(/^\$([\w-]+):/gm)].map(match => match[1]));
const cssNames = new Set([...fs.readFileSync(new URL('../src/styles/_tokens.scss', import.meta.url), 'utf8').matchAll(/(--[\w-]+)\s*:/g)].map(match => match[1]));
const spacing = /^(margin(?:-.+)?|padding(?:-.+)?|gap|row-gap|column-gap|text-indent|scroll-margin(?:-.+)?|scroll-padding(?:-.+)?)$/;
const typography = /^(font(?:-size|-weight|-family)?|line-height|letter-spacing)$/;
function inspectTokens(value) {
  let unknown = false;
  const rest = value.replace(/var\(([^)]*)\)/g, (reference, contents) => {
    const [name, ...fallback] = contents.split(',');
    if (!cssNames.has(name.trim())) unknown = true;
    return fallback.join(',');
  }).replace(/([\w-]+\.)?\$([\w-]+)/g, (reference, namespace, name) => {
    if (namespace !== 'metrics.' || !metricNames.has(name)) unknown = true;
    return '';
  });
  return { rest: rest.trim(), unknown };
}
const rule = (enabled) => (root, result) => {
  if (!stylelint.utils.validateOptions(result, ruleName, { actual: enabled, possible: [true] })) return;
  root.walkDecls((declaration) => {
    if (!spacing.test(declaration.prop) && !typography.test(declaration.prop)) return;
    const { rest, unknown } = inspectTokens(declaration.value);
    if (unknown || spacing.test(declaration.prop) && /-?\d*\.?\d+(?:[a-z]+|%)/i.test(rest) ||
        typography.test(declaration.prop) && rest && !/^(inherit|normal|initial|unset|revert|0)$/.test(rest)) {
      stylelint.utils.report({ ruleName, result, node: declaration, message: messages.expected(declaration.prop) });
    }
  });
  root.walkAtRules('media', (rule) => {
    if (/(?:min-|max-)?(?:width|height)\s*:/.test(rule.params)) {
      stylelint.utils.report({ ruleName, result, node: rule, message: messages.breakpoint });
    }
  });
};
rule.ruleName = ruleName;
rule.messages = messages;
export default stylelint.createPlugin(ruleName, rule);
