# helper/shazamd — 듣고 알아맞히는 한 조각

`/shazam` 이 부르는 외부 프로세스다. 마이크로 몇 초 듣고 JSON 한 줄을 뱉고 죽는다.

Go 에서 ShazamKit 을 부를 수 없어서 존재한다. 상주하지 않는 이유는 듣는 일이
사용자가 시킬 때만 일어나기 때문이다 — 데몬이면 마이크를 계속 쥐고 있게 된다.

```
make                 서명 없이 빌드 → build/Shazamd.app
make listen          스피커로 노래를 틀어 두고 직접 확인
```

## 출력

성공도 실패도 JSON 한 줄이다. 종료 코드는 언제나 0 이고, 판정은 `error` 가 한다.

```json
{"ok":true,"title":"Holocene","artist":"Bon Iver","shazamId":"…","appleMusicId":"…","isrc":"…","genre":"Alternative","year":2011,"artworkUrl":"…"}
{"ok":false,"error":"no-match","message":"Nothing recognized — is the music playing out loud?"}
```

| `error` | 뜻 |
|---|---|
| `no-match` | 조용했거나 모르는 곡이다. **고장이 아니다** |
| `microphone-denied` | 터미널에 마이크 권한이 없다 |
| `catalog-refused` | 카탈로그 조회가 거부됐다 — 망이 끊겼거나 **엔타이틀먼트가 없다** |
| `audio` · `failed` | 그 밖 |

## 엔타이틀먼트 — 여기가 유일한 수동 관문

카탈로그 대조에는 `com.apple.developer.shazamkit` 이 필요하다. **제한
엔타이틀먼트라 프로비저닝 프로파일 없이는 붙일 수 없고**, ad-hoc 서명에
억지로 붙이면 AMFI 가 실행 즉시 죽인다(SIGKILL). 그래서 프로파일이 없는
빌드는 서명을 아예 안 하고, `catalog-refused` 로 정직하게 실패한다.
그 상태로도 마이크·오디오·시그니처 생성까지는 전부 확인된다.

붙이는 순서 — 한 번만 하면 된다.

1. developer.apple.com → Identifiers → App ID 하나 만든다.
   Bundle ID 는 `com.minkyojung.amcli.shazamd` (다르게 하려면 `BUNDLE_ID=` 로 넘긴다)
2. 그 App ID 의 **ShazamKit App Services** 를 켠다
3. Profiles → **Developer ID** 프로파일을 그 App ID 로 만들고 내려받는다
4. 프로파일과 인증서를 주고 다시 빌드한다

```
make PROFILE=~/Downloads/shazamd.provisionprofile \
     IDENTITY="Developer ID Application: Minkyo Jung (6DQK5MQC4H)"
```

`make listen` 이 곡 이름을 뱉으면 끝이다.

macOS 는 프로파일을 **번들에만** 담을 수 있다. 이 헬퍼가 단일 실행파일이
아니라 `.app` 인 이유가 그것뿐이다.

## 마이크

`.app` 의 `NSMicrophoneUsageDescription` 이 프롬프트 문구를 준다. 다만 권한은
터미널(Terminal·iTerm)에게 물어보고 그쪽에 기록된다 — 하위 프로세스는 부모의
권한을 따르기 때문이다. 거부되어 있으면
시스템 설정 → 개인정보 보호 및 보안 → 마이크에서 터미널을 켠다.
