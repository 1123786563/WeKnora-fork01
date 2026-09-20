"""Replay the bounded Semantica 0.6.8 CPython 3.12 runtime probe."""
from __future__ import annotations
import argparse, hashlib, importlib.metadata, json, os, subprocess, sys, tempfile, zipfile
from pathlib import Path

SHA = "0af4d9dd9b01503e0d72c0ae6b0703364d01dd1443e42bc7e9a6f415835917d7"
REQ = Path(__file__).with_name("requirements-probe.txt")
PACKAGES = ("numpy", "scipy", "networkx", "python-dateutil", "PyYAML", "rdflib", "pytest")
def write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile("w", dir=path.parent, delete=False) as f: json.dump(data,f,indent=2); f.write("\n"); name=f.name
    os.replace(name,path)
def execute(wheel):
    with tempfile.TemporaryDirectory(prefix="semantica-wheel-") as d:
        with zipfile.ZipFile(wheel) as z: z.extractall(d)
        sys.path.insert(0,d)
        from semantica.reasoning import Reasoner
        from semantica.context import ContextGraph
        import semantica, semantica.reasoning.graph_reasoner as gr
        assert Path(semantica.__file__).is_relative_to(Path(d))
        r=Reasoner(); r.add_rule("IF DependsOn(?a, ?b) AND DependsOn(?b, ?c) THEN IndirectDependsOn(?a, ?c)"); r.add_fact("DependsOn(A, B)"); r.add_fact("DependsOn(B, C)"); pos=r.forward_chain(); assert len(pos)==1 and pos[0].conclusion=="IndirectDependsOn(A, C)" and pos[0].premises==["DependsOn(A, B)","DependsOn(B, C)"]
        n=Reasoner(); n.add_rule("IF DependsOn(?a, ?b) AND DependsOn(?b, ?c) THEN IndirectDependsOn(?a, ?c)"); n.add_fact("DependsOn(A, B)"); assert n.forward_chain()==[]
        g=ContextGraph(); nodes=g.add_nodes([{"id":"A","properties":{"document_id":"doc-a"}}]); edges=g.add_edges([{"source":"A","target":"B","type":"DependsOn","properties":{"chunk_id":"c1"}}]); assert nodes==edges==1
        class Provider:
            def __init__(self): self.prompt=""
            def generate(self,prompt,**_): self.prompt=prompt; return "controlled-result"
        provider=Provider(); old=gr.create_provider; gr.create_provider=lambda *_a,**_k:provider
        try: result=gr.GraphReasoner(provider="controlled").reason({"entities":[{"id":"A","properties":{"document_id":"doc-a"}}],"relationships":[{"source":"A","target":"B","type":"DependsOn","properties":{"chunk_id":"c1"}}]},"q")
        finally: gr.create_provider=old
        assert result=="controlled-result" and "doc-a" in provider.prompt and "chunk_id" in provider.prompt
        return {"semantica_file":semantica.__file__,"reasoner":{"positive":{"conclusion":pos[0].conclusion,"premises":pos[0].premises},"missing_second_premise":{"results":[]}},"context":{"classification":"actual-runtime","add_nodes_return":nodes,"add_edges_return":edges},"graph_reasoner":{"classification":"mock","factory_patch":True,"result":result,"entity_property_in_prompt":"doc-a" in provider.prompt,"relationship_property_in_prompt":"chunk_id" in provider.prompt,"limitation":"not a real provider or Go gateway"}}
def main():
    p=argparse.ArgumentParser(); p.add_argument("--wheel",required=True,type=Path); p.add_argument("--output",required=True,type=Path); p.add_argument("--child",action="store_true"); a=p.parse_args()
    if not a.wheel.exists(): p.error(f"wheel does not exist: {a.wheel}")
    digest=hashlib.sha256(a.wheel.read_bytes()).hexdigest()
    if digest!=SHA: p.error(f"wheel SHA-256 mismatch: got {digest}, expected {SHA}")
    if a.child: print(json.dumps(execute(a.wheel))); return 0
    data={"schema_version":1,"classification":"actual-runtime","command":sys.argv,"python":{"version":sys.version,"executable":sys.executable},"wheel_sha256":digest,"requirements_sha256":hashlib.sha256(REQ.read_bytes()).hexdigest(),"dependencies":{x:importlib.metadata.version(x) for x in PACKAGES}}
    try:
        r=subprocess.run([sys.executable,str(Path(__file__)),"--child","--wheel",str(a.wheel),"--output",str(a.output)],capture_output=True,text=True,timeout=60,check=True); data["public_import"]={"exit_code":0}; data.update(json.loads(r.stdout))
    except subprocess.TimeoutExpired as e:
        data.update({"classification":"failure","child_execution":{"exit_code":None,"timeout":True},"failure":{"kind":"timeout","seconds":60,"stderr":e.stderr.decode() if isinstance(e.stderr,bytes) else (e.stderr or "")}}); write(a.output,data); return 1
    except (subprocess.CalledProcessError,json.JSONDecodeError) as e:
        data.update({"classification":"failure","child_execution":{"exit_code":getattr(e,"returncode",None),"timeout":False},"failure":{"kind":"child-error","detail":getattr(e,"stderr",str(e))}}); write(a.output,data); return 1
    write(a.output,data); return 0
if __name__=="__main__": raise SystemExit(main())
