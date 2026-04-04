import * as THREE from 'three';

/** Decoded mesh ready for Three.js BufferGeometry. */
export interface DecodedMesh {
  vertices: Float32Array;
  indices: Uint32Array;
  faceGroups?: Uint32Array;
  faceColors?: Record<string, string>;
  expanded?: Float32Array;   // pre-expanded non-indexed positions (3 floats * 3 verts * numTri)
  edgeLines?: Float32Array;  // pre-computed edge line segments (6 floats per edge)
}

export interface DebugMeshRef {
  role: string;
  mesh: BinaryMeshMeta | null;
}

export interface DebugStepData {
  op: string;
  meshes: DebugMeshRef[];
  line: number;
  col: number;
  file: string;
}

/** Metadata for a binary mesh section (from eval response JSON header). */
export interface BinaryMeshMeta {
  vertexCount: number;
  indexCount: number;
  faceGroupCount: number;
  faceColors?: Record<string, string>;
  vertices: { offset: number; size: number };
  indices: { offset: number; size: number };
  faceGroups?: { offset: number; size: number };
  // Pre-expanded non-indexed positions (replaces toNonIndexed)
  expanded?: { offset: number; size: number };
  expandedCount?: number;
  // Pre-computed edge line segments (replaces EdgesGeometry)
  edgeLines?: { offset: number; size: number };
  edgeCount?: number;
}

/** Decode a mesh from a binary ArrayBuffer using offset metadata (zero-copy views). */
export function decodeBinaryMesh(binary: ArrayBuffer, meta: BinaryMeshMeta): DecodedMesh {
  const result: DecodedMesh = {
    vertices: new Float32Array(binary, meta.vertices.offset, meta.vertices.size / 4),
    indices: new Uint32Array(binary, meta.indices.offset, meta.indices.size / 4),
  };
  if (meta.faceGroups) {
    result.faceGroups = new Uint32Array(binary, meta.faceGroups.offset, meta.faceGroups.size / 4);
  }
  if (meta.faceColors) {
    result.faceColors = meta.faceColors;
  }
  if (meta.expanded) {
    result.expanded = new Float32Array(binary, meta.expanded.offset, meta.expanded.size / 4);
  }
  if (meta.edgeLines) {
    result.edgeLines = new Float32Array(binary, meta.edgeLines.offset, meta.edgeLines.size / 4);
  }
  return result;
}

/**
 * Build a LineSegments geometry showing only face-group boundary edges.
 * Welds split vertices by position (handles Manifold's normal-split vertices),
 * then emits an edge wherever adjacent triangles belong to different face groups,
 * plus all silhouette/boundary edges (triangles with only one neighbor).
 */
export function buildFaceGroupWireframe(
  vertices: Float32Array,
  indices: Uint32Array,
  faceGroups: Uint32Array,
): THREE.BufferGeometry {
  const numTris = faceGroups.length;

  // Weld split vertices by quantised position → canonical vertex index.
  const posMap = new Map<string, number>();
  const canon = (vi: number): number => {
    const x = Math.round(vertices[vi * 3]     * 1e4);
    const y = Math.round(vertices[vi * 3 + 1] * 1e4);
    const z = Math.round(vertices[vi * 3 + 2] * 1e4);
    const k = `${x}:${y}:${z}`;
    if (!posMap.has(k)) posMap.set(k, vi);
    return posMap.get(k)!;
  };

  // edge key → [tri0, tri1]  (-1 means no second triangle yet)
  const edgeMap = new Map<string, [number, number]>();
  for (let t = 0; t < numTris; t++) {
    const a = canon(indices[t * 3]), b = canon(indices[t * 3 + 1]), c = canon(indices[t * 3 + 2]);
    for (const [va, vb] of [[a, b], [b, c], [c, a]] as [number, number][]) {
      const lo = va < vb ? va : vb, hi = va < vb ? vb : va;
      const key = `${lo}:${hi}`;
      const e = edgeMap.get(key);
      if (!e) edgeMap.set(key, [t, -1]);
      else if (e[1] === -1) e[1] = t;
    }
  }

  const lineVerts: number[] = [];
  for (const [key, [t0, t1]] of edgeMap) {
    if (t1 !== -1 && faceGroups[t0] === faceGroups[t1]) continue; // interior, same group
    const [loStr, hiStr] = key.split(':');
    const lo = parseInt(loStr), hi = parseInt(hiStr);
    lineVerts.push(
      vertices[lo * 3], vertices[lo * 3 + 1], vertices[lo * 3 + 2],
      vertices[hi * 3], vertices[hi * 3 + 1], vertices[hi * 3 + 2],
    );
  }

  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.BufferAttribute(new Float32Array(lineVerts), 3));
  return geo;
}
