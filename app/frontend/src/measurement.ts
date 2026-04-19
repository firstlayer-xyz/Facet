// measurement.ts — pure logic for viewer dimensioning.
//
// Deliberately free of Three.js Object3D / scene dependencies. Consumes
// DecodedMesh plus a raycaster hit and emits Snap / Measurement values.
// Anything that constructs scene objects lives in measurement_render.ts.

import * as THREE from 'three';

export type Vec3 = [number, number, number];

export type SnapKind = 'vertex' | 'edgeMid' | 'edgeEnd' | 'edgePerp' | 'faceCentroid' | 'circleCenter';

export interface Snap {
  kind: SnapKind;
  point: Vec3;
  faceID?: number;   // faceCentroid
  normal?: Vec3;     // faceCentroid (unit)
  radius?: number;   // circleCenter
  axis?: Vec3;       // circleCenter (circle plane normal, unit)
  /** For edgeEnd / edgeMid / edgePerp: the two endpoints of the source edge, in
   *  the same coordinate space as `point`. Drives the on-hover edge highlight
   *  and, for edge-to-edge angular measurements, the edge tangent. */
  edge?: { a: Vec3; b: Vec3 };
  /** For circleCenter: welded points of the snapped loop, same space as `point`. */
  loop?: Vec3[];
}

export type Measurement =
  | { kind: 'linear';      a: Snap; b: Snap; distance: number }
  | { kind: 'faceToFace';  a: Snap; b: Snap; distance: number }
  | { kind: 'radial';      c: Snap; diameter: number; radius: number }
  | { kind: 'angular';     a: Snap; b: Snap; degrees: number }
  /** Interior corner angle at a chain vertex — two consecutive dim segments
   *  meeting at `vertex`. prevDir/nextDir are unit vectors from `vertex`
   *  toward the neighbouring chain points. */
  | { kind: 'cornerAngle'; vertex: Snap; prevDir: Vec3; nextDir: Vec3; degrees: number }
  | { kind: 'extents';     min: Vec3; max: Vec3 };

export interface FacePlane {
  centroid: Vec3;
  normal: Vec3;      // unit, area-weighted average of triangle normals
  coplanar: boolean; // every triangle within ~1° of the mean normal
}

export interface CircularEdge {
  center: Vec3;
  radius: number;
  axis: Vec3;        // unit plane normal
  loopPoints: Vec3[]; // welded vertices making up the closed loop
}

// ---------------------------------------------------------------------------
// Vector helpers (kept tiny — avoid pulling in a whole vec lib)
// ---------------------------------------------------------------------------

const sub = (a: Vec3, b: Vec3): Vec3 => [a[0] - b[0], a[1] - b[1], a[2] - b[2]];
const add = (a: Vec3, b: Vec3): Vec3 => [a[0] + b[0], a[1] + b[1], a[2] + b[2]];
const scale = (v: Vec3, s: number): Vec3 => [v[0] * s, v[1] * s, v[2] * s];
const dot = (a: Vec3, b: Vec3): number => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
const cross = (a: Vec3, b: Vec3): Vec3 => [
  a[1] * b[2] - a[2] * b[1],
  a[2] * b[0] - a[0] * b[2],
  a[0] * b[1] - a[1] * b[0],
];
const length = (v: Vec3): number => Math.hypot(v[0], v[1], v[2]);
const normalize = (v: Vec3): Vec3 => {
  const l = length(v);
  return l > 0 ? [v[0] / l, v[1] / l, v[2] / l] : [0, 0, 0];
};
const midpoint = (a: Vec3, b: Vec3): Vec3 => [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (a[2] + b[2]) / 2];

// ---------------------------------------------------------------------------
// Weld-by-quantised-position — shared by facePlanes + circularEdges detection.
// Mirrors the scheme in buildFaceGroupWireframe so indexing is consistent.
// ---------------------------------------------------------------------------

function weldKey(x: number, y: number, z: number): string {
  return `${Math.round(x * 1e4)}:${Math.round(y * 1e4)}:${Math.round(z * 1e4)}`;
}

