// measurement_render.ts — Three.js scene object construction for dimensions.
//
// Pure factory functions. Callers manage parent groups and disposal. No
// cross-frame state.

import * as THREE from 'three';
import type { Measurement, MeasurementFormat, Snap, Vec3 } from './measurement';
import { formatMeasurementLabel, formatAngle } from './measurement';

/** Visual + formatting settings for measurement rendering, sourced from the active theme + user prefs. */
export interface MeasurementStyle {
  /** Line color for dimension lines, arcs, extension stubs, and snap glyphs (as "#rrggbb"). */
  lineColor: string;
  /** Label text color for on-model sprite labels (as "#rrggbb"). */
  labelColor: string;
  /** Number formatting for label values. */
  format: MeasurementFormat;
}

/** Create a canvas-textured sprite label. Sprite scales with `worldHeight`. */
export function createLabelSprite(text: string, opts: { color: string; fontPx?: number; worldHeight?: number }): THREE.Sprite {
  const color = opts.color;
  const fontPx = opts.fontPx ?? 48;
  const worldHeight = opts.worldHeight ?? 2.5;

  // Measure text to size the canvas appropriately.
  const measureCanvas = document.createElement('canvas');
  const mctx = measureCanvas.getContext('2d')!;
  mctx.font = `bold ${fontPx}px sans-serif`;
  const metrics = mctx.measureText(text);
  const pad = Math.ceil(fontPx * 0.4);
  const w = Math.max(32, Math.ceil(metrics.width) + pad * 2);
  const h = Math.ceil(fontPx * 1.4) + pad * 2;

  const canvas = document.createElement('canvas');
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext('2d')!;
  // Transparent background box for contrast.
  ctx.fillStyle = 'rgba(0, 0, 0, 0.55)';
  ctx.fillRect(0, 0, w, h);
  ctx.font = `bold ${fontPx}px sans-serif`;
  ctx.textBaseline = 'middle';
  ctx.textAlign = 'center';
  ctx.fillStyle = color;
  ctx.fillText(text, w / 2, h / 2);

  const texture = new THREE.CanvasTexture(canvas);
  texture.minFilter = THREE.LinearFilter;
  const mat = new THREE.SpriteMaterial({ map: texture, transparent: true, depthTest: false });
  const sprite = new THREE.Sprite(mat);
  // Scale so the sprite's height in world units equals worldHeight; width
  // scales to preserve the canvas aspect ratio.
  sprite.scale.set(worldHeight * (w / h), worldHeight, 1);
  sprite.renderOrder = 10;
  return sprite;
}

/** LineSegments geometry from a flat array of (x,y,z) pairs. */
function makeSegments(points: Vec3[], color: string, opts?: { dashed?: boolean; renderOrder?: number }): THREE.LineSegments {
  const verts: number[] = [];
  for (const p of points) verts.push(p[0], p[1], p[2]);
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.BufferAttribute(new Float32Array(verts), 3));
  const mat = opts?.dashed
    ? new THREE.LineDashedMaterial({ color, dashSize: 0.5, gapSize: 0.3, depthTest: false, transparent: true })
    : new THREE.LineBasicMaterial({ color, depthTest: false, transparent: true });
  const seg = new THREE.LineSegments(geo, mat);
  if (opts?.dashed) seg.computeLineDistances();
  seg.renderOrder = opts?.renderOrder ?? 9;
  return seg;
}

/** Tessellate an arc from vector `from` to vector `to` about `center`, on the plane defined by their cross product. */
function arcPoints(center: Vec3, fromDir: Vec3, toDir: Vec3, radius: number, steps = 24): Vec3[] {
  // Build an orthonormal basis (u, v) in the arc plane.
  const fd = new THREE.Vector3(fromDir[0], fromDir[1], fromDir[2]).normalize();
  const td = new THREE.Vector3(toDir[0],   toDir[1],   toDir[2]).normalize();
  const axis = new THREE.Vector3().crossVectors(fd, td);
  if (axis.lengthSq() < 1e-10) {
    // Parallel — degenerate arc. Return a straight segment.
    const p0: Vec3 = [center[0] + fd.x * radius, center[1] + fd.y * radius, center[2] + fd.z * radius];
    return [p0, p0];
  }
  axis.normalize();
  const v = new THREE.Vector3().crossVectors(axis, fd).normalize();
  const totalAngle = Math.acos(Math.max(-1, Math.min(1, fd.dot(td))));
  const pts: Vec3[] = [];
  for (let i = 0; i <= steps; i++) {
    const a = (i / steps) * totalAngle;
    const x = Math.cos(a) * radius;
    const y = Math.sin(a) * radius;
    const px = center[0] + fd.x * x + v.x * y;
    const py = center[1] + fd.y * x + v.y * y;
    const pz = center[2] + fd.z * x + v.z * y;
    pts.push([px, py, pz]);
  }
  return pts;
}

