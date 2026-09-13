import * as cdk from 'aws-cdk-lib';

export interface EnvironmentConfig {
  readonly projectName: string;
  readonly environmentName: string;
  readonly region: string;
  readonly openAiSecretArn?: string;
}

export function loadEnvironmentConfig(app: cdk.App): EnvironmentConfig {
  const openAiSecretArn = app.node.tryGetContext('openAiSecretArn') as string | undefined;
  const region = app.node.tryGetContext('region') as string | undefined;
  return {
    projectName: 'spendable-today',
    environmentName: 'dev',
    region: region ?? 'ap-northeast-1',
    openAiSecretArn,
  };
}
