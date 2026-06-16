import Cocoa
import SceneKit
import simd

// Shared geometry/scene helpers used by both the preview and thumbnail
// extensions. Loads a Facet source or a mesh file via the embedded evaluator+kernel
// (FacetRenderFile, from the c-archive built by cmd/facetrender) and assembles a
// framed SceneKit scene.
enum FacetMesh {

    // geometry builds an SCNGeometry from an expanded-triangle position buffer (9
    // floats per triangle) with per-face flat normals and, when colorPtr is non-nil
    // and sized to match (colorBytes == count), a per-vertex color source. Returns
    // nil when the buffer is empty or malformed. Shared by the static buildModel
    // path and the animation preview's per-frame geometry swap.
    static func geometry(positions: UnsafeMutablePointer<Float>, count: Int,
                         colors colorPtr: UnsafeMutablePointer<UInt8>?, colorBytes: Int) -> SCNGeometry? {
        guard count >= 9, count % 9 == 0 else { return nil }
        let hasColor = colorPtr != nil && colorBytes == count

        var verts = [SCNVector3](); verts.reserveCapacity(count / 3)
        var norms = [SCNVector3](); norms.reserveCapacity(count / 3)
        // Resolve the per-vertex color buffer once: non-nil only when present and
        // correctly sized (hasColor). It does not change across the loop.
        let colors: UnsafeMutablePointer<UInt8>? = hasColor ? colorPtr : nil
        var cols = [SCNVector3](); if colors != nil { cols.reserveCapacity(count / 3) }
        var i = 0
        while i < count {
            let a = simd_float3(positions[i + 0], positions[i + 1], positions[i + 2])
            let b = simd_float3(positions[i + 3], positions[i + 4], positions[i + 5])
            let c = simd_float3(positions[i + 6], positions[i + 7], positions[i + 8])
            let n = simd_normalize(simd_cross(b - a, c - a)) // CCW-from-outside → outward
            verts.append(SCNVector3(a.x, a.y, a.z))
            verts.append(SCNVector3(b.x, b.y, b.z))
            verts.append(SCNVector3(c.x, c.y, c.z))
            for _ in 0..<3 { norms.append(SCNVector3(n.x, n.y, n.z)) }
            if let cp = colors {
                let v = i / 3 // expanded-vertex index of vert a
                for k in 0..<3 {
                    let o = (v + k) * 3
                    cols.append(SCNVector3(CGFloat(cp[o]) / 255.0,
                                           CGFloat(cp[o + 1]) / 255.0,
                                           CGFloat(cp[o + 2]) / 255.0))
                }
            }
            i += 9
        }

        let vSource = SCNGeometrySource(vertices: verts)
        let nSource = SCNGeometrySource(normals: norms)
        var sources = [vSource, nSource]
        if hasColor {
            sources.append(colorSource(cols))
        }
        let indices = (0..<verts.count).map { UInt32($0) }
        let element = SCNGeometryElement(indices: indices, primitiveType: .triangles)
        let geom = SCNGeometry(sources: sources, elements: [element])

        let mat = SCNMaterial()
        mat.lightingModel = .physicallyBased
        // White diffuse so per-vertex colors show through; gray when uncolored.
        mat.diffuse.contents = hasColor
            ? NSColor.white
            : NSColor(calibratedRed: 0.80, green: 0.82, blue: 0.86, alpha: 1)
        mat.metalness.contents = 0.15
        mat.roughness.contents = 0.55
        mat.isDoubleSided = true
        geom.materials = [mat]
        return geom
    }

    // Load a file (Facet source or .stl/.obj/.3mf mesh) → mesh → an SCNNode with
    // per-face (flat) normals and per-vertex color when present, or nil when the
    // file fails to load/evaluate or produces no geometry.
    static func buildModel(path: String) -> SCNNode? {
        var nFloats: Int32 = 0
        var colorPtr: UnsafeMutablePointer<UInt8>? = nil
        var nColorBytes: Int32 = 0
        let buf: UnsafeMutablePointer<Float>? = path.withCString {
            FacetRenderFile(UnsafeMutablePointer(mutating: $0), &nFloats, &colorPtr, &nColorBytes)
        }
        guard let positions = buf else { return nil }
        defer {
            FacetFree(positions)
            if let c = colorPtr { FacetFreeBytes(c) }
        }
        guard let geom = geometry(positions: positions, count: Int(nFloats),
                                  colors: colorPtr, colorBytes: Int(nColorBytes)) else { return nil }
        return SCNNode(geometry: geom)
    }

