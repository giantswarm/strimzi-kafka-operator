#!/usr/bin/env python3
"""Patch Strimzi Grafana dashboards with GS-specific tweaks.

For every dashboard JSON given as an argument:
- Prepend a `cluster_id` (label "Cluster") query variable as the first entry
  in `templating.list`.
- Inject `cluster_id="$cluster_id"` into every panel `expr` and every
  templating variable `query` / `definition`:
    metric_name{...}    -> metric_name{cluster_id="$cluster_id",...}
    metric_name         -> metric_name{cluster_id="$cluster_id"}
  Detection of bare metric names is restricted to identifiers starting with
  one of the prefixes used in these dashboards: kafka_, strimzi_, jvm_,
  process_, container_, kubelet_.
- Relabel cluster-picker template variables away from the upstream
  "Cluster Name" so they don't collide with the new `cluster_id` ("Cluster")
  variable:
    strimzi_cluster_name              -> "Kafka Cluster"
    strimzi_connect_cluster_name      -> "Connect Cluster"
    strimzi_mirror_maker_cluster_name -> "MirrorMaker Cluster"
- Apply giantswarm/dashboards conventions:
    * rename the Prometheus datasource template variable from DS_PROMETHEUS
      to datasource (also rewrites every ${DS_PROMETHEUS} reference)
    * relabel that variable to "Data source"
    * set `editable: false`
    * add `owner:team-atlas`, `topic:kafka`, `component:<name>` tags
"""
import json
import os
import re
import sys

METRIC_PREFIX = r'(?:kafka_|strimzi_|jvm_|process_|container_|kubelet_)'
CLUSTER_FILTER = 'cluster_id="$cluster_id"'

# Existing `{ ... }` selector. The body never contains nested braces in these
# dashboards, so `[^}]*` is enough. Captures (name, body) so we can skip
# injection when the cluster filter is already present (idempotent).
SELECTOR_RE = re.compile(rf'\b({METRIC_PREFIX}[A-Za-z0-9_]+)\{{([^}}]*)\}}')
# Bare metric: identifier with a known metric prefix, not preceded by `$`
# (which would make it a template variable reference inside a string), and
# followed by something that terminates a metric expression (closing paren,
# range bracket, whitespace, comma, or end). This deliberately excludes `=`
# and `!` so label names like `strimzi_io_cluster="..."` are not rewritten,
# and excludes `{` so already-converted selectors don't re-match.
BARE_RE = re.compile(rf'(?<!\$)\b({METRIC_PREFIX}[A-Za-z0-9_]+)(?=[\)\[\s,]|$)')

CLUSTER_VAR_RELABEL = {
    "strimzi_cluster_name": "Kafka Cluster",
    "strimzi_connect_cluster_name": "Connect Cluster",
    "strimzi_mirror_maker_cluster_name": "MirrorMaker Cluster",
}

BASE_TAGS = ["owner:team-atlas", "topic:kafka"]

CLUSTER_VAR = {
    "current": {},
    "datasource": "${datasource}",
    "definition": "label_values(cluster_id)",
    "hide": 0,
    "includeAll": False,
    "label": "Cluster",
    "multi": False,
    "name": "cluster_id",
    "options": [],
    "query": {
        "query": "label_values(cluster_id)",
        "refId": "PrometheusVariableQueryEditor-VariableQuery",
    },
    "refresh": 1,
    "regex": "",
    "skipUrlSync": False,
    "sort": 1,
    "type": "query",
}


def inject(expr: str) -> str:
    if not expr:
        return expr

    # Step 1: inject into existing { } selectors, skipping ones already filtered.
    def selector_sub(m: re.Match) -> str:
        name, body = m.group(1), m.group(2)
        if CLUSTER_FILTER in body:
            return m.group(0)
        if not body.strip():
            return f'{name}{{{CLUSTER_FILTER}}}'
        return f'{name}{{{CLUSTER_FILTER},{body}}}'
    expr = SELECTOR_RE.sub(selector_sub, expr)

    # Step 2: bare metric names get a fresh selector. BARE_RE's lookahead
    # excludes `{`, so metrics rewritten in step 1 don't match here.
    expr = BARE_RE.sub(lambda m: f'{m.group(1)}{{{CLUSTER_FILTER}}}', expr)
    return expr


def transform_variable(var: dict) -> None:
    if var.get("type") != "query":
        return
    if var.get("name") == "cluster_id":
        return
    if isinstance(var.get("definition"), str) and var["definition"]:
        var["definition"] = inject(var["definition"])
    q = var.get("query")
    if isinstance(q, str):
        var["query"] = inject(q)
    elif isinstance(q, dict) and isinstance(q.get("query"), str):
        q["query"] = inject(q["query"])


def walk_inject_expr(node) -> None:
    if isinstance(node, dict):
        if "expr" in node and isinstance(node["expr"], str):
            node["expr"] = inject(node["expr"])
        for v in node.values():
            walk_inject_expr(v)
    elif isinstance(node, list):
        for item in node:
            walk_inject_expr(item)


def transform_dashboard(path: str) -> None:
    with open(path) as f:
        dash = json.load(f)

    templating = dash.setdefault("templating", {}).setdefault("list", [])

    # Drop any pre-existing cluster_id variable so the script is idempotent.
    templating[:] = [v for v in templating if v.get("name") != "cluster_id"]

    # Transform existing variable queries and panel expressions.
    for var in templating:
        transform_variable(var)
        new_label = CLUSTER_VAR_RELABEL.get(var.get("name"))
        if new_label and var.get("label") == "Cluster Name":
            var["label"] = new_label
        if var.get("type") == "datasource":
            var["label"] = "Data source"
    walk_inject_expr(dash.get("panels", []))
    walk_inject_expr(dash.get("rows", []))

    # Prepend the new cluster_id variable so it is the first one.
    templating.insert(0, json.loads(json.dumps(CLUSTER_VAR)))

    # Dashboards repo conventions.
    dash["editable"] = False
    component = f"component:{os.path.splitext(os.path.basename(path))[0]}"
    existing = dash.get("tags") or []
    dash["tags"] = sorted(set(existing) | set(BASE_TAGS) | {component})

    with open(path, "w") as f:
        f.write(json.dumps(dash, indent=2))
        f.write("\n")


def main():
    for path in sys.argv[1:]:
        transform_dashboard(path)
        print(f"updated {path}")


if __name__ == "__main__":
    main()
