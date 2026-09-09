// camerad — 카메라 프레임만 내보내는 헬퍼.
//
// 화면에 대해서는 아무것도 모른다. ASCII 로 바꾸는 일도, 터미널 크기도
// 전부 Go 쪽(internal/cameraapp)의 몫이다. 여기가 하는 일은 둘이다 —
// 프리뷰를 계속 흘려보내고, 요청이 오면 원본 한 장을 얹어 보낸다.
//
// 이렇게 나눈 이유는 테스트다. 완성된 화면을 여기서 만들어 보내면
// 그 앱의 렌더링은 카메라 권한 없이 검증할 수 없게 된다.
//
// # 형식
//
//	"CAM2"                                       ← 시작할 때 한 번
//	그 뒤로 메시지가 끝없이 이어진다:
//	  kind 1바이트 + width uint16be + height uint16be + width*height*3 RGB
//	  'P' 프리뷰 — 160x120 으로 줄인 것. 15fps
//	  'S' 원본  — 촬영. 요청받았을 때만 한 장
//
// 요청은 stdin 으로 한 줄 온다. "S" 면 다음 프레임에 원본을 얹는다.
//
// 좌우 반전은 하지 않는다. 그것도 보여주는 쪽의 결정이다.
import AVFoundation
import Foundation

let previewW = 160
let previewH = 120
let interval = 1.0 / 15.0  // 터미널이 소화할 수 있는 만큼만 보낸다

// stdin 을 읽는 스레드와 프레임 콜백이 함께 본다.
final class Flag {
    private let lock = NSLock()
    private var value = false
    func raise() { lock.lock(); value = true; lock.unlock() }
    func take() -> Bool {
        lock.lock()
        defer { lock.unlock() }
        let v = value
        value = false
        return v
    }
}

let wantStill = Flag()

/// 메시지 하나를 stdout 에 붓는다.
///
/// 읽는 쪽이 사라지면 SIGPIPE 로 여기서 죽는다. 그래야 부모가 예기치 않게
/// 종료해도 카메라가 켜진 채 남지 않는다.
func emit(_ kind: UInt8, _ w: Int, _ h: Int, _ rgb: UnsafePointer<UInt8>, _ count: Int) {
    let head = [kind, UInt8(w >> 8), UInt8(w & 0xff), UInt8(h >> 8), UInt8(h & 0xff)]
    head.withUnsafeBufferPointer { _ = fwrite($0.baseAddress, 1, $0.count, stdout) }
    _ = fwrite(rgb, 1, count, stdout)
    fflush(stdout)
}

final class Pump: NSObject, AVCaptureVideoDataOutputSampleBufferDelegate {
    private var last = 0.0
    private var preview = [UInt8](repeating: 0, count: previewW * previewH * 3)
    private var still = [UInt8]()

