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
    names={'client':'syn-client','adapter':'syn-adapter','direct':'syn-direct','helper':'syn-helper','config':'syn-config','data':'syn-data','private':'syn-private','external':'syn-external'}
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
    for name in mod.NEGATIVE_CASES:
        cases[name]['command']=mod.fixed_invocation(name,names,'172.20.0.2')
        (logs/f'{name}.stdout.txt').write_text('HTTP=000 EXIT=7\n')
    for case in ('hostile_project_url','hostile_global_url','hostile_project_url_post','hostile_project_dns_post','hostile_global_url_post'):
        base='http://mock-hostile-provider:8081/v1' if case in {'hostile_project_dns_post','custom_url_dns_pre'} else 'http://172.20.0.2:8081/v1'
        cases[case].update(ledger_delta=zero.copy(),intended_base_url=base,selected_base_url=base)
    cases['custom_url_dns_pre'].update(intended_base_url='http://mock-hostile-provider:8081/v1',adapter_requests=0,ledger_delta=zero.copy())
    for case in ('redirect_307_pre','redirect_308_pre','redirect_307_post','redirect_308_post'):
        cases[case].update(curl_follow_control_exit=0,curl_follow_provider_requests=1,adapter_original_posts=1,ledger_delta=zero.copy())
    for case in ('adapter_proxy_pre','adapter_proxy_post'):
        cases[case].update(external_delta=0)
    cleanup_resources=['syn-client','syn-adapter','syn-direct','syn-helper','syn-private','syn-external','syn-config','syn-data']
    cases['cleanup'].update(resources_absent=True,cleanup_exit=0,registered_resources=cleanup_resources)
    (snaps/'runtime.json').write_text(json.dumps({'stdout':'1.18.4\n3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e  /usr/local/bin/opencode\n'}))
    (snaps/f"pre-{names['adapter']}-inspect.json").write_text(json.dumps([{'HostConfig':{'Sysctls':{'net.ipv4.ip_forward':'0'}}}]))
    safe_client_inspect = [{'Mounts': [{'Type':'volume','Source':'syn-config','Name':'syn-config','Destination':'/home/craft/.config/opencode'},{'Type':'volume','Source':'syn-data','Name':'syn-data','Destination':'/home/craft/.local/share/opencode'}], 'HostConfig': {'Privileged':False,'CapAdd':None,'NetworkMode':'syn-private'}}]
    (snaps/f"pre-{names['client']}-inspect.json").write_text(json.dumps(safe_client_inspect))
    (snaps/f"post-restart-{names['client']}-inspect.json").write_text(json.dumps(safe_client_inspect))
    synthetic_inspect={'pre':{},'post_restart':{}}
    for stage in ('pre','post_restart'):
        for participant in ('client','adapter','direct','helper'):
            networks={
                'client':{names['private']},
                'adapter':{names['private'],names['external']},
                'direct':{names['external']},
                'helper':{names['external']},
            }[participant]
            container={'Id':f'synthetic-{participant}-id','Image':IMAGE,'Config':{'User':'10001:10001'},'Mounts':[],'HostConfig':{'Privileged':False,'CapAdd':None,'CapDrop':['ALL'],'PortBindings':{'8083/tcp':[{'HostIp':'0.0.0.0','HostPort':'49153'}]} if participant=='direct' else {},'NetworkMode':names['private'] if participant in {'client','adapter'} else names['external']},'NetworkSettings':{'Networks':{network:{} for network in networks}}}
            if participant=='client': container['Mounts']=safe_client_inspect[0]['Mounts']
            if participant=='adapter': container['HostConfig']['Sysctls']={'net.ipv4.ip_forward':'0'}
            if participant=='direct': container['Mounts']=[{'Type':'volume','Source':'/var/lib/docker/volumes/direct-ledger/_data','Name':'direct-ledger','Destination':'/ledger','RW':True}]
            if participant=='adapter': container['Mounts']=[{'Type':'volume','Source':'/var/lib/docker/volumes/adapter-ledger/_data','Name':'adapter-ledger','Destination':'/ledger','RW':True}]
            synthetic_inspect[stage][participant]={'container':container}
        synthetic_inspect[stage]['private_network']={'Name':'syn-private','Internal':True,'Options':{'com.docker.network.bridge.gateway_mode_ipv4':'isolated'},'Containers':{names['client']:{},names['adapter']:{}}}
        synthetic_inspect[stage]['external_network']={'Name':'syn-external','Internal':False,'Containers':{names['direct']:{'IPv4Address':'172.20.0.2/24'},names['adapter']:{'IPv4Address':'172.20.0.4/24'},'syn-helper':{'IPv4Address':'172.20.0.3/24'}}}
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
    host_operations=[]
    for resource,resource_type in [('syn-client','container'),('syn-adapter','container'),('syn-direct','container'),('syn-helper','container'),('syn-private','network'),('syn-external','network'),('syn-config','volume'),('syn-data','volume')]:
        creation_argv={'container':['docker','run','-d','--name',resource],'network':['docker','network','create',resource],'volume':['docker','volume','create',resource]}[resource_type]
        if resource=='syn-private': creation_argv=['docker','network','create','--internal','-o','com.docker.network.bridge.gateway_mode_ipv4=isolated',resource]
        host_operations.append({'kind':'operation','operation':'resource_create','operation_id':f'create:{resource}','resource':resource,'resource_type':resource_type,'argv':creation_argv,'exit_code':0,'timed_out':False})
    phase_classes={}
    def host_command(op, phase_id, mode=None, case_id=None, window=None):
        ordinal=len(host_commands)+1
        args={'ordinal':ordinal,'operation_id':f'phase:{ordinal}','op':op,'phase_id':phase_id}
        if mode is not None: args['mode']=mode
        if op=='begin_phase' and mode=='control':
            args.update(case_id=case_id,window=window,stop_command_id=f'{phase_id}:stop-client',state_inspect_id=f'{phase_id}:inspect-client',helper_start_id=f'{phase_id}:start-helper',helper_stop_id=f'{phase_id}:stop-helper')
            host_operations.extend([
                {'kind':'operation','operation_id':args['stop_command_id'],'operation':'docker_stop','phase_id':phase_id,'position':'before_phase','argv':['docker','stop','--timeout','2',names['client']],'exit_code':0,'timed_out':False,'container_id':'synthetic-client-id'},
                {'kind':'operation','operation_id':args['state_inspect_id'],'operation':'client_state_inspect','phase_id':phase_id,'position':'before_phase','argv':['docker','inspect',names['client']],'exit_code':0,'timed_out':False,'container_id':'synthetic-client-id','state':{'Running':False,'Restarting':False,'Pid':0}},
                {'kind':'operation','operation_id':args['helper_start_id'],'operation':'docker_start','phase_id':phase_id,'position':'after_begin','argv':['docker','start',names['helper']],'exit_code':0,'timed_out':False,'container_id':'synthetic-helper-id','peer_ip':'172.20.0.3'},
                {'kind':'operation','operation_id':args['helper_stop_id'],'operation':'docker_stop','phase_id':phase_id,'position':'after_barrier','argv':['docker','stop','--timeout','2',names['helper']],'exit_code':0,'timed_out':False,'container_id':'synthetic-helper-id'},
            ])
        if op=='begin_phase' and mode=='restart':
            args.update(restart_command_id='restart:client',post_restart_inspect_id='restart:inspect-client')
            host_operations.extend([
                {'kind':'operation','operation_id':args['restart_command_id'],'operation':'container_restart','phase_id':'restart','position':'before_phase','argv':['docker','restart',names['client']],'exit_code':0,'timed_out':False},
                {'kind':'operation','operation_id':args['post_restart_inspect_id'],'operation':'client_post_restart_inspect','phase_id':'restart','position':'after_restart','argv':['docker','inspect',names['client']],'exit_code':0,'container_id':'synthetic-client-id'},
            ])
        if op=='begin_phase':
            phase_class='control' if mode=='control' else ('normal' if mode=='normal' else ('restart' if mode=='restart' else 'negative'))
            phase_classes[phase_id]=phase_class
            for source in ('direct','adapter'):
                emit(source,'phase_begin',ordinal=ordinal,phase_id=phase_id,phase_class=phase_class,mode=mode,active_connections=0,active_requests=0)
                if mode=='control':
                    emit(source,'helper_start',phase_id=phase_id,boundary='helper_start',operation_id=args['helper_start_id'],controller_ordinal=0,active_connections=0,active_requests=0)
        elif op=='barrier':
            for source in ('direct','adapter'):
                emit(source,'phase_barrier',ordinal=ordinal,phase_id=phase_id,phase_class=phase_classes[phase_id],active_connections=0,active_requests=0)
        elif op=='seal':
            for source in ('direct','adapter'):
                emit(source,'stream_end',ordinal=ordinal,final_seq=len(source_rows[source])+1,event_count=len(source_rows[source])+1,active_connections=0,active_requests=0)
        args['source_cursors']={source:source_rows[source][-2]['seq'] if op=='begin_phase' and mode=='control' else source_rows[source][-1]['seq'] for source in ('direct','adapter')}
        host_commands.append(args)

    direct_counter=0
    adapter_counter=0
    def emit_direct_controls(phase_id):
        nonlocal_dummy=None
        global direct_counter
        expectations=[(8080,'GET','/v1/models',200),(8081,'GET','/v1/models',200),(8082,'GET','http://mock-provider:8081/v1/models',200),(8082,'CONNECT','mock-provider:443',200)]
        for port,method,path,status in expectations:
            direct_counter += 1
            conn=f'direct-c{direct_counter}'; req=f'{conn}-r1'
            emit('direct','tcp_accept',connection_id=conn,peer=f'172.20.0.3:{41000+direct_counter}',local=f'172.20.0.2:{port}',local_port=port,phase_id=phase_id)
            emit('direct','request_start',connection_id=conn,request_id=req,request_ordinal=1,phase_id=phase_id,method=method,path=path,untrusted_case_header='forged-control')
            emit('direct','request_body',request_id=req,phase_id=phase_id,body_length=0,body_sha256='e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',body_complete=True,observations={})
            emit('direct','response_sent',request_id=req,status=status)
            emit('direct','connection_close',connection_id=conn,reason='helper-complete',phase_id=phase_id)

    def emit_normal_route(phase_id):
        global adapter_counter
        for number,purpose in enumerate(('title','task'),1):
            adapter_counter += 1
            conn=f'adapter-c{adapter_counter}'; req=f'{conn}-r1'
            emit('adapter','tcp_accept',connection_id=conn,peer=f'172.20.0.4:{42000+adapter_counter}',local='172.20.0.2:8080',local_port=8080,phase_id=phase_id)
            emit('adapter','request_start',connection_id=conn,request_id=req,request_ordinal=1,phase_id=phase_id,method='POST',path='/v1/chat/completions',untrusted_case_header=phase_id)
            emit('adapter','request_body',request_id=req,phase_id=phase_id,body_length=10,body_sha256='1'*64,body_complete=True,observations={'purpose':purpose,'model':'mock-model','stream':True,'session_id':'syn-session'})
            emit('adapter','response_sent',request_id=req,status=200)
            emit('adapter','connection_close',connection_id=conn,reason='response-complete',phase_id=phase_id)

    for phase_id, phase_class, case_id, window in mod.expected_phase_schedule():
        mode={'control':'control','negative':'negative','normal':'normal','restart':'restart'}[phase_class]
        host_command('begin_phase',phase_id,mode,case_id,window)
        if phase_class=='control':
            emit_direct_controls(phase_id)
        elif phase_class=='normal':
            emit_normal_route(phase_id)
        host_command('barrier',phase_id)
    host_command('seal',None)
    for source in ('direct','adapter'):
        (root/'sources'/source).mkdir(parents=True,exist_ok=True)
        (root/'sources'/source/'events.jsonl').write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in source_rows[source]))
    command_file=root/'phase-commands.jsonl'
    command_file.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in host_commands))
    host_rows=[]
    host_rows.extend(host_operations)
    for case_name in sorted(mod.REQUIRED_CASES):
        case=cases[case_name]
        host_rows.append({'kind':'case','case_id':case_name,'argv':case['command'],'exit_code':case['exit_code'],'timed_out':case.get('timed_out',False),'stdout_file':case['stdout_file'],'stderr_file':case['stderr_file'],'elapsed_s':0.01})
    for case_name in sorted(mod.CONFIG_CASES):
        base=cases[case_name].get('intended_base_url','http://mock-hostile-provider:8081/v1')
        config_rel=f'configs/{case_name}.json'; trace_rel=f'logs/{case_name}.trace.jsonl'
        (root/'configs').mkdir(exist_ok=True)
        provider_id, model = ('evil','model') if case_name in {'hostile_global_url','hostile_global_url_post'} else ('mock','mock-model')
        config_bytes=json.dumps({'$schema':'https://opencode.ai/config.json','provider':{provider_id:{'options':{'baseURL':base},'models':{model:{'name':model}}}}},sort_keys=True).encode()
        (root/config_rel).write_bytes(config_bytes)
        pid=7000+len(host_rows); argv_sha=hashlib.sha256(json.dumps(cases[case_name]['command'],sort_keys=True).encode()).hexdigest()
        trace=[{'event':'process_start','case_id':case_name,'pid':pid,'argv_sha256':argv_sha,'config_file':config_rel,'config_sha256':hashlib.sha256(config_bytes).hexdigest()},
          {'event':'provider_selection','case_id':case_name,'pid':pid,'provider_id':provider_id,'model':model,'selected_base_url':base},
          {'event':'network_attempt','case_id':case_name,'pid':pid,'target_url':base+'/chat/completions','outcome':'connection_refused'}]
        (root/trace_rel).write_text(''.join(json.dumps(item,sort_keys=True)+'\n' for item in trace))
        cases[case_name].update(intended_base_url=base,selected_base_url=base)
        host_rows.append({'kind':'config','case_id':case_name,'config_file':config_rel,'config_sha256':hashlib.sha256(config_bytes).hexdigest(),'trace_file':trace_rel})
    host_command_file=root/'host-commands.jsonl'
    host_command_file.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in host_rows))
    cleanup_rows=[]
    resource_types={resource:('network' if resource in {'syn-private','syn-external'} else ('volume' if resource in {'syn-config','syn-data'} else 'container')) for resource in cleanup_resources}
    for resource in cleanup_resources:
        resource_type=resource_types[resource]
        remove_argv={'container':['docker','rm','-f','-v',resource],'network':['docker','network','rm',resource],'volume':['docker','volume','rm',resource]}[resource_type]
        inspect_argv={'container':['docker','inspect',resource],'network':['docker','network','inspect',resource],'volume':['docker','volume','inspect',resource]}[resource_type]
        cleanup_rows.append({'kind':'remove','resource':resource,'resource_type':resource_type,'argv':remove_argv,'exit_code':0})
        cleanup_rows.append({'kind':'not_found_inspect','resource':resource,'resource_type':resource_type,'argv':inspect_argv,'exit_code':1,'stderr':f'Error: no such {resource_type} {resource}'})
    cleanup_file=root/'cleanup.jsonl'
    case_by_id={row['case_id']:row for row in host_rows if row.get('kind')=='case'}
    op_by_id={row['operation_id']:row for row in host_operations}
    timeline=[]
    def append_timeline(row, operation_id, phase_id=None):
        row['controller_ordinal']=len(timeline)+1
        row['operation_id']=operation_id
        if row.get('kind')=='operation' and phase_id is not None:
            row['phase_id']=phase_id
        elif row.get('kind')=='case':
            row['phase_id']=phase_id
        timeline.append(row)
    for operation in host_operations:
        if operation['operation']=='resource_create': append_timeline(operation,operation['operation_id'])
    preflight=('image','topology','baseline_dns','baseline_ip','baseline_adapter_proxy')
    for case_name in preflight:
        append_timeline(case_by_id[case_name],f'case:{case_name}')
    phase_begin_by_id={row.get('phase_id'):row for row in host_commands if row.get('op')=='begin_phase'}
    config_by_case={row['case_id']:row for row in host_rows if row.get('kind')=='config'}
    for command in host_commands:
        phase_id=command.get('phase_id'); op=command.get('op')
        if op=='begin_phase':
            if command.get('mode')=='control':
                append_timeline(op_by_id[command['stop_command_id']],command['stop_command_id'],phase_id)
                append_timeline(op_by_id[command['state_inspect_id']],command['state_inspect_id'],phase_id)
            elif command.get('mode')=='restart':
                append_timeline(op_by_id[command['restart_command_id']],command['restart_command_id'],phase_id)
                append_timeline(op_by_id[command['post_restart_inspect_id']],command['post_restart_inspect_id'],phase_id)
            append_timeline(command,command['operation_id'],phase_id)
            if command.get('mode')=='control': append_timeline(op_by_id[command['helper_start_id']],command['helper_start_id'],phase_id)
            if command.get('mode')=='restart': append_timeline(case_by_id['restart'],'case:restart',phase_id)
            if command.get('mode') in {'negative','normal'} and phase_id in case_by_id:
                append_timeline(case_by_id[phase_id],f'case:{phase_id}',phase_id)
                if phase_id in config_by_case:
                    append_timeline(config_by_case[phase_id],f'config:{phase_id}',phase_id)
        elif op=='barrier':
            append_timeline(command,command['operation_id'],phase_id)
            phase_begin=phase_begin_by_id.get(phase_id)
            if phase_begin and phase_begin.get('mode')=='control':
                stop_id=phase_begin['helper_stop_id']; append_timeline(op_by_id[stop_id],stop_id,phase_id)
        else:
            for case_name in ('fault_503','fault_disconnect'):
                append_timeline(case_by_id[case_name],f'case:{case_name}')
            append_timeline(command,command['operation_id'],phase_id)
    for index,row in enumerate(cleanup_rows):
        action='remove' if row['kind']=='remove' else 'not-found'
        append_timeline(row,f'cleanup:{action}:{row["resource"]}')
    append_timeline(case_by_id['cleanup'],'case:cleanup')
    command_by_local_ordinal={row['ordinal']:row for row in host_commands}
    for rows in source_rows.values():
        for row in rows:
            if row.get('event') in {'phase_begin','phase_barrier','stream_end'}:
                command=command_by_local_ordinal[row['ordinal']]
                row['controller_ordinal']=command['controller_ordinal']; row['operation_id']=command['operation_id']
            elif row.get('event')=='helper_start':
                operation=op_by_id[row['operation_id']]
                row['controller_ordinal']=operation['controller_ordinal']
    for source in ('direct','adapter'):
        (root/'sources'/source/'events.jsonl').write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in source_rows[source]))
    command_file.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in host_commands))
    host_command_file.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in host_rows))
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
    (root/'logs/hostile_project_url.trace.jsonl').write_text('')
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
reject_mutation('fix2-untagged-direct-get', untagged_get, 'direct control phase positive-controls differs from verifier-owned exact events/helper peer')
reject_mutation('fix2-forged-control-direct-get', forged_control_get, 'direct control phase positive-controls differs from verifier-owned exact events/helper peer')
reject_mutation('fix2-unexpected-direct-connect', unexpected_connect, 'direct control phase positive-controls differs from verifier-owned exact events/helper peer')
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
    conn=f"direct-c{sum(1 for row in rows if row.get('event')=='tcp_accept')+1}"; peer='172.20.0.3:43000'; local='172.20.0.2:8081'
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
reject_mutation('fix3-keepalive-first-bypass-then-expected-control',keepalive_bypass_then_expected,'exactly one request and close per connection')
reject_mutation('fix3-wrong-generated-request-id',wrong_request_id,'request ID/ordinal')
reject_mutation('fix3-wrong-generated-request-ordinal',wrong_request_ordinal,'request ID/ordinal')
reject_mutation('fix3-missing-request-terminal',missing_terminal,'exactly one terminal')
reject_mutation('fix3-double-terminal-response-then-error',double_terminal,'more than one terminal')

