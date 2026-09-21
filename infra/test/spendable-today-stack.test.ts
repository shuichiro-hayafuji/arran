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

  test('runs one private Fargate task without a public IP', () => {
    const t = template();
    t.hasResourceProperties('AWS::ECS::Service', {
      DesiredCount: 1,
      NetworkConfiguration: {
        AwsvpcConfiguration: { AssignPublicIp: 'DISABLED' },
      },
    });
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
    t.hasResourceProperties('AWS::EC2::SecurityGroupIngress', {
      FromPort: 8080,
      ToPort: 8080,
      SourceSecurityGroupId: Match.anyValue(),
    });
    t.hasResourceProperties('AWS::EC2::SecurityGroupIngress', {
      FromPort: 5432,
      ToPort: 5432,
      SourceSecurityGroupId: Match.anyValue(),
    });
  });
});
