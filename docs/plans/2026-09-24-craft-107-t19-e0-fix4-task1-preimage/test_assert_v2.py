#!/usr/bin/env python3
"""Synthetic RED suite for the evidence verifier; never measured E0 evidence."""
import json, subprocess, sys, tempfile, shutil, hashlib
from pathlib import Path
import importlib.util
SCRIPT = Path(__file__).with_name('assert_v2.py')
IMAGE = 'sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11'
# All synthetic and live evidence now runs through the same full source-stream schema.

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
    (logs/'normal_route.stdout.txt').write_text('assistant: probe-ok\n')
    (logs/'normal_route_post.stdout.txt').write_text('assistant: probe-ok\n')
    (logs/'restore_route.stdout.txt').write_text('assistant: probe-ok\n')
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
    cleanup_resources=['syn-client','syn-adapter','syn-direct','syn-helper','syn-private','syn-external','syn-xdg']
    cases['cleanup'].update(resources_absent=True,cleanup_exit=0,registered_resources=cleanup_resources)
    (snaps/'runtime.json').write_text(json.dumps({'stdout':'1.18.4\n3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e  /usr/local/bin/opencode\n'}))
    (snaps/f"pre-{names['adapter']}-inspect.json").write_text(json.dumps([{'HostConfig':{'Sysctls':{'net.ipv4.ip_forward':'0'}}}]))
    safe_client_inspect = [{'Mounts': [{'Type':'volume','Source':'syn-xdg','Destination':'/home/opencode/.local/share/opencode'}], 'HostConfig': {'Privileged':False,'CapAdd':None,'NetworkMode':'syn-private'}}]
    (snaps/f"pre-{names['client']}-inspect.json").write_text(json.dumps(safe_client_inspect))
    (snaps/f"post-restart-{names['client']}-inspect.json").write_text(json.dumps(safe_client_inspect))
    synthetic_inspect={'pre':{},'post_restart':{}}
    for stage in ('pre','post_restart'):
        for participant in ('client','adapter','direct','helper'):
            container={'Id':f'synthetic-{participant}-id','Mounts':[],'HostConfig':{'Privileged':False,'CapAdd':None,'NetworkMode':'syn-private' if participant=='client' else 'bridge'}}
            if participant=='client': container['Mounts']=safe_client_inspect[0]['Mounts']
            if participant=='adapter': container['HostConfig']['Sysctls']={'net.ipv4.ip_forward':'0'}
            synthetic_inspect[stage][participant]={'container':container}
        synthetic_inspect[stage]['private_network']={'Name':'syn-private','Containers':{names['client']:{},names['adapter']:{}}}
        synthetic_inspect[stage]['external_network']={'Name':'syn-external','Containers':{names['direct']:{},names['adapter']:{},'syn-helper':{}}}
    (snaps/'owned-inspect.json').write_text(json.dumps(synthetic_inspect))
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
    # Build one complete chronological direct/adapter stream and host command log.
    source_rows = {'direct': [], 'adapter': []}
    source_boots = {'direct': 'direct-boot', 'adapter': 'adapter-boot'}
    def emit(source, event, **fields):
        row = {'run_id':rid,'source':source,'boot_id':source_boots[source],'seq':len(source_rows[source])+1,'time_ns':len(source_rows[source])+1,'monotonic_ns':len(source_rows[source])+1,'event':event,'phase_id':None,**fields}
        source_rows[source].append(row)
        return row
    emit('direct','stream_start',source_identity='direct',ports=[8080,8081,8082,8083],initial_mode='normal')
    emit('adapter','stream_start',source_identity='adapter',ports=[8080],initial_mode='normal')
    host_commands=[]
    def host_command(op, phase_id, mode=None, expected_direct=None):
        ordinal=len(host_commands)+1
        args={'ordinal':ordinal,'op':op,'phase_id':phase_id}
        if mode is not None: args['mode']=mode
        if expected_direct is not None: args.update(client_stopped=True,helper_active=True,helper_id='syn-helper',expected_direct=expected_direct)
        if op=='begin_phase':
            phase_class='control' if mode=='control' else 'negative'
            for source in ('direct','adapter'):
                emit(source,'phase_begin',ordinal=ordinal,phase_id=phase_id,phase_class=phase_class,mode=mode,active_connections=0,active_requests=0)
        elif op=='barrier':
            for source in ('direct','adapter'):
                emit(source,'phase_barrier',ordinal=ordinal,phase_id=phase_id,phase_class='control' if phase_id=='positive-controls' else 'negative',active_connections=0,active_requests=0)
        elif op=='seal':
            for source in ('direct','adapter'):
                emit(source,'stream_end',ordinal=ordinal,final_seq=len(source_rows[source])+1,event_count=len(source_rows[source])+1,active_connections=0,active_requests=0)
        args['source_cursors']={source:source_rows[source][-1]['seq'] for source in ('direct','adapter')}
        host_commands.append(args)
    expectations=[{'local_port':8080,'method':'GET','path':'/v1/models','status':200,'peer_ip':'172.20.0.3','count':1},
      {'local_port':8081,'method':'GET','path':'/v1/models','status':200,'peer_ip':'172.20.0.3','count':1},
      {'local_port':8082,'method':'GET','path':'http://mock-provider:8081/v1/models','status':200,'peer_ip':'172.20.0.3','count':1},
      {'local_port':8082,'method':'CONNECT','path':'mock-provider:443','status':200,'peer_ip':'172.20.0.3','count':1}]
    host_command('begin_phase','positive-controls','control',expectations)
    for number,item in enumerate(expectations,1):
        conn=f'direct-c{number}'; req=f'{conn}-r1'; peer=f"{item['peer_ip']}:{41000+number}"; local=f"172.20.0.2:{item['local_port']}"
        emit('direct','tcp_accept',connection_id=conn,peer=peer,local=local,local_port=item['local_port'],phase_id='positive-controls')
        emit('direct','request_start',connection_id=conn,request_id=req,request_ordinal=1,phase_id='positive-controls',method=item['method'],path=item['path'],untrusted_case_header='forged-control' if number==1 else None)
        emit('direct','request_body',request_id=req,phase_id='positive-controls',body_length=0,body_sha256='e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',body_complete=True,observations={})
        emit('direct','response_sent',request_id=req,status=item['status'])
        emit('direct','connection_close',connection_id=conn,reason='helper-complete',phase_id='positive-controls')
    host_command('barrier','positive-controls')
    host_command('begin_phase','normal_route','normal')
    for number,purpose in enumerate(('title','task'),1):
        conn=f'adapter-c{number}'; req=f'{conn}-r1'
        emit('adapter','tcp_accept',connection_id=conn,peer=f'172.20.0.4:{42000+number}',local='172.20.0.2:8080',local_port=8080,phase_id='normal_route')
        emit('adapter','request_start',connection_id=conn,request_id=req,request_ordinal=1,phase_id='normal_route',method='POST',path='/v1/chat/completions',untrusted_case_header='normal_route')
        emit('adapter','request_body',request_id=req,phase_id='normal_route',body_length=10,body_sha256='1'*64,body_complete=True,observations={'purpose':purpose,'model':'mock-model','stream':True,'session_id':'syn-session'})
        emit('adapter','response_sent',request_id=req,status=200)
        emit('adapter','connection_close',connection_id=conn,reason='response-complete',phase_id='normal_route')
    host_command('barrier','normal_route')
    host_command('seal',None)
    for source in ('direct','adapter'):
        (root/'sources'/source).mkdir(parents=True,exist_ok=True)
        (root/'sources'/source/'events.jsonl').write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in source_rows[source]))
    command_file=root/'phase-commands.jsonl'
    command_file.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in host_commands))
    host_rows=[]
    for case_name in sorted(mod.REQUIRED_CASES):
        case=cases[case_name]
        host_rows.append({'kind':'case','case_id':case_name,'argv':case['command'],'exit_code':case['exit_code'],'timed_out':case.get('timed_out',False),'stdout_file':case['stdout_file'],'stderr_file':case['stderr_file'],'elapsed_s':0.01})
    for case_name in sorted(mod.CONFIG_CASES):
        base=cases[case_name].get('intended_base_url','http://mock-hostile-provider:8081/v1')
        config_rel=f'configs/{case_name}.json'; trace_rel=f'logs/{case_name}.trace.txt'
        (root/'configs').mkdir(exist_ok=True)
        config_bytes=json.dumps({'provider':'mock','baseURL':base}).encode()
        (root/config_rel).write_bytes(config_bytes)
        (root/trace_rel).write_text(f'loaded config {config_rel}; selected baseURL={base}\n')
        cases[case_name].update(intended_base_url=base,selected_base_url=base,effective_config_trace=f'synthetic load providerID=mock baseURL={base}')
        host_rows.append({'kind':'config','case_id':case_name,'config_file':config_rel,'config_sha256':hashlib.sha256(config_bytes).hexdigest(),'trace_file':trace_rel,'selected_base_url':base,'load_observed':True})
    host_command_file=root/'host-commands.jsonl'
    host_command_file.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in host_rows))
    cleanup_rows=[]
    for resource in cleanup_resources:
        cleanup_rows.append({'kind':'remove','resource':resource,'argv':['docker','rm','-f',resource],'exit_code':0})
        cleanup_rows.append({'kind':'not_found_inspect','resource':resource,'argv':['docker','inspect',resource],'exit_code':1,'stderr':f'Error: {resource} not found'})
    cleanup_file=root/'cleanup.jsonl'
    cleanup_file.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in cleanup_rows))
    raw_paths={'direct':'sources/direct/events.jsonl','adapter':'sources/adapter/events.jsonl'}
    hash_paths=list(raw_paths.values())+['phase-commands.jsonl','host-commands.jsonl','snapshots/owned-inspect.json','cleanup.jsonl']
    raw_hashes={relative:hashlib.sha256((root/relative).read_bytes()).hexdigest() for relative in hash_paths}
    (root/'manifest.json').write_text(json.dumps({'schema':'craft-e0-v2','run_id':rid,'evidence_type':'synthetic','verdict':'PASS','image':{'id':IMAGE,'platform':'linux/arm64','version':'1.18.4'},'names':names,'cases':cases,'cleanup':{'registered_resources':cleanup_resources},'ledgers':{r:f'ledgers/{r}.jsonl' for r in roles},'source_streams':raw_paths,'phase_commands_file':'phase-commands.jsonl','host_commands_file':'host-commands.jsonl','inspect_evidence_file':'snapshots/owned-inspect.json','cleanup_evidence_file':'cleanup.jsonl','raw_evidence_sha256':raw_hashes}))
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

