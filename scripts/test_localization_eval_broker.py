"""Offline tests: provider and credentials are never accessed."""
import contextlib
from decimal import Decimal
import io
import json
from pathlib import Path
import tempfile
import threading
import urllib.request
import urllib.error
import unittest
from unittest.mock import Mock
import localization_eval_broker as broker
import run_localization_eval as common

ROUTE='/english/workflow/known_mismatch/chat/completions'
def request():
    return {'model':common.MODEL,'messages':[{'role':'system','content':'Only local fixtures.'},{'role':'user','content':'Read the synthetic record.'}],'max_tokens':1536,'stream':False,'reasoning_effort':'none'}
def reply():
    return {'model':common.MODEL,'usage':{'cost':0.001},'choices':[{'finish_reason':'stop','message':{'role':'assistant','content':'Recorded.'}}]}

class BrokerTests(unittest.TestCase):
    def setUp(self):
        tmp=tempfile.TemporaryDirectory();self.addCleanup(tmp.cleanup);self.root=Path(tmp.name)
        self.budget=common.Budget(self.root/'budget.json')
        self.opener=Mock();self.opener.open.side_effect=lambda *a,**k:io.BytesIO(common.encode(reply()))
        self.broker=broker.Broker(self.root,self.budget,'fake-provider-secret','fake-local-token',self.opener)

    def call(self,path=ROUTE,body=None):
        with contextlib.redirect_stdout(io.StringIO()):return self.broker.complete(path,body or request())

    def test_route_and_remote_features_rejected_before_reservation(self):
        for route in ['https://elsewhere.invalid/chat/completions','/english/other/x/chat/completions','/original/workflow/../../x/chat/completions']:
            with self.assertRaises(ValueError):self.call(route)
        for field,value in [('model','other-model'),('stream',True),('max_tokens',99999),('reasoning_effort','high'),('service_tier','priority'),('provider',{'allow_fallbacks':True})]:
            body=request();body[field]=value
            with self.assertRaises(ValueError):self.call(body=body)
        body=request();body['messages'][1]['content']=[{'type':'image_url','image_url':{'url':'https://elsewhere.invalid/x'}}]
        with self.assertRaises(ValueError):self.call(body=body)
        self.opener.open.assert_not_called();self.assertEqual(self.budget.reserved,0)

    def test_fixed_endpoint_provider_limits_and_durable_shared_budget(self):
        self.budget.reserve('previous-run',Decimal('0.10'))
        def inspect(req,**kwargs):
            d=json.loads((self.root/'budget.json').read_text());self.assertEqual(d['attempts'][-1]['state'],'pending')
            self.assertEqual(req.full_url,common.ENDPOINT)
            body=json.loads(req.data);self.assertEqual(body['provider']['only'],['openai'])
            self.assertFalse(body['provider']['allow_fallbacks']);self.assertEqual(body['max_tokens'],1536)
            return io.BytesIO(common.encode(reply()))
        self.opener.open.side_effect=inspect
        self.call()
        d=json.loads((self.root/'budget.json').read_text());self.assertEqual(len(d['attempts']),2)
        self.assertEqual(d['attempts'][0]['job_id'],'previous-run')
        self.assertGreater(self.budget.reserved,Decimal('0.10'))
        self.assertNotIn('fake-provider-secret',(self.root/'requests.jsonl').read_text())

    def test_errors_stop_all_further_calls_and_keep_reservation(self):
        self.opener.open.side_effect=TimeoutError('fake-provider-secret')
        with self.assertRaises(TimeoutError):self.call()
        reserved=self.budget.reserved;self.assertGreater(reserved,0)
        with self.assertRaises(ValueError):self.call('/original/workflow/known_mismatch/chat/completions')
        self.assertEqual(self.budget.reserved,reserved);self.assertEqual(self.opener.open.call_count,1)
        self.assertNotIn('fake-provider-secret',(self.root/'requests.jsonl').read_text())

    def test_incomplete_wrong_model_or_cost_rejected(self):
        for mutation in ['incomplete','model','cost']:
            with self.subTest(mutation=mutation):
                self.broker.failed=False
                value=reply()
                if mutation=='incomplete':value['choices'][0]['finish_reason']='length'
                if mutation=='model':value['model']='other-model'
                if mutation=='cost':value['usage']['cost']='NaN'
                self.opener.open.side_effect=lambda *a,**k:io.BytesIO(common.encode(value))
                with self.assertRaises(ValueError):self.call()
                self.assertTrue(self.broker.failed)

    def test_boundary_once_and_total_call_limits(self):
        route='/original/boundary/safe_read/chat/completions'
        self.call(route)
        with self.assertRaises(ValueError):self.call(route)
        self.broker.calls=broker.MAX_CALLS
        with self.assertRaises(ValueError):self.call()
        self.assertEqual(self.opener.open.call_count,1)

    def test_http_auth_and_payload_bounds_prevent_provider_access(self):
        server=broker.ThreadingHTTPServer(('127.0.0.1',0),broker.Handler)
        server.broker=self.broker
        thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
        try:
            opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
            endpoint='http://127.0.0.1:'+str(server.server_port)+ROUTE
            for token,body,status in [('wrong-token',common.encode(request()),403),
                                      ('fake-local-token',b'x'*(common.MAX_BODY_BYTES+1),400)]:
                req=urllib.request.Request(endpoint,data=body,method='POST',headers={'Authorization':'Bearer '+token})
                with self.assertRaises(urllib.error.HTTPError) as error:
                    opener.open(req,timeout=3)
                self.assertEqual(error.exception.code,status)
                error.exception.close()
            self.opener.open.assert_not_called()
            self.assertEqual(self.budget.reserved,0)
        finally:
            server.shutdown();server.server_close();thread.join(timeout=3)

    def test_budget_exhaustion_sends_nothing(self):
        self.budget.reserve('all-reserved',common.CAP)
        with self.assertRaises(ValueError):self.call()
        self.opener.open.assert_not_called()

if __name__=='__main__':unittest.main()
