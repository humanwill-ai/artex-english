#!/usr/bin/env python3
"""Opt-in loopback-only inference broker for local workflow evaluations.

Real credentials remain in broker memory; clients receive only a temporary local
nonce. No tool is executed here. The fixed provider endpoint is the only remote
request destination; request bodies and bounded replies stay in ignored work/.
"""
import argparse
import datetime as dt
from decimal import Decimal
import fcntl
import hashlib
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import re
import secrets
import threading
import urllib.error
import urllib.request
import uuid

import run_localization_eval as common

ROUTE = re.compile(r'^/(original|english)/(workflow|boundary)/([a-z0-9_]{1,80})/chat/completions$')
MAX_CALLS = 64

def prepare(path, incoming):
    match = ROUTE.fullmatch(path)
    if not match or not isinstance(incoming, dict):
        raise ValueError('Invalid evaluation route or request')
    if incoming.get('model') != common.MODEL or incoming.get('stream', False) is not False:
        raise ValueError('Only the selected non-streaming model is allowed')
    messages = incoming.get('messages')
    if not isinstance(messages, list) or not messages:
        raise ValueError('Messages are required')
    for message in messages:
        if not isinstance(message, dict) or message.get('role') not in {'system','user','assistant','tool'}:
            raise ValueError('Unsupported message')
        content = message.get('content')
        if content is not None and not isinstance(content, str):
            raise ValueError('Only text messages are allowed')
    # Reject potentially remote multimodal/attachment features and opaque fields.
    allowed = {'model','messages','tools','tool_choice','parallel_tool_calls','max_tokens',
               'max_completion_tokens','stream','reasoning_effort','temperature','top_p','stop'}
    if set(incoming) - allowed:
        raise ValueError('Unexpected provider request fields')
    body = dict(incoming)
    if body.get('reasoning_effort', 'none') != 'none':
        raise ValueError('Reasoning configuration must match the evaluation')
    for field in ('max_tokens','max_completion_tokens'):
        if field in body and (type(body[field]) is not int or not 0 < body[field] <= common.MAX_OUTPUT):
            raise ValueError('Output token limit exceeds evaluation bound')
    body.pop('max_completion_tokens', None)
    body.update(max_tokens=common.MAX_OUTPUT, stream=False, reasoning_effort='none')
    config = common.MODEL_CONFIG[common.MODEL]
    body['provider'] = {'only':['openai'],'allow_fallbacks':False,'require_parameters':True,
        'max_price':{'prompt':float(config['prompt_per_million']),
                     'completion':float(config['output_per_million']),'request':0}}
    bound = common.reservation(body)
    return {'edition':match[1],'suite':match[2],'case':match[3]}, body, bound

