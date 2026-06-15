import QuickLookThumbnailing
import SceneKit
import Metal
import Cocoa

// Quick Look thumbnail for .fct files (icon and gallery views): an offscreen
// SceneKit snapshot of the evaluated model at a fixed 3/4 angle. Returns nil
// (no thumbnail) when the source produces no geometry.
@objc(ThumbnailProvider)
final class ThumbnailProvider: QLThumbnailProvider {
    override func provideThumbnail(for request: QLFileThumbnailRequest,
                                   _ handler: @escaping (QLThumbnailReply?, Error?) -> Void) {
        guard let source = try? String(contentsOf: request.fileURL, encoding: .utf8),
              let scene = FacetMesh.scene(source: source, animate: false) else {
            handler(nil, nil)
            return
        }
        let size = request.maximumSize
        let renderer = SCNRenderer(device: MTLCreateSystemDefaultDevice(), options: nil)
        renderer.scene = scene
        let image = renderer.snapshot(atTime: 0, with: size, antialiasingMode: .multisampling4X)

        let reply = QLThumbnailReply(contextSize: size) { () -> Bool in
            image.draw(in: CGRect(origin: .zero, size: size))
            return true
        }
        handler(reply, nil)
    }
}
