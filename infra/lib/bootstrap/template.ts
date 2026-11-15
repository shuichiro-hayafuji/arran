// Deliberately scoped derivative of CDK modern bootstrap v32. No default policy
// parameter, account-wide trust, lookup role, ECR repository or customer KMS key.
type Value = string | Record<string, unknown>;
type Statement = Record<string, unknown>;
const sub = (value: string): Value => ({ 'Fn::Sub': value });
const ref = (value: string): Value => ({ Ref: value });
const arn = (service: string, resource: string, regional = true): Value =>
  sub(`arn:\${AWS::Partition}:${service}:${regional ? '${AWS::Region}' : ''}:\${AWS::AccountId}:${resource}`);
const document = (...statements: Statement[]): Record<string, unknown> => ({ Version: '2012-10-17', Statement: statements });
const allow = (Action: string[], Resource: Value | Value[], Condition?: Record<string, unknown>): Statement =>
  ({ Effect: 'Allow', Action, Resource, ...(Condition ? { Condition } : {}) });
const context = (why: string, ...must: string[]): Record<string, unknown> =>
  ({ 'com.aws.cloudformation.Context': { why, must } });
export const backupBucketName = 'arran-production-backups-${AWS::AccountId}-${AWS::Region}';
export const backupBoundaryName = 'arran-production-backup-writer-boundary';
const writer = arn('iam', 'user/arran-production-backup-writer', false);
const boundary = arn('iam', `policy/${backupBoundaryName}`, false);
const backupBucket = sub(`arn:\${AWS::Partition}:s3:::${backupBucketName}`);
const backupObjects = sub(`arn:\${AWS::Partition}:s3:::${backupBucketName}/postgres/*`);
const toolkitBucket = sub('arn:${AWS::Partition}:s3:::cdk-hnb659fds-assets-${AWS::AccountId}-${AWS::Region}');
const toolkitObjects = sub('arn:${AWS::Partition}:s3:::cdk-hnb659fds-assets-${AWS::AccountId}-${AWS::Region}/*');
const roleName = (kind: string): string => `cdk-hnb659fds-${kind}-role-\${AWS::AccountId}-\${AWS::Region}`;
const roleArn = (kind: string): Value => arn('iam', `role/${roleName(kind)}`, false);
const versionArn = arn('ssm', 'parameter/cdk-bootstrap/hnb659fds/version');
const stackArn = arn('cloudformation', 'stack/arran-production/*');
const toolkitStack = arn('cloudformation', 'stack/CDKToolkit/*');
const tokyo = { StringEquals: { 'aws:RequestedRegion': 'ap-northeast-1' } };
const trustedDeployer = arn('iam', 'role/ArranCdkDeployerTokyo', false);
const assume = document({ Effect: 'Allow', Action: ['sts:AssumeRole', 'sts:TagSession'],
  Principal: { AWS: trustedDeployer }, Condition: { Null: { 'sts:ExternalId': 'true' } } });
const role = (kind: string, policy: Record<string, unknown>): Record<string, unknown> => ({
  Type: 'AWS::IAM::Role',
  Metadata: context('Existing deployment broker is the sole caller; one hour sessions.', 'No account-root or cross-account trust', 'No managed administrator policies'),
  Properties: { RoleName: sub(roleName(kind)), MaxSessionDuration: 3600, AssumeRolePolicyDocument: assume,
    Tags: [{ Key: 'aws-cdk:bootstrap-role', Value: kind === 'file-publishing' ? 'file-publishing' : 'deploy' }],
    Policies: [{ PolicyName: 'arran-limited', PolicyDocument: policy }] },
});