/** Convert a sequence of arc points to LineSegments vertex pairs. */
function polylineToSegments(pts: Vec3[]): Vec3[] {
  const segs: Vec3[] = [];
  for (let i = 0; i < pts.length - 1; i++) {
    segs.push(pts[i], pts[i + 1]);
  }
  return segs;
}

const midpoint = (a: Vec3, b: Vec3): Vec3 => [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (a[2] + b[2]) / 2];

/**
 * Build a Three.js Group containing the dimension line + label for a measurement.
 * The group has no parent — caller adds to scene.
 */
export function buildMeasurementGroup(
  m: Measurement,
  style: MeasurementStyle,
  opts?: { labelWorldHeight?: number },
): THREE.Group {
  const lineColor = style.lineColor;
  const labelColor = style.labelColor;
  const labelH = opts?.labelWorldHeight ?? 2.5;
  const group = new THREE.Group();
  const mkLabel = (text: string): THREE.Sprite =>
    createLabelSprite(text, { color: labelColor, worldHeight: labelH });

  switch (m.kind) {
    case 'linear':
    case 'faceToFace': {
      const line = makeSegments([m.a.point, m.b.point], lineColor);
      group.add(line);
      const mid = midpoint(m.a.point, m.b.point);
      const lbl = mkLabel(formatMeasurementLabel(m, style.format));
      lbl.position.set(mid[0], mid[1], mid[2]);
      group.add(lbl);
      break;
    }
    case 'radial': {
      // Draw a diameter chord through the center, in the circle's plane.
      const c = m.c.point;
      const r = m.radius;
      const axis = m.c.axis ?? [0, 0, 1];
      // Pick any in-plane direction.
      const ax = new THREE.Vector3(axis[0], axis[1], axis[2]);
      const tmp = Math.abs(ax.x) < 0.9
        ? new THREE.Vector3(1, 0, 0)
        : new THREE.Vector3(0, 1, 0);
      const u = new THREE.Vector3().crossVectors(ax, tmp).normalize();
      const p0: Vec3 = [c[0] - u.x * r, c[1] - u.y * r, c[2] - u.z * r];
      const p1: Vec3 = [c[0] + u.x * r, c[1] + u.y * r, c[2] + u.z * r];
      group.add(makeSegments([p0, p1], lineColor));
      const lbl = mkLabel(formatMeasurementLabel(m, style.format));
      lbl.position.set(c[0], c[1], c[2]);
      group.add(lbl);
      break;
    }
    case 'angular': {
      // Draw two rays from the midpoint between the snap points along their
      // normals/directions, plus an arc between them.
      const origin = midpoint(m.a.point, m.b.point);
      const na = m.a.normal;
      const nb = m.b.normal;
      if (!na || !nb) break; // only face-face angular supported in v1
      // Ray length: visually readable, tied to the distance between picks.
      const R = Math.max(1, 0.5 * Math.hypot(
        m.b.point[0] - m.a.point[0],
        m.b.point[1] - m.a.point[1],
        m.b.point[2] - m.a.point[2],
      ));
      const endA: Vec3 = [origin[0] + na[0] * R, origin[1] + na[1] * R, origin[2] + na[2] * R];
      const endB: Vec3 = [origin[0] + nb[0] * R, origin[1] + nb[1] * R, origin[2] + nb[2] * R];
      group.add(makeSegments([origin, endA, origin, endB], lineColor));
      const arcPts = arcPoints(origin, na, nb, R * 0.6);
      group.add(makeSegments(polylineToSegments(arcPts), lineColor));
      const lblPos = arcPts[Math.floor(arcPts.length / 2)];
      const lbl = mkLabel(formatMeasurementLabel(m, style.format));
      lbl.position.set(lblPos[0], lblPos[1], lblPos[2]);
      group.add(lbl);
      break;
    }
    case 'cornerAngle': {
      // Arc at the chain vertex between the two adjacent dim segments.
      const v = m.vertex.point;
      // Radius: small enough to sit inside the corner, large enough to read.
      const R = labelH * 2.5;
      const arcPts = arcPoints(v, m.prevDir, m.nextDir, R, 24);
      group.add(makeSegments(polylineToSegments(arcPts), lineColor));
      const lblPos = arcPts[Math.floor(arcPts.length / 2)];
      const lbl = mkLabel(formatMeasurementLabel(m, style.format));
      lbl.position.set(lblPos[0], lblPos[1], lblPos[2]);
      group.add(lbl);
      break;
    }
    case 'extents': {
      // Draw an axis-aligned box outline and a label near the max corner.
      const [x0, y0, z0] = m.min;
      const [x1, y1, z1] = m.max;
      const corners: Vec3[] = [
        [x0, y0, z0], [x1, y0, z0],
        [x1, y0, z0], [x1, y1, z0],
        [x1, y1, z0], [x0, y1, z0],
        [x0, y1, z0], [x0, y0, z0],
        [x0, y0, z1], [x1, y0, z1],
        [x1, y0, z1], [x1, y1, z1],
        [x1, y1, z1], [x0, y1, z1],
        [x0, y1, z1], [x0, y0, z1],
        [x0, y0, z0], [x0, y0, z1],
        [x1, y0, z0], [x1, y0, z1],
        [x1, y1, z0], [x1, y1, z1],
        [x0, y1, z0], [x0, y1, z1],
      ];
      group.add(makeSegments(corners, lineColor, { dashed: true }));
      const lbl = mkLabel(formatMeasurementLabel(m, style.format));
      lbl.position.set((x0 + x1) / 2, y1 + labelH, (z0 + z1) / 2);
      group.add(lbl);
      break;
    }
  }

  return group;
}