class Broker:
    def __init__(self, work, budget, secret, nonce, opener):
        self.work, self.budget, self.secret, self.nonce, self.opener = work, budget, secret, nonce, opener
        self.lock = threading.Lock()
        self.run = str(uuid.uuid4())
        self.calls = 0
        self.failed = False
        self.case_counts = {}

    def complete(self, path, incoming):
        meta, body, bound = prepare(path, incoming)
        with self.lock:
            key = (meta['edition'],meta['suite'],meta['case'])
            limit = 20 if meta['suite']=='workflow' else 1
            if self.failed or self.calls >= MAX_CALLS or self.case_counts.get(key,0) >= limit:
                raise ValueError('Evaluation stopped or call limit reached')
            self.calls += 1
            self.case_counts[key] = self.case_counts.get(key,0)+1
            identity = {**meta,'run':self.run,'sequence':self.calls,
                        'request_sha256':hashlib.sha256(common.encode(body)).hexdigest()}
            job_id = hashlib.sha256(common.encode(identity)).hexdigest()
            attempt = self.budget.reserve(job_id, bound)
            record = {**identity,'job_id':job_id,'reserved_usd':str(bound),
                      'request':common.scrub(body,self.secret)}
            try:
                request = urllib.request.Request(common.ENDPOINT, data=common.encode(body),method='POST',
                    headers={'Authorization':'Bearer '+self.secret,'Content-Type':'application/json',
                             'X-OpenRouter-Title':'ARTEX isolated local workflow parity'})
                with self.opener.open(request,timeout=75) as response:
                    reply = common.read_reply(response)
                cost = Decimal(str(reply.get('usage',{}).get('cost')))
                if not cost.is_finite() or cost < 0 or cost > bound:
                    raise ValueError('Provider returned invalid or unbounded cost')
                if reply.get('model') not in common.MODEL_IDS:
                    raise ValueError('Provider returned a different model')
                if not reply.get('choices') or reply['choices'][0].get('finish_reason') not in {'stop','tool_calls'}:
                    raise ValueError('Incomplete or malformed provider response')
                reply = common.scrub(reply,self.secret)
                record.update(state='completed',response=reply)
                self.budget.finish(attempt,'completed',cost)
            except Exception as exc:
                self.failed = True
                record.update(state='failed',error_type=type(exc).__name__)
                if isinstance(exc,urllib.error.HTTPError):record['http_status']=exc.code
                self.budget.finish(attempt,'failed')
                raise
            finally:
                fd=os.open(self.work/'requests.jsonl',os.O_WRONLY|os.O_CREAT|os.O_APPEND,0o600)
                with os.fdopen(fd,'ab') as out:
                    out.write(common.encode(record)+b'\n');out.flush();os.fsync(out.fileno())
                print(json.dumps({**meta,'sequence':self.calls,'state':record['state'],
                                  'reserved_total_usd':str(self.budget.reserved)}),flush=True)
            return reply

class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args): pass
    def do_POST(self):
        broker = self.server.broker
        token = self.headers.get('Authorization','')
        if not hmac.compare_digest(token,'Bearer '+broker.nonce):
            self.send_error(403,'Local authentication required');return
        try:
            size=int(self.headers.get('Content-Length','0'))
            if size <= 0 or size > common.MAX_BODY_BYTES or self.headers.get('Transfer-Encoding'):
                raise ValueError('Invalid body size')
            incoming=json.loads(self.rfile.read(size))
            reply=broker.complete(self.path,incoming)
            raw=common.encode(reply)
            self.send_response(200);self.send_header('Content-Type','application/json')
            self.send_header('Content-Length',str(len(raw)));self.end_headers();self.wfile.write(raw)
        except Exception:
            # No provider response bodies, credentials, or exception strings leave
            # the broker. The local log retains a type/status only on failures.
            self.send_error(400,'Evaluation request rejected or stopped')

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--execute',action='store_true')
    parser.add_argument('--keychain-service',required=True)
    parser.add_argument('--keychain-account',default='api-key')
    args=parser.parse_args()
    if not args.execute:parser.error('Explicit --execute is required')
    work=common.WORK/'local-workflows';work.mkdir(mode=0o700,parents=True,exist_ok=True);work.chmod(0o700)
    with (common.WORK/'run.lock').open('a') as lock:
        fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
        budget=common.Budget(common.WORK/'budget.json')
        secret=common.keychain_key(args.keychain_service,args.keychain_account)
        nonce=secrets.token_urlsafe(32)
        opener=urllib.request.build_opener(urllib.request.ProxyHandler({}),common.NoRedirect())
        server=ThreadingHTTPServer(('127.0.0.1',0),Handler)
        server.broker=Broker(work,budget,secret,nonce,opener)
        server.daemon_threads=True
        common.atomic_json(work/'broker-info.json',{'base_url':'http://127.0.0.1:'+str(server.server_port),
            'local_token':nonce,'pid':os.getpid(),'run':server.broker.run,'cap_usd':str(common.CAP),
            'started_utc':dt.datetime.now(dt.timezone.utc).isoformat()})
        print('Loopback broker ready; local client details stored in ignored work directory.',flush=True)
        try:server.serve_forever()
        finally:server.server_close()

if __name__=='__main__':
    try:main()
    except Exception as exc:
        print('Broker stopped: '+type(exc).__name__,flush=True)
        raise SystemExit(2)