# Fix4 RED and regression: no successful control request can whitelist later
# malformed, partial, or out-of-window direct-sink error observations.
def insert_direct_error(root, connection_id, phase_id, position='before-close', partial=False):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['source_streams']['direct']
    rows=[json.loads(line) for line in path.read_text().splitlines()]
    close=next(i for i,row in enumerate(rows) if row.get('event')=='connection_close' and row.get('connection_id')==connection_id)
    if position=='after-close': close += 1
    base={'run_id':m['run_id'],'source':'direct','boot_id':'direct-boot','seq':0,'time_ns':1,'monotonic_ns':1,'phase_id':phase_id}
    if partial:
        request_id=f'{connection_id}-r2'
        events=[dict(base,event='request_start',connection_id=connection_id,request_id=request_id,request_ordinal=2,method='POST',path='/partial',untrusted_case_header=None),
          dict(base,event='request_body',request_id=request_id,body_length=3,body_sha256=hashlib.sha256(b'abc').hexdigest(),body_complete=False,observations={}),
          dict(base,event='request_error',request_id=request_id,kind='incomplete_body',detail='partial request')]
    else:
        events=[dict(base,event='parse_error',connection_id=connection_id,detail='malformed second request')]
    rows[close:close]=events
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
    recalculate_source(root)