/** Build vi → canonical vi map, welding split vertices at the same position. */
function buildWeld(vertices: Float32Array): Uint32Array {
  const canon = new Uint32Array(vertices.length / 3);
  const map = new Map<string, number>();
  for (let i = 0; i < canon.length; i++) {
    const k = weldKey(vertices[i * 3], vertices[i * 3 + 1], vertices[i * 3 + 2]);
    const existing = map.get(k);
    if (existing === undefined) {
      map.set(k, i);
      canon[i] = i;
    } else {
      canon[i] = existing;
    }
  }
  return canon;
}

// ---------------------------------------------------------------------------
// Face planes — per face group, area-weighted centroid + normal + coplanar flag
// ---------------------------------------------------------------------------

export function computeFacePlanes(
  vertices: Float32Array,
  indices: Uint32Array,
  faceGroups: Uint32Array,
): Map<number, FacePlane> {
  // Accumulators per face group.
  interface Acc {
    nx: number; ny: number; nz: number;  // area-weighted normal sum (unnormalized → magnitude is 2× area sum)
    cx: number; cy: number; cz: number;  // area-weighted centroid sum
    areaSum: number;
    // For coplanar check: store per-triangle unit normals, then compare to final mean.
    triNormals: Vec3[];
  }
  const accs = new Map<number, Acc>();
  const numTris = faceGroups.length;
  for (let t = 0; t < numTris; t++) {
    const ia = indices[t * 3], ib = indices[t * 3 + 1], ic = indices[t * 3 + 2];
    const ax = vertices[ia * 3], ay = vertices[ia * 3 + 1], az = vertices[ia * 3 + 2];
    const bx = vertices[ib * 3], by = vertices[ib * 3 + 1], bz = vertices[ib * 3 + 2];
    const cx = vertices[ic * 3], cy = vertices[ic * 3 + 1], cz = vertices[ic * 3 + 2];
    const ux = bx - ax, uy = by - ay, uz = bz - az;
    const vx = cx - ax, vy = cy - ay, vz = cz - az;
    const nx = uy * vz - uz * vy;
    const ny = uz * vx - ux * vz;
    const nz = ux * vy - uy * vx;
    const area2 = Math.hypot(nx, ny, nz); // 2 × triangle area
    if (area2 === 0) continue; // degenerate
    const invA2 = 1 / area2;
    const triUnit: Vec3 = [nx * invA2, ny * invA2, nz * invA2];
    const centX = (ax + bx + cx) / 3;
    const centY = (ay + by + cy) / 3;
    const centZ = (az + bz + cz) / 3;

    const fg = faceGroups[t];
    let acc = accs.get(fg);
    if (!acc) {
      acc = { nx: 0, ny: 0, nz: 0, cx: 0, cy: 0, cz: 0, areaSum: 0, triNormals: [] };
      accs.set(fg, acc);
    }
    // Area-weighted: multiply by area (= area2 / 2), so use area2 directly and
    // divide at the end (constant factor cancels in normalize).
    acc.nx += nx;
    acc.ny += ny;
    acc.nz += nz;
    acc.cx += centX * area2;
    acc.cy += centY * area2;
    acc.cz += centZ * area2;
    acc.areaSum += area2;
    acc.triNormals.push(triUnit);
  }

  const result = new Map<number, FacePlane>();
  const COPLANAR_COS = Math.cos(1 * Math.PI / 180); // within 1° ⇒ coplanar
  for (const [fg, acc] of accs) {
    if (acc.areaSum === 0) continue;
    const normal = normalize([acc.nx, acc.ny, acc.nz]);
    const centroid: Vec3 = [acc.cx / acc.areaSum, acc.cy / acc.areaSum, acc.cz / acc.areaSum];
    let coplanar = true;
    for (const tn of acc.triNormals) {
      if (Math.abs(dot(tn, normal)) < COPLANAR_COS) { coplanar = false; break; }
    }
    result.set(fg, { centroid, normal, coplanar });
  }
  return result;
}

// ---------------------------------------------------------------------------
// Circular edges — walk edgeLines into loops, fit circles
// ---------------------------------------------------------------------------

