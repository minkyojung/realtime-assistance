"""한국어 STT 스모크 테스트 — 지원 여부 / 물음표 / 영문약어 보존."""
import json, os, subprocess, sys

KEY = os.environ["OPENAI_API_KEY"]
MODELS = ["gpt-live-transcribe", "gpt-4o-transcribe", "gpt-4o-mini-transcribe"]
S = json.load(open("spikes/audio/script.json"))

def transcribe(wav, model, lang=True):
    cmd = ["curl", "-s", "https://api.openai.com/v1/audio/transcriptions",
           "-H", f"Authorization: Bearer {KEY}",
           "-F", f"file=@{wav}", "-F", f"model={model}"]
    if lang:
        cmd += ["-F", "language=ko"]
    r = json.loads(subprocess.run(cmd, capture_output=True, text=True).stdout or "{}")
    return r.get("text", f"[ERR] {r.get('error',{}).get('message','?')}")

results = {}
for model in MODELS:
    print(f"\n{'='*78}\n{model}\n{'='*78}")
    rows = []
    for sid, text, kind in S:
        got = transcribe(f"spikes/audio/{sid}.wav", model).strip()
        rows.append({"id": sid, "kind": kind, "expected": text, "got": got})
        qm = "?" if got.endswith("?") else " "
        print(f"  {sid} [{qm}] {kind:<18} {got}")
    results[model] = rows

json.dump(results, open("spikes/out/smoke_ko_stt.json", "w"), ensure_ascii=False, indent=2)

# 요약
print(f"\n{'='*78}\n요약\n{'='*78}")
print(f"{'모델':<24} {'물음표(질문5개中)':>18} {'약어보존(3개中)':>16}")
QIDS = {"s1","s2","s3","s7","s8"}   # 물음표가 붙어야 하는 것 (s6 'SSO요'는 애매)
for model, rows in results.items():
    q = sum(1 for r in rows if r["id"] in QIDS and r["got"].endswith("?"))
    a = sum(1 for r in rows if
            (r["id"]=="s2" and "SAML" in r["got"] and "SSO" in r["got"]) or
            (r["id"]=="s3" and "SOC" in r["got"]) or
            (r["id"]=="s8" and "Edge Functions" in r["got"]))
    print(f"{model:<24} {q:>10}/5 {a:>14}/3")