/**
 * Snap marker: a canvas-sprite glyph whose shape encodes the snap kind so the
 * user can tell at a glance what they're about to pick. Glyph color follows
 * the theme (supplied by the caller) so it reads against any background.
 *
 * Glyphs follow a consistent visual family — a feature-of-geometry diagram
 * plus a filled dot at the actual snap point:
 *
 *  - vertex       — two perpendicular stubs meeting at the dot (a corner)
 *  - edgeEnd      — one stub out from the dot (one end of a line)
 *  - edgeMid      — line through the dot (middle of a line)
 *  - edgePerp     — a right-angle bracket with the dot at its corner (90° foot)
 *  - faceCentroid — outlined square with the dot at its center (planar patch)
 *  - circleCenter — circle with a crosshair at the dot (hub of a circle)
 */
const SNAP_GLYPH: Record<Snap['kind'], GlyphKind> = {
  vertex:       'corner',
  edgeEnd:      'lineEnd',
  edgeMid:      'lineWithDot',
  edgePerp:     'rightAngle',
  faceCentroid: 'squareWithDot',
  circleCenter: 'circleCross',
  circleEdge:   'circleRimDot',
  angleLock:    'angleRay',
};

type GlyphKind = 'corner' | 'lineEnd' | 'lineWithDot' | 'rightAngle' | 'squareWithDot' | 'circleCross' | 'circleRimDot' | 'angleRay';

/** Build the path/shape for a glyph without touching stroke/fill styles.
 *  Used by drawGlyph (which draws a halo pass then a color pass). */
