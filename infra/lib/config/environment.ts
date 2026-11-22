import * as cdk from 'aws-cdk-lib';

export interface EnvironmentConfig {
  readonly projectName: string;
  readonly environmentName: 'production';
  readonly region: 'ap-northeast-1';
  readonly domainName: string;
  readonly adminCidr: string;
  readonly keyPairName: string;
}

export function loadEnvironmentConfig(app: cdk.App): EnvironmentConfig {
  const required = (name: string): string => {
    const value: unknown = app.node.tryGetContext(name);
    if (typeof value !== 'string' || !value.trim()) throw new Error(`Required context: ${name}`);
    return value.trim();
  };
  const domainName = required('domainName');
  if (!/^(?=.{1,253}$)([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$/.test(domainName)
      || domainName.endsWith('.example') || domainName.endsWith('.invalid') || domainName === 'example.com') {
    throw new Error('domainName must be a real, owned DNS name; HTTPS cannot use an IP address');
  }
  const adminCidr = required('adminCidr');
  const match = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\/(\d{1,2})$/.exec(adminCidr);
  if (!match || match.slice(1, 5).some(value => Number(value) > 255) || Number(match[5]) < 24 || Number(match[5]) > 32) {
    throw new Error('adminCidr must be an explicit IPv4 /24 to /32 network; prefer your current /32');
  }
  if (app.node.tryGetContext('region') && app.node.tryGetContext('region') !== 'ap-northeast-1') {
    throw new Error('Production is restricted to Tokyo (ap-northeast-1)');
  }
  return { projectName: 'arran', environmentName: 'production', region: 'ap-northeast-1',
    domainName, adminCidr, keyPairName: required('keyPairName') };
}
