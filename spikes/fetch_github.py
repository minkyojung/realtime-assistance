"""Supabase 레포에서 스파이크용 원천 데이터를 수집한다.

세 종류를 구분해서 받는다. 이 구분이 곧 audience_level의 근거다.
  releases      확정 사실  -> public
  merged PR     확정 사실  -> public
  open issue    미확정     -> internal
"""
import json, os, sys, time, urllib.request, urllib.parse

REPO = os.environ["GITHUB_REPO"]
TOKEN = os.environ["GITHUB_TOKEN"]
OUT = os.path.join(os.path.dirname(__file__), "data")


def get(path, **params):
    url = f"https://api.github.com{path}"
    if params:
        url += "?" + urllib.parse.urlencode(params)
    req = urllib.request.Request(url, headers={
        "User-Agent": "relay-spike",
        "Authorization": f"Bearer {TOKEN}",
        "Accept": "application/vnd.github+json",
    })
    with urllib.request.urlopen(req) as r:
        return json.load(r)


def dump(name, rows):
    path = os.path.join(OUT, f"{name}.json")
    with open(path, "w") as f:
        json.dump(rows, f, ensure_ascii=False, indent=2)
    print(f"  {name:20} {len(rows):>4}건  -> {path}")


print(f"수집 대상: {REPO}")

# 1. 릴리즈 (확정)
releases = [{
    "tag": r["tag_name"], "name": r["name"], "published_at": r["published_at"],
    "body": (r.get("body") or "")[:4000], "url": r["html_url"],
} for r in get(f"/repos/{REPO}/releases", per_page=30)]
dump("releases", releases)

# 2. 머지된 PR (확정)
merged = []
for page in (1, 2, 3):
    for p in get(f"/repos/{REPO}/pulls", state="closed", per_page=100, page=page,
                 sort="updated", direction="desc"):
        if p.get("merged_at"):
            merged.append({
                "number": p["number"], "title": p["title"],
                "body": (p.get("body") or "")[:2000],
                "merged_at": p["merged_at"], "url": p["html_url"],
                "labels": [l["name"] for l in p.get("labels", [])],
            })
    time.sleep(0.2)
dump("merged_prs", merged)

# 3. 열린 이슈 (미확정)
open_issues = []
for page in (1, 2, 3):
    for i in get(f"/repos/{REPO}/issues", state="open", per_page=100, page=page,
                 sort="comments", direction="desc"):
        if "pull_request" in i:
            continue
        open_issues.append({
            "number": i["number"], "title": i["title"],
            "body": (i.get("body") or "")[:2000],
            "comments": i["comments"], "created_at": i["created_at"],
            "url": i["html_url"], "labels": [l["name"] for l in i.get("labels", [])],
            "milestone": (i.get("milestone") or {}).get("title"),
        })
    time.sleep(0.2)
dump("open_issues", open_issues)

# 4. 세일즈 관련 키워드로 이슈 검색 (확정/미확정 혼재 - 질문 역생성의 재료)
SALES_TERMS = ["SSO", "SAML", "self-hosted", "pricing", "compliance",
               "HIPAA", "SOC 2", "enterprise", "on-premise", "SLA"]
found = {}
for term in SALES_TERMS:
    q = f'repo:{REPO} "{term}" in:title'
    try:
        res = get("/search/issues", q=q, per_page=20, sort="comments", order="desc")
    except Exception as e:
        print(f"  검색 실패 {term}: {e}", file=sys.stderr)
        continue
    for i in res.get("items", []):
        found[i["number"]] = {
            "number": i["number"], "title": i["title"],
            "body": (i.get("body") or "")[:1500],
            "state": i["state"], "comments": i["comments"],
            "is_pr": "pull_request" in i, "url": i["html_url"],
            "labels": [l["name"] for l in i.get("labels", [])],
            "matched_term": term,
        }
    time.sleep(2)  # search API는 분당 30회 제한
dump("sales_issues", list(found.values()))
print("완료")
