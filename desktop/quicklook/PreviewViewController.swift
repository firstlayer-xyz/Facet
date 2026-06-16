import Cocoa
import Quartz
import SceneKit

// Quick Look interactive preview for .fct/.stl/.obj/.3mf files: renders the loaded
// model in a drag-to-orbit SceneKit view. A .fct whose Main returns an Animation
// plays back — a background producer pulls frames over time and swaps the model's
// geometry on the main thread. Everything else renders as a static turntable. If
// the file fails to produce geometry, it shows why instead: a Facet/source file
// shows its text (so the user can see the broken code), and a mesh file shows the
// compile/load error.
@objc(PreviewViewController)
final class PreviewViewController: NSViewController, QLPreviewingController {

    // Animation playback state, guarded by `lock`. The background producer reads
    // these to know which session to pull and where to write geometry; the main
    // thread tears them down in stopAnimation.
    private let lock = NSLock()
    private var animHandle: Int32 = 0     // 0 = no active session
    private var modelNode: SCNNode?       // node whose geometry the producer swaps
    private var running = false

    override func loadView() {
        view = NSView(frame: NSRect(x: 0, y: 0, width: 720, height: 540))
    }

    func preparePreviewOfFile(at url: URL, completionHandler handler: @escaping (Error?) -> Void) {
        let content: NSView
        if let animated = animatedView(url: url) {
            content = animated
        } else if let scene = FacetMesh.scene(path: url.path, animate: true) {
            content = makeSCNView(scene: scene)
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

    // animatedView opens the file as an Animation session; if Main isn't animated
    // (handle 0) or the first frame fails, it returns nil and the caller falls back
    // to a static render. On success it builds the initial-frame node, frames the
    // scene (no turntable — the geometry itself moves), retains the session, and
    // starts the producer.
    private func animatedView(url: URL) -> SCNView? {
        let handle = url.path.withCString {
            FacetOpenAnimation(UnsafeMutablePointer(mutating: $0))
        }
        guard handle != 0 else { return nil }
        guard let geom = frameGeometry(handle: handle, timeMs: nowMs()) else {
            FacetCloseAnimation(handle)
            return nil
        }
        let node = SCNNode(geometry: geom)
        let scene = FacetMesh.framedScene(modelNode: node, turntable: false)
        let scn = makeSCNView(scene: scene)

        lock.lock()
        animHandle = handle
        modelNode = node
        running = true
        lock.unlock()
        startProducing()
        return scn
    }

    // startProducing runs the animation off the main thread: each iteration renders
    // the current wall-clock frame (heavy cgo), hands the geometry to the main
    // thread to swap onto the model node, then sleeps to cap the rate (~30 fps,
    // slower when a frame takes longer — self-throttling). It exits when running
    // clears.
    private func startProducing() {
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            let targetMs = 1000.0 / 30.0
            while true {
                guard let self = self else { return }
                self.lock.lock()
                let handle = self.animHandle
                let isRunning = self.running
                self.lock.unlock()
                if !isRunning || handle == 0 { return }

                let frameStart = self.nowMs()
                guard let geom = self.frameGeometry(handle: handle, timeMs: frameStart) else {
                    Thread.sleep(forTimeInterval: targetMs / 1000.0)
                    continue
                }
                DispatchQueue.main.async { [weak self] in
                    guard let self = self else { return }
                    self.lock.lock()
                    let node = self.running ? self.modelNode : nil
                    self.lock.unlock()
                    node?.geometry = geom
                }
                let elapsed = self.nowMs() - frameStart
                if elapsed < targetMs {
                    Thread.sleep(forTimeInterval: (targetMs - elapsed) / 1000.0)
                }
            }
        }
    }

    // frameGeometry pulls one animation frame (expanded positions + optional
    // per-vertex color) and builds an SCNGeometry, or nil on a bad handle or frame
    // error. The c-archive owns the buffers until we free them here.
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

    // nowMs is wall-clock epoch milliseconds — the same time base the app feeds
    // Facet's Date.now(), so the preview plays an animation at real speed.
    private func nowMs() -> Double {
        return Date().timeIntervalSince1970 * 1000.0
    }

    // stopAnimation halts the producer and releases the session. Idempotent: a
    // second call (deinit after viewWillDisappear) finds handle 0 and no-ops. Safe
    // against an in-flight producer frame — FacetAnimationFrame on a closed handle
    // just returns nil (the registry lookup misses).
    private func stopAnimation() {
        lock.lock()
        let handle = animHandle
        running = false
        animHandle = 0
        modelNode = nil
        lock.unlock()
        if handle != 0 {
            FacetCloseAnimation(handle)
        }
    }

    override func viewWillDisappear() {
        stopAnimation()
        super.viewWillDisappear()
    }

    deinit {
        stopAnimation()
    }

    // makeSCNView wraps a scene in a drag-to-orbit SCNView. isPlaying advances
    // scene time (the static turntable action) and rendersContinuously forces the
    // redraw loop — a hosted Quick Look SCNView won't start it on its own, so
    // without this the turntable freezes and animation geometry swaps wouldn't draw.
    private func makeSCNView(scene: SCNScene) -> SCNView {
        let scn = SCNView(frame: view.bounds)
        scn.backgroundColor = NSColor(calibratedWhite: 0.12, alpha: 1)
        scn.allowsCameraControl = true
        scn.antialiasingMode = .multisampling4X
        scn.scene = scene
        scn.isPlaying = true
        scn.rendersContinuously = true
        return scn
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