function paintGlyphShape(ctx: CanvasRenderingContext2D, size: number, glyph: GlyphKind): void {
  const cx = size / 2, cy = size / 2;
  const r = size * 0.35;
  const dotR = Math.max(2, size * 0.08);
  const fillDot = () => {
    ctx.beginPath();
    ctx.arc(cx, cy, dotR, 0, Math.PI * 2);
    ctx.fill();
  };
  switch (glyph) {
    case 'corner': {
      // Two perpendicular stubs meeting at the center — a geometry corner.
      ctx.beginPath();
      ctx.moveTo(cx, cy); ctx.lineTo(cx + r, cy); // right
      ctx.moveTo(cx, cy); ctx.lineTo(cx, cy - r); // up
      ctx.stroke();
      fillDot();
      break;
    }
    case 'lineEnd': {
      // Single stub to the right of the dot — the endpoint of a line.
      ctx.beginPath();
      ctx.moveTo(cx, cy); ctx.lineTo(cx + r, cy);
      ctx.stroke();
      fillDot();
      break;
    }
    case 'lineWithDot': {
      // Full line through the dot — the midpoint of a line.
      ctx.beginPath();
      ctx.moveTo(cx - r, cy); ctx.lineTo(cx + r, cy);
      ctx.stroke();
      fillDot();
      break;
    }
    case 'rightAngle': {
      // Right-angle bracket (⌐ rotated) with the dot at the corner — the
      // perpendicular foot of the pending pick onto an edge. One stub
      // represents the edge, the other the 90° dimension direction.
      const s = r * 0.8;
      ctx.beginPath();
      ctx.moveTo(cx - s, cy); ctx.lineTo(cx + s, cy); // edge stub
      ctx.moveTo(cx, cy); ctx.lineTo(cx, cy - s);    // perp stub
      // Small square notch at the corner to read as "90°".
      const n = s * 0.3;
      ctx.moveTo(cx + n, cy); ctx.lineTo(cx + n, cy - n);
      ctx.moveTo(cx + n, cy - n); ctx.lineTo(cx, cy - n);
      ctx.stroke();
      fillDot();
      break;
    }
    case 'squareWithDot': {
      // Outlined square + centered dot — centroid of a planar patch.
      ctx.strokeRect(cx - r, cy - r, r * 2, r * 2);
      fillDot();
      break;
    }
    case 'angleRay': {
      // Two rays from the dot forming an acute angle, with a small arc
      // connecting them — the visual language of an angle constraint. One ray
      // is the reference edge, the other is the locked dim direction.
      const s = r * 0.95;
      const ang = Math.PI / 4; // depict the 45° case (generic preset)
      ctx.beginPath();
      ctx.moveTo(cx, cy); ctx.lineTo(cx + s, cy); // edge ray (→)
      ctx.moveTo(cx, cy); ctx.lineTo(cx + s * Math.cos(ang), cy - s * Math.sin(ang)); // locked ray
      ctx.stroke();
      // Arc between the rays.
      const arcR = s * 0.35;
      ctx.beginPath();
      ctx.arc(cx, cy, arcR, -ang, 0);
      ctx.stroke();
      fillDot();
      break;
    }
    case 'circleRimDot': {
      // Circle outline with the dot on the perimeter — a point ON the circle
      // (vs circleCross where the dot sits at the center).
      ctx.beginPath();
      ctx.arc(cx, cy, r, 0, Math.PI * 2);
      ctx.stroke();
      ctx.beginPath();
      ctx.arc(cx + r, cy, dotR, 0, Math.PI * 2);
      ctx.fill();
      break;
    }
    case 'circleCross': {
      // Circle + crosshair + dot at hub — center of a circular edge.
      ctx.beginPath();
      ctx.arc(cx, cy, r, 0, Math.PI * 2);
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(cx - r, cy); ctx.lineTo(cx + r, cy);
      ctx.moveTo(cx, cy - r); ctx.lineTo(cx, cy + r);
      ctx.stroke();
      fillDot();
      break;
    }
  }
}

/**
 * Render a glyph to the given canvas context. Draws a dark halo pass first,
 * then the themed-color pass on top, so the glyph reads on any background.
 */
function drawGlyph(ctx: CanvasRenderingContext2D, size: number, glyph: GlyphKind, color: string): void {
  ctx.lineJoin = 'round';
  ctx.lineCap = 'round';
  // Halo pass: slightly thicker, near-black — provides contrast against any
  // background color (bright geometry, dark theme, busy textures).
  ctx.strokeStyle = 'rgba(0, 0, 0, 0.9)';
  ctx.fillStyle   = 'rgba(0, 0, 0, 0.9)';
  ctx.lineWidth   = Math.max(4, size * 0.14);
  paintGlyphShape(ctx, size, glyph);
  // Color pass: the themed glyph color on top of the halo.
  ctx.strokeStyle = color;
  ctx.fillStyle   = color;
  ctx.lineWidth   = Math.max(2, size * 0.08);
  paintGlyphShape(ctx, size, glyph);
}

