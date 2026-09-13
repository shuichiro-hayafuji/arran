#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib';
import { loadEnvironmentConfig } from '../lib/config/environment';
import { SpendableTodayStack } from '../lib/stacks/spendable-today-stack';

const app = new cdk.App();
const config = loadEnvironmentConfig(app);
new SpendableTodayStack(app, 'SpendableTodayDevStack', config);
