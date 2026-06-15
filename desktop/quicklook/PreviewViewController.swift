import Cocoa
import Quartz
import SceneKit

// Quick Look interactive preview for .fct files: renders the evaluated model in
// a drag-to-orbit SceneKit view. If the source fails to evaluate to geometry
// (parse/type error, or it isn't a Solid), it falls back to showing the source.
@objc(PreviewViewController)
final class PreviewViewController: NSViewController, QLPreviewingController {

    override func loadView() {
        view = NSView(frame: NSRect(x: 0, y: 0, width: 720, height: 540))
    }

    func preparePreviewOfFile(at url: URL, completionHandler handler: @escaping (Error?) -> Void) {
        let source = (try? String(contentsOf: url, encoding: .utf8)) ?? ""
        let content: NSView
        if let scene = FacetMesh.scene(source: source, animate: true) {
            let scn = SCNView(frame: view.bounds)
            scn.backgroundColor = NSColor(calibratedWhite: 0.12, alpha: 1)
            scn.allowsCameraControl = true
            scn.antialiasingMode = .multisampling4X
            scn.scene = scene
            content = scn
        } else {
            content = Self.sourceView(source, frame: view.bounds)
        }
        content.autoresizingMask = [.width, .height]
        view.subviews.forEach { $0.removeFromSuperview() }
        view.addSubview(content)
        handler(nil)
    }

    // A read-only monospaced text view, used when the model can't be rendered.
    private static func sourceView(_ text: String, frame: NSRect) -> NSView {
        let scroll = NSScrollView(frame: frame)
        scroll.hasVerticalScroller = true
        scroll.borderType = .noBorder
        let tv = NSTextView(frame: scroll.bounds)
        tv.isEditable = false
        tv.string = text
        tv.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        tv.textContainerInset = NSSize(width: 12, height: 12)
        scroll.documentView = tv
        return scroll
    }
}