/**
 * Fit a plane + circle to a set of coplanar-ish points in 3D.
 * Returns null if the points are degenerate (collinear or noisy beyond tolerance).
 *
 * Approach:
 *   1. Find plane normal = normalized cross of two spread vectors from centroid.
 *   2. Project all points into the plane's 2D coords.
 *   3. Algebraic least-squares circle fit (Kåsa): solve for (a, b, c) in
 *      x² + y² = a·x + b·y + c, then center = (a/2, b/2), r = sqrt(c + a²/4 + b²/4).
 *   4. Verify residual ≤ tolerance · r.
 */
export function fitCircle(points: Vec3[], residualTolFrac = 0.01): { center: Vec3; radius: number; axis: Vec3 } | null {
  if (points.length < 3) return null;

  // Centroid.
  const N = points.length;
  let cx = 0, cy = 0, cz = 0;
  for (const p of points) { cx += p[0]; cy += p[1]; cz += p[2]; }
  const centroid: Vec3 = [cx / N, cy / N, cz / N];

  // Plane normal from two spread vectors. Pick points with largest cross product.
  let bestNormal: Vec3 = [0, 0, 0];
  let bestMag = 0;
  const v0 = sub(points[0], centroid);
  for (let i = 1; i < N; i++) {
    const vi = sub(points[i], centroid);
    const c = cross(v0, vi);
    const m = length(c);
    if (m > bestMag) { bestMag = m; bestNormal = c; }
  }
  if (bestMag < 1e-9) return null; // collinear
  const axis = normalize(bestNormal);

  // Two in-plane basis vectors.
  const u = normalize(sub(points[1], centroid));
  const v = cross(axis, u); // already unit since axis⊥u by construction

  // Project points to 2D.
  const xs: number[] = new Array(N);
  const ys: number[] = new Array(N);
  for (let i = 0; i < N; i++) {
    const d = sub(points[i], centroid);
    xs[i] = dot(d, u);
    ys[i] = dot(d, v);
  }

  // Algebraic LS: minimize Σ(x²+y² - a·x - b·y - c)²
  // Normal equations: [Σx² Σxy Σx; Σxy Σy² Σy; Σx Σy N][a;b;c] = [Σx(x²+y²); Σy(x²+y²); Σ(x²+y²)]
  let Sx = 0, Sy = 0, Sxx = 0, Syy = 0, Sxy = 0;
  let Sxr = 0, Syr = 0, Sr = 0;
  for (let i = 0; i < N; i++) {
    const x = xs[i], y = ys[i], r2 = x * x + y * y;
    Sx += x; Sy += y; Sxx += x * x; Syy += y * y; Sxy += x * y;
    Sxr += x * r2; Syr += y * r2; Sr += r2;
  }
  // Solve 3x3 via Cramer.
  const m = [
    [Sxx, Sxy, Sx],
    [Sxy, Syy, Sy],
    [Sx,  Sy,  N ],
  ];
  const rhs = [Sxr, Syr, Sr];
  const det = (M: number[][]) =>
    M[0][0] * (M[1][1] * M[2][2] - M[1][2] * M[2][1]) -
    M[0][1] * (M[1][0] * M[2][2] - M[1][2] * M[2][0]) +
    M[0][2] * (M[1][0] * M[2][1] - M[1][1] * M[2][0]);
  const D = det(m);
  if (Math.abs(D) < 1e-12) return null;
  const col = (M: number[][], ci: number, r: number[]) => M.map((row, i) => row.map((val, j) => (j === ci ? r[i] : val)));
  const a = det(col(m, 0, rhs)) / D;
  const b = det(col(m, 1, rhs)) / D;
  const c = det(col(m, 2, rhs)) / D;
  const cx2 = a / 2, cy2 = b / 2;
  const r2val = c + cx2 * cx2 + cy2 * cy2;
  if (r2val <= 0) return null;
  const radius = Math.sqrt(r2val);

  // Lift center back to 3D.
  const center: Vec3 = add(add(centroid, scale(u, cx2)), scale(v, cy2));

  // Residual check.
  let maxRes = 0;
  for (let i = 0; i < N; i++) {
    const dx = xs[i] - cx2, dy = ys[i] - cy2;
    const res = Math.abs(Math.hypot(dx, dy) - radius);
    if (res > maxRes) maxRes = res;
  }
  if (maxRes > residualTolFrac * radius) return null;

  return { center, radius, axis };
}

