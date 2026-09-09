// render.swift 의 asciiFrame 을 카메라 없이 검증한다.
// 실행: swiftc render.swift rendertest/main.swift -o rendertest.bin && ./rendertest.bin [--mono]
import Foundation

let srcW = 64, srcH = 32, stride = srcW * 4
var pixels = [UInt8](repeating: 0, count: stride * srcH)
for y in 0..<srcH {
    for x in 0..<srcW {
        // 왼쪽이 검정, 오른쪽이 흰색인 가로 그라데이션.
        let v = UInt8(x * 255 / (srcW - 1))
        let p = y * stride + x * 4
        pixels[p] = v; pixels[p + 1] = v; pixels[p + 2] = v; pixels[p + 3] = 255
    }
}

let termCols = 40, termRows = 10
let out = pixels.withUnsafeBufferPointer {
    asciiFrame(buf: $0.baseAddress!, width: srcW, height: srcH, stride: stride,
               termCols: termCols, termRows: termRows)
}

var failures = 0
func check(_ label: String, _ ok: Bool) {
    print("\(ok ? "ok  " : "FAIL") \(label)")
    if !ok { failures += 1 }
}

let lines = out.components(separatedBy: "\n")
check("화면 높이가 터미널 행 수와 같다 (\(lines.count) == \(termRows))", lines.count == termRows)

let stripped = lines.map { line -> String in
    var s = "", i = line.startIndex
    while i < line.endIndex {
        if line[i] == "\u{1b}" {
            while i < line.endIndex, !"mHK".contains(line[i]) { i = line.index(after: i) }
            if i < line.endIndex { i = line.index(after: i) }
        } else {
            s.append(line[i]); i = line.index(after: i)
        }
    }
    return s
}
check("어떤 줄도 터미널 폭을 넘지 않는다", stripped.allSatisfy { $0.count <= termCols })

// 그라데이션이 있는(=공백이 아닌) 줄을 하나 골라 왼→오른쪽으로 밝아지는지 본다.
// 램프의 첫 글자가 공백이므로 자르지 않고 줄 전체를 본다.
guard let row = stripped.max(by: { $0.count < $1.count }), row.count > 4 else {
    print("FAIL 이미지 줄을 찾지 못했다"); exit(1)
}
let idx = Array(row).map { ramp.firstIndex(of: $0) ?? -1 }
check("모든 문자가 램프 안에 있다", !idx.contains(-1))
// 셀카처럼 좌우를 뒤집으므로, 원본의 왼쪽(검정)이 화면 오른쪽에 온다.
check("거울 반전: 좌→우로 밝기가 단조 감소한다", zip(idx, idx.dropFirst()).allSatisfy { $0 >= $1 })
check("가장 어두운 칸과 가장 밝은 칸이 램프 양끝이다",
      idx.first == ramp.count - 1 && idx.last == 0)

let hasColor = out.contains("\u{1b}[38;2;")
check(mono ? "--mono 에서는 색 이스케이프가 없다" : "컬러 모드에서 트루컬러 이스케이프가 있다",
      mono ? !hasColor : hasColor)

print(failures == 0 ? "\nPASS" : "\n\(failures) FAILED")
exit(failures == 0 ? 0 : 1)
