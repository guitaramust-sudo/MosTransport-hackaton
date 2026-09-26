// Encode numbered PNG frames to a 30 fps H.264 MP4 using macOS AVFoundation.
// Usage: swift encode_previews.swift path/to/frames_side path/to/walk_side.mp4

import Foundation
import AVFoundation
import CoreGraphics
import CoreVideo
import ImageIO

let args = CommandLine.arguments
guard args.count == 3 else {
    fatalError("Usage: swift encode_previews.swift FRAMES_DIRECTORY OUTPUT.mp4")
}
let inputURL = URL(fileURLWithPath: args[1], isDirectory: true)
let outputURL = URL(fileURLWithPath: args[2])
let fm = FileManager.default
let files = try fm.contentsOfDirectory(at: inputURL, includingPropertiesForKeys: nil)
    .filter { $0.pathExtension.lowercased() == "png" }
    .sorted { $0.lastPathComponent < $1.lastPathComponent }
guard files.count == 30 else { fatalError("Expected 30 numbered PNG frames, got \(files.count)") }
guard let firstSource = CGImageSourceCreateWithURL(files[0] as CFURL, nil),
      let firstImage = CGImageSourceCreateImageAtIndex(firstSource, 0, nil) else {
    fatalError("Cannot read first frame")
}
let width = firstImage.width
let height = firstImage.height
try? fm.removeItem(at: outputURL)

let writer = try AVAssetWriter(outputURL: outputURL, fileType: .mp4)
let settings: [String: Any] = [
    AVVideoCodecKey: AVVideoCodecType.h264,
    AVVideoWidthKey: width,
    AVVideoHeightKey: height,
    AVVideoCompressionPropertiesKey: [AVVideoAverageBitRateKey: 1_800_000],
]
let input = AVAssetWriterInput(mediaType: .video, outputSettings: settings)
input.expectsMediaDataInRealTime = false
let attrs: [String: Any] = [
    kCVPixelBufferPixelFormatTypeKey as String: Int(kCVPixelFormatType_32BGRA),
    kCVPixelBufferWidthKey as String: width,
    kCVPixelBufferHeightKey as String: height,
    kCVPixelBufferCGImageCompatibilityKey as String: true,
    kCVPixelBufferCGBitmapContextCompatibilityKey as String: true,
]
let adaptor = AVAssetWriterInputPixelBufferAdaptor(
    assetWriterInput: input, sourcePixelBufferAttributes: attrs)
guard writer.canAdd(input) else { fatalError("Cannot add video input") }
writer.add(input)
guard writer.startWriting() else { fatalError("Writer start failed: \(String(describing: writer.error))") }
writer.startSession(atSourceTime: .zero)

for (i, file) in files.enumerated() {
    while !input.isReadyForMoreMediaData {
        Thread.sleep(forTimeInterval: 0.005)
    }
    guard let source = CGImageSourceCreateWithURL(file as CFURL, nil),
          let image = CGImageSourceCreateImageAtIndex(source, 0, nil) else {
        fatalError("Cannot read \(file.path)")
    }
    var maybeBuffer: CVPixelBuffer?
    guard let pool = adaptor.pixelBufferPool,
          CVPixelBufferPoolCreatePixelBuffer(nil, pool, &maybeBuffer) == kCVReturnSuccess,
          let buffer = maybeBuffer else {
        fatalError("Pixel buffer allocation failed")
    }
    CVPixelBufferLockBaseAddress(buffer, [])
    let bitmapInfo = CGImageAlphaInfo.premultipliedFirst.rawValue |
                     CGBitmapInfo.byteOrder32Little.rawValue
    guard let context = CGContext(
        data: CVPixelBufferGetBaseAddress(buffer),
        width: width, height: height,
        bitsPerComponent: 8,
        bytesPerRow: CVPixelBufferGetBytesPerRow(buffer),
        space: CGColorSpaceCreateDeviceRGB(),
        bitmapInfo: bitmapInfo) else {
        fatalError("Cannot create bitmap context")
    }
    context.draw(image, in: CGRect(x: 0, y: 0, width: width, height: height))
    CVPixelBufferUnlockBaseAddress(buffer, [])
    let time = CMTime(value: Int64(i), timescale: 30)
    guard adaptor.append(buffer, withPresentationTime: time) else {
        fatalError("Append failed: \(String(describing: writer.error))")
    }
}
input.markAsFinished()
let done = DispatchSemaphore(value: 0)
writer.finishWriting { done.signal() }
done.wait()
guard writer.status == .completed else {
    fatalError("MP4 failed: \(String(describing: writer.error))")
}
print("ENCODED \(outputURL.path) \(files.count) frames \(width)x\(height) 30 fps")