/**
 * CSS `cursor:` value that renders the snap glyph under the mouse, centred on
 * the hotspot. Caller assigns it to `canvas.style.cursor`. Pass `null` to get
 * a plain crosshair.
 */
export function snapCursorCSS(kind: Snap['kind'] | null, color: string): string {
  if (!kind) return 'crosshair';
  const px = 32; // browsers cap cursor size; 32px is safely supported everywhere
  const canvas = document.createElement('canvas');
  canvas.width = px;
  canvas.height = px;
  const ctx = canvas.getContext('2d')!;
  drawGlyph(ctx, px, SNAP_GLYPH[kind], color);
  const url = canvas.toDataURL('image/png');
  const hot = Math.floor(px / 2);
  // Fallback to crosshair if the data URL is rejected (older browsers, CSP).
  return `url('${url}') ${hot} ${hot}, crosshair`;
}

/**
 * Snap marker group: highlight of the specific geometry feature the snap
 * resolved against (the source edge for edgeEnd/edgeMid/edgePerp, or the full
 * loop for circleCenter), and optionally a kind-indicator glyph sprite at the
 * snap point. The highlight makes it unambiguous *which* feature the click
 * will lock in; the sprite is useful for a persistent on-model marker at a
 * locked-in point, and should be omitted when the live cursor already carries
 * the glyph.
 */
export function buildPendingMarker(
  s: Snap,
  opts: { color: string; worldSize?: number; sprite?: boolean },
): THREE.Object3D {
  const group = new THREE.Group();
  group.renderOrder = 11;

  // 1. Feature highlight — thick line over the source edge or loop.
  const lineColor = opts.color;
  if (s.edge) {
    // edgeEnd / edgeMid / edgePerp: highlight the specific edge segment.
    group.add(makeSegments([s.edge.a, s.edge.b], lineColor, { renderOrder: 11 }));
  } else if (s.loop && s.loop.length >= 2) {
    // circleCenter: highlight the full closed loop.
    const closed = s.loop.concat([s.loop[0]]);
    group.add(makeSegments(polylineToSegments(closed), lineColor, { renderOrder: 11 }));
  }

  // 2. A small dot at the actual snap point. Always drawn — the cursor-carried
  //    glyph shows intent, but the anchor can be far from the cursor (e.g. a
  //    face-centroid snap or an off-mesh perpendicular foot), so the dot is
  //    what tells the user *where* the click will land.
  const dotWorldSize = (opts.worldSize ?? 0.4) * 1.5;
  group.add(makeDotSprite(s.point, opts.color, dotWorldSize));

  // 3. Kind-indicator glyph sprite at the snap point. Skipped when the caller
  //    has the cursor carrying the glyph (hover previews).
  if (opts.sprite ?? true) {
    const worldSize = (opts.worldSize ?? 0.4) * 6; // sprite extent; glyph fills ~60% of it
    const glyph = SNAP_GLYPH[s.kind];
    const px = 64;
    const canvas = document.createElement('canvas');
    canvas.width = px;
    canvas.height = px;
    const ctx = canvas.getContext('2d')!;
    drawGlyph(ctx, px, glyph, opts.color);
    const texture = new THREE.CanvasTexture(canvas);
    texture.minFilter = THREE.LinearFilter;
    const mat = new THREE.SpriteMaterial({ map: texture, transparent: true, depthTest: false });
    const sprite = new THREE.Sprite(mat);
    sprite.scale.set(worldSize, worldSize, 1);
    sprite.position.set(s.point[0], s.point[1], s.point[2]);
    sprite.renderOrder = 12;
    group.add(sprite);
  }

  return group;
}

/**
 * Hover annotation: if `anchor` is a snap attached to an edge, draw an arc +
 * label at anchor.point showing the angle between that edge and the preview
 * dim line toward `other`. Used only in hoverMeasurementGroup so it is
 * cleared on the next pointermove / click; once a dim is committed, only
 * chain-vertex (dim-to-dim) corner angles persist.
 *
 * Returns null if the snap isn't edge-based, or the geometry is degenerate,
 * or the angle would be too small to place readably.
 */