def refresh_raw_hashes(root):
    m=json.loads((root/'manifest.json').read_text())
    paths=list(m['source_streams'].values())+[m['phase_commands_file'],m['host_commands_file'],m['inspect_evidence_file'],m['cleanup_evidence_file']]
    m['raw_evidence_sha256']={relative:hashlib.sha256((root/relative).read_bytes()).hexdigest() for relative in paths}
    (root/'manifest.json').write_text(json.dumps(m))

def reject_mutation(label, mutate, expected_fragment=None, refresh_hashes=True):
    root = clone_artifact(full_fixture_root, label)
    mutate(root)
    if refresh_hashes: refresh_raw_hashes(root)
    p = subprocess.run([sys.executable, str(SCRIPT), str(root)], capture_output=True, text=True)
    if p.returncode == 0:
        raise SystemExit(f'{label}: forged fixture unexpectedly passed\n{p.stdout}\n{p.stderr}')
    if expected_fragment and expected_fragment not in p.stdout:
        raise SystemExit(f'{label}: rejected for the wrong reason; expected {expected_fragment!r}\n{p.stdout}\n{p.stderr}')
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
    m['cases']['hostile_project_url'].pop('effective_config_trace', None); m['cases']['hostile_project_url'].pop('effective_config_trace_file', None)
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