    func captureOutput(_ output: AVCaptureOutput,
                       didOutput sampleBuffer: CMSampleBuffer,
                       from connection: AVCaptureConnection) {
        let shooting = wantStill.take()
        let now = CFAbsoluteTimeGetCurrent()
        // 촬영 요청은 프레임 간격을 기다리지 않는다. 셔터는 즉시 눌려야 한다.
        guard shooting || now - last >= interval else { return }

        guard let pixels = CMSampleBufferGetImageBuffer(sampleBuffer) else { return }
        CVPixelBufferLockBaseAddress(pixels, .readOnly)
        defer { CVPixelBufferUnlockBaseAddress(pixels, .readOnly) }
        guard let base = CVPixelBufferGetBaseAddress(pixels) else { return }

        let srcW = CVPixelBufferGetWidth(pixels)
        let srcH = CVPixelBufferGetHeight(pixels)
        let stride = CVPixelBufferGetBytesPerRow(pixels)
        guard srcW >= previewW, srcH >= previewH else { return }
        let buf = base.assumingMemoryBound(to: UInt8.self)

        if shooting {
            // 원본 그대로. 줄이지 않는다 — 사진으로 남을 유일한 픽셀이다.
            let need = srcW * srcH * 3
            if still.count != need { still = [UInt8](repeating: 0, count: need) }
            for y in 0..<srcH {
                let row = y * stride
                for x in 0..<srcW {
                    let p = row + x * 4  // 32BGRA
                    let o = (y * srcW + x) * 3
                    still[o] = buf[p + 2]
                    still[o + 1] = buf[p + 1]
                    still[o + 2] = buf[p]
                }
            }
            still.withUnsafeBufferPointer {
                emit(UInt8(ascii: "S"), srcW, srcH, $0.baseAddress!, $0.count)
            }
        }

        guard now - last >= interval else { return }
        last = now

        // 블록 평균. 최근접 샘플링보다 노이즈가 훨씬 적고, 픽셀 100만 개짜리
        // 합이라 15fps 에서도 코어 하나를 다 쓰지 않는다.
        for oy in 0..<previewH {
            let y0 = oy * srcH / previewH
            let y1 = max(y0 + 1, (oy + 1) * srcH / previewH)
            for ox in 0..<previewW {
                let x0 = ox * srcW / previewW
                let x1 = max(x0 + 1, (ox + 1) * srcW / previewW)

                var sumB = 0, sumG = 0, sumR = 0, n = 0
                for y in y0..<y1 {
                    let row = y * stride
                    for x in x0..<x1 {
                        let p = row + x * 4
                        sumB += Int(buf[p])
                        sumG += Int(buf[p + 1])
                        sumR += Int(buf[p + 2])
                        n += 1
                    }
                }
                let o = (oy * previewW + ox) * 3
                preview[o] = UInt8(sumR / n)
                preview[o + 1] = UInt8(sumG / n)
                preview[o + 2] = UInt8(sumB / n)
            }
        }
        preview.withUnsafeBufferPointer {
            emit(UInt8(ascii: "P"), previewW, previewH, $0.baseAddress!, $0.count)
        }
    }
}

func fail(_ message: String) -> Never {
    FileHandle.standardError.write(Data((message + "\n").utf8))
    exit(1)
}

let pump = Pump()
var session: AVCaptureSession?

func start() {
    let device = AVCaptureDevice.default(.builtInWideAngleCamera, for: .video, position: .front)
        ?? AVCaptureDevice.default(for: .video)
    guard let device else { fail("카메라를 찾지 못했습니다") }

    let s = AVCaptureSession()
    // 프리뷰만 보면 VGA 로 충분하지만 촬영이 원본을 쓴다. 못 하는 기계도
    // 있으므로 물어보고 정한다.
    s.sessionPreset = s.canSetSessionPreset(.hd1280x720) ? .hd1280x720 : .vga640x480

    do {
        let input = try AVCaptureDeviceInput(device: device)
        guard s.canAddInput(input) else { fail("카메라 입력을 붙일 수 없습니다") }
        s.addInput(input)
    } catch {
        fail("카메라 입력 실패: \(error.localizedDescription)")
    }

    let output = AVCaptureVideoDataOutput()
    output.videoSettings = [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA]
    output.alwaysDiscardsLateVideoFrames = true
    output.setSampleBufferDelegate(pump, queue: DispatchQueue(label: "camerad.frames"))
    guard s.canAddOutput(output) else { fail("카메라 출력을 붙일 수 없습니다") }
    s.addOutput(output)

    let head = Data("CAM2".utf8)
    head.withUnsafeBytes { _ = fwrite($0.baseAddress, 1, head.count, stdout) }
    fflush(stdout)

    s.startRunning()
    session = s
}

// 요청을 받는 통로. 블로킹 읽기라 전용 스레드에 둔다.
// stdin 이 닫히면 부모가 사라진 것이므로 같이 끝난다.
func listen() {
    let t = Thread {
        while let line = readLine(strippingNewline: true) {
            if line == "S" { wantStill.raise() }
        }
        exit(0)
    }
    t.start()
}

AVCaptureDevice.requestAccess(for: .video) { granted in
    DispatchQueue.main.async {
        guard granted else {
            fail("카메라 권한이 없습니다. 시스템 설정 > 개인정보 보호 및 보안 > 카메라에서 이 터미널 앱을 허용하세요")
        }
        start()
        listen()
    }
}

dispatchMain()
