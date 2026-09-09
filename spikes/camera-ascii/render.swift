import Foundation

// 어두운 → 밝은 순서. 인덱스 = 휘도 * (count-1).
let ramp = Array(" .:-=+*#%@")

let mono = CommandLine.arguments.contains("--mono")

// MARK: - 터미널

func terminalSize() -> (cols: Int, rows: Int) {
    var ws = winsize()
    guard ioctl(STDOUT_FILENO, UInt(TIOCGWINSZ), &ws) == 0, ws.ws_col > 0, ws.ws_row > 0 else {
        return (80, 24)
    }
    return (Int(ws.ws_col), Int(ws.ws_row))
}

func enterAltScreen() {
    fputs("\u{1b}[?1049h\u{1b}[?25l", stdout)
    fflush(stdout)
}

func leaveAltScreen() {
    fputs("\u{1b}[0m\u{1b}[?25h\u{1b}[?1049l", stdout)
    fflush(stdout)
}

// MARK: - 렌더러

/// BGRA 프레임 하나를 터미널 한 화면 분량의 ASCII 문자열로 바꾼다.
/// 커서 이동(`ESC[H`)까지 포함하므로 그대로 stdout 에 쓰면 된다.
func asciiFrame(buf: UnsafePointer<UInt8>,
                width srcW: Int,
                height srcH: Int,
                stride: Int,
                termCols: Int,
                termRows: Int) -> String {
    // 문자 칸은 대략 세로:가로 = 2:1 이므로 가로 칸 수를 두 배로 잡아야 비율이 맞는다.
    let aspect = Double(srcW) / Double(srcH)
    var rows = termRows
    var cols = Int(2.0 * aspect * Double(rows))
    if cols > termCols {
        cols = termCols
        rows = Int(Double(cols) / (2.0 * aspect))
    }
    guard cols > 0, rows > 0 else { return "" }

    let padTop = (termRows - rows) / 2
    let padLeft = String(repeating: " ", count: (termCols - cols) / 2)

    var out = "\u{1b}[H"
    out.reserveCapacity(termCols * termRows * 8)

    for _ in 0..<padTop { out += "\u{1b}[0m\u{1b}[K\n" }

    for oy in 0..<rows {
        let y0 = oy * srcH / rows
        let y1 = max(y0 + 1, (oy + 1) * srcH / rows)
        var line = padLeft
        var lastKey = -1

        for ox in 0..<cols {
            let x0 = ox * srcW / cols
            let x1 = max(x0 + 1, (ox + 1) * srcW / cols)

            var sumB = 0, sumG = 0, sumR = 0, n = 0
            for y in y0..<y1 {
                let row = y * stride
                for x in x0..<x1 {
                    // 좌우 반전: 셀카처럼 보이게.
                    let p = row + (srcW - 1 - x) * 4
                    sumB += Int(buf[p])
                    sumG += Int(buf[p + 1])
                    sumR += Int(buf[p + 2])
                    n += 1
                }
            }
            let r = sumR / n, g = sumG / n, b = sumB / n
            let luma = (299 * r + 587 * g + 114 * b) / 1000

            if !mono {
                // 색이 실제로 바뀔 때만 이스케이프를 낸다 (프레임당 바이트 수 절감).
                let key = (r >> 3) << 10 | (g >> 3) << 5 | (b >> 3)
                if key != lastKey {
                    line += "\u{1b}[38;2;\(r);\(g);\(b)m"
                    lastKey = key
                }
            }
            line.append(ramp[luma * (ramp.count - 1) / 255])
        }
        out += line
        out += "\u{1b}[0m\u{1b}[K"
        if padTop + oy < termRows - 1 { out += "\n" }
    }

    for i in (padTop + rows)..<termRows {
        out += "\u{1b}[0m\u{1b}[K"
        if i < termRows - 1 { out += "\n" }
    }

    return out
}