# Reproduced Fix2 false accepts must fail for the raw-evidence reason.
def manifest_blocked(root):
    m=json.loads((root/'manifest.json').read_text()); m['verdict']='BLOCKED'; (root/'manifest.json').write_text(json.dumps(m))
def normal_output_missing(root):
    (root/'logs/normal_route.stdout.txt').write_text('fatal: no model completion\n')
def writable_state_mount(root):
    m=json.loads((root/'manifest.json').read_text())
    n=m['names']['client']
    p=root/'snapshots'/f"pre-{n}-inspect.json"
    p.write_text(json.dumps([{'Mounts':[{'Type':'bind','Source':'/tmp/state','Destination':'/state','RW':True}],'HostConfig':{'Privileged':False,'CapAdd':None,'NetworkMode':m['names']['private']}}]))
def normal_timeout(root):
    m=json.loads((root/'manifest.json').read_text()); m['cases']['normal_route'].update(exit_code=None,timed_out=True); (root/'manifest.json').write_text(json.dumps(m))

def add_unexpected_direct(root, shape, header=None):
    m=json.loads((root/'manifest.json').read_text())
    path=root/m['source_streams']['direct']
    rows=[json.loads(line) for line in path.read_text().splitlines()]
    barrier=next(i for i,row in enumerate(rows) if row.get('event')=='phase_barrier' and row.get('phase_id')=='positive-controls')
    conn='direct-c5'; req=conn+'-r1'; phase='positive-controls'
    def event(kind, **fields):
        return {'run_id':m['run_id'],'source':'direct','boot_id':'direct-boot','seq':0,'time_ns':1,'monotonic_ns':1,'event':kind,'phase_id':phase,**fields}
    extra=[event('tcp_accept',connection_id=conn,peer='172.20.0.99:49999',local='172.20.0.2:8081',local_port=8081)]
    if shape=='tcp-only':
        extra.append(event('connection_close',connection_id=conn,reason='close',phase_id=phase))
    elif shape=='malformed':
        extra.extend([event('parse_error',connection_id=conn,detail='400 malformed'),event('connection_close',connection_id=conn,reason='parse-error',phase_id=phase)])
    else:
        method='CONNECT' if shape=='connect' else 'GET'
        request_path='mock-provider:443' if method=='CONNECT' else '/bypass'
        extra.extend([event('request_start',connection_id=conn,request_id=req,request_ordinal=1,phase_id=phase,method=method,path=request_path,untrusted_case_header=header),event('request_body',request_id=req,phase_id=phase,body_length=0,body_sha256='e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',body_complete=True,observations={}),event('response_sent',request_id=req,status=200),event('connection_close',connection_id=conn,reason='close',phase_id=phase)])
    rows[barrier:barrier]=extra
    for i,row in enumerate(rows,1): row['seq']=i; row['time_ns']=i; row['monotonic_ns']=i
    rows[-1]['final_seq']=len(rows); rows[-1]['event_count']=len(rows)
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
    commands=root/m['phase_commands_file']; cmds=[json.loads(line) for line in commands.read_text().splitlines()]
    by_ord={row.get('ordinal'):row for row in rows if row.get('event') in {'phase_begin','phase_barrier','stream_end'}}
    for command in cmds: command['source_cursors']['direct']=by_ord[command['ordinal']]['seq']
    commands.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in cmds))

