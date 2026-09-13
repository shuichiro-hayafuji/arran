import * as cdk from 'aws-cdk-lib';
import * as secretsmanager from 'aws-cdk-lib/aws-secretsmanager';
import { Construct } from 'constructs';
import { EnvironmentConfig } from '../config/environment';
import { AlbConstruct } from '../constructs/alb-construct';
import { ContainerConstruct } from '../constructs/container-construct';
import { DatabaseConstruct } from '../constructs/database-construct';
import { NetworkConstruct } from '../constructs/network-construct';

export class SpendableTodayStack extends cdk.Stack {
  public constructor(scope: Construct, id: string, config: EnvironmentConfig, props?: cdk.StackProps) {
    super(scope, id, {
      ...props,
      env: { account: process.env.CDK_DEFAULT_ACCOUNT, region: config.region },
      stackName: `${config.projectName}-${config.environmentName}`,
      description: 'AWS platform for the Spendable Today Go API',
    });

    cdk.Tags.of(this).add('Project', config.projectName);
    cdk.Tags.of(this).add('Environment', config.environmentName);
    cdk.Tags.of(this).add('ManagedBy', 'aws-cdk');

    const network = new NetworkConstruct(this, 'Network', config);
    const database = new DatabaseConstruct(this, 'Database', { ...config, network });
    const openAiSecret = config.openAiSecretArn
      ? secretsmanager.Secret.fromSecretCompleteArn(this, 'OpenAiSecret', config.openAiSecretArn)
      : new secretsmanager.Secret(this, 'OpenAiSecret', {
          secretName: `${config.projectName}-${config.environmentName}-openai`,
          description: 'Populate this empty secret with the OpenAI API key after deployment',
          removalPolicy: cdk.RemovalPolicy.DESTROY,
        });
    const container = new ContainerConstruct(this, 'Container', { ...config, network, database, openAiSecret });
    const alb = new AlbConstruct(this, 'Alb', { ...config, network, service: container.service });

    new cdk.CfnOutput(this, 'AlbDnsName', { value: alb.loadBalancer.loadBalancerDnsName });
    new cdk.CfnOutput(this, 'ApiBaseUrl', { value: `http://${alb.loadBalancer.loadBalancerDnsName}` });
    new cdk.CfnOutput(this, 'EcsClusterName', { value: container.cluster.clusterName });
    new cdk.CfnOutput(this, 'EcsServiceName', { value: container.service.serviceName });
    new cdk.CfnOutput(this, 'EcrRepositoryUri', { value: container.repository.repositoryUri });
    new cdk.CfnOutput(this, 'OpenAiSecretArn', { value: openAiSecret.secretArn });
    new cdk.CfnOutput(this, 'RdsEndpoint', { value: database.instance.dbInstanceEndpointAddress });
  }
}
