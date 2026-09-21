import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as rds from 'aws-cdk-lib/aws-rds';
import * as secretsmanager from 'aws-cdk-lib/aws-secretsmanager';
import { Construct } from 'constructs';
import { NetworkConstruct } from './network-construct';

export interface DatabaseConstructProps { readonly projectName: string; readonly environmentName: string; readonly network: NetworkConstruct; }

/** ローカルの Docker と同じ PostgreSQL アダプターで接続する RDS を構成する。
 * 開発用に削除保護を無効化し、スタック削除時はDBも削除する設定。
 */
export class DatabaseConstruct extends Construct {
  public readonly instance: rds.DatabaseInstance;
  public readonly secret: secretsmanager.ISecret;
  public constructor(scope: Construct, id: string, props: DatabaseConstructProps) {
    super(scope, id);
    this.instance = new rds.DatabaseInstance(this, 'Postgres', {
      engine: rds.DatabaseInstanceEngine.postgres({ version: rds.PostgresEngineVersion.VER_16 }),
      instanceType: ec2.InstanceType.of(ec2.InstanceClass.BURSTABLE3, ec2.InstanceSize.MICRO),
      vpc: props.network.vpc, vpcSubnets: { subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS },
      securityGroups: [props.network.databaseSecurityGroup], databaseName: 'spendable_today',
      port: 5432,
      allocatedStorage: 20, storageType: rds.StorageType.GP3, storageEncrypted: true,
      publiclyAccessible: false, multiAz: false, backupRetention: cdk.Duration.days(1),
      deletionProtection: false, deleteAutomatedBackups: true, removalPolicy: cdk.RemovalPolicy.DESTROY,
      credentials: rds.Credentials.fromGeneratedSecret('spendable_today_admin', { secretName: `${props.projectName}-${props.environmentName}-db-credentials` }),
    });
    this.secret = this.instance.secret!;
  }
}
