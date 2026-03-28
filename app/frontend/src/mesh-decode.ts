import * as THREE from 'three';

/** Wire format from Go MarshalJSON: base64-encoded binary arrays. */
export interface MeshData {
  vertices: string;    // base64-encoded float32 LE
  indices: string;     // base64-encoded uint32 LE
  faceGroups?: string; // base64-encoded uint32 LE (per-triangle face group IDs)
  faceColors?: Record<string, string>; // faceGroupID → hex color (e.g. {"5": "#FF0000"})
  vertexCount: number;
  indexCount: number;
}

/** Decoded mesh ready for Three.js BufferGeometry. */
export interface DecodedMesh {
  vertices: Float32Array;
  indices: Uint32Array;
  faceGroups?: Uint32Array;
  faceColors?: Record<string, string>;
}

interface DebugMeshData {
  role: string;
  mesh: MeshData;
}

export interface DebugStepData {
  op: string;
  meshes: DebugMeshData[];
  line: number;
  col: number;
  file: string;
}

function decodeFloat32(b64: string): Float32Array {
  if (!b64) return new Float32Array(0);
  const bin = atob(b64);
  const bytes = Uint8Array.from(bin, c => c.charCodeAt(0));
  return new Float32Array(bytes.buffer);
}

function decodeUint32(b64: string): Uint32Array {
  if (!b64) return new Uint32Array(0);
  const bin = atob(b64);
  const bytes = Uint8Array.from(bin, c => c.charCodeAt(0));
  return new Uint32Array(bytes.buffer);
}

export function decodeMesh(data: MeshData): DecodedMesh {
  const result: DecodedMesh = {
    vertices: decodeFloat32(data.vertices),
    indices: decodeUint32(data.indices),
  };
  if (data.faceGroups) {
    result.faceGroups = decodeUint32(data.faceGroups);
  }
  if (data.faceColors) {
    result.faceColors = data.faceColors;
  }
  return result;
}

/** Merge multiple MeshData into one DecodedMesh (client-side equivalent of MergeMeshes). */
export function mergeMeshes(meshes: MeshData[]): DecodedMesh {
  if (meshes.length === 1) return decodeMesh(meshes[0]);
  let totalVerts = 0, totalIdx = 0, totalTris = 0;
  const decoded = meshes.map(m => {
    const d = decodeMesh(m);
    totalVerts += d.vertices.length;
    totalIdx += d.indices.length;
    totalTris += d.indices.length / 3;
    return d;
  });
  const hasFG = decoded.some(d => d.faceGroups !== undefined);
  const vertices = new Float32Array(totalVerts);
  const indices = new Uint32Array(totalIdx);
  const faceGroups = hasFG ? new Uint32Array(totalTris) : undefined;
  let vi = 0, ii = 0, ti = 0, vertOffset = 0, fgOffset = 0;
  for (const d of decoded) {
    vertices.set(d.vertices, vi);
    vi += d.vertices.length;
    for (let i = 0; i < d.indices.length; i++) indices[ii++] = d.indices[i] + vertOffset;
    if (faceGroups) {
      const nTris = d.indices.length / 3;
      if (d.faceGroups) {
        for (let i = 0; i < nTris; i++) faceGroups[ti++] = d.faceGroups[i] + fgOffset;
        fgOffset += d.faceGroups.reduce((a, b) => Math.max(a, b), 0) + 1;
      } else {
        for (let i = 0; i < nTris; i++) faceGroups[ti++] = fgOffset;
        fgOffset++;
      }
    }
    vertOffset += d.vertices.length / 3;
  }
  const result: DecodedMesh = { vertices, indices };
  if (faceGroups) result.faceGroups = faceGroups;

  // Merge faceColors with offset keys
  const hasFC = decoded.some(d => d.faceColors !== undefined);
  if (hasFC) {
    const mergedColors: Record<string, string> = {};
    let fcOffset = 0;
    for (const d of decoded) {
      if (d.faceColors) {
        for (const [k, v] of Object.entries(d.faceColors)) {
          mergedColors[String(Number(k) + fcOffset)] = v;
        }
      }
      if (d.faceGroups) {
        fcOffset += d.faceGroups.reduce((a, b) => Math.max(a, b), 0) + 1;
      } else {
        fcOffset++;
      }
    }
    result.faceColors = mergedColors;
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
