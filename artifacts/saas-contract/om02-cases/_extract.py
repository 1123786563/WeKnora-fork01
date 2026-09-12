import re, sys
from pathlib import Path

def components(path):
    lines = Path(path).read_text().splitlines()
    comps = {}
    i = 0
    n = len(lines)
    while i < n:
        m = re.match(r"^    ([A-Za-z0-9_]+):\s*$", lines[i])
        if m:
            name = m.group(1)
            j = i + 1
            while j < n and not re.match(r"^    [A-Za-z0-9_]+:\s*$", lines[j]):
                j += 1
            comps[name] = "\n".join(lines[i+1:j])
            i = j
        else:
            i += 1
    return comps

def brief(body, depth=0):
    out = []
    for line in body.splitlines():
        s = line.strip()
        lead = len(line) - len(line.lstrip())
        if re.match(r"^[A-Za-z0-9_-]+:$", s) or re.match(r"^\$ref:", s) or s in ("type: object","type: string","type: integer","type: number","type: boolean","type: array") or s.startswith("enum:") or s.startswith("- ") or "format:" in s or "description:" in s or s in ("additionalProperties: true","nullable: true"):
            if s.startswith("description:"):
                continue
            if lead <= 8 + depth*2 or s.startswith("enum:") or s.startswith("- ") or "format:" in s or s.startswith("$ref:"):
                out.append("  "*(lead//2) + s)
    return "\n".join(out[:80])

v1v2 = components("artifacts/saas-contract/schemas/api/openapi.yaml")
v3 = components("artifacts/saas-contract/schemas/api/v3/openapi.yaml")

print("== V1V2 component names matching:")
print([k for k in v1v2 if re.search(r"CustomerCreate|Entitlement(V2|GrantV2)|IngestEventsBody|^Event$|^Subscription(Create)?$|FeatureCreate|PlanCreate", k)])
print()
for name in ["CustomerCreate","Event","IngestEventsBody","SubscriptionCreate","EntitlementV2CreateInputs","EntitlementV2"]:
    if name in v1v2:
        print(f"#### V1V2 {name}:")
        print(brief(v1v2[name]))
        print()

print("== V3 component names matching:")
print([k for k in v3 if re.search(r"[Cc]ustomer|[Gg]rant|[Aa]djustment|[Ss]ubscription|[Tt]ransaction", k)][:60])
