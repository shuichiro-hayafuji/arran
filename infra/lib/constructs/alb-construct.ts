import * as cdk from 'aws-cdk-lib';
import * as elbv2 from 'aws-cdk-lib/aws-elasticloadbalancingv2';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as ecs from 'aws-cdk-lib/aws-ecs';
import { Construct } from 'constructs';
import { NetworkConstruct } from './network-construct';

export interface AlbConstructProps {
  readonly projectName: string;
  readonly environmentName: string;
  readonly network: NetworkConstruct;
  readonly service: ecs.FargateService;
}

export class AlbConstruct extends Construct {
  public readonly loadBalancer: elbv2.ApplicationLoadBalancer;

  public constructor(scope: Construct, id: string, props: AlbConstructProps) {
    super(scope, id);
    this.loadBalancer = new elbv2.ApplicationLoadBalancer(this, 'LoadBalancer', {
      loadBalancerName: `${props.projectName}-${props.environmentName}`,
      vpc: props.network.vpc,
      internetFacing: true,
      securityGroup: props.network.albSecurityGroup,
      vpcSubnets: { subnetType: ec2.SubnetType.PUBLIC },
      deletionProtection: false,
      idleTimeout: cdk.Duration.seconds(60),
    });
    const listener = this.loadBalancer.addListener('HttpListener', {
      port: 80,
      protocol: elbv2.ApplicationProtocol.HTTP,
      open: false,
    });
    listener.addTargets('ApiTarget', {
      port: 8080,
      protocol: elbv2.ApplicationProtocol.HTTP,
      targets: [props.service.loadBalancerTarget({ containerName: 'api', containerPort: 8080 })],
      healthCheck: { path: '/health', healthyHttpCodes: '200' },
    });
  }
}