def untagged_get(root): add_unexpected_direct(root,'get',None)
def forged_control_get(root): add_unexpected_direct(root,'get','control:forged:provider:dns')
def unexpected_connect(root): add_unexpected_direct(root,'connect','control:forged:proxy:connect')
def unexpected_malformed(root): add_unexpected_direct(root,'malformed')
def tcp_only_accept(root): add_unexpected_direct(root,'tcp-only')

reject_mutation('fix2-manifest-blocked-claim', manifest_blocked, 'manifest verdict is not PASS')
reject_mutation('fix2-missing-normal-process-output', normal_output_missing, 'normal route lacks actual successful process output')
reject_mutation('fix2-raw-writable-state-mount', writable_state_mount, 'raw pre client inspect exposes recorder/control fixture mounts')
reject_mutation('fix2-normal-route-timeout', normal_timeout, 'case normal_route: timed-out or missing process result')
reject_mutation('fix2-untagged-direct-get', untagged_get, 'direct control phase positive-controls observed events differ')
reject_mutation('fix2-forged-control-direct-get', forged_control_get, 'direct control phase positive-controls observed events differ')
reject_mutation('fix2-unexpected-direct-connect', unexpected_connect, 'direct control phase positive-controls observed events differ')
reject_mutation('fix2-malformed-direct-accept', unexpected_malformed, 'direct')
reject_mutation('fix2-tcp-only-direct-accept', tcp_only_accept, 'direct control phase positive-controls requires exactly one request')

