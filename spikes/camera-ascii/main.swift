import AVFoundation
import Foundation

final class Renderer: NSObject, AVCaptureVideoDataOutputSampleBufferDelegate {
    func captureOutput(_ output: AVCaptureOutput,
                       didOutput sampleBuffer: CMSampleBuffer,
                       from connection: AVCaptureConnection) {
        guard let pixels = CMSampleBufferGetImageBuffer(sampleBuffer) else { return }
        CVPixelBufferLockBaseAddress(pixels, .readOnly)
        defer { CVPixelBufferUnlockBaseAddress(pixels, .readOnly) }
        guard let base = CVPixelBufferGetBaseAddress(pixels) else { return }

        let srcW = CVPixelBufferGetWidth(pixels)
        let srcH = CVPixelBufferGetHeight(pixels)
        guard srcW > 0, srcH > 0 else { return }

        let (termCols, termRows) = terminalSize()
        let out = asciiFrame(buf: base.assumingMemoryBound(to: UInt8.self),
                             width: srcW,
                             height: srcH,
                             stride: CVPixelBufferGetBytesPerRow(pixels),
                             termCols: termCols,
                             termRows: termRows)
        guard !out.isEmpty else { return }
        fputs(out, stdout)
        fflush(stdout)
    }
}

// MARK: - 캡처

private func startCapture() -> AVCaptureSession? {
    let device = AVCaptureDevice.default(.builtInWideAngleCamera, for: .video, position: .front)
        ?? AVCaptureDevice.default(for: .video)
    guard let device else {
        FileHandle.standardError.write(Data("카메라를 찾지 못했습니다.\n".utf8))
        return nil
    }

    let session = AVCaptureSession()
    session.sessionPreset = .vga640x480

    do {
        let input = try AVCaptureDeviceInput(device: device)
        guard session.canAddInput(input) else { return nil }
        session.addInput(input)
    } catch {
        FileHandle.standardError.write(Data("카메라 입력 실패: \(error.localizedDescription)\n".utf8))
        return nil
    }

    let output = AVCaptureVideoDataOutput()
    output.videoSettings = [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA]
    output.alwaysDiscardsLateVideoFrames = true
    output.setSampleBufferDelegate(renderer, queue: DispatchQueue(label: "camera-ascii.frames"))
    guard session.canAddOutput(output) else { return nil }
    session.addOutput(output)

    session.startRunning()
    return session
}

// MARK: - main

let renderer = Renderer()
var session: AVCaptureSession?

func shutdown(_ code: Int32) -> Never {
    session?.stopRunning()
    leaveAltScreen()
    exit(code)
}

signal(SIGINT, SIG_IGN)
signal(SIGTERM, SIG_IGN)
let sigint = DispatchSource.makeSignalSource(signal: SIGINT, queue: .main)
let sigterm = DispatchSource.makeSignalSource(signal: SIGTERM, queue: .main)
for src in [sigint, sigterm] {
    src.setEventHandler { shutdown(0) }
    src.resume()
}

AVCaptureDevice.requestAccess(for: .video) { granted in
    DispatchQueue.main.async {
        guard granted else {
            FileHandle.standardError.write(Data(
                "카메라 권한이 없습니다. 시스템 설정 > 개인정보 보호 및 보안 > 카메라에서 이 터미널 앱을 허용하세요.\n".utf8))
            exit(1)
        }
        guard let s = startCapture() else { exit(1) }
        session = s
        enterAltScreen()
    }
}

dispatchMain()
