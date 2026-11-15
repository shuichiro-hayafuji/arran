import * as fs from 'node:fs';
import * as path from 'node:path';
import * as os from 'node:os';
import * as cdk from 'aws-cdk-lib';
import { Template } from 'aws-cdk-lib/assertions';
import { bootstrapTemplate } from '../lib/bootstrap/template';
import { SpendableTodayStack } from '../lib/stacks/spendable-today-stack';
import { loadEnvironmentConfig } from '../lib/config/environment';
import committed from '../bootstrap/arran-toolkit.template.json';
import reference from '../bootstrap/authorization-reference.json';

const resources = committed.Resources;
const execution = resources.CloudFormationExecutionRole.Properties.Policies[0].PolicyDocument.Statement;
const deploy = resources.DeploymentActionRole.Properties.Policies[0].PolicyDocument.Statement;
const asText = (value: unknown): string => JSON.stringify(value);

test('generated upload artifact stays synchronized and contains no administrator fallback', () => {
  expect(bootstrapTemplate()).toEqual(committed);
  expect(asText(committed)).not.toMatch(/AdministratorAccess|ReadOnlyAccess|TrustedAccounts|CloudFormationExecutionPolicies/);
  expect(Object.values(resources).filter(r => r.Type === 'AWS::IAM::Role')).toHaveLength(3);
  expect(Object.values(resources).some(r => r.Type === 'AWS::ECR::Repository' || r.Type === 'AWS::KMS::Key')).toBe(false);
  expect(committed.Outputs.BootstrapVersion.Value).toBe('32');
  expect(Buffer.byteLength(asText(committed))).toBeLessThan(51200);
});

test('only existing deployment broker can assume publishing and deployment roles', () => {
  for (const role of [resources.DeploymentActionRole, resources.FilePublishingRole]) {
    expect(role.Properties.MaxSessionDuration).toBe(3600);
    expect(role.Properties.AssumeRolePolicyDocument.Statement).toEqual([{
      Effect: 'Allow', Action: ['sts:AssumeRole', 'sts:TagSession'],
      Principal: { AWS: { 'Fn::Sub': 'arn:${AWS::Partition}:iam::${AWS::AccountId}:role/ArranCdkDeployerTokyo' } },
      Condition: { Null: { 'sts:ExternalId': 'true' } },
    }]);
  }
  expect(resources.CloudFormationExecutionRole.Properties.AssumeRolePolicyDocument.Statement).toEqual([
    { Effect: 'Allow', Action: ['sts:AssumeRole'], Principal: { Service: 'cloudformation.amazonaws.com' } },
  ]);
});

test('application deployment is stack scoped, and PassRole can target only its execution role', () => {
  for (const statement of deploy.filter(s => s.Action.some(a => a.startsWith('cloudformation:')))) {
    expect(asText(statement.Resource)).toMatch(/stack\/(arran-production|CDKToolkit)\/\*/);
    if (asText(statement.Resource).includes('CDKToolkit')) {
      expect(statement.Action.every(a => a.startsWith('cloudformation:Describe'))).toBe(true);
    }
  }
  const pass = deploy.filter(s => s.Action.includes('iam:PassRole'));
  expect(pass).toHaveLength(1);
  expect(asText(pass[0].Resource)).toContain('role/cdk-hnb659fds-cfn-exec-role-${AWS::AccountId}-${AWS::Region}');
  expect(pass[0].Condition).toEqual({ StringEquals: { 'iam:PassedToService': 'cloudformation.amazonaws.com' } });
  expect(execution.some(s => s.Action.includes('iam:PassRole'))).toBe(false);
});

test('CloudFormation execution can resolve only the exact CDK bootstrap version parameter', () => {
  const statements = execution.filter(s => s.Action.some(a => a.startsWith('ssm:')));
  expect(statements).toEqual([{
    Effect: 'Allow', Action: ['ssm:GetParameters'],
    Resource: { 'Fn::Sub': 'arn:${AWS::Partition}:ssm:${AWS::Region}:${AWS::AccountId}:parameter/cdk-bootstrap/hnb659fds/version' },
  }]);
});

test('writer creation requires exact immutable boundary; execution cannot administer bootstrap identities', () => {
  const creation = execution.find(s => s.Effect === 'Allow' && s.Action.includes('iam:CreateUser'));
  expect(asText(creation?.Resource)).toContain('user/arran-production-backup-writer');
  expect(asText(creation?.Condition)).toContain('iam:PermissionsBoundary');
  expect(asText(creation?.Condition)).toContain('policy/arran-production-backup-writer-boundary');
  expect(execution.some(s => s.Effect === 'Deny' && s.Action.includes('iam:DeleteUserPermissionsBoundary'))).toBe(true);
  expect(execution.some(s => s.Effect === 'Deny' && s.Action.includes('iam:CreatePolicyVersion'))).toBe(true);
  for (const s of execution.filter(s => s.Effect === 'Allow' && s.Action.some(a => a.startsWith('iam:')))) {
    expect(asText(s.Resource)).toMatch(/(user\/arran-production-backup-writer|policy\/arran-production-backup-writer-boundary)/);
    expect(s.Action).not.toEqual(expect.arrayContaining(['iam:CreateAccessKey']));
    expect(s.Action).not.toContain('iam:AttachUserPolicy');
    expect(s.Action).not.toContain('iam:PutRolePolicy');
  }
  expect(resources.BackupWriterBoundary.Properties.PolicyDocument.Statement).toEqual([{
    Effect: 'Allow', Action: ['s3:PutObject', 's3:AbortMultipartUpload'],
    Resource: { 'Fn::Sub': 'arn:${AWS::Partition}:s3:::arran-production-backups-${AWS::AccountId}-${AWS::Region}/postgres/*' },
  }]);
  expect(resources.OperatorBootstrapReadPolicy.Type).toBe('AWS::IAM::ManagedPolicy');
  expect(resources.OperatorBootstrapReadPolicy.Properties.Users).toEqual([{ Ref: 'OperatorUserName' }]);
});