/**
 * Detect circular edge loops in the face-group boundary edges.
 *
 * edgeLines is flat Float32Array of vertex pairs (6 floats per edge: 3 per endpoint).
 * We weld by quantised position, build adjacency, walk closed loops, and fit circles.
 */
export function detectCircularEdges(edgeLines: Float32Array): CircularEdge[] {
  if (edgeLines.length === 0) return [];

  // Dedupe vertices by weld key.
  const vertMap = new Map<string, number>();
  const verts: Vec3[] = [];
  const getIdx = (x: number, y: number, z: number): number => {
    const k = weldKey(x, y, z);
    let idx = vertMap.get(k);
    if (idx === undefined) {
      idx = verts.length;
      vertMap.set(k, idx);
      verts.push([x, y, z]);
    }
    return idx;
  };

  // Build adjacency: vertex → [neighbors].
  const adj = new Map<number, number[]>();
  const edgeCount = edgeLines.length / 6;
  for (let e = 0; e < edgeCount; e++) {
    const ax = edgeLines[e * 6], ay = edgeLines[e * 6 + 1], az = edgeLines[e * 6 + 2];
    const bx = edgeLines[e * 6 + 3], by = edgeLines[e * 6 + 4], bz = edgeLines[e * 6 + 5];
    const ia = getIdx(ax, ay, az);
    const ib = getIdx(bx, by, bz);
    if (ia === ib) continue;
    (adj.get(ia) ?? adj.set(ia, []).get(ia)!).push(ib);
    (adj.get(ib) ?? adj.set(ib, []).get(ib)!).push(ia);
  }

  // Walk loops: start at any vertex whose degree is exactly 2, follow until we
  // return. Mark visited. Vertices with degree != 2 are branch points (corners
  // between face groups) — skip those as loop starts.
  const visited = new Set<number>();
  const loops: number[][] = [];
  for (const [start, neighbors] of adj) {
    if (visited.has(start)) continue;
    if (neighbors.length !== 2) continue;
    const loop: number[] = [];
    let prev = -1;
    let cur = start;
    while (!visited.has(cur)) {
      visited.add(cur);
      loop.push(cur);
      const ns = adj.get(cur)!;
      if (ns.length !== 2) { // hit a branch — not a clean closed loop
        loop.length = 0;
        break;
      }
      const next = ns[0] === prev ? ns[1] : ns[0];
      prev = cur;
      cur = next;
      if (cur === start) break;
    }
    if (loop.length >= 6 && cur === start) {
      loops.push(loop);
    }
  }

  const result: CircularEdge[] = [];
  for (const loop of loops) {
    const pts = loop.map(i => verts[i]);
    const fit = fitCircle(pts);
    if (!fit) continue;
    result.push({ ...fit, loopPoints: pts });
  }
  return result;
}

// ---------------------------------------------------------------------------
// Snap resolver
// ---------------------------------------------------------------------------

/**
 * Precomputed per-mesh data for fast snapping.
 * Attached to DecodedMesh in mesh-decode.ts.
 */
export interface MeasurementCache {
  facePlanes: Map<number, FacePlane>;
  circularEdges: CircularEdge[];
}

/**
 * Project a point to pixel coordinates using the active camera. If
 * `matrixWorld` is supplied, `p` is treated as mesh-local and transformed to
 * world space first; otherwise `p` is assumed to already be in world space.
 */
function worldToPixel(
  p: Vec3,
  camera: THREE.Camera,
  w: number,
  h: number,
  tmp: THREE.Vector3,
  matrixWorld?: THREE.Matrix4,
): { x: number; y: number } {
  tmp.set(p[0], p[1], p[2]);
  if (matrixWorld) tmp.applyMatrix4(matrixWorld);
  tmp.project(camera);
  return { x: (tmp.x + 1) * 0.5 * w, y: (-tmp.y + 1) * 0.5 * h };
}

/**
 * Resolve the best snap target for a raycaster hit.
 *
 * Priority: circleCenter → vertex → edgeEnd → edgeMid → faceCentroid.
 * The first candidate whose screen-space distance to the cursor is ≤ tol wins;
 * faceCentroid is the always-available fallback.
 */
