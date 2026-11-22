import * as cdk from 'aws-cdk-lib';
import * as lightsail from 'aws-cdk-lib/aws-lightsail';
import * as s3 from 'aws-cdk-lib/aws-s3';
import * as iam from 'aws-cdk-lib/aws-iam';
import { Construct } from 'constructs';
import { EnvironmentConfig } from '../config/environment';
import { backupBoundaryName, backupBucketName } from '../bootstrap/template';

export class SpendableTodayStack extends cdk.Stack {
  public constructor(scope: Construct, id: string, config: EnvironmentConfig, props?: cdk.StackProps) {
    super(scope, id, { ...props,
      // CDK diff reads the deployed template through the lookup role. Reuse the
      // stack-scoped deploy role; do not introduce general context lookup access.
      synthesizer: new cdk.DefaultStackSynthesizer({
        lookupRoleArn: 'arn:${AWS::Partition}:iam::${AWS::AccountId}:role/cdk-hnb659fds-deploy-role-${AWS::AccountId}-${AWS::Region}',
      }),
      env: { account: process.env.CDK_DEFAULT_ACCOUNT, region: config.region },
      stackName: 'arran-production', description: 'Tokyo single-host production API and PostgreSQL; no NAT, ECS, ALB or RDS',
      terminationProtection: true });
    cdk.Tags.of(this).add('Project', config.projectName);
    cdk.Tags.of(this).add('Environment', config.environmentName);
    const instance = new lightsail.CfnInstance(this, 'Host', {
      instanceName: 'arran-production',
      blueprintId: 'ubuntu_24_04',
      bundleId: 'small_3_0',
      keyPairName: config.keyPairName,
      addOns: [{ addOnType: 'AutoSnapshot', status: 'Enabled',
        autoSnapshotAddOnRequest: { snapshotTimeOfDay: '18:00' } }],
      networking: { ports: [
        { protocol: 'tcp', fromPort: 80, toPort: 80, cidrs: ['0.0.0.0/0'], ipv6Cidrs: [] },
        { protocol: 'tcp', fromPort: 443, toPort: 443, cidrs: ['0.0.0.0/0'], ipv6Cidrs: [] },
        { protocol: 'tcp', fromPort: 22, toPort: 22, cidrs: [config.adminCidr], ipv6Cidrs: [] },
      ] },
      // Only non-secret setup instructions. No git token, password or AWS key in cloud-init.
      userData: '#!/bin/sh\nset -eu\ninstall -d -m 0700 /opt/arran/secrets\n',
    });
    instance.applyRemovalPolicy(cdk.RemovalPolicy.RETAIN);
    const ip = new lightsail.CfnStaticIp(this, 'PublicAddress', {
      staticIpName: 'arran-production-ip', attachedTo: instance.ref,
    });
    ip.applyRemovalPolicy(cdk.RemovalPolicy.RETAIN);
    const backups = new s3.Bucket(this, 'Backups', {
      bucketName: cdk.Fn.sub(backupBucketName),
      encryption: s3.BucketEncryption.S3_MANAGED, blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      enforceSSL: true, versioned: true, removalPolicy: cdk.RemovalPolicy.RETAIN,
      lifecycleRules: [{ id: 'ExpireDatabaseBackups', prefix: 'postgres/', expiration: cdk.Duration.days(7),
        noncurrentVersionExpiration: cdk.Duration.days(1), abortIncompleteMultipartUploadAfter: cdk.Duration.days(1) },
      { id: 'RemoveExpiredDatabaseBackupMarkers', prefix: 'postgres/', expiredObjectDeleteMarker: true }],
    });
    // Lightsail cannot attach EC2 instance profiles. Create no access key in CloudFormation.
    const writer = new iam.User(this, 'BackupWriter', { userName: 'arran-production-backup-writer',
      permissionsBoundary: iam.ManagedPolicy.fromManagedPolicyArn(this, 'BackupBoundary',
        this.formatArn({ service: 'iam', region: '', resource: 'policy', resourceName: backupBoundaryName })),
    });
    writer.addToPolicy(new iam.PolicyStatement({
      actions: ['s3:PutObject', 's3:AbortMultipartUpload'], resources: [backups.arnForObjects('postgres/*')],
    }));
    writer.applyRemovalPolicy(cdk.RemovalPolicy.RETAIN);
    new cdk.CfnOutput(this, 'HostName', { value: instance.ref });
    new cdk.CfnOutput(this, 'StaticIp', { value: ip.attrIpAddress });
    new cdk.CfnOutput(this, 'ApiBaseUrl', { value: `https://${config.domainName}` });
    new cdk.CfnOutput(this, 'BackupBucket', { value: backups.bucketName });
    new cdk.CfnOutput(this, 'BackupWriterName', { value: writer.userName });
  }
}