def missing_raw_normal_request(root):
    m=json.loads((root/'manifest.json').read_text())
    path=root/m['source_streams']['adapter']
    rows=[json.loads(line) for line in path.read_text().splitlines()]
    drop=next(row['connection_id'] for row in rows if row.get('event')=='tcp_accept' and row.get('phase_id')=='normal_route')
    rows=[row for row in rows if row.get('connection_id')!=drop and not (row.get('request_id') or '').startswith(drop+'-')]
    for i,row in enumerate(rows,1): row['seq']=i
    rows[-1]['final_seq']=len(rows); rows[-1]['event_count']=len(rows)
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
    mpath=root/m['phase_commands_file']; cmds=[json.loads(line) for line in mpath.read_text().splitlines()]
    by_ord={row.get('ordinal'):row for row in rows if row.get('event') in {'phase_begin','phase_barrier','stream_end'}}
    for command in cmds: command['source_cursors']['adapter']=by_ord[command['ordinal']]['seq']
    mpath.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in cmds))
reject_mutation('fix2-missing-source-owned-normal-post', missing_raw_normal_request, 'adapter normal phase normal_route must contain exactly one title and one task request')

def source_hash_mismatch(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['source_streams']['adapter']; rows=[json.loads(line) for line in path.read_text().splitlines()]; rows[0]['time_ns']+=1; path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
reject_mutation('fix2-source-hash-mismatch', source_hash_mismatch, 'raw evidence SHA256 mismatch', refresh_hashes=False)

def mutate_stream(root, mutate):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['source_streams']['direct']
    rows=[json.loads(line) for line in path.read_text().splitlines()]
    mutate(rows)
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def shifted_sequence(root): mutate_stream(root, lambda rows: rows[4].update(seq=99))
def changed_boot(root): mutate_stream(root, lambda rows: rows[2].update(boot_id='changed-boot'))
def removed_seal(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['source_streams']['direct']; rows=[json.loads(line) for line in path.read_text().splitlines()]; rows.pop(); path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def stale_phase_cursor(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['phase_commands_file']; rows=[json.loads(line) for line in path.read_text().splitlines()]; rows[1]['source_cursors']['direct']+=1; path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
reject_mutation('fix2-shifted-source-sequence', shifted_sequence, 'source sequence is not contiguous')
reject_mutation('fix2-changed-source-boot', changed_boot, 'source boot identity changed')
reject_mutation('fix2-missing-stream-seal', removed_seal, 'missing its start or final seal')
reject_mutation('fix2-forged-phase-cursor', stale_phase_cursor, 'phase_barrier lacks matching host acknowledgment')

# Fix3 RED: connection-level summaries must not collapse repeated requests.
def recalculate_source(root, source='direct'):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['source_streams'][source]
    rows=[json.loads(line) for line in path.read_text().splitlines()]
    for index,row in enumerate(rows,1):
        row['seq']=index; row['time_ns']=index; row['monotonic_ns']=index
    rows[-1]['final_seq']=len(rows); rows[-1]['event_count']=len(rows)
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
    commands=root/m['phase_commands_file']; command_rows=[json.loads(line) for line in commands.read_text().splitlines()]
    ack={row.get('ordinal'):row for row in rows if row.get('event') in {'phase_begin','phase_barrier','stream_end'}}
    for command in command_rows: command['source_cursors'][source]=ack[command['ordinal']]['seq']
    commands.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in command_rows))

def keepalive_bypass_then_expected(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['source_streams']['direct']
    rows=[json.loads(line) for line in path.read_text().splitlines()]
    barrier=next(i for i,row in enumerate(rows) if row.get('event')=='phase_barrier' and row.get('phase_id')=='positive-controls')
    command_path=root/m['phase_commands_file']; commands=[json.loads(line) for line in command_path.read_text().splitlines()]
    control=next(row for row in commands if row.get('op')=='begin_phase' and row.get('phase_id')=='positive-controls')
    control['expected_direct'].append({'local_port':8081,'method':'GET','path':'/v1/models','status':200,'peer_ip':'172.20.0.3','count':1})
    conn='direct-keepalive'; peer='172.20.0.3:43000'; local='172.20.0.2:8081'
    def event(kind, **fields): return {'run_id':m['run_id'],'source':'direct','boot_id':'direct-boot','seq':0,'time_ns':100,'monotonic_ns':100,'event':kind,'phase_id':'positive-controls',**fields}
    events=[event('tcp_accept',connection_id=conn,peer=peer,local=local,local_port=8081)]
    for ordinal,(path_value,method) in enumerate((('/unapproved-bypass','GET'),('/v1/models','GET')),1):
        request_id=f'{conn}-r{ordinal}'
        events.extend([event('request_start',connection_id=conn,request_id=request_id,request_ordinal=ordinal,method=method,path=path_value,untrusted_case_header='control:forged'),
          event('request_body',request_id=request_id,body_length=0,body_sha256='e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',body_complete=True,observations={}),
          event('response_sent',request_id=request_id,status=200)])
    events.append(event('connection_close',connection_id=conn,reason='keepalive-complete'))
    rows[barrier:barrier]=events
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
    command_path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in commands))
    recalculate_source(root)