export function resolveSnap(args: {
  hit: THREE.Intersection;
  vertices: Float32Array;
  indices: Uint32Array;
  faceGroups?: Uint32Array;
  edgeLines?: Float32Array;
  cache: MeasurementCache;
  camera: THREE.Camera;
  cursorPx: { x: number; y: number };
  viewportSize: { w: number; h: number };
  /** Mesh world transform. Required when the hit mesh has been translated
   *  (e.g. by `centerOnBed`) — without it snap points would be returned in
   *  mesh-local space while the rendered geometry is in world space. */
  matrixWorld?: THREE.Matrix4;
  tolPx?: number;
  /** First pick in a two-pick measurement. When supplied, edge candidates
   *  include a perpendicular-foot snap from this point onto the edge. */
  pending?: Snap | null;
}): Snap | null {
  const tol = args.tolPx ?? 12;
  const tmp = new THREE.Vector3();
  const hit = args.hit;
  if (hit.faceIndex == null || !hit.face) return null;

  const faceID = args.faceGroups ? args.faceGroups[hit.faceIndex] : undefined;

  // `hit.point` is world space; all of `vertices`, `edgeLines`,
  // `facePlanes`, `circularEdges` are mesh-local. Keep two versions of the hit
  // point so local-vs-local and world-vs-world comparisons are both clean.
  const hitPointWorld: Vec3 = [hit.point.x, hit.point.y, hit.point.z];
  const invMW = args.matrixWorld ? args.matrixWorld.clone().invert() : null;
  let hitPointLocal: Vec3 = hitPointWorld;
  if (invMW) {
    tmp.set(hitPointWorld[0], hitPointWorld[1], hitPointWorld[2]).applyMatrix4(invMW);
    hitPointLocal = [tmp.x, tmp.y, tmp.z];
  }
  const toWorld = (p: Vec3): Vec3 => {
    if (!args.matrixWorld) return p;
    tmp.set(p[0], p[1], p[2]).applyMatrix4(args.matrixWorld);
    return [tmp.x, tmp.y, tmp.z];
  };
  const w = args.viewportSize.w, h = args.viewportSize.h;
  const mw = args.matrixWorld;

  // --- 1. circleCenter: if cursor is near any circular edge, snap to its center.
  // "Near" means cursor within tol of the nearest loop point in screen space.
  for (const ce of args.cache.circularEdges) {
    // cheap early-out: local-space distance from local hit to circle center bounded by loop radius + slack
    const dToCenter = length(sub(hitPointLocal, ce.center));
    if (dToCenter > ce.radius * 2.5) continue;
    // screen-space distance from cursor to nearest loop point
    let minPx = Infinity;
    for (const lp of ce.loopPoints) {
      const sp = worldToPixel(lp, args.camera, w, h, tmp, mw);
      const dx = sp.x - args.cursorPx.x, dy = sp.y - args.cursorPx.y;
      const d = Math.hypot(dx, dy);
      if (d < minPx) minPx = d;
    }
    if (minPx <= tol) {
      return {
        kind: 'circleCenter',
        point: toWorld(ce.center),
        radius: ce.radius,
        axis: ce.axis,
        loop: ce.loopPoints.map(p => toWorld(p)),
      };
    }
  }

  // --- 2. vertex: three triangle corners.
  const tri = hit.faceIndex;
  const vIdxs = [
    args.indices[tri * 3],
    args.indices[tri * 3 + 1],
    args.indices[tri * 3 + 2],
  ];
  let bestVertPx = Infinity;
  let bestVert: Vec3 | null = null;
  for (const vi of vIdxs) {
    const vp: Vec3 = [args.vertices[vi * 3], args.vertices[vi * 3 + 1], args.vertices[vi * 3 + 2]];
    const sp = worldToPixel(vp, args.camera, w, h, tmp, mw);
    const d = Math.hypot(sp.x - args.cursorPx.x, sp.y - args.cursorPx.y);
    if (d < bestVertPx) { bestVertPx = d; bestVert = vp; }
  }
  if (bestVert && bestVertPx <= tol) {
    return { kind: 'vertex', point: toWorld(bestVert) };
  }

  // --- 3/4. edgeEnd / edgePerp / edgeMid: scan edgeLines near the hit triangle.
  if (args.edgeLines && args.edgeLines.length > 0) {
    // Bound the search: only consider edges with an endpoint within a local-space
    // radius of the local hit point (proportional to triangle size).
    const triMaxEdge = Math.max(
      length(sub(
        [args.vertices[vIdxs[0] * 3], args.vertices[vIdxs[0] * 3 + 1], args.vertices[vIdxs[0] * 3 + 2]],
        [args.vertices[vIdxs[1] * 3], args.vertices[vIdxs[1] * 3 + 1], args.vertices[vIdxs[1] * 3 + 2]],
      )),
      length(sub(
        [args.vertices[vIdxs[1] * 3], args.vertices[vIdxs[1] * 3 + 1], args.vertices[vIdxs[1] * 3 + 2]],
        [args.vertices[vIdxs[2] * 3], args.vertices[vIdxs[2] * 3 + 1], args.vertices[vIdxs[2] * 3 + 2]],
      )),
    );
    const worldR = Math.max(triMaxEdge * 3, 1e-3);
    const worldR2 = worldR * worldR;

    // Pending pick in local space — used to compute perpendicular foot points
    // onto each edge. Falls back to hitPointLocal for the proximity cheap-out.
    const pendingLocal: Vec3 | null = (() => {
      if (!args.pending) return null;
      if (!invMW) return args.pending.point;
      tmp.set(args.pending.point[0], args.pending.point[1], args.pending.point[2]).applyMatrix4(invMW);
      return [tmp.x, tmp.y, tmp.z];
    })();

    let bestEndPx = Infinity, bestMidPx = Infinity, bestPerpPx = Infinity;
    let bestEnd: Vec3 | null = null, bestMid: Vec3 | null = null, bestPerp: Vec3 | null = null;
    // Track which edge each best candidate came from so we can highlight it.
    let bestEndA: Vec3 | null = null, bestEndB: Vec3 | null = null;
    let bestMidA: Vec3 | null = null, bestMidB: Vec3 | null = null;
    let bestPerpA: Vec3 | null = null, bestPerpB: Vec3 | null = null;
    const nEdges = args.edgeLines.length / 6;
    for (let e = 0; e < nEdges; e++) {
      const ax = args.edgeLines[e * 6],     ay = args.edgeLines[e * 6 + 1], az = args.edgeLines[e * 6 + 2];
      const bx = args.edgeLines[e * 6 + 3], by = args.edgeLines[e * 6 + 4], bz = args.edgeLines[e * 6 + 5];
      const dax = ax - hitPointLocal[0], day = ay - hitPointLocal[1], daz = az - hitPointLocal[2];
      const dbx = bx - hitPointLocal[0], dby = by - hitPointLocal[1], dbz = bz - hitPointLocal[2];
      if (dax * dax + day * day + daz * daz > worldR2 &&
          dbx * dbx + dby * dby + dbz * dbz > worldR2) continue;

      const aP: Vec3 = [ax, ay, az];
      const bP: Vec3 = [bx, by, bz];
      const mP: Vec3 = midpoint(aP, bP);

      const sa = worldToPixel(aP, args.camera, w, h, tmp, mw);
      const sb = worldToPixel(bP, args.camera, w, h, tmp, mw);
      const sm = worldToPixel(mP, args.camera, w, h, tmp, mw);
      const da = Math.hypot(sa.x - args.cursorPx.x, sa.y - args.cursorPx.y);
      const db = Math.hypot(sb.x - args.cursorPx.x, sb.y - args.cursorPx.y);
      const dm = Math.hypot(sm.x - args.cursorPx.x, sm.y - args.cursorPx.y);
      if (da < bestEndPx) { bestEndPx = da; bestEnd = aP; bestEndA = aP; bestEndB = bP; }
      if (db < bestEndPx) { bestEndPx = db; bestEnd = bP; bestEndA = aP; bestEndB = bP; }
      if (dm < bestMidPx) { bestMidPx = dm; bestMid = mP; bestMidA = aP; bestMidB = bP; }

      // Perpendicular foot of the pending pick onto this edge line, accepted
      // only if the foot lies strictly within the segment. Candidate strength
      // is the cursor's screen-space distance to the projected edge line so
      // hovering "on" the edge selects it even if the foot sits elsewhere.
      if (pendingLocal) {
        const d: Vec3 = sub(bP, aP);
        const d2 = dot(d, d);
        if (d2 > 1e-18) {
          const t = dot(sub(pendingLocal, aP), d) / d2;
          if (t > 1e-6 && t < 1 - 1e-6) {
            const foot: Vec3 = add(aP, scale(d, t));
            // Screen-space distance from cursor to the edge segment.
            const ex = sb.x - sa.x, ey = sb.y - sa.y;
            const elen2 = ex * ex + ey * ey;
            let edgePx: number;
            if (elen2 < 1e-6) {
              edgePx = Math.min(da, db);
            } else {
              const tt = Math.max(0, Math.min(1, ((args.cursorPx.x - sa.x) * ex + (args.cursorPx.y - sa.y) * ey) / elen2));
              const px = sa.x + tt * ex, py = sa.y + tt * ey;
              edgePx = Math.hypot(args.cursorPx.x - px, args.cursorPx.y - py);
            }
            if (edgePx < bestPerpPx) {
              bestPerpPx = edgePx;
              bestPerp = foot;
              bestPerpA = aP;
              bestPerpB = bP;
            }
          }
        }
      }
    }
    if (bestEnd && bestEndPx <= tol && bestEndA && bestEndB) {
      return {
        kind: 'edgeEnd',
        point: toWorld(bestEnd),
        edge: { a: toWorld(bestEndA), b: toWorld(bestEndB) },
      };
    }
    if (bestPerp && bestPerpPx <= tol && bestPerpA && bestPerpB) {
      return {
        kind: 'edgePerp',
        point: toWorld(bestPerp),
        edge: { a: toWorld(bestPerpA), b: toWorld(bestPerpB) },
      };
    }
    if (bestMid && bestMidPx <= tol && bestMidA && bestMidB) {
      return {
        kind: 'edgeMid',
        point: toWorld(bestMid),
        edge: { a: toWorld(bestMidA), b: toWorld(bestMidB) },
      };
    }
  }

  // --- 5. faceCentroid: always-available fallback.
  if (faceID !== undefined) {
    const fp = args.cache.facePlanes.get(faceID);
    if (fp) {
      return {
        kind: 'faceCentroid',
        point: toWorld(fp.centroid),
        faceID,
        normal: fp.normal,
      };
    }
  }

  // As a last resort, snap to the raw hit point with no identity.
  return { kind: 'vertex', point: hitPointWorld };
}

