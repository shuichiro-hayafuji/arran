import * as cdk from 'aws-cdk-lib';
import { Match, Template } from 'aws-cdk-lib/assertions';
import { loadEnvironmentConfig } from '../lib/config/environment';
import { SpendableTodayStack } from '../lib/stacks/spendable-today-stack';

const context = { domainName: 'api.arran-test.net', adminCidr: '203.0.113.7/32', keyPairName: 'operator' };
function template(): Template {
  const app = new cdk.App({ context });
  return Template.fromStack(new SpendableTodayStack(app, 'TestStack', loadEnvironmentConfig(app)));
}

describe('Tokyo production', () => {
  test.each(['domainName', 'adminCidr', 'keyPairName'])('requires explicit %s', key => {
    expect(() => loadEnvironmentConfig(new cdk.App({ context: { ...context, [key]: '' } }))).toThrow();
  });
  test.each([
    { domainName: '127.0.0.1' }, { domainName: 'example.com' }, { domainName: 'api.invalid' },
    { adminCidr: '0.0.0.0/0' }, { adminCidr: '999.0.0.1/32' }, { region: 'ap-southeast-2' },
  ])('rejects unsafe or unusable configuration %j', override => {
    expect(() => loadEnvironmentConfig(new cdk.App({ context: { ...context, ...override } }))).toThrow();
  });
  test('Lightsail physical names are unique across resource types', () => {
    const resources = template().toJSON().Resources as Record<string, { Type: string; Properties: Record<string, unknown> }>;
    const names = Object.values(resources)
      .filter(resource => resource.Type.startsWith('AWS::Lightsail::'))
      .flatMap(resource => Object.entries(resource.Properties)
        .filter(([key]) => key === `${resource.Type.split('::').at(-1)}Name`)
        .map(([, value]) => value));
    expect(names).toHaveLength(2);
    expect(new Set(names).size).toBe(names.length);
    expect(names).toContain('arran-production-ip');
  });

  test('contains one retained host and static IP, no legacy services', () => {
    const t = template();
    t.resourceCountIs('AWS::Lightsail::Instance', 1);
    t.resourceCountIs('AWS::Lightsail::StaticIp', 1);
    for (const type of ['AWS::EC2::VPC', 'AWS::EC2::NatGateway', 'AWS::ECS::Service',
      'AWS::RDS::DBInstance', 'AWS::ElasticLoadBalancingV2::LoadBalancer', 'AWS::IAM::AccessKey']) {
      t.resourceCountIs(type, 0);
    }
    t.hasResource('AWS::Lightsail::Instance', {
      DeletionPolicy: 'Retain', UpdateReplacePolicy: 'Retain',
      Properties: Match.objectLike({ BundleId: 'small_3_0', BlueprintId: 'ubuntu_24_04',
        KeyPairName: 'operator',
        AddOns: [{ AddOnType: 'AutoSnapshot', Status: 'Enabled', AutoSnapshotAddOnRequest: { SnapshotTimeOfDay: '18:00' } }],
        Networking: { Ports: [
          { Protocol: 'tcp', FromPort: 80, ToPort: 80, Cidrs: ['0.0.0.0/0'], Ipv6Cidrs: [] },
          { Protocol: 'tcp', FromPort: 443, ToPort: 443, Cidrs: ['0.0.0.0/0'], Ipv6Cidrs: [] },
          { Protocol: 'tcp', FromPort: 22, ToPort: 22, Cidrs: ['203.0.113.7/32'], Ipv6Cidrs: [] },
        ] },
      }),
    });
    t.hasResource('AWS::Lightsail::StaticIp', { DeletionPolicy: 'Retain', UpdateReplacePolicy: 'Retain' });
  });
  test('retains encrypted versioned private backups with bounded retention', () => {
    const t = template();
    t.hasResource('AWS::S3::Bucket', { DeletionPolicy: 'Retain', UpdateReplacePolicy: 'Retain',
      Properties: Match.objectLike({
        VersioningConfiguration: { Status: 'Enabled' },
        BucketEncryption: { ServerSideEncryptionConfiguration: [{ ServerSideEncryptionByDefault: { SSEAlgorithm: 'AES256' } }] },
        PublicAccessBlockConfiguration: { BlockPublicAcls: true, BlockPublicPolicy: true, IgnorePublicAcls: true, RestrictPublicBuckets: true },
        LifecycleConfiguration: { Rules: [Match.objectLike({ Prefix: 'postgres/', ExpirationInDays: 30,
          NoncurrentVersionExpiration: { NoncurrentDays: 7 } })] },
      }),
    });
    t.hasResourceProperties('AWS::S3::BucketPolicy', { PolicyDocument: { Statement: Match.arrayWith([
      Match.objectLike({ Effect: 'Deny', Condition: { Bool: { 'aws:SecureTransport': 'false' } } }),
    ]) } });
  });
  test('backup credentials are not generated and permissions cannot read or delete backups', () => {
    const t = template();
    t.resourceCountIs('AWS::IAM::AccessKey', 0);
    const policies = Object.values(t.findResources('AWS::IAM::Policy'));
    expect(policies).toHaveLength(1);
    expect(policies[0].Properties.PolicyDocument.Statement).toEqual([
      expect.objectContaining({ Action: ['s3:PutObject', 's3:AbortMultipartUpload'], Effect: 'Allow',
        Resource: expect.objectContaining({ 'Fn::Join': expect.any(Array) }) }),
    ]);
    const raw = t.toJSON();
    expect(JSON.stringify(raw.Outputs)).not.toMatch(/password|secret|accesskey|token/i);
    const host = Object.values(t.findResources('AWS::Lightsail::Instance'))[0];
    expect(host.Properties.UserData).toBe('#!/bin/sh\nset -eu\ninstall -d -m 0700 /opt/arran/secrets\n');
  });
});