export function bootstrapTemplate(): Record<string, unknown> {
  const execution = document(
    allow(['lightsail:CreateInstances'], '*', { StringEquals: {
      'aws:RequestedRegion': 'ap-northeast-1', 'aws:RequestTag/Project': 'arran', 'aws:RequestTag/Environment': 'production',
    } }),
    // These operations have no resource-level authorization in the official SAR.
    allow(['lightsail:AllocateStaticIp', 'lightsail:EnableAddOn', 'lightsail:DisableAddOn',
      'lightsail:GetInstance', 'lightsail:GetInstances', 'lightsail:GetStaticIp', 'lightsail:GetStaticIps',
      'lightsail:GetOperation', 'lightsail:GetOperations', 'lightsail:GetRegions'], '*', tokyo),
    allow(['lightsail:DeleteInstance', 'lightsail:StartInstance', 'lightsail:StopInstance', 'lightsail:PutInstancePublicPorts', 'lightsail:TagResource', 'lightsail:UntagResource'],
      arn('lightsail', 'Instance/*'), { StringEquals: { 'aws:RequestedRegion': 'ap-northeast-1',
        'aws:ResourceTag/Project': 'arran', 'aws:ResourceTag/Environment': 'production' } }),
    allow(['lightsail:AttachStaticIp'], [arn('lightsail', 'Instance/*'), arn('lightsail', 'StaticIp/*')], tokyo),
    allow(['lightsail:DetachStaticIp', 'lightsail:ReleaseStaticIp'], arn('lightsail', 'StaticIp/*'), tokyo),
    allow(['s3:CreateBucket', 's3:DeleteBucket', 's3:GetBucketLocation', 's3:GetBucketTagging', 's3:PutBucketTagging',
      's3:GetBucketVersioning', 's3:PutBucketVersioning', 's3:GetEncryptionConfiguration', 's3:PutEncryptionConfiguration',
      's3:GetBucketPublicAccessBlock', 's3:PutBucketPublicAccessBlock', 's3:GetLifecycleConfiguration', 's3:PutLifecycleConfiguration',
      's3:GetBucketPolicy', 's3:PutBucketPolicy', 's3:DeleteBucketPolicy', 's3:GetBucketOwnershipControls',
      's3:PutBucketOwnershipControls', 's3:ListBucket',
      's3:GetAccelerateConfiguration', 's3:GetAnalyticsConfiguration', 's3:GetBucketCORS', 's3:GetInventoryConfiguration',
      's3:GetBucketLogging', 's3:GetMetricsConfiguration', 's3:GetBucketNotification', 's3:GetReplicationConfiguration',
      's3:GetBucketWebsite', 's3:GetBucketObjectLockConfiguration', 's3:ListTagsForResource', 's3:GetBucketAbac',
      's3:GetIntelligentTieringConfiguration', 's3:GetBucketMetadataTableConfiguration',
      's3:GetBucketAcl'], backupBucket, tokyo),
    allow(['iam:CreateUser', 'iam:PutUserPermissionsBoundary'], writer,
      { ArnEquals: { 'iam:PermissionsBoundary': boundary } }),
    allow(['iam:GetUser', 'iam:TagUser', 'iam:UntagUser', 'iam:ListUserPolicies', 'iam:GetUserPolicy',
      'iam:PutUserPolicy', 'iam:DeleteUserPolicy', 'iam:ListAttachedUserPolicies', 'iam:ListGroupsForUser',
      'iam:GetLoginProfile', 'iam:ListUserTags'], writer),
    allow(['iam:GetPolicy', 'iam:GetPolicyVersion'], boundary),
    allow(['s3:GetObject', 's3:GetObjectVersion'], toolkitObjects),
    { Effect: 'Deny', Action: ['iam:DeleteUserPermissionsBoundary'], Resource: writer },
    { Effect: 'Deny', Action: ['iam:CreateUser', 'iam:PutUserPermissionsBoundary'], Resource: writer,
      Condition: { ArnNotEquals: { 'iam:PermissionsBoundary': boundary } } },
    { Effect: 'Deny', Action: ['iam:CreatePolicyVersion', 'iam:DeletePolicy', 'iam:DeletePolicyVersion', 'iam:SetDefaultPolicyVersion'], Resource: boundary },
  );
  return {
    AWSTemplateFormatVersion: '2010-09-09',
    Description: 'Arran limited CDK bootstrap: one production stack, bounded backup writer, no administrator fallback.',
    Metadata: { AWSToolsMetrics: { AWSAgentToolkit: 'aws-cloudformation@3' },
      ...context('Derived from modern CDK bootstrap version 32; managed independently by administrator.',
        'Tokyo only; stack name CDKToolkit', 'Never run default cdk bootstrap over this stack',
        'No ECR/image assets, lookups, cross-account publishing or arbitrary application stacks') },
    Parameters: {
      BootstrapVariant: { Type: 'String', Default: 'Arran limited production v1', AllowedValues: ['Arran limited production v1'] },
      OperatorUserName: { Type: 'String', Default: 'arran-operator', AllowedPattern: '[A-Za-z0-9+=,.@_-]{1,64}' },
    },
    Rules: { TokyoOnly: { Assertions: [{ Assert: { 'Fn::Equals': [ref('AWS::Region'), 'ap-northeast-1'] }, AssertDescription: 'Deploy only in Tokyo' }] } },
    Resources: {
      StagingBucket: {
        Type: 'AWS::S3::Bucket', DeletionPolicy: 'Retain', UpdateReplacePolicy: 'Retain',
        Metadata: context('Retained staging for CloudFormation templates and file assets.', 'Private, TLS only, SSE-S3, versioned', 'Never store application secrets'),
        Properties: { BucketName: sub('cdk-hnb659fds-assets-${AWS::AccountId}-${AWS::Region}'),
          BucketEncryption: { ServerSideEncryptionConfiguration: [{ ServerSideEncryptionByDefault: { SSEAlgorithm: 'AES256' } }] },
          PublicAccessBlockConfiguration: { BlockPublicAcls: true, BlockPublicPolicy: true, IgnorePublicAcls: true, RestrictPublicBuckets: true },
          OwnershipControls: { Rules: [{ ObjectOwnership: 'BucketOwnerEnforced' }] },
          VersioningConfiguration: { Status: 'Enabled' },
          LifecycleConfiguration: { Rules: [{ Id: 'OldAssetVersions', Status: 'Enabled', NoncurrentVersionExpiration: { NoncurrentDays: 30 },
            AbortIncompleteMultipartUpload: { DaysAfterInitiation: 1 } }] } },
      },
      StagingBucketPolicy: { Type: 'AWS::S3::BucketPolicy',
        Metadata: context('Block cleartext transport without granting new access.', 'No public or user-ARN allow grants'),
        Properties: { Bucket: ref('StagingBucket'), PolicyDocument: document({ Effect: 'Deny', Action: ['s3:*'], Principal: '*',
          Resource: [toolkitBucket, toolkitObjects], Condition: { Bool: { 'aws:SecureTransport': 'false' } } }) } },
      BackupWriterBoundary: { Type: 'AWS::IAM::ManagedPolicy', DeletionPolicy: 'Retain', UpdateReplacePolicy: 'Retain',
        Metadata: context('Administrator-owned ceiling for the application backup user.', 'Execution role cannot change or detach this ceiling'),
        Properties: { ManagedPolicyName: backupBoundaryName, PolicyDocument: document(
          allow(['s3:PutObject', 's3:AbortMultipartUpload'], backupObjects)) } },
      CloudFormationExecutionRole: { Type: 'AWS::IAM::Role',
        Metadata: context('CloudFormation service role for application resources only.', 'No bootstrap or administrator policy management',
          'SourceArn/SourceAccount not assumed to exist for ordinary stack service-role sessions'),
        Properties: { RoleName: sub(roleName('cfn-exec')), MaxSessionDuration: 3600,
          AssumeRolePolicyDocument: document({ Effect: 'Allow', Action: ['sts:AssumeRole'], Principal: { Service: 'cloudformation.amazonaws.com' } }),
          Policies: [{ PolicyName: 'arran-production-only', PolicyDocument: execution }] } },
      DeploymentActionRole: role('deploy', document(
        allow(['cloudformation:CreateChangeSet', 'cloudformation:CreateStack', 'cloudformation:UpdateStack'], stackArn,
          { StringEquals: { 'cloudformation:RoleArn': roleArn('cfn-exec'), 'aws:RequestedRegion': 'ap-northeast-1' } }),
        allow(['cloudformation:DeleteChangeSet', 'cloudformation:ExecuteChangeSet', 'cloudformation:DescribeChangeSet',
          'cloudformation:DescribeStacks', 'cloudformation:DescribeStackEvents', 'cloudformation:GetTemplate',
          'cloudformation:GetTemplateSummary', 'cloudformation:ListStackResources', 'cloudformation:DescribeStackResources',
          'cloudformation:RollbackStack', 'cloudformation:ContinueUpdateRollback', 'cloudformation:UpdateTerminationProtection'], stackArn, tokyo),
        allow(['cloudformation:DescribeStacks', 'cloudformation:DescribeStackEvents'], toolkitStack, tokyo),
        allow(['iam:PassRole'], roleArn('cfn-exec'), { StringEquals: { 'iam:PassedToService': 'cloudformation.amazonaws.com' } }),
        allow(['s3:GetBucketLocation', 's3:ListBucket'], toolkitBucket),
        allow(['s3:GetObject', 's3:GetObjectVersion'], toolkitObjects),
        allow(['ssm:GetParameter', 'ssm:GetParameters'], versionArn),
      )),
      FilePublishingRole: role('file-publishing', document(
        allow(['s3:GetBucketLocation', 's3:ListBucket', 's3:ListBucketMultipartUploads'], toolkitBucket),
        allow(['s3:GetObject', 's3:PutObject', 's3:AbortMultipartUpload', 's3:ListMultipartUploadParts'], toolkitObjects),
      )),
      CdkBootstrapVersion: { Type: 'AWS::SSM::Parameter',
        Metadata: context('Modern CDK CLI bootstrap compatibility marker.', 'Marker does not advertise support for omitted image/lookup roles'),
        Properties: { Type: 'String', Name: '/cdk-bootstrap/hnb659fds/version', Value: '32' } },
      OperatorBootstrapReadPolicy: { Type: 'AWS::IAM::ManagedPolicy',
        Metadata: context('Add separate verification policy without replacing existing operator access.', 'Application execution role cannot edit this user'),
        Properties: { Users: [ref('OperatorUserName')], ManagedPolicyName: 'ArranCdkToolkitReadTokyo', PolicyDocument: document(
          allow(['cloudformation:DescribeStacks', 'cloudformation:DescribeStackEvents', 'cloudformation:ListStackResources'], toolkitStack, tokyo),
          allow(['iam:GetRole', 'iam:ListRolePolicies', 'iam:ListAttachedRolePolicies', 'iam:GetRolePolicy'],
            [roleArn('deploy'), roleArn('file-publishing'), roleArn('cfn-exec')]),
          allow(['iam:GetPolicy', 'iam:GetPolicyVersion'], boundary),
          allow(['ssm:GetParameter'], versionArn),
          allow(['s3:GetEncryptionConfiguration', 's3:GetBucketPublicAccessBlock', 's3:GetBucketPolicy',
            's3:GetBucketVersioning', 's3:GetLifecycleConfiguration', 's3:GetBucketOwnershipControls'], toolkitBucket),
        ) } },
    },
    Outputs: { BucketName: { Value: ref('StagingBucket') },
      BucketDomainName: { Value: { 'Fn::GetAtt': ['StagingBucket', 'RegionalDomainName'] } },
      BootstrapVersion: { Value: '32' }, BootstrapVariant: { Value: ref('BootstrapVariant') }, BackupWriterBoundaryArn: { Value: ref('BackupWriterBoundary') },
    },
  };
}
