import * as cdk from 'aws-cdk-lib';
import { Match, Template } from 'aws-cdk-lib/assertions';
import { loadEnvironmentConfig } from '../lib/config/environment';
import { SpendableTodayStack } from '../lib/stacks/spendable-today-stack';

function template(): Template {
  const app = new cdk.App();
  const stack = new SpendableTodayStack(app, 'TestStack', loadEnvironmentConfig(app));
  return Template.fromStack(stack);
}

describe('SpendableTodayStack', () => {
  test('creates a VPC and internet-facing ALB', () => {
    const t = template();
    t.resourceCountIs('AWS::EC2::VPC', 1);
    t.hasResourceProperties('AWS::ElasticLoadBalancingV2::LoadBalancer', {
      Scheme: 'internet-facing',
    });
  });

  test('uses public task subnets without a NAT gateway and isolates the database', () => {
    const t = template();
    t.resourceCountIs('AWS::EC2::NatGateway', 0);
    t.resourceCountIs('AWS::EC2::EIP', 0);

    const subnets = t.findResources('AWS::EC2::Subnet');
    const publicSubnetIds = Object.keys(subnets).filter(id => id.includes('PublicSubnet'));
    const databaseSubnetIds = Object.keys(subnets).filter(id => id.includes('DatabaseSubnet'));
    expect(publicSubnetIds).toHaveLength(2);
    expect(databaseSubnetIds).toHaveLength(2);

    t.hasResourceProperties('AWS::ECS::Service', {
      DesiredCount: 1,
      NetworkConfiguration: {
        AwsvpcConfiguration: {
          AssignPublicIp: 'ENABLED',
          Subnets: Match.arrayWith(publicSubnetIds.map(id => ({ Ref: id }))),
        },
      },
    });
    t.hasResourceProperties('AWS::RDS::DBSubnetGroup', {
      SubnetIds: Match.arrayWith(databaseSubnetIds.map(id => ({ Ref: id }))),
    });
    const publicRoutes = t.findResources('AWS::EC2::Route');
    expect(Object.keys(publicRoutes)).toHaveLength(2);
    expect(Object.keys(publicRoutes).every(routeId => routeId.includes('PublicSubnet'))).toBe(true);
  });

  test('uses private PostgreSQL and does not create an EFS or S3 data store', () => {
    const t = template();
    t.hasResourceProperties('AWS::RDS::DBInstance', { PubliclyAccessible: false, DBName: 'spendable_today', Port: '5432' });
    t.resourceCountIs('AWS::EFS::FileSystem', 0);
    t.resourceCountIs('AWS::S3::Bucket', 0);
  });

  test('uses server/ settings and permits only ALB and PostgreSQL task ingress', () => {
    const t = template();
    t.hasResourceProperties('AWS::ECS::TaskDefinition', {
      ContainerDefinitions: Match.arrayWith([Match.objectLike({
        Environment: Match.arrayWith([
          { Name: 'API_ADDR', Value: '0.0.0.0:8080' },
          { Name: 'DB_NAME', Value: 'spendable_today' },
          { Name: 'OPENAI_MODEL', Value: 'gpt-5.6-terra' },
          { Name: 'OPENAI_REASONING_EFFORT', Value: 'medium' },
          { Name: 'APP_ENVIRONMENT', Value: 'dev' },
        ]),
      })]),
    });
    t.hasResourceProperties('AWS::ECS::TaskDefinition', {
      ContainerDefinitions: Match.arrayWith([Match.objectLike({
        Secrets: Match.arrayWith([
          Match.objectLike({ Name: 'OPENAI_API_KEY' }),
          Match.objectLike({ Name: 'PAGERDUTY_ROUTING_KEY' }),
        ]),
      })]),
    });
    const groups = t.findResources('AWS::EC2::SecurityGroup');
    const groupId = (description: string): string => {
      const ids = Object.keys(groups).filter(id => groups[id].Properties.GroupDescription === description);
      expect(ids).toHaveLength(1);
      return ids[0];
    };
    const albGroupId = groupId('Internet HTTP ingress and ECS-only egress for the ALB');
    const ecsGroupId = groupId('ALB-only ingress for the Go API tasks');
    const databaseGroupId = groupId('ECS-only PostgreSQL ingress');
    expect(groups[ecsGroupId].Properties.SecurityGroupIngress).toBeUndefined();
    expect(groups[databaseGroupId].Properties.SecurityGroupIngress).toBeUndefined();
    const ingressRules = Object.values(t.findResources('AWS::EC2::SecurityGroupIngress'));
    expect(ingressRules).toHaveLength(2);
    expect(ingressRules.map(rule => rule.Properties)).toEqual(expect.arrayContaining([
      expect.objectContaining({
        GroupId: { 'Fn::GetAtt': [ecsGroupId, 'GroupId'] },
        SourceSecurityGroupId: { 'Fn::GetAtt': [albGroupId, 'GroupId'] },
        FromPort: 8080,
        ToPort: 8080,
      }),
      expect.objectContaining({
        GroupId: { 'Fn::GetAtt': [databaseGroupId, 'GroupId'] },
        SourceSecurityGroupId: { 'Fn::GetAtt': [ecsGroupId, 'GroupId'] },
        FromPort: 5432,
        ToPort: 5432,
      }),
    ]));
  });
});
