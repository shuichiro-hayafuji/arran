import * as ec2 from 'aws-cdk-lib/aws-ec2';
import { Construct } from 'constructs';

export interface NetworkConstructProps {
  readonly projectName: string;
  readonly environmentName: string;
}

export class NetworkConstruct extends Construct {
  public readonly vpc: ec2.Vpc;
  public readonly albSecurityGroup: ec2.SecurityGroup;
  public readonly ecsSecurityGroup: ec2.SecurityGroup;
  public readonly databaseSecurityGroup: ec2.SecurityGroup;

  public constructor(scope: Construct, id: string, props: NetworkConstructProps) {
    super(scope, id);

    // ALB と RDS のサブネットグループの要件を満たすため、2つの AZ に配置する。
    this.vpc = new ec2.Vpc(this, 'Vpc', {
      vpcName: `${props.projectName}-${props.environmentName}-vpc`,
      ipAddresses: ec2.IpAddresses.cidr('10.20.0.0/16'),
      maxAzs: 2,
      natGateways: 1,
      subnetConfiguration: [
        { name: 'Public', subnetType: ec2.SubnetType.PUBLIC, cidrMask: 24 },
        { name: 'Private', subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS, cidrMask: 24 },
      ],
    });

    this.albSecurityGroup = new ec2.SecurityGroup(this, 'AlbSecurityGroup', {
      vpc: this.vpc,
      securityGroupName: `${props.projectName}-${props.environmentName}-alb-sg`,
      allowAllOutbound: false,
      description: 'Internet HTTP ingress and ECS-only egress for the ALB',
    });
    this.albSecurityGroup.addIngressRule(ec2.Peer.anyIpv4(), ec2.Port.tcp(80), 'Public HTTP');

    // DNS、ECR、CloudWatch、OpenAI、DBへの通信を通すため、MVPでは送信を許可する。
    // 制限する場合は、必要なエンドポイントと送信ルールも併せて定義する。
    this.ecsSecurityGroup = new ec2.SecurityGroup(this, 'EcsSecurityGroup', {
      vpc: this.vpc,
      securityGroupName: `${props.projectName}-${props.environmentName}-ecs-sg`,
      allowAllOutbound: true,
      description: 'ALB-only ingress for the Go API tasks',
    });
    this.ecsSecurityGroup.addIngressRule(this.albSecurityGroup, ec2.Port.tcp(8080), 'ALB to API');

    this.databaseSecurityGroup = new ec2.SecurityGroup(this, 'DatabaseSecurityGroup', {
      vpc: this.vpc, securityGroupName: `${props.projectName}-${props.environmentName}-db-sg`,
      allowAllOutbound: false, description: 'ECS-only PostgreSQL ingress',
    });
    this.databaseSecurityGroup.addIngressRule(this.ecsSecurityGroup, ec2.Port.tcp(5432), 'API to PostgreSQL');

    this.albSecurityGroup.addEgressRule(this.ecsSecurityGroup, ec2.Port.tcp(8080), 'ALB to API');
  }
}