def expected_get_then_same_connection_parse_error(root): insert_direct_error(root,'direct-c1','positive-controls')
def expected_connect_then_same_connection_parse_error(root): insert_direct_error(root,'direct-c4','positive-controls')
def expected_get_then_partial_second_request(root): insert_direct_error(root,'direct-c1','positive-controls',partial=True)
def expected_connect_then_partial_second_request(root): insert_direct_error(root,'direct-c4','positive-controls',partial=True)
def parse_error_outside_control(root): insert_direct_error(root,'direct-c1',None,position='after-close')
def parse_error_in_negative_window(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['source_streams']['direct']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    begin=next(i for i,row in enumerate(rows) if row.get('event')=='phase_begin' and row.get('phase_id')=='normal_route')
    base={'run_id':m['run_id'],'source':'direct','boot_id':'direct-boot','seq':0,'time_ns':1,'monotonic_ns':1,'phase_id':'normal_route'}
    rows.insert(begin+1,dict(base,event='parse_error',connection_id='direct-c1',detail='error in negative window'))
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows)); recalculate_source(root)
reject_mutation('fix4-expected-get-then-same-connection-parse-error',expected_get_then_same_connection_parse_error,'direct sink parse_error is unconditionally fail-closed')
reject_mutation('fix4-expected-connect-then-same-connection-parse-error',expected_connect_then_same_connection_parse_error,'direct sink parse_error is unconditionally fail-closed')
reject_mutation('fix4-connect-then-partial-http-request',expected_get_then_partial_second_request,'direct sink request_error is unconditionally fail-closed')
reject_mutation('fix4-connect-then-partial-second-request',expected_connect_then_partial_second_request,'direct sink request_error is unconditionally fail-closed')
reject_mutation('fix4-parse-error-in-negative-window',parse_error_in_negative_window,'direct sink parse_error is unconditionally fail-closed')
reject_mutation('fix4-parse-error-outside-control-window',parse_error_outside_control,'direct sink parse_error is unconditionally fail-closed')

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
def cleanup_missing_created_resource(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['host_commands_file']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    rows=[row for row in rows if not (row.get('operation')=='resource_create' and row.get('resource')=='syn-helper')]
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def cleanup_wrong_not_found_kind(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['cleanup_evidence_file']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    next(row for row in rows if row['kind']=='not_found_inspect')['stderr']='Error: permission denied'
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def aggregate_client_mount_escape(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['inspect_evidence_file']; data=json.loads(path.read_text())
    data['pre']['client']['container']['Mounts'].append({'Type':'bind','Source':'/host','Destination':'/state/ledger','RW':True})
    path.write_text(json.dumps(data))
reject_mutation('fix2-raw-command-timeout-disagrees-with-summary',alter_host_command,'raw host command for normal_route')
reject_mutation('fix2-config-bytes-differ-from-retained-hash',alter_config_bytes,'config bytes SHA256 mismatch')
reject_mutation('fix2-inspect-missing-helper',remove_inspect_participant,'raw pre inspect does not cover')
reject_mutation('fix2-cleanup-remove-failed',false_cleanup,'raw cleanup removal command failed')
reject_mutation('fix3-cleanup-inventory-must-come-from-creation-records',cleanup_missing_created_resource,'raw cleanup inventory must be derived')
reject_mutation('fix3-cleanup-permission-error-is-not-not-found',cleanup_wrong_not_found_kind,'raw cleanup not-found inspection did not prove')
reject_mutation('fix3-aggregate-inspect-mount-escape',aggregate_client_mount_escape,'raw pre client mount set is not the fixed XDG config/data volume pair')

# Fix3 Task2: the phase set is verifier-owned and paired around every fixed
# negative; the client/helper assertions come from raw operation records.
def drop_required_pre_control(root):
    m=json.loads((root/'manifest.json').read_text()); p=root/m['phase_commands_file']
    rows=[json.loads(line) for line in p.read_text().splitlines()]
    rows=[row for row in rows if not (row.get('op')=='begin_phase' and row.get('case_id')=='direct_ip_pre' and row.get('window')=='before')]
    p.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def forge_client_stopped(root):
    m=json.loads((root/'manifest.json').read_text()); p=root/m['host_commands_file']
    rows=[json.loads(line) for line in p.read_text().splitlines()]
    operation=next(row for row in rows if row.get('kind')=='operation' and row.get('operation')=='client_state_inspect')
    operation['state']['Running']=True
    p.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def forge_helper_identity(root):
    m=json.loads((root/'manifest.json').read_text()); p=root/m['host_commands_file']
    rows=[json.loads(line) for line in p.read_text().splitlines()]
    operation=next(row for row in rows if row.get('kind')=='operation' and row.get('operation')=='docker_start')
    operation['container_id']='other-helper-id'
    p.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
reject_mutation('fix3-missing-required-pre-control',drop_required_pre_control,'host phase sequence differs from verifier-owned paired control schedule')
reject_mutation('fix3-forged-client-stopped-summary',forge_client_stopped,'did not prove the inspected client stopped')
reject_mutation('fix3-forged-helper-identity',forge_helper_identity,'started a different helper identity')

def change_config_selected_url(root):
    path=root/'logs/hostile_project_url.trace.jsonl'; rows=[json.loads(line) for line in path.read_text().splitlines()]
    next(row for row in rows if row['event']=='provider_selection')['selected_base_url']='http://different.invalid/v1'
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def change_config_attempt_target(root):
    path=root/'logs/hostile_project_url.trace.jsonl'; rows=[json.loads(line) for line in path.read_text().splitlines()]
    next(row for row in rows if row['event']=='network_attempt').update(target_url='http://different.invalid/v1/chat/completions',outcome='connection_refused')
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def change_direct_target_and_summary(root):
    m=json.loads((root/'manifest.json').read_text()); case='direct_ip_pre'
    argv=m['cases'][case]['command']; index=argv.index('http://172.20.0.2:8080/direct'); argv[index]='http://172.20.0.99:8080/direct'
    path=root/m['host_commands_file']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    next(row for row in rows if row.get('kind')=='case' and row.get('case_id')==case)['argv']=argv
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows)); (root/'manifest.json').write_text(json.dumps(m))
def change_proxy_environment_and_summary(root):
    m=json.loads((root/'manifest.json').read_text()); case='proxy_ip_pre'
    argv=m['cases'][case]['command']; index=argv.index('HTTPS_PROXY=http://172.20.0.2:8082'); argv[index]='HTTPS_PROXY=http://elsewhere.invalid:8082'
    path=root/m['host_commands_file']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    next(row for row in rows if row.get('kind')=='case' and row.get('case_id')==case)['argv']=argv
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows)); (root/'manifest.json').write_text(json.dumps(m))
def change_stopped_client_raw_state(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['host_commands_file']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    next(row for row in rows if row.get('operation')=='client_state_inspect')['state']['Pid']=77
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def change_helper_peer(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['source_streams']['direct']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    next(row for row in rows if row.get('event')=='tcp_accept' and row.get('phase_id')=='positive-controls')['peer']='172.20.0.99:41001'
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows)); recalculate_source(root)
reject_mutation('fix3-config-trace-selected-url-is-not-summary-authority',change_config_selected_url,'provider-selection trace does not prove')
reject_mutation('fix3-config-trace-attempt-target-mismatch',change_config_attempt_target,'network-attempt trace does not prove')
reject_mutation('fix3-raw-command-target-changed-with-summary',change_direct_target_and_summary,'does not equal the exact verifier-owned curl argv')
reject_mutation('fix3-raw-command-proxy-environment-changed-with-summary',change_proxy_environment_and_summary,'does not equal the exact verifier-owned curl argv')
reject_mutation('fix3-raw-inspect-client-state-contradiction',change_stopped_client_raw_state,'did not prove the inspected client stopped')
reject_mutation('fix3-control-helper-peer-does-not-match-inspect',change_helper_peer,'differs from verifier-owned exact events/helper peer')

