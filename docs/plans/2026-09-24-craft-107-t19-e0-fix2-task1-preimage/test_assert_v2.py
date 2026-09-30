#!/usr/bin/env python3
"""Synthetic RED suite for the evidence verifier; never measured E0 evidence."""
import json, subprocess, sys, tempfile, shutil
from pathlib import Path
import importlib.util
SCRIPT = Path(__file__).with_name('assert_v2.py')
IMAGE = 'sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11'
BASE = dict(baseline_http=200, image_id=IMAGE, adapter_requests=2, process_exit=0,
            task_content='probe-ok', bypass_delta=0, boot_id_stable=True,
            sequence_monotonic=True, restart_snapshot=True)
CASES = {
 'missing-baseline': {'baseline_http': None}, 'baseline-http-000': {'baseline_http': 0},
 'wrong-image-id': {'image_id': 'sha256:wrong'}, 'zero-adapter-requests': {'adapter_requests': 0},
 'exit-zero-missing-task-content': {'task_content': ''}, 'bypass-get-or-connect': {'bypass_delta': 1},
 'stale-boot-or-reset-counters': {'boot_id_stable': False, 'sequence_monotonic': False},
 'missing-restart-snapshot': {'restart_snapshot': False},
}

def run(name, data, expected):
    with tempfile.TemporaryDirectory(prefix='e0-v2-synthetic-') as td:
        root=Path(td); (root/'manifest.json').write_text(json.dumps({'schema':'craft-e0-v2','synthetic':True,'fixture':name,'required':data}))
        p=subprocess.run([sys.executable,str(SCRIPT),str(root)],capture_output=True,text=True)
        if (p.returncode == 0) != expected:
            raise SystemExit(f'{name}: expected pass={expected}, rc={p.returncode}\n{p.stdout}\n{p.stderr}')
        print(f'{name}: rc={p.returncode} (expected)')

for name, mutation in CASES.items():
    data=BASE|mutation; run(name,data,False)
run('complete-synthetic-pass',BASE,True)

