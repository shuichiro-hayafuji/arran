#!/usr/bin/env python3
"""Read-only Compose contract check; no containers or provider calls."""
import json
import os
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parent
env = dict(os.environ, API_IMAGE='arran-api:validation',
           POSTGRES_IMAGE='postgres:16', CADDY_IMAGE='caddy:2', DOMAIN='api.arran-test.net')
result = subprocess.run(['docker', 'compose', '-f', str(root / 'compose.yaml'),
                         '--profile', 'maintenance', 'config', '--format', 'json'],
                        env=env, check=True, capture_output=True, text=True)
config = json.loads(result.stdout)
services = config['services']
assert set(services) == {'db', 'migrate', 'api', 'caddy'}
assert all(not services[name].get('ports') for name in ('db', 'api', 'migrate'))
assert {str(port['published']) for port in services['caddy']['ports']} == {'80', '443'}
assert services['api']['image'] == services['migrate']['image'] == 'arran-api:validation'
assert services['api']['environment']['APP_ENVIRONMENT'] == 'production'
assert 'DATABASE_URL' not in services['api']['environment']
assert {s['source'] for s in services['api']['secrets']} == {'runtime_url', 'openai_key'}
assert {s['source'] for s in services['migrate']['secrets']} == {'migration_url'}
assert services['migrate']['command'] == ['migrate']
assert config['networks']['backend']['internal'] is True
assert set(services['db']['networks']) == {'backend'}
assert services['api']['read_only'] is True
assert services['api']['tmpfs'] == ['/tmp:size=32m,mode=1777']
print('PASS: Compose public ports, role separation, production mode and DB network isolation')