# Fix5 RED: these are self-consistent forgeries: all existing file hashes are
# refreshed and the public summary is changed along with its raw counterpart.
def reorder_helper_stop_before_control(root):
    m=json.loads((root/'manifest.json').read_text())
    phase=json.loads((root/m['phase_commands_file']).read_text().splitlines()[0])
    commands=[json.loads(line) for line in (root/m['phase_commands_file']).read_text().splitlines()]
    control=next(row for row in commands if row.get('op')=='begin_phase' and row.get('case_id')=='direct_ip_pre' and row.get('window')=='before')
    operation_id=control['helper_stop_id']; path=root/m['host_commands_file']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    operation=next(row for row in rows if row.get('operation_id')==operation_id)
    if 'controller_ordinal' in operation:
        start=next(row for row in rows if row.get('operation_id')==control['helper_start_id'])
        operation['controller_ordinal'],start['controller_ordinal']=start['controller_ordinal'],operation['controller_ordinal']
    else:
        rows.remove(operation); rows.insert(0,operation)
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def cleanup_rows_before_seal(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['cleanup_evidence_file']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    if rows and 'controller_ordinal' in rows[0]:
        phase=root/m['phase_commands_file']; commands=[json.loads(line) for line in phase.read_text().splitlines()]
        seal=next(row for row in commands if row.get('op')=='seal'); removal=next(row for row in rows if row.get('kind')=='remove')
        seal['controller_ordinal'],removal['controller_ordinal']=removal['controller_ordinal'],seal['controller_ordinal']
        phase.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in commands))
    else:
        rows.reverse()
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def shell_executes_wrong_target(root):
    m=json.loads((root/'manifest.json').read_text()); case='direct_ip_pre'; wrong="curl -sS --connect-timeout 1 --max-time 3 --noproxy '*' http://different.invalid:8080/direct; echo 'http://172.20.0.2:8080/direct'"
    m['cases'][case]['command']=['docker','exec',m['names']['client'],'sh','-c',wrong]
    path=root/m['host_commands_file']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    next(row for row in rows if row.get('kind')=='case' and row.get('case_id')==case)['argv']=m['cases'][case]['command']
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows)); (root/'manifest.json').write_text(json.dumps(m))
def self_consistent_wrong_hostile_origin(root):
    m=json.loads((root/'manifest.json').read_text()); case='hostile_project_url'; wrong='http://wrong-origin.invalid/v1'
    cfg=root/'configs/hostile_project_url.json'; doc=json.loads(cfg.read_text()); doc['provider']['mock']['options']['baseURL']=wrong; cfg.write_text(json.dumps(doc,sort_keys=True))
    trace=root/'logs/hostile_project_url.trace.jsonl'; rows=[json.loads(line) for line in trace.read_text().splitlines()]
    actual_hash=hashlib.sha256(cfg.read_bytes()).hexdigest()
    next(row for row in rows if row['event']=='process_start')['config_sha256']=actual_hash
    selection=next(row for row in rows if row['event']=='provider_selection'); selection['selected_base_url']=wrong
    next(row for row in rows if row['event']=='network_attempt')['target_url']=wrong+'/chat/completions'
    trace.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
    host=root/m['host_commands_file']; rows=[json.loads(line) for line in host.read_text().splitlines()]
    config=next(row for row in rows if row.get('kind')=='config' and row.get('case_id')==case); config['config_sha256']=actual_hash
    host.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
    m['cases'][case]['intended_base_url']=wrong; m['cases'][case]['selected_base_url']=wrong
    (root/'manifest.json').write_text(json.dumps(m))