# A labelled synthetic artifact also traverses the complete live-manifest
# parser/ledger path, so parser-shape regressions cannot hide behind the compact
# RED inputs above. This artifact is temporary and never enters measured E0.
spec=importlib.util.spec_from_file_location('assert_v2',SCRIPT)
mod=importlib.util.module_from_spec(spec); spec.loader.exec_module(mod)
with tempfile.TemporaryDirectory(prefix='e0-v2-synthetic-full-') as td:
    root=Path(td); rid='synthetic-e0-parser-pass'; roles=('gateway','provider','proxy','host','adapter')
    names={'client':'syn-client','adapter':'syn-adapter','direct':'syn-direct','private':'syn-private','external':'syn-external'}
    logs=root/'logs'; logs.mkdir(); ledgers=root/'ledgers'; ledgers.mkdir(); snaps=root/'snapshots'; snaps.mkdir()
    for case in mod.REQUIRED_CASES:
        (logs/f'{case}.stdout.txt').write_text('synthetic stdout\n')
        (logs/f'{case}.stderr.txt').write_text('synthetic stderr\n')
    start={r:0 for r in roles}; end=start.copy(); zero={r:0 for r in roles}
    cases={}
    for case in mod.REQUIRED_CASES:
        cases[case]={'status':'pass','reason':'','command':['synthetic only'],'exit_code':0,'stdout_file':f'logs/{case}.stdout.txt','stderr_file':f'logs/{case}.stderr.txt','ledger_start':start.copy(),'ledger_end':end.copy(),'ledger_delta':zero.copy()}
    cases['image'].update(actual={'id':IMAGE,'platform':'linux/arm64'})
    cases['topology'].update(client_mounts_ledger=False,client_ledger_write_denied=True,client_ledger_write_stdout='denied',client_attachment_count=1,private_members=sorted([names['client'],names['adapter']]),private_internal=True,isolated_gateway_mode=True,client_networks=[names['private']],adapter_networks=sorted([names['private'],names['external']]),direct_networks=[names['external']],client_privileged=False,client_cap_add=[],client_port_bindings={},adapter_ip_forward_disabled=True,binary_sha256_match=True,ipv6_disabled=True,non_loopback_ipv6_routes=0,runtime_facts_file='snapshots/runtime.json')
    cases['normal_route'].update(task_content='probe-ok',adapter_requests=2,ledger_end={**end,'adapter':2},ledger_delta={**zero,'adapter':2})
    cases['normal_route_post'].update(task_content='probe-ok')
    cases['restart'].update(same_container=True,same_mounts=True,marker_persisted=True,global_config_persisted=True)
    cases['host_route_pre'].update(positive_control_http=200,positive_control_after_http=200,bypass_delta=0)
    cases['host_route_post'].update(positive_control_http=200,positive_control_after_http=200,bypass_delta=0)
    for case in mod.NEGATIVE_CASES:
        cases[case].update(exit_code=7,http_000_required=True,intended_attempt='synthetic curl route',ledger_delta=zero.copy())
    for case in ('hostile_project_url','hostile_global_url','hostile_project_url_post','hostile_project_dns_post','hostile_global_url_post'):
        base='http://mock-hostile-provider:8081/v1' if case=='hostile_project_dns_post' else 'http://synthetic-direct:8081/v1'
        cases[case].update(ledger_delta=zero.copy(),intended_base_url=base,selected_base_url=base,effective_config_trace=f'synthetic load providerID=mock baseURL={base}')
    cases['custom_url_dns_pre'].update(intended_base_url='http://mock-hostile-provider:8081/v1',adapter_requests=0,ledger_delta=zero.copy())
    for case in ('redirect_307_pre','redirect_308_pre','redirect_307_post','redirect_308_post'):
        cases[case].update(curl_follow_control_exit=0,curl_follow_provider_requests=1,adapter_original_posts=1,ledger_delta=zero.copy())
    for case in ('adapter_proxy_pre','adapter_proxy_post'):
        cases[case].update(external_delta=0)
    cases['cleanup'].update(resources_absent=True,cleanup_exit=0)
    (snaps/'runtime.json').write_text(json.dumps({'stdout':'1.18.4\n3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e  /usr/local/bin/opencode\n'}))
    (snaps/f"pre-{names['adapter']}-inspect.json").write_text(json.dumps([{'HostConfig':{'Sysctls':{'net.ipv4.ip_forward':'0'}}}]))
    for role in roles:
        path=ledgers/f'{role}.jsonl'
        events=[{'run_id':rid,'boot_id':'synthetic-boot','seq':1,'event':'tcp_accept','role':role}]
        if role in ('gateway','provider','proxy'):
            events.append({'run_id':rid,'boot_id':'synthetic-boot','seq':2,'event':'http','case':f'control:synthetic:{role}:dns','method':'GET','role':role,'status':200,'complete':True})
        if role=='proxy':
            events.append({'run_id':rid,'boot_id':'synthetic-boot','seq':3,'event':'http','case':'control:synthetic:proxy:connect','method':'CONNECT','role':role,'status':200,'complete':True})
        if role=='adapter':
            events += [
                {'run_id':rid,'boot_id':'synthetic-boot','seq':2,'event':'http','case':'normal_route','method':'POST','purpose':'title','model':'mock-model','stream':True},
                {'run_id':rid,'boot_id':'synthetic-boot','seq':3,'event':'http','case':'normal_route','method':'POST','purpose':'task','model':'mock-model','stream':True},
            ]
        path.write_text(''.join(json.dumps(e)+'\n' for e in events))
    (root/'manifest.json').write_text(json.dumps({'schema':'craft-e0-v2','run_id':rid,'evidence_type':'synthetic','image':{'id':IMAGE,'platform':'linux/arm64','version':'1.18.4'},'names':names,'cases':cases,'ledgers':{r:f'ledgers/{r}.jsonl' for r in roles}}))
    p=subprocess.run([sys.executable,str(SCRIPT),str(root)],capture_output=True,text=True)
    if p.returncode != 0: raise SystemExit('synthetic full-parser pass fixture rejected:\n'+p.stdout+'\n'+p.stderr)
    if '"synthetic": true' not in p.stdout: raise SystemExit('full-parser fixture was not visibly labelled synthetic')
    print('complete-synthetic-live-schema: rc=0 (expected; excluded from measured E0)')
    full_fixture_root=Path(tempfile.mkdtemp(prefix='e0-v2-full-base-'))
    shutil.copytree(root, full_fixture_root, dirs_exist_ok=True)

# Fix1 trust-boundary mutations operate on a copy of a complete synthetic
# schema artifact. They must not mutate the immutable measured run.
import shutil

def clone_artifact(source, label):
    target = Path(tempfile.mkdtemp(prefix=f'e0-v2-{label}-'))
    shutil.copytree(source, target, dirs_exist_ok=True)
    return target

