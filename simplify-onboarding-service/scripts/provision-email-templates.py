#!/usr/bin/env python3
"""
Provision the onboarding email templates INTO MailForge (under the "Simplify Onboarding"
project). After this, the onboarding service sends by template_id + variables and holds
no email HTML of its own.

Templates' HTML source lives in ./scripts/email-templates/ (version-controlled inputs);
this script uploads them as MailForge template collections + published versions.

Usage:
    MAILFORGE_URL=http://localhost:8099 \
    MAILFORGE_ADMIN_KEY=mf_proj_<super-admin> \
    PROJECT_ID=d57439a2-3c1f-46dc-a92f-1f1b6df16ae0 \
    python3 scripts/provision-email-templates.py

Prints the three template (collection) ids to put in the onboarding env:
    MAILFORGE_TEMPLATE_CONFIRMATION / _TEAM_NOTIFY / _INVITE
"""
import json
import os
import sys
import urllib.request

BASE = os.environ.get("MAILFORGE_URL", "http://localhost:8099").rstrip("/")
ADMIN = os.environ["MAILFORGE_ADMIN_KEY"]
PROJECT = os.environ["PROJECT_ID"]
HTML_DIR = os.path.join(os.path.dirname(__file__), "email-templates")

# slug, name, html file, subject template, text template
TEMPLATES = [
    ("onb-demo-confirmation", "Onboarding — Demo Confirmation", "confirmation.html",
     "We've got your {{.RequestLabel}} — Simplify",
     "We've got your {{.RequestLabel}} for {{.ProductName}}. A Simplify Solutions Engineer will reach out within {{.ReplyWindow}}."),
    ("onb-demo-team-notify", "Onboarding — Team Notification", "team-notify.html",
     "New {{.RequestTypeLabel}} request: {{.Company}}",
     "New {{.RequestTypeLabel}} request from {{.ContactName}} ({{.ContactEmail}}) at {{.Company}}. Request {{.RequestID}}."),
    ("onb-demo-invite", "Onboarding — Meeting Invite", "invite.html",
     "Your Simplify meeting is scheduled — {{.WhenLine}}",
     "Your {{.RequestLabel}} is scheduled for {{.WhenLine}}. Join: {{.MeetingURL}}"),
]


def call(method, path, body=None, key=ADMIN):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, method=method)
    req.add_header("Authorization", "Bearer " + key)
    req.add_header("Content-Type", "application/json")
    with urllib.request.urlopen(req, timeout=30) as r:
        raw = r.read().decode().strip()
        return json.loads(raw) if raw else {}


def main():
    # 1. Issue a template:write key for the project.
    k = call("POST", f"/v1/projects/{PROJECT}/api-keys",
             {"Name": "onboarding-templates", "Scopes": ["template:write", "template:read"]})
    tkey = k["data"]["key"]
    print("issued template key:", tkey[:12] + "...\n")

    # Upsert: reuse an existing collection by slug (stable ids); else create it. Then
    # add a NEW published version each run — so edits roll out without changing ids.
    existing = call("GET", "/v1/templates?limit=200", key=tkey).get("data", [])
    by_slug = {c.get("slug"): c.get("id") for c in existing}

    out = {}
    for slug, name, htmlfile, subject, text in TEMPLATES:
        html = open(os.path.join(HTML_DIR, htmlfile), encoding="utf-8").read()
        cid = by_slug.get(slug)
        if cid:
            print(f"  {slug:24} -> {cid}  (reusing)")
        else:
            c = call("POST", "/v1/templates",
                     {"slug": slug, "name": name, "category": "transactional"}, key=tkey)
            cid = c["data"]["id"]
        # new version + publish
        v = call("POST", f"/v1/templates/{cid}/versions",
                 {"subject": subject, "html_body": html, "text_body": text}, key=tkey)
        vid = v["data"]["id"]
        call("POST", f"/v1/templates/{cid}/versions/{vid}/publish", {}, key=tkey)
        out[slug] = cid
        print(f"  {slug:24} -> {cid}  (published v{v['data'].get('version','?')})")

    print("\nAdd to onboarding .env:")
    print("MAILFORGE_TEMPLATE_CONFIRMATION=" + out["onb-demo-confirmation"])
    print("MAILFORGE_TEMPLATE_TEAM_NOTIFY=" + out["onb-demo-team-notify"])
    print("MAILFORGE_TEMPLATE_INVITE=" + out["onb-demo-invite"])


if __name__ == "__main__":
    try:
        main()
    except urllib.error.HTTPError as e:
        print("HTTP", e.code, e.read().decode()[:400], file=sys.stderr)
        sys.exit(1)