test('restore operator reads only backup objects and lists only the postgres prefix', () => {
  const statements = resources.OperatorBootstrapReadPolicy.Properties.PolicyDocument.Statement
    .filter(statement => asText(statement.Resource).includes('s3:::arran-production-backups-'));
  expect(statements).toEqual([
    { Effect: 'Allow', Action: ['s3:GetObject', 's3:GetObjectVersion'],
      Resource: { 'Fn::Sub': 'arn:${AWS::Partition}:s3:::arran-production-backups-${AWS::AccountId}-${AWS::Region}/postgres/*' } },
    { Effect: 'Allow', Action: ['s3:ListBucket'],
      Resource: { 'Fn::Sub': 'arn:${AWS::Partition}:s3:::arran-production-backups-${AWS::AccountId}-${AWS::Region}' },
      Condition: { StringLike: { 's3:prefix': ['postgres/*'] } } },
    { Effect: 'Allow', Action: ['s3:GetLifecycleConfiguration', 's3:GetBucketVersioning'],
      Resource: { 'Fn::Sub': 'arn:${AWS::Partition}:s3:::arran-production-backups-${AWS::AccountId}-${AWS::Region}' } },
  ]);
});

test('allow actions match official SAR; wildcard resources are only Tokyo Lightsail operations without ARN support', () => {
  const actions = new Map<string, unknown>(Object.entries(reference.services).flatMap(([service, value]) =>
    Object.entries(value.actions).map(([name, action]) => [`${service}:${name}`, action] as const)));
  const inspect = (node: unknown): void => {
    if (Array.isArray(node)) { node.forEach(inspect); return; }
    if (!node || typeof node !== 'object') return;
    const object = node as Record<string, unknown>;
    if (object.Effect === 'Allow' && Array.isArray(object.Action)) {
      for (const action of object.Action as string[]) expect(actions.has(action)).toBe(true);
      if (object.Resource === '*') {
        for (const action of object.Action as string[]) {
          expect(action.startsWith('lightsail:')).toBe(true);
          expect(actions.get(action)).not.toHaveProperty('Resources');
        }
        expect(asText(object.Condition)).toContain('"aws:RequestedRegion":"ap-northeast-1"');
      }
    }
    Object.values(object).forEach(inspect);
  };
  inspect(committed);
  expect(asText(resources.CloudFormationExecutionRole.Properties.Policies).length).toBeLessThan(10240);
});

test('application requires boundary and synth uses file publishing without lookups or Docker assets', () => {
  const outdir = fs.mkdtempSync(path.join(os.tmpdir(), 'arran-bootstrap-synth-'));
  try {
    const app = new cdk.App({ outdir, context: { domainName: 'api.arran-test.net', adminCidr: '203.0.113.7/32', keyPairName: 'operator' } });
    const stack = new SpendableTodayStack(app, 'TestStack', loadEnvironmentConfig(app));
    const t = Template.fromStack(stack);
    t.hasResourceProperties('AWS::IAM::User', {
      PermissionsBoundary: { 'Fn::Join': ['', ['arn:', { Ref: 'AWS::Partition' }, ':iam::', { Ref: 'AWS::AccountId' }, ':policy/arran-production-backup-writer-boundary']] },
    });
    t.hasResourceProperties('AWS::S3::Bucket', { BucketName: { 'Fn::Sub': 'arran-production-backups-${AWS::AccountId}-${AWS::Region}' } });
    const assembly = app.synth();
    expect(assembly.manifest.missing ?? []).toHaveLength(0);
    const artifact = assembly.manifest.artifacts?.['TestStack.assets'];
    const file = (artifact?.properties as { file?: string })?.file;
    expect(file).toBeDefined();
    const assets = JSON.parse(fs.readFileSync(path.join(outdir, file!), 'utf8')) as { files: Record<string, unknown>; dockerImages?: Record<string, unknown> };
    expect(Object.keys(assets.dockerImages ?? {})).toHaveLength(0);
    expect(asText(assets.files)).toContain('cdk-hnb659fds-file-publishing-role-');
    expect(asText(assets.files)).not.toContain('image-publishing');
  } finally { fs.rmSync(outdir, { recursive: true, force: true }); }
});