def reject_mutation(label, mutate):
    root = clone_artifact(full_fixture_root, label)
    mutate(root)
    p = subprocess.run([sys.executable, str(SCRIPT), str(root)], capture_output=True, text=True)
    if p.returncode == 0:
        raise SystemExit(f'{label}: forged fixture unexpectedly passed\n{p.stdout}')
    print(f'{label}: rc={p.returncode} (forgery rejected)')

# A client ledger bind mount is a trust-boundary failure even if all other
# manifest claims say pass.
def client_mount(root):
    m = json.loads((root/'manifest.json').read_text())
    m['cases']['topology']['client_mounts_ledger'] = True
    (root/'manifest.json').write_text(json.dumps(m))

# Raw provider traffic attributed to a denied direct-IP case invalidates the
# verdict even when manifest deltas and ranges claim zero.
def raw_bypass(root):
    m = json.loads((root/'manifest.json').read_text())
    p = root/m['ledgers']['provider']
    event = {'run_id':m['run_id'],'boot_id':'synthetic-boot','seq':3,'event':'http','case':'direct_ip_pre','method':'GET','path':'/bypass','role':'provider'}
    with p.open('a') as out: out.write(json.dumps(event)+'\n')

# Hostile URL must be tied to process-observed config loading/selection.
def hostile_no_trace(root):
    m = json.loads((root/'manifest.json').read_text())
    m['cases']['hostile_project_url'].update(status='pass', intended_base_url='http://mock-hostile-provider:8081/v1', selected_base_url='http://mock-hostile-provider:8081/v1')
    m['cases']['hostile_project_url'].pop('effective_config_trace_file', None)
    (root/'manifest.json').write_text(json.dumps(m))

def missing_connect_control(root):
    m=json.loads((root/'manifest.json').read_text()); p=root/m['ledgers']['proxy']
    rows=[json.loads(x) for x in p.read_text().splitlines() if x]
    rows=[e for e in rows if e.get('method')!='CONNECT']
    p.write_text(''.join(json.dumps(e)+'\n' for e in rows))

def positive_count_forgery(root):
    m=json.loads((root/'manifest.json').read_text())
    m['cases']['normal_route']['adapter_requests']=3
    (root/'manifest.json').write_text(json.dumps(m))

def forged_range(root):
    m=json.loads((root/'manifest.json').read_text())
    m['cases']['direct_ip_pre']['ledger_start']={r:999 for r in roles}
    m['cases']['direct_ip_pre']['ledger_end']={r:1000 for r in roles}
    (root/'manifest.json').write_text(json.dumps(m))

def missing_pass(root):
    m=json.loads((root/'manifest.json').read_text())
    m['cases']['direct_ip_pre'].pop('status',None)
    (root/'manifest.json').write_text(json.dumps(m))

reject_mutation('client-can-mount-ledger', client_mount)
reject_mutation('raw-ledger-bypass-with-forged-zero-range', raw_bypass)
reject_mutation('missing-raw-connect-control', missing_connect_control)
reject_mutation('nonzero-positive-turn-count-forgery', positive_count_forgery)
reject_mutation('falsified-case-range', forged_range)
reject_mutation('missing-manifest-pass-status', missing_pass)
reject_mutation('hostile-config-without-effective-load-trace', hostile_no_trace)

import importlib.util
RUNNER=SCRIPT.with_name('run_v2.py')
rspec=importlib.util.spec_from_file_location('run_v2',RUNNER)
rmod=importlib.util.module_from_spec(rspec);rspec.loader.exec_module(rmod)
state={'tick':0,'events':0}
def alive():
    state['tick']+=1
    return state['tick'] < 4
def counters():
    if state['tick']==4: state['events']+=1  # a late child request after SIGTERM
    return {'provider':state['events']}
late=rmod.wait_for_quiescence(alive,counters,timeout=2,stable_samples=2,interval=0.01)
if not late['quiescent'] or late['final_counts']['provider'] != 1:
    raise SystemExit(f'late child was not included before quiescence: {late}')
stuck=rmod.wait_for_quiescence(lambda: True,lambda: {'provider':0},timeout=0.03,stable_samples=2,interval=0.01)
if stuck['quiescent']:
    raise SystemExit('timeout quiescence passed while child remained alive')
print('timeout-late-child-quiescence: late request included before stable counters')
print('timeout-live-child: blocked as expected')
