import Cocoa
import Quartz
import SceneKit

// Quick Look interactive preview for .fct/.stl/.obj/.3mf files: renders the loaded
// model in a drag-to-orbit SceneKit view. If the file fails to produce geometry,
// it shows why instead: a Facet/source file shows its text (so the user can see
// the broken code), and a mesh file shows the compile/load error.
@objc(PreviewViewController)
final class PreviewViewController: NSViewController, QLPreviewingController {

    override func loadView() {
        view = NSView(frame: NSRect(x: 0, y: 0, width: 720, height: 540))
    }

    func preparePreviewOfFile(at url: URL, completionHandler handler: @escaping (Error?) -> Void) {
        let content: NSView
        if let scene = FacetMesh.scene(path: url.path, animate: true) {
            let scn = SCNView(frame: view.bounds)
            scn.backgroundColor = NSColor(calibratedWhite: 0.12, alpha: 1)
            scn.allowsCameraControl = true
            scn.antialiasingMode = .multisampling4X
            scn.scene = scene
            // Drive the turntable: isPlaying advances scene time, and
            // rendersContinuously forces the redraw loop — a hosted Quick Look
            // SCNView won't start it on its own, so without this the spin freezes.
            scn.isPlaying = true
            scn.rendersContinuously = true
            content = scn
        } else {
            // Rendering failed. Show why instead of a blank pane: a Facet/source
            // file shows its text so the user can see the broken code; a mesh file
            // has no useful source, so show the compile/load error explaining the
            // failure.
            let isMesh = ["stl", "obj", "3mf"].contains(url.pathExtension.lowercased())
            if !isMesh, let source = try? String(contentsOf: url, encoding: .utf8) {
                content = Self.sourceView(source, frame: view.bounds)
            } else {
                let message = FacetMesh.loadError(path: url.path)
                    ?? "Could not load \(url.lastPathComponent)."
                content = Self.sourceView(message, frame: view.bounds)
            }
        }
        content.autoresizingMask = [.width, .height]
        view.subviews.forEach { $0.removeFromSuperview() }
        view.addSubview(content)
        handler(nil)
    }

    // A read-only monospaced text view shown when the model can't be rendered:
    // either the file's source or an error message.
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