// ---------------------------------------------------------------------------
// Measurement kind classifier
// ---------------------------------------------------------------------------

/**
 * Build a Measurement from two completed snaps. Returns null if the snaps
 * describe something we cannot measure (e.g. two circle-center picks on
 * non-coaxial circles — arguably a linear between centers, but the user
 * probably meant something else and a clearer UI is to ignore).
 */
export function buildMeasurement(a: Snap, b: Snap): Measurement | null {
  // Two face centroids — parallel ⇒ faceToFace, else angular.
  if (a.kind === 'faceCentroid' && b.kind === 'faceCentroid' && a.normal && b.normal) {
    const d = dot(a.normal, b.normal);
    const PAR = 0.999;
    if (Math.abs(d) >= PAR) {
      // Parallel: project centroid-to-centroid onto normal to get plane separation.
      const delta = sub(b.point, a.point);
      const distance = Math.abs(dot(delta, a.normal));
      return { kind: 'faceToFace', a, b, distance };
    }
    const deg = Math.acos(Math.min(1, Math.abs(d))) * 180 / Math.PI;
    return { kind: 'angular', a, b, degrees: deg };
  }

  // Default: straight 3D distance. Edge-anchored endpoints are annotated with
  // the dim-line-to-edge angle in the renderer, not by changing the kind.
  const distance = length(sub(b.point, a.point));
  return { kind: 'linear', a, b, distance };
}

