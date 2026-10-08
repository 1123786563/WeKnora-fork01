import json
import subprocess
import os

def run_gql(query):
    p = subprocess.run(["gh", "api", "graphql", "-f", f"query={query}"], capture_output=True, text=True)
    if p.returncode != 0:
        raise RuntimeError(f"GQL error: {p.stderr}")
    return json.loads(p.stdout)["data"]["repository"]["issue"]

def main():
    root_query = """
    query {
      repository(owner: "1123786563", name: "WeKnora-fork01") {
        issue(number: 140) {
          number
          title
          state
          body
          labels(first: 20) { nodes { name } }
          subIssues(first: 50) {
            totalCount
            nodes {
              number
              title
              state
            }
          }
        }
      }
    }
    """
    root = run_gql(root_query)
    print(f"Root: #{root['number']} {root['title']} ({root['state']})")
    sub_nodes = root['subIssues']['nodes']
    print(f"Direct sub-issues count: {len(sub_nodes)}")

    sub_dict = {}
    nested_total = 0
    for s in sorted(sub_nodes, key=lambda x: x['number']):
        num = s['number']
        sub_query = f"""
        query {{
          repository(owner: "1123786563", name: "WeKnora-fork01") {{
            issue(number: {num}) {{
              number
              title
              state
              body
              labels(first: 20) {{ nodes {{ name }} }}
              subIssues(first: 50) {{
                totalCount
                nodes {{
                  number
                  title
                  state
                }}
              }}
            }}
          }}
        }}
        """
        data = run_gql(sub_query)
        sub_dict[str(num)] = data
        nested_count = data['subIssues']['totalCount']
        nested_total += nested_count
        print(f"  #{num:3d} ({data['state']:6s}): {data['title']} (nested sub-issues: {nested_count})")

    os.makedirs("docs/plans/issue-140/issues", exist_ok=True)
    out_file = "docs/plans/issue-140-fetched-tree.json"
    with open(out_file, "w", encoding="utf-8") as f:
        json.dump({"root": root, "sub_issues": sub_dict, "nested_total": nested_total}, f, ensure_ascii=False, indent=2)

    # Also save each issue markdown snapshot
    with open("docs/plans/issue-140/issues/issue-140.md", "w", encoding="utf-8") as f:
        f.write(f"# Issue #140: {root['title']}\n\nState: {root['state']}\n\n{root['body']}\n")

    for num_str, data in sub_dict.items():
        fname = f"docs/plans/issue-140/issues/issue-{num_str}.md"
        with open(fname, "w", encoding="utf-8") as f:
            f.write(f"# Issue #{num_str}: {data['title']}\n\nState: {data['state']}\n\n{data['body']}\n")

    print(f"Successfully saved tree to {out_file} and snapshots to docs/plans/issue-140/issues/")

if __name__ == "__main__":
    main()