def unsafe_adapter_inspect(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['inspect_evidence_file']; data=json.loads(path.read_text())
    data['pre']['adapter']['container']['Mounts']=[{'Type':'bind','Source':'/host/ledger','Destination':'/ledger','RW':True}]
    data['pre']['adapter']['container']['HostConfig']['Privileged']=True
    path.write_text(json.dumps(data))
def wrong_hostile_provider_self_consistent(root):
    m=json.loads((root/'manifest.json').read_text()); case='hostile_project_url'
    config_path=root/'configs/hostile_project_url.json'; document=json.loads(config_path.read_text())
    document['provider']['rogue']=document['provider'].pop('mock'); config_path.write_text(json.dumps(document,sort_keys=True))
    actual_hash=hashlib.sha256(config_path.read_bytes()).hexdigest()
    trace_path=root/'logs/hostile_project_url.trace.jsonl'; rows=[json.loads(line) for line in trace_path.read_text().splitlines()]
    next(row for row in rows if row['event']=='process_start')['config_sha256']=actual_hash
    next(row for row in rows if row['event']=='provider_selection')['provider_id']='rogue'
    trace_path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
    host=root/m['host_commands_file']; rows=[json.loads(line) for line in host.read_text().splitlines()]
    next(row for row in rows if row.get('kind')=='config' and row.get('case_id')==case)['config_sha256']=actual_hash
    host.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def adapter_shares_direct_evidence(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['inspect_evidence_file']; data=json.loads(path.read_text())
    direct=data['pre']['direct']['container']['Mounts'][0]
    adapter=data['pre']['adapter']['container']['Mounts'][0]
    adapter.update(Source=direct['Source'],Name=direct['Name'])
    path.write_text(json.dumps(data))
def helper_attached_to_extra_network(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['inspect_evidence_file']; data=json.loads(path.read_text())
    data['pre']['helper']['container']['NetworkSettings']['Networks']['untrusted-extra']={}
    path.write_text(json.dumps(data))
def missing_controller_ordinal(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['host_commands_file']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    next(row for row in rows if row.get('kind')=='config' and row.get('case_id')=='hostile_project_url').pop('controller_ordinal')
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def duplicate_controller_ordinal(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['host_commands_file']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    cases=[row for row in rows if row.get('kind')=='case' and row.get('controller_ordinal')]
    cases[1]['controller_ordinal']=cases[0]['controller_ordinal']
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def cross_phase_operation_reference(root):
    m=json.loads((root/'manifest.json').read_text()); phase=root/m['phase_commands_file']; commands=[json.loads(line) for line in phase.read_text().splitlines()]
    control=next(row for row in commands if row.get('op')=='begin_phase' and row.get('mode')=='control')
    host=root/m['host_commands_file']; rows=[json.loads(line) for line in host.read_text().splitlines()]
    next(row for row in rows if row.get('operation_id')==control['helper_stop_id'])['phase_id']='other-phase'
    host.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def live_provenance_unavailable_blocks(root):
    path=root/'manifest.json'; manifest=json.loads(path.read_text()); manifest['evidence_type']='measured'; path.write_text(json.dumps(manifest))
def wrong_attempt_endpoint_self_consistent(root):
    path=root/'logs/hostile_project_url.trace.jsonl'; rows=[json.loads(line) for line in path.read_text().splitlines()]
    next(row for row in rows if row['event']=='network_attempt')['target_url']='http://172.20.0.2:8081/v1/metrics'
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def helper_can_mount_direct_evidence_under_alias(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['inspect_evidence_file']; data=json.loads(path.read_text())
    for stage in ('pre','post_restart'):
        mount=dict(data[stage]['direct']['container']['Mounts'][0]); mount['Destination']='/stolen-evidence'
        data[stage]['helper']['container']['Mounts']=[mount]
    path.write_text(json.dumps(data))
def fixed_case_null_phase_with_early_ordinal(root):
    m=json.loads((root/'manifest.json').read_text()); path=root/m['host_commands_file']; rows=[json.loads(line) for line in path.read_text().splitlines()]
    fixed=next(row for row in rows if row.get('kind')=='case' and row.get('case_id')=='direct_ip_pre')
    preflight=next(row for row in rows if row.get('kind')=='case' and row.get('case_id')=='baseline_ip')
    fixed['phase_id']=None
    fixed['controller_ordinal'],preflight['controller_ordinal']=preflight['controller_ordinal'],fixed['controller_ordinal']
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def missing_source_helper_start_ack(root):
    m=json.loads((root/'manifest.json').read_text())
    for source in ('direct','adapter'):
        path=root/m['source_streams'][source]; rows=[json.loads(line) for line in path.read_text().splitlines()]
        rows=[row for row in rows if row.get('event')!='helper_start']
        for index,row in enumerate(rows,1):
            row['seq']=index; row['time_ns']=index; row['monotonic_ns']=index
        rows[-1]['final_seq']=len(rows); rows[-1]['event_count']=len(rows)
        path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
def control_tcp_accept_precedes_helper_ack(root):
    m=json.loads((root/'manifest.json').read_text()); source='direct'; path=root/m['source_streams'][source]
    rows=[json.loads(line) for line in path.read_text().splitlines()]
    marker=next(i for i,row in enumerate(rows) if row.get('event')=='helper_start' and row.get('phase_id')=='positive-controls')
    accept=next(row for row in rows if row.get('event')=='tcp_accept' and row.get('phase_id')=='positive-controls')
    connection=accept['connection_id']; request_ids={row.get('request_id') for row in rows if row.get('event')=='request_start' and row.get('connection_id')==connection}
    block=[row for row in rows if row.get('connection_id')==connection or row.get('request_id') in request_ids]
    rows=[row for row in rows if row not in block]
    rows[marker:marker]=block
    for index,row in enumerate(rows,1): row['seq']=index; row['time_ns']=index; row['monotonic_ns']=index
    rows[-1]['final_seq']=len(rows); rows[-1]['event_count']=len(rows)
    path.write_text(''.join(json.dumps(row,sort_keys=True)+'\n' for row in rows))
reject_mutation('fix5-red-helper-stop-reordered-before-control',reorder_helper_stop_before_control,'violates stop/inspect/begin/helper/barrier/stop ordinal order')
reject_mutation('fix5-red-cleanup-reordered-before-seals',cleanup_rows_before_seal,'cleanup action is not ordered after both recorder seals')
reject_mutation('fix5-red-shell-executes-wrong-target-with-inert-expected-url',shell_executes_wrong_target,'does not equal the exact verifier-owned curl argv')
reject_mutation('fix5-red-self-consistent-wrong-hostile-origin',self_consistent_wrong_hostile_origin,'fixed hostile provider')
reject_mutation('fix5-red-unsafe-adapter-inspect-with-safe-summary',unsafe_adapter_inspect,'adapter inspect')
reject_mutation('fix5-red-self-consistent-wrong-hostile-provider',wrong_hostile_provider_self_consistent,'provider-selection trace does not prove the fixed provider/model/baseURL')
reject_mutation('fix5-red-adapter-shares-direct-evidence-volume',adapter_shares_direct_evidence,'A can write the direct recorder evidence mount')
reject_mutation('fix5-red-helper-extra-network-attachment',helper_attached_to_extra_network,'helper network attachments differ from the fixed topology')
reject_mutation('fix5-red-missing-controller-ordinal',missing_controller_ordinal,'controller timeline ordinals are missing')
reject_mutation('fix5-red-duplicate-controller-ordinal',duplicate_controller_ordinal,'controller timeline ordinals are missing')
reject_mutation('fix5-red-cross-phase-operation-reference',cross_phase_operation_reference,'references a cross-phase operation')
reject_mutation('fix5-live-without-independent-opencode-provenance-blocks',live_provenance_unavailable_blocks,'independent pinned OpenCode provider-selection and actual network-attempt provenance is unavailable')
reject_mutation('fix5-self-consistent-wrong-opencode-request-path',wrong_attempt_endpoint_self_consistent,'OpenCode network-attempt trace does not prove denied traffic to selected baseURL')
reject_mutation('fix6-red-source-helper-start-boundary-missing',missing_source_helper_start_ack,'source-owned helper-start boundary is absent')
reject_mutation('fix6-red-control-tcp-accept-before-helper-ack',control_tcp_accept_precedes_helper_ack,'control traffic precedes the source-owned helper-start boundary')
reject_mutation('fix6-red-fixed-curl-null-phase-and-early-ordinal',fixed_case_null_phase_with_early_ordinal,'case direct_ip_pre is not bound to its verifier-owned phase')
reject_mutation('fix6-red-helper-alias-mounts-direct-evidence',helper_can_mount_direct_evidence_under_alias,'helper inspect mount inventory is not the fixed empty set')

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