/** Single-pick radial: the user clicked a circle edge and nothing else. */
export function buildRadial(s: Snap): Measurement | null {
  if (s.kind !== 'circleCenter' || s.radius === undefined) return null;
  return { kind: 'radial', c: s, radius: s.radius, diameter: s.radius * 2 };
}

/**
 * Interior corner angle at a chain vertex. Given three consecutive chain
 * points prev→vertex→next, returns the angle at `vertex` along with unit
 * direction vectors from the vertex toward its neighbours (for rendering).
 * Returns null if either leg is degenerate.
 */
export function buildCornerAngle(prev: Snap, vertex: Snap, next: Snap): Measurement | null {
  const prevDir = normalize(sub(prev.point, vertex.point));
  const nextDir = normalize(sub(next.point, vertex.point));
  if (length(prevDir) < 1e-9 || length(nextDir) < 1e-9) return null;
  const d = Math.max(-1, Math.min(1, dot(prevDir, nextDir)));
  const degrees = Math.acos(d) * 180 / Math.PI;
  return { kind: 'cornerAngle', vertex, prevDir, nextDir, degrees };
}

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

/** User-facing number formatting options for measurement labels. */
export interface MeasurementFormat {
  units: 'metric' | 'imperial';
  /** Only meaningful when units === 'imperial'. */
  imperialFormat: 'fraction' | 'decimal';
  /** Only meaningful when imperialFormat === 'fraction'. Power of 2, 4..128. */
  imperialDenominator: number;
}

