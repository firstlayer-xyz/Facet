import Cocoa
import Quartz
import SceneKit

// Quick Look interactive preview for .fct/.stl/.obj/.3mf files: renders the loaded
// model in a drag-to-orbit SceneKit view. If the file fails to produce geometry,
// it falls back to showing the file's text (useful for .fct parse/type errors;
// binary mesh files read as "" which leaves a blank preview).
@objc(PreviewViewController)
final class PreviewViewController: NSViewController, QLPreviewingController {

    private let producer = DispatchQueue(label: "xyz.firstlayer.facet.ql.anim")
    private let stateLock = NSLock()
    private var running = false        // guarded by stateLock
    private var animHandle: Int32 = 0  // guarded by stateLock
    private var modelNode: SCNNode?    // main-thread only

    override func loadView() {
        view = NSView(frame: NSRect(x: 0, y: 0, width: 720, height: 540))
    }

    func preparePreviewOfFile(at url: URL, completionHandler handler: @escaping (Error?) -> Void) {
        let content: NSView
        if let scn = animatedView(url: url) ?? staticView(url: url) {
            content = scn
        } else {
            // Rendering failed — for a Facet/text file show its source. A binary
            // mesh reads as "" here, leaving a blank preview.
            let source = (try? String(contentsOf: url, encoding: .utf8)) ?? ""
            content = Self.sourceView(source, frame: view.bounds)
        }
        content.autoresizingMask = [.width, .height]
        view.subviews.forEach { $0.removeFromSuperview() }
        view.addSubview(content)
        handler(nil)
    }

    // animatedView returns a playing SCNView for an animated .fct, or nil when the
    // file is not an animation (caller falls back to the static view).
    private func animatedView(url: URL) -> SCNView? {
        let handle = url.path.withCString { FacetOpenAnimation(UnsafeMutablePointer(mutating: $0)) }
        guard handle != 0 else { return nil }
        guard let first = frameGeometry(handle: handle, timeMs: Self.nowMs()) else {
            FacetCloseAnimation(handle)
            return nil
        }
        let node = SCNNode(geometry: first)
        let scn = makeSCNView()
        // The model's own motion is the animation, so no turntable.
        scn.scene = FacetMesh.framedScene(modelNode: node, turntable: false)
        modelNode = node
        stateLock.lock()
        animHandle = handle
        running = true
        stateLock.unlock()
        startProducing()
        return scn
    }

    private func staticView(url: URL) -> SCNView? {
        guard let scene = FacetMesh.scene(path: url.path, animate: true) else { return nil }
        let scn = makeSCNView()
        scn.scene = scene
        return scn
    }

    private func makeSCNView() -> SCNView {
        let scn = SCNView(frame: view.bounds)
        scn.backgroundColor = NSColor(calibratedWhite: 0.12, alpha: 1)
        scn.allowsCameraControl = true
        scn.antialiasingMode = .multisampling4X
        scn.isPlaying = true
        scn.rendersContinuously = true
        return scn
    }

    // startProducing renders frames on a background queue and swaps the model's
    // geometry on the main thread, requesting the next frame only after the last
    // completes (self-throttling — a slow model plays at a lower fps).
    private func startProducing() {
        producer.async { [weak self] in
            while true {
                guard let self = self else { return }
                self.stateLock.lock()
                let go = self.running
                let h = self.animHandle
                self.stateLock.unlock()
                if !go { return }
                if let g = self.frameGeometry(handle: h, timeMs: Self.nowMs()) {
                    // Re-weaken so the hop to main can't keep self alive past teardown.
                    DispatchQueue.main.async { [weak self] in self?.modelNode?.geometry = g }
                }
                Thread.sleep(forTimeInterval: 1.0 / 30.0) // cap ~30fps; eval cost may make it slower
            }
        }
    }

    private func frameGeometry(handle: Int32, timeMs: Double) -> SCNGeometry? {
        var nFloats: Int32 = 0
        var colorPtr: UnsafeMutablePointer<UInt8>? = nil
        var nColorBytes: Int32 = 0
        let buf = FacetAnimationFrame(handle, timeMs, &nFloats, &colorPtr, &nColorBytes)
        guard let positions = buf else { return nil }
        defer {
            FacetFree(positions)
            if let c = colorPtr { FacetFreeBytes(c) }
        }
        return FacetMesh.geometry(positions: positions, count: Int(nFloats),
                                  colors: colorPtr, colorBytes: Int(nColorBytes))
    }

    private static func nowMs() -> Double { Date().timeIntervalSince1970 * 1000.0 }

    override func viewWillDisappear() {
        super.viewWillDisappear()
        stopAnimation()
    }

    deinit { stopAnimation() }

    // stopAnimation halts the producer and releases the session. Relies on the OS
    // firing viewWillDisappear/deinit on preview dismissal; if the extension
    // process is killed outright, its Go handle map dies with the process — nothing
    // leaks across processes.
    private func stopAnimation() {
        stateLock.lock()
        let h = animHandle
        running = false
        animHandle = 0
        stateLock.unlock()
        // Close outside the lock — the Go side is safe against an in-flight frame.
        if h != 0 { FacetCloseAnimation(h) }
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
