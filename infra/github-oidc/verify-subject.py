#!/usr/bin/env python3
"""Fail closed if GitHub emits an unexpected OIDC context; never print the JWT."""
import base64
import json
import os
import urllib.parse
import urllib.request

url = os.environ['ACTIONS_ID_TOKEN_REQUEST_URL']
separator = '&' if '?' in url else '?'
request = urllib.request.Request(
    url + separator + urllib.parse.urlencode({'audience': 'sts.amazonaws.com'}),
    headers={'Authorization': 'Bearer ' + os.environ['ACTIONS_ID_TOKEN_REQUEST_TOKEN']},
)
with urllib.request.urlopen(request, timeout=15) as response:
    jwt = json.load(response)['value']
payload = jwt.split('.')[1]
claims = json.loads(base64.urlsafe_b64decode(payload + '=' * (-len(payload) % 4)))
expected = {
    'iss': 'https://token.actions.githubusercontent.com',
    'aud': 'sts.amazonaws.com',
    'sub': 'repo:shuichiro-hayafuji@107348618/arran@1318128437:ref:refs/heads/main',
    'repository_id': '1318128437',
    'repository_owner_id': '107348618',
    'ref': 'refs/heads/main',
}
if any(claims.get(key) != value for key, value in expected.items()):
    raise SystemExit('OIDC identity did not match expected repository and main branch')
print('OIDC subject and repository IDs matched; AWS validates the token signature')