export const DEFAULT_MEASUREMENT_FORMAT: MeasurementFormat = {
  units: 'metric',
  imperialFormat: 'fraction',
  imperialDenominator: 64,
};

/** Format a millimetre length in metric (mm, to 3dp) form, trailing zeros stripped. */
function formatMetric(mm: number): string {
  const s = mm.toFixed(3);
  const trimmed = s.replace(/\.?0+$/, '');
  return `${trimmed} mm`;
}

function gcd(a: number, b: number): number {
  return b === 0 ? a : gcd(b, a % b);
}

/** Decimal inches, 3 dp, trailing zeros stripped (e.g. 1.25 → "1.25\""). */
function formatImperialDecimal(mm: number): string {
  const inches = mm / 25.4;
  const s = inches.toFixed(3);
  const trimmed = s.replace(/\.?0+$/, '');
  return `${trimmed}"`;
}

/**
 * Inches as a reduced fraction, rounded to the nearest 1/denom. e.g. at
 * denom = 64: 32/64 → 1/2, 16/64 → 1/4. Whole and fractional parts separated
 * by a space ("1 1/4\""); pure whole or pure fraction collapse ("2\"", "3/8\"").
 */
function formatImperialFraction(mm: number, denominator: number): string {
  const inches = mm / 25.4;
  const sign = inches < 0 ? '-' : '';
  const abs = Math.abs(inches);
  const totalTicks = Math.round(abs * denominator);
  const whole = Math.floor(totalTicks / denominator);
  const num = totalTicks % denominator;
  if (num === 0) return `${sign}${whole}"`;
  const g = gcd(num, denominator);
  const reducedNum = num / g;
  const reducedDen = denominator / g;
  if (whole === 0) return `${sign}${reducedNum}/${reducedDen}"`;
  return `${sign}${whole} ${reducedNum}/${reducedDen}"`;
}

export function formatLength(mm: number, fmt: MeasurementFormat = DEFAULT_MEASUREMENT_FORMAT): string {
  if (fmt.units !== 'imperial') return formatMetric(mm);
  if (fmt.imperialFormat === 'decimal') return formatImperialDecimal(mm);
  return formatImperialFraction(mm, fmt.imperialDenominator);
}

export function formatAngle(deg: number): string {
  return `${deg.toFixed(1)}\u00b0`;
}

export function formatMeasurementLabel(m: Measurement, fmt: MeasurementFormat = DEFAULT_MEASUREMENT_FORMAT): string {
  switch (m.kind) {
    case 'linear':     return formatLength(m.distance, fmt);
    case 'faceToFace': return formatLength(m.distance, fmt);
    case 'radial':     return `\u2300 ${formatLength(m.diameter, fmt)}`;
    case 'angular':    return formatAngle(m.degrees);
    case 'cornerAngle': return formatAngle(m.degrees);
    case 'extents': {
      const dx = m.max[0] - m.min[0];
      const dy = m.max[1] - m.min[1];
      const dz = m.max[2] - m.min[2];
      return `${formatLength(dx, fmt)} \u00d7 ${formatLength(dy, fmt)} \u00d7 ${formatLength(dz, fmt)}`;
    }
  }
}
