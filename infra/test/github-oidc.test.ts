import * as fs from 'node:fs';
import * as path from 'node:path';
import template from '../github-oidc/arran-github-oidc.template.json';

test('GitHub trust pins audience, immutable repository identity and main', () => {
  const role = template.Resources.GitHubCdkRole.Properties;
  const statement = role.AssumeRolePolicyDocument.Statement[0];
  expect(statement.Action).toBe('sts:AssumeRoleWithWebIdentity');
  expect(statement.Condition.StringEquals).toEqual({
    'token.actions.githubusercontent.com:aud': 'sts.amazonaws.com',
    'token.actions.githubusercontent.com:sub': 'repo:shuichiro-hayafuji@107348618/arran@1318128437:ref:refs/heads/main',
  });
  expect(role.Policies[0].PolicyDocument.Statement[0].Resource).toHaveLength(2);
  expect(JSON.stringify(role.Policies)).not.toMatch(/AdministratorAccess|iam:PassRole|cloudformation:/);
  expect(template.Parameters.ProviderMode.AllowedValues).toEqual(['Create', 'Reuse']);
  expect(template.Resources.GitHubProvider.DeletionPolicy).toBe('Retain');
});

test('manual workflow is main-only, pinned and rechecks main before deployment', () => {
  const workflow = fs.readFileSync(path.join(__dirname, '../../.github/workflows/cdk-production.yml'), 'utf8');
  expect(workflow).toContain('workflow_dispatch:');
  expect(workflow).toContain("github.ref == 'refs/heads/main'");
  expect(workflow.match(/git ls-remote/g)).toHaveLength(2);
  expect(workflow).toContain('allowed-account-ids: \'435013325489\'');
  expect(workflow).toContain('id-token: write');
  expect(workflow).toContain('synth --no-lookups');
  expect(workflow).not.toMatch(/^\s+uses: [^@]+@(?![a-f0-9]{40})/m);
  expect(workflow.indexOf('git ls-remote', workflow.indexOf('Confirm latest main'))).toBeLessThan(workflow.indexOf('npm run cdk -- --app cdk.out deploy'));
});
