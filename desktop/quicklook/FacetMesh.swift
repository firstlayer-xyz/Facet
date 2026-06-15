import Cocoa
import SceneKit
import simd

// Shared geometry/scene helpers used by both the preview and thumbnail
// extensions. Evaluates Facet source to a mesh via the embedded evaluator+kernel
// (FacetRenderMesh, from the c-archive built by cmd/facetrender) and assembles a
// framed SceneKit scene.
enum FacetMesh {

    // Evaluate source → mesh → an SCNNode with per-face (flat) normals, or nil
    // when the source fails to load/check/evaluate or produces no geometry.
    static func buildModel(source: String) -> SCNNode? {
        var nFloats: Int32 = 0
        let buf: UnsafeMutablePointer<Float>? = source.withCString {
            FacetRenderMesh(UnsafeMutablePointer(mutating: $0), &nFloats)
        }
        guard let positions = buf else { return nil }
        defer { FacetFree(positions) }
        let count = Int(nFloats)
        guard count >= 9, count % 9 == 0 else { return nil }

        var verts = [SCNVector3](); verts.reserveCapacity(count / 3)
        var norms = [SCNVector3](); norms.reserveCapacity(count / 3)
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
            i += 9
        }

        let vSource = SCNGeometrySource(vertices: verts)
        let nSource = SCNGeometrySource(normals: norms)
        let indices = (0..<verts.count).map { UInt32($0) }
        let element = SCNGeometryElement(indices: indices, primitiveType: .triangles)
        let geom = SCNGeometry(sources: [vSource, nSource], elements: [element])

        let mat = SCNMaterial()
        mat.lightingModel = .physicallyBased
        mat.diffuse.contents = NSColor(calibratedRed: 0.80, green: 0.82, blue: 0.86, alpha: 1)
        mat.metalness.contents = 0.15
        mat.roughness.contents = 0.55
        mat.isDoubleSided = true
        geom.materials = [mat]
        return SCNNode(geometry: geom)
    }

    // Build a framed scene from source: centered model, 3/4 camera, lights. When
    // animate is true a slow turntable is added (for the interactive preview);
    // thumbnails pass false for a fixed pose. Returns nil if the source fails.
    static func scene(source: String, animate: Bool) -> SCNScene? {
        guard let model = buildModel(source: source) else { return nil }
        let scene = SCNScene()
        let (minB, maxB) = model.boundingBox
        model.position = SCNVector3(-(minB.x + maxB.x) / 2,
                                    -(minB.y + maxB.y) / 2,
                                    -(minB.z + maxB.z) / 2)
        let turntable = SCNNode()
        turntable.addChildNode(model)
        scene.rootNode.addChildNode(turntable)

        let size = max(maxB.x - minB.x, max(maxB.y - minB.y, maxB.z - minB.z))
        let d = size * 2.0

        let cam = SCNNode()
        cam.camera = SCNCamera()
        cam.camera?.zNear = 0.01
        cam.camera?.zFar = Double(size) * 40 + 100
        cam.position = SCNVector3(d, d * 0.7, d)
        cam.constraints = [SCNLookAtConstraint(target: turntable)]
        scene.rootNode.addChildNode(cam)

        let key = SCNNode()
        key.light = SCNLight()
        key.light?.type = .directional
        key.light?.intensity = 850
        key.position = SCNVector3(d, d * 2, d * 1.5)
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
}
