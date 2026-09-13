import * as path from 'node:path';
import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as ecr from 'aws-cdk-lib/aws-ecr';
import * as ecrAssets from 'aws-cdk-lib/aws-ecr-assets';
import * as ecs from 'aws-cdk-lib/aws-ecs';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as secretsmanager from 'aws-cdk-lib/aws-secretsmanager';
import { Construct } from 'constructs';
import { NetworkConstruct } from './network-construct';
import { DatabaseConstruct } from './database-construct';

export interface ContainerConstructProps {
  readonly projectName: string;
  readonly environmentName: string;
  readonly region: string;
  readonly network: NetworkConstruct;
  readonly database: DatabaseConstruct;
  readonly openAiSecret: secretsmanager.ISecret;
}

export class ContainerConstruct extends Construct {
  public readonly cluster: ecs.Cluster;
  public readonly service: ecs.FargateService;
  public readonly repository: ecr.Repository;

  public constructor(scope: Construct, id: string, props: ContainerConstructProps) {
    super(scope, id);

    this.repository = new ecr.Repository(this, 'Repository', {
      repositoryName: `${props.projectName}-${props.environmentName}`,
      imageScanOnPush: true,
      lifecycleRules: [{ maxImageCount: 10 }],
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      emptyOnDelete: true,
    });

    const image = new ecrAssets.DockerImageAsset(this, 'ApiImage', {
      directory: path.resolve(__dirname, '../../../server'),
      platform: ecrAssets.Platform.LINUX_AMD64,
    });

    this.cluster = new ecs.Cluster(this, 'Cluster', {
      clusterName: `${props.projectName}-${props.environmentName}`,
      vpc: props.network.vpc,
      containerInsights: false,
    });

    const logGroup = new logs.LogGroup(this, 'LogGroup', {
      logGroupName: `/ecs/${props.projectName}-${props.environmentName}`,
      retention: logs.RetentionDays.ONE_WEEK,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
    });
    const executionRole = new iam.Role(this, 'TaskExecutionRole', {
      roleName: `${props.projectName}-${props.environmentName}-execution`,
      assumedBy: new iam.ServicePrincipal('ecs-tasks.amazonaws.com'),
    });
    // DockerImageAsset is published to the CDK bootstrap ECR repository. The
    // repository name is account-specific, but CDK exposes it so pulls can
    // still be scoped to one repository instead of all ECR repositories.
    const assetRepositoryArn = cdk.Stack.of(this).formatArn({
      service: 'ecr',
      resource: 'repository',
      resourceName: image.repositoryName,
    });
    executionRole.addToPolicy(new iam.PolicyStatement({
      actions: ['ecr:BatchCheckLayerAvailability', 'ecr:BatchGetImage', 'ecr:GetDownloadUrlForLayer'],
      resources: [assetRepositoryArn],
    }));
    executionRole.addToPolicy(new iam.PolicyStatement({
      actions: ['ecr:GetAuthorizationToken'],
      resources: ['*'],
    }));
    executionRole.addToPolicy(new iam.PolicyStatement({
      actions: ['logs:CreateLogStream', 'logs:PutLogEvents'],
      resources: [logGroup.logGroupArn],
    }));

    const taskRole = new iam.Role(this, 'TaskRole', {
      roleName: `${props.projectName}-${props.environmentName}-task`,
      assumedBy: new iam.ServicePrincipal('ecs-tasks.amazonaws.com'),
    });
    // ECS Exec uses these APIs with resource "*" because AWS does not expose
    // resource-level permissions for the execute-command message channels.
    taskRole.addToPolicy(new iam.PolicyStatement({
      actions: ['ssmmessages:CreateControlChannel', 'ssmmessages:CreateDataChannel', 'ssmmessages:OpenControlChannel', 'ssmmessages:OpenDataChannel'],
      resources: ['*'],
    }));

    props.database.secret.grantRead(executionRole);
    props.openAiSecret.grantRead(executionRole);

    const taskDefinition = new ecs.FargateTaskDefinition(this, 'TaskDefinition', {
      family: `${props.projectName}-${props.environmentName}`,
      cpu: 256,
      memoryLimitMiB: 512,
      executionRole,
      taskRole,
      runtimePlatform: {
        cpuArchitecture: ecs.CpuArchitecture.X86_64,
        operatingSystemFamily: ecs.OperatingSystemFamily.LINUX,
      },
    });
    const container = taskDefinition.addContainer('ApiContainer', {
      image: ecs.ContainerImage.fromDockerImageAsset(image),
      containerName: 'api',
      logging: ecs.LogDrivers.awsLogs({ streamPrefix: 'api', logGroup }),
      environment: {
        API_ADDR: '0.0.0.0:8080',
        DB_HOST: props.database.instance.dbInstanceEndpointAddress,
        DB_PORT: '5432',
        DB_NAME: 'spendable_today',
        DB_SSLMODE: 'require',
        USE_MOCK_LLM: 'false',
      },
      secrets: {
        DB_USER: ecs.Secret.fromSecretsManager(props.database.secret, 'username'),
        DB_PASSWORD: ecs.Secret.fromSecretsManager(props.database.secret, 'password'),
        OPENAI_API_KEY: ecs.Secret.fromSecretsManager(props.openAiSecret),
      },
      healthCheck: {
        command: ['CMD-SHELL', 'wget -q -O - http://127.0.0.1:8080/health || exit 1'],
        interval: cdk.Duration.seconds(30),
        timeout: cdk.Duration.seconds(5),
        retries: 3,
        startPeriod: cdk.Duration.seconds(10),
      },
    });
    container.addPortMappings({ containerPort: 8080, protocol: ecs.Protocol.TCP });

    this.service = new ecs.FargateService(this, 'Service', {
      serviceName: `${props.projectName}-${props.environmentName}`,
      cluster: this.cluster,
      taskDefinition,
      desiredCount: 1,
      assignPublicIp: false,
      securityGroups: [props.network.ecsSecurityGroup],
      vpcSubnets: { subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS },
      enableExecuteCommand: true,
      circuitBreaker: { rollback: true },
      minHealthyPercent: 0,
      maxHealthyPercent: 200,
    });

  }
}