def wrong_request_id(root):
    m=json.loads((root/'manifest.json').read_text()); p=root/m['source_streams']['direct']; rows=[json.loads(x) for x in p.read_text().splitlines()]
    req=next(r for r in rows if r.get('event')=='request_start'); old=req['request_id']; req['request_id']=req['connection_id']+'-r9'
    for row in rows:
        if row.get('request_id')==old: row['request_id']=req['request_id']
    p.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def wrong_request_ordinal(root):
    mutate_stream(root,lambda rows: next(r for r in rows if r.get('event')=='request_start').update(request_ordinal=9))
def missing_terminal(root):
    m=json.loads((root/'manifest.json').read_text()); p=root/m['source_streams']['direct']; rows=[json.loads(x) for x in p.read_text().splitlines()]
    request=next(r['request_id'] for r in rows if r.get('event')=='request_start')
    rows=[r for r in rows if not (r.get('event')=='response_sent' and r.get('request_id')==request)]
    p.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows)); recalculate_source(root)
def double_terminal(root):
    m=json.loads((root/'manifest.json').read_text()); p=root/m['source_streams']['direct']; rows=[json.loads(x) for x in p.read_text().splitlines()]
    response=next(i for i,r in enumerate(rows) if r.get('event')=='response_sent'); req=rows[response]['request_id']
    base=rows[response]
    rows.insert(response+1,{'run_id':m['run_id'],'source':'direct','boot_id':'direct-boot','seq':0,'time_ns':base['time_ns']+1,'monotonic_ns':base['monotonic_ns']+1,'event':'request_error','phase_id':base['phase_id'],'request_id':req,'kind':'late-error','detail':'contradictory terminal'})
    p.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows)); recalculate_source(root)
reject_mutation('fix3-keepalive-first-bypass-then-expected-control',keepalive_bypass_then_expected,'direct control phase positive-controls observed events differ')
reject_mutation('fix3-wrong-generated-request-id',wrong_request_id,'request ID/ordinal')
reject_mutation('fix3-wrong-generated-request-ordinal',wrong_request_ordinal,'request ID/ordinal')
reject_mutation('fix3-missing-request-terminal',missing_terminal,'exactly one terminal')
reject_mutation('fix3-double-terminal-response-then-error',double_terminal,'more than one terminal')

def alter_host_command(root):
    m=json.loads((root/'manifest.json').read_text()); p=root/m['host_commands_file']
    rows=[json.loads(line) for line in p.read_text().splitlines()]
    next(row for row in rows if row.get('kind')=='case' and row.get('case_id')=='normal_route')['exit_code']=0
    next(row for row in rows if row.get('kind')=='case' and row.get('case_id')=='normal_route')['timed_out']=True
    p.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def alter_config_bytes(root):
    (root/'configs/hostile_project_url.json').write_text('{"baseURL":"http://other.invalid"}')
def remove_inspect_participant(root):
    m=json.loads((root/'manifest.json').read_text()); p=root/m['inspect_evidence_file']; data=json.loads(p.read_text())
    del data['pre']['helper']; p.write_text(json.dumps(data))
def false_cleanup(root):
    m=json.loads((root/'manifest.json').read_text()); p=root/m['cleanup_evidence_file']
    rows=[json.loads(line) for line in p.read_text().splitlines()]
    next(row for row in rows if row['kind']=='remove')['exit_code']=1
    p.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
reject_mutation('fix2-raw-command-timeout-disagrees-with-summary',alter_host_command,'raw host command for normal_route')
reject_mutation('fix2-config-bytes-differ-from-retained-hash',alter_config_bytes,'config bytes SHA256 mismatch')
reject_mutation('fix2-inspect-missing-helper',remove_inspect_participant,'raw pre inspect does not cover')
reject_mutation('fix2-cleanup-remove-failed',false_cleanup,'raw cleanup removal command failed')

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