export function buildHoverEdgeAngle(
  anchor: Snap,
  other: Snap,
  style: MeasurementStyle,
  labelH: number,
): THREE.Object3D | null {
  if (!anchor.edge) return null;
  if (anchor.kind === 'edgePerp') return null; // always 90° — uninformative

  const ex = anchor.edge.b[0] - anchor.edge.a[0];
  const ey = anchor.edge.b[1] - anchor.edge.a[1];
  const ez = anchor.edge.b[2] - anchor.edge.a[2];
  const eLen = Math.hypot(ex, ey, ez);
  if (eLen < 1e-9) return null;
  const edgeDir: Vec3 = [ex / eLen, ey / eLen, ez / eLen];

  const dx = other.point[0] - anchor.point[0];
  const dy = other.point[1] - anchor.point[1];
  const dz = other.point[2] - anchor.point[2];
  const dLen = Math.hypot(dx, dy, dz);
  if (dLen < 1e-9) return null;
  const dimDir: Vec3 = [dx / dLen, dy / dLen, dz / dLen];

  const dotv = edgeDir[0] * dimDir[0] + edgeDir[1] * dimDir[1] + edgeDir[2] * dimDir[2];
  const deg = Math.acos(Math.max(-1, Math.min(1, Math.abs(dotv)))) * 180 / Math.PI;
  // Orient the edge ray toward the acute side so the arc reads as the reported
  // angle rather than its supplement.
  const edgeFrom: Vec3 = dotv < 0
    ? [-edgeDir[0], -edgeDir[1], -edgeDir[2]]
    : edgeDir;

  // Small arc sized to sit at the endpoint without crowding the mid-line label.
  const arcR = Math.min(dLen * 0.2, eLen * 0.4, labelH * 3);
  if (arcR < labelH * 0.3) return null;

  const group = new THREE.Group();
  const arcPts = arcPoints(anchor.point, edgeFrom, dimDir, arcR, 16);
  group.add(makeSegments(polylineToSegments(arcPts), style.lineColor, { renderOrder: 11 }));

  const lblPos = arcPts[Math.floor(arcPts.length / 2)];
  const lbl = createLabelSprite(formatAngle(deg), { color: style.labelColor, worldHeight: labelH * 0.75 });
  lbl.position.set(lblPos[0], lblPos[1], lblPos[2]);
  group.add(lbl);
  return group;
}

/** A themed dot sprite (with dark halo for contrast) placed at a world point. */
function makeDotSprite(pos: Vec3, color: string, worldSize: number): THREE.Sprite {
  const px = 32;
  const canvas = document.createElement('canvas');
  canvas.width = px;
  canvas.height = px;
  const ctx = canvas.getContext('2d')!;
  const cx = px / 2, cy = px / 2;
  // Halo
  ctx.fillStyle = 'rgba(0, 0, 0, 0.9)';
  ctx.beginPath();
  ctx.arc(cx, cy, px * 0.28, 0, Math.PI * 2);
  ctx.fill();
  // Color fill
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(cx, cy, px * 0.2, 0, Math.PI * 2);
  ctx.fill();
  const texture = new THREE.CanvasTexture(canvas);
  texture.minFilter = THREE.LinearFilter;
  const mat = new THREE.SpriteMaterial({ map: texture, transparent: true, depthTest: false });
  const sprite = new THREE.Sprite(mat);
  sprite.scale.set(worldSize, worldSize, 1);
  sprite.position.set(pos[0], pos[1], pos[2]);
  sprite.renderOrder = 12;
  return sprite;
}

/** Dispose all geometries and materials under a Group. Use before removing. */
export function disposeMeasurementGroup(g: THREE.Object3D): void {
  g.traverse(obj => {
    if ((obj as THREE.Mesh).geometry) {
      (obj as THREE.Mesh).geometry.dispose();
    }
    const m = (obj as THREE.Mesh | THREE.Sprite).material as THREE.Material | THREE.Material[] | undefined;
    if (!m) return;
    if (Array.isArray(m)) m.forEach(x => x.dispose());
    else {
      const tex = (m as THREE.SpriteMaterial).map;
      if (tex) tex.dispose();
      m.dispose();
    }
  });
}
