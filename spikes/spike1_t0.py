"""스파이크 1 — T0(즉답 캐시)가 성립하는가.

검증 대상은 "임베딩이 같은 질문을 알아보는가"가 아니라
"같은 질문의 다른 표현"과 "비슷하지만 다른 질문" 사이에
임계값을 그을 틈이 있는가 이다.

판정
  margin > 0.05   T0 채택
  0 < m <= 0.05   T0 채택하되 컨텍스트 일치를 필수 조건으로
  margin <= 0     T0 폐기 -> 2티어(T1/T2)
"""
import json, os, math, urllib.request, statistics as st

KEY = os.environ["OPENAI_API_KEY"]
MODELS = ["text-embedding-3-small", "text-embedding-3-large"]
D = json.load(open(os.path.join(os.path.dirname(__file__), "questions.json")))


def embed(texts, model):
    out = []
    for i in range(0, len(texts), 128):
        body = json.dumps({"model": model, "input": texts[i:i + 128]}).encode()
        req = urllib.request.Request(
            "https://api.openai.com/v1/embeddings", data=body,
            headers={"Authorization": f"Bearer {KEY}", "Content-Type": "application/json"})
        with urllib.request.urlopen(req) as r:
            out += [d["embedding"] for d in json.load(r)["data"]]
    return out


def cos(a, b):
    return sum(x * y for x, y in zip(a, b)) / (
        math.sqrt(sum(x * x for x in a)) * math.sqrt(sum(y * y for y in b)))


def pct(xs, p):
    xs = sorted(xs)
    return xs[min(len(xs) - 1, int(len(xs) * p))]


canon = D["canonical"]
cids = [c["id"] for c in canon]
report = {}

for model in MODELS:
    cv = embed([c["text"] for c in canon], model)
    pv = embed([p["text"] for p in D["paraphrases"]], model)
    nv = embed([n["text"] for n in D["hard_negatives"]], model)

    # A: 패러프레이즈 -> 자기 원형 유사도 / top1이 자기 원형인가
    A, top1_ok = [], 0
    for p, v in zip(D["paraphrases"], pv):
        sims = [cos(v, c) for c in cv]
        own = sims[cids.index(p["cid"])]
        A.append(own)
        if cids[sims.index(max(sims))] == p["cid"]:
            top1_ok += 1

    # B: 하드 네거티브 -> 원형 전체 중 최고 유사도 (오탐 위험의 실제 값)
    B, B_detail = [], []
    for n, v in zip(D["hard_negatives"], nv):
        sims = [cos(v, c) for c in cv]
        mx = max(sims)
        B.append(mx)
        B_detail.append((n["text"], cids[sims.index(mx)], mx))

    # C: 참고용 easy negative (패러프레이즈 -> 다른 원형)
    C = [cos(v, c) for p, v in zip(D["paraphrases"], pv)
         for cid, c in zip(cids, cv) if cid != p["cid"]]

    margin = min(A) - max(B)
    # 안전 임계값: 하드 네거티브를 단 하나도 통과시키지 않는 최소 임계값
    tau_safe = max(B) + 1e-6
    covered = sum(1 for p, v, a in zip(D["paraphrases"], pv, A)
                  if a >= tau_safe and cids[[cos(v, c) for c in cv].index(
                      max(cos(v, c) for c in cv))] == p["cid"])

    report[model] = {
        "A_paraphrase": {"min": min(A), "p10": pct(A, .1), "median": st.median(A), "max": max(A)},
        "B_hard_neg":   {"min": min(B), "median": st.median(B), "p90": pct(B, .9), "max": max(B)},
        "C_easy_neg_median": st.median(C),
        "top1_accuracy": top1_ok / len(A),
        "margin": margin,
        "tau_safe": tau_safe,
        "t0_coverage_at_tau_safe": covered / len(A),
        "worst_hard_negatives": sorted(B_detail, key=lambda x: -x[2])[:5],
    }

    print(f"\n{'='*70}\n{model}\n{'='*70}")
    r = report[model]
    print(f"  A 패러프레이즈→원형   min={r['A_paraphrase']['min']:.4f}  "
          f"p10={r['A_paraphrase']['p10']:.4f}  median={r['A_paraphrase']['median']:.4f}")
    print(f"  B 하드네거티브→최근접 median={r['B_hard_neg']['median']:.4f}  "
          f"p90={r['B_hard_neg']['p90']:.4f}  max={r['B_hard_neg']['max']:.4f}")
    print(f"  C 이지네거티브 median  {r['C_easy_neg_median']:.4f}")
    print(f"  top1 정확도            {r['top1_accuracy']:.1%}")
    print(f"  margin (minA - maxB)   {r['margin']:+.4f}")
    print(f"  안전 임계값 tau_safe   {r['tau_safe']:.4f}")
    print(f"  그때 T0 커버리지       {r['t0_coverage_at_tau_safe']:.1%}")
    print("  가장 위험한 하드네거티브:")
    for t, c, s in r["worst_hard_negatives"]:
        print(f"     {s:.4f}  {t[:44]:<46} → {c}")

    v = "T0 채택" if margin > .05 else ("T0 채택(컨텍스트 필수)" if margin > 0 else "T0 폐기 → 2티어")
    print(f"  판정 ▶ {v}")

os.makedirs(os.path.join(os.path.dirname(__file__), "out"), exist_ok=True)
json.dump(report, open(os.path.join(os.path.dirname(__file__), "out", "spike1.json"), "w"),
          ensure_ascii=False, indent=2)
print("\n결과 저장: spikes/out/spike1.json")
