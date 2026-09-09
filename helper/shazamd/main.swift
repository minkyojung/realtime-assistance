// shazamd — 마이크로 한 번 듣고, 한 줄의 JSON 을 남기고 죽는다.
//
// Go 쪽에서 ShazamKit 을 부를 방법이 없어서 존재한다. 데몬이 아니라
// 한 번 쓰고 버리는 프로세스인 이유는, 듣는 일이 사용자가 시킬 때만
// 일어나기 때문이다 — 상주하면 마이크를 계속 쥐고 있게 된다.
//
// 왜 .app 번들인가: 카탈로그 대조에 필요한 com.apple.developer.shazamkit 은
// 제한 엔타이틀먼트라 프로비저닝 프로파일이 함께 있어야 하고, macOS 는
// 프로파일을 번들에만 담을 수 있다. 단일 실행파일에 붙이면 AMFI 가 죽인다.

import AVFoundation
import Foundation
import ShazamKit

// 출력은 언제나 JSON 한 줄이다. 성공도 실패도 같은 통로로 나간다 —
// 부르는 쪽이 종료 코드와 stderr 를 따로 해석하지 않아도 되게.
func emit(_ obj: [String: Any]) -> Never {
    let data = try! JSONSerialization.data(withJSONObject: obj)
    FileHandle.standardOutput.write(data)
    FileHandle.standardOutput.write("\n".data(using: .utf8)!)
    exit(0)
}

func fail(_ code: String, _ message: String) -> Never {
    emit(["ok": false, "error": code, "message": message])
}

// 듣는 시간. 스펙의 인식 목표가 5초라(docs/03) 그 안에 끝나야 하고,
// 조용한 방에서 영원히 듣고 있으면 안 된다.
var seconds = 8.0
if let i = CommandLine.arguments.firstIndex(of: "-seconds"),
   i + 1 < CommandLine.arguments.count,
   let v = Double(CommandLine.arguments[i + 1]) {
    seconds = min(max(v, 1), 30)
}

// 마이크 권한. 물어보지 않고 세션을 열면 무음을 듣다가 "일치 없음" 으로
// 끝나서, 권한이 없다는 사실이 영영 화면에 안 나온다.
func microphone() async -> Bool {
    switch AVCaptureDevice.authorizationStatus(for: .audio) {
    case .authorized: return true
    case .notDetermined: return await AVCaptureDevice.requestAccess(for: .audio)
    default: return false
    }
}

// ShazamKit 의 에러 코드. 우리가 말이 되게 옮길 수 있는 것만 옮긴다.
func classify(_ error: Error) -> (String, String) {
    let e = error as NSError
    guard e.domain == "com.apple.ShazamKit" else {
        return ("failed", e.localizedDescription)
    }
    switch e.code {
    case 202:
        // 서명은 멀쩡한데 조회가 거부됐다. 엔타이틀먼트가 없을 때 이렇게 온다.
        // 망이 끊겨도 같은 코드가 오므로 둘 다 말한다 — 우리는 못 가른다.
        return ("catalog-refused",
                "ShazamKit could not reach the catalog. "
                    + "Check your network, and that this helper is signed with the "
                    + "ShazamKit entitlement (helper/README.md)")
    case 100, 101:
        return ("audio", e.localizedDescription)
    default:
        return ("failed", e.localizedDescription)
    }
}

func item(_ m: SHMediaItem) -> [String: Any] {
    var out: [String: Any] = ["ok": true, "title": m.title ?? "", "artist": m.artist ?? ""]
    if let id = m.shazamID { out["shazamId"] = id }
    if let id = m.appleMusicID { out["appleMusicId"] = id }
    if let isrc = m.isrc { out["isrc"] = isrc }
    if let genre = m.genres.first { out["genre"] = genre }
    if let url = m.artworkURL { out["artworkUrl"] = url.absoluteString }
    if let d = m.creationDate {
        out["year"] = Calendar(identifier: .gregorian).component(.year, from: d)
    }
    return out
}

let done = DispatchSemaphore(value: 0)

Task {
    guard await microphone() else {
        fail("microphone-denied",
             "Microphone access is off. Turn it on for your terminal in "
                 + "System Settings → Privacy & Security → Microphone")
    }

    let session = SHManagedSession()

    // 시간 제한은 우리가 건다. SHManagedSession 은 일치할 때까지 듣는다.
    let timer = Task {
        try? await Task.sleep(nanoseconds: UInt64(seconds * 1_000_000_000))
        session.cancel()
    }

    let result = await session.result()
    timer.cancel()

    switch result {
    case .match(let match):
        guard let media = match.mediaItems.first else {
            fail("no-match", "Nothing recognized")
        }
        emit(item(media))
    case .noMatch:
        // 못 알아들은 것은 고장이 아니다. 조용했거나 모르는 곡이다.
        fail("no-match", "Nothing recognized — is the music playing out loud?")
    case .error(let error, _):
        let (code, message) = classify(error)
        fail(code, message)
    @unknown default:
        fail("failed", "ShazamKit returned something we do not know")
    }
}

done.wait()