    // loadError returns the compile/load error explaining why a file produced no
    // geometry (the failure buildModel/scene return nil for), or nil if it loads.
    // The preview uses it to show the reason instead of a blank pane.
    static func loadError(path: String) -> String? {
        guard let cstr = path.withCString({
            FacetRenderError(UnsafeMutablePointer(mutating: $0))
        }) else { return nil }
        defer { FacetFreeString(cstr) }
        return String(cString: cstr)
    }

    // colorSource builds a per-vertex .color geometry source from float RGB.
    private static func colorSource(_ cols: [SCNVector3]) -> SCNGeometrySource {
        let stride = MemoryLayout<SCNVector3>.stride
        let data = cols.withUnsafeBytes { Data($0) }
        return SCNGeometrySource(
            data: data,
            semantic: .color,
            vectorCount: cols.count,
            usesFloatComponents: true,
            componentsPerVector: 3,
            bytesPerComponent: MemoryLayout<CGFloat>.size,
            dataOffset: 0,
            dataStride: stride)
    }

    // framedScene wraps an already-built model node in a centered, 3/4-camera,
    // lit scene (Z-up corrected). When turntable is true a slow Y rotation is
    // added (interactive preview); thumbnails pass false for a fixed pose. The
    // animation preview keeps the modelNode and swaps its geometry per frame, so
    // framing is computed once from the initial frame's bounds. Shared by scene()
    // and the animation preview.
    static func framedScene(modelNode model: SCNNode, turntable animate: Bool) -> SCNScene {
        let scene = SCNScene()

        // Center the model at the origin.
        let (minB, maxB) = model.boundingBox
        model.position = SCNVector3(-(minB.x + maxB.x) / 2,
                                    -(minB.y + maxB.y) / 2,
                                    -(minB.z + maxB.z) / 2)

        // Facet geometry is Z-up; SceneKit is Y-up. Tip the centered model -90°
        // about X so Z points up (matching the in-app viewport and the software
        // renderer). The turntable then spins this node about Y (= Facet Z).
        let upright = SCNNode()
        upright.eulerAngles = SCNVector3(CGFloat(-Double.pi / 2), 0, 0)
        upright.addChildNode(model)
        let turntable = SCNNode()
        turntable.addChildNode(upright)
        scene.rootNode.addChildNode(turntable)

        // Frame from the bounding sphere (rotation-invariant, so the model never
        // clips as it spins) with a narrow FOV for a consistent ~85% fill.
        let dx = Double(maxB.x - minB.x)
        let dy = Double(maxB.y - minB.y)
        let dz = Double(maxB.z - minB.z)
        let radius = max(0.5 * sqrt(dx * dx + dy * dy + dz * dz), 0.001)
        let fov = 35.0
        let dist = radius / tan(fov * Double.pi / 360.0) * 1.15
        let inv = 1.0 / sqrt(1.0 + 0.7 * 0.7 + 1.0) // normalize the (1, 0.7, 1) 3/4 direction

        let cam = SCNNode()
        cam.camera = SCNCamera()
        cam.camera?.fieldOfView = CGFloat(fov)
        cam.camera?.zNear = max(0.01, dist - radius * 2)
        cam.camera?.zFar = dist + radius * 4 + 10
        cam.position = SCNVector3(CGFloat(dist * inv), CGFloat(dist * 0.7 * inv), CGFloat(dist * inv))
        cam.constraints = [SCNLookAtConstraint(target: turntable)]
        scene.rootNode.addChildNode(cam)

        let key = SCNNode()
        key.light = SCNLight()
        key.light?.type = .directional
        key.light?.intensity = 850
        key.position = SCNVector3(CGFloat(dist), CGFloat(dist * 2), CGFloat(dist * 1.5))
        key.constraints = [SCNLookAtConstraint(target: turntable)]
        scene.rootNode.addChildNode(key)

        let ambient = SCNNode()
        ambient.light = SCNLight()
        ambient.light?.type = .ambient
        ambient.light?.intensity = 350
        scene.rootNode.addChildNode(ambient)

        if animate {
            turntable.runAction(.repeatForever(.rotateBy(x: 0, y: .pi * 2, z: 0, duration: 16)))
        }
        return scene
    }

    // Build a framed scene from a file path: centered model, 3/4 camera, lights. When
    // animate is true a slow turntable is added (for the interactive preview);
    // thumbnails pass false for a fixed pose. Returns nil if the file fails to load.
    static func scene(path: String, animate: Bool) -> SCNScene? {
        guard let model = buildModel(path: path) else { return nil }
        return framedScene(modelNode: model, turntable: animate)
    }
}
