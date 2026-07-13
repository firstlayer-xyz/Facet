import * as THREE from 'three';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';
import { HeadTracker } from './headtrack';
import { hexToInt } from './color';
import { decodeBinaryMesh, buildFaceGroupWireframe } from './mesh-decode';
import type { DecodedMesh, DebugStepData } from './mesh-decode';
import {
  resolveSnap,
  buildMeasurement,
  buildRadial,
  buildCornerAngle,
  computeFacePlanes,
  detectCircularEdges,
  DEFAULT_MEASUREMENT_FORMAT,
  type Measurement,
  type MeasurementFormat,
  type Snap,
  type MeasurementCache,
} from './measurement';
import {
  buildMeasurementGroup,
  buildPendingMarker,
  buildHoverEdgeAngle,
  disposeMeasurementGroup,
  snapCursorCSS,
  type MeasurementStyle,
} from './measurement_render';

export type { DecodedMesh, DebugStepData };
/** Theme-derived colors + bed settings passed to the viewer. */
interface ViewerAppearance {
  backgroundColor: string;
  meshColor: string;
  meshMetalness: number;
  meshRoughness: number;
  edgeColor: string;
  edgeOpacity: number;
  edgeThreshold: number;
  ambientIntensity: number;
  gridMajorColor: string;
  gridMinorColor: string;
  /** Line/glyph color for dimension visuals (hex like "#rrggbb"). */
  measurementLineColor: string;
  /** Label text color for dimension labels (hex like "#rrggbb"). */
  measurementLabelColor: string;
  /** Number formatting for dimension labels. */
  measurementFormat: MeasurementFormat;
  bed: string;
  gridSize: number;
  gridSpacing: number;
}

/** PosEntry maps a source position to the face IDs produced there. */
export interface PosEntry {
  file: string;  // "" = main, "folder/name" = library path
  line: number;
  col: number;
  faceIDs: number[];
}


export type DrawingViewpoint = 'iso' | 'top' | 'bot' | 'home' | 'front' | 'back' | 'right' | 'left';

type StyleDef = { color: number; opacity: number };

const STEP_STYLES: Record<string, Record<string, StyleDef>> = {
  Difference: {
    lhs: { color: 0x4488ff, opacity: 0.25 },
    rhs: { color: 0xff4444, opacity: 0.5 },
    result: { color: 0x4488ff, opacity: 1.0 },
  },
  Union: {
    lhs: { color: 0x4488ff, opacity: 0.25 },
    rhs: { color: 0x44bb44, opacity: 0.25 },
    result: { color: 0x4488ff, opacity: 1.0 },
  },
  Intersection: {
    lhs: { color: 0x4488ff, opacity: 0.25 },
    rhs: { color: 0x44bb44, opacity: 0.25 },
    result: { color: 0xddaa22, opacity: 1.0 },
  },
  _default: {
    result: { color: 0x4488ff, opacity: 1.0 },
    input: { color: 0x888888, opacity: 0.2 },
  },
};

export class Viewer {
  private renderer: THREE.WebGLRenderer;
  private scene: THREE.Scene;

  // Perspective mode
  private perspCamera: THREE.PerspectiveCamera;
  private perspControls: OrbitControls;

  // Drawing / orthographic mode
  private orthoCamera: THREE.OrthographicCamera;
  private orthoControls: OrbitControls;

  // Active pointers — swap between persp and ortho
  private activeCamera: THREE.Camera;
  private activeControls: OrbitControls;

  private container: HTMLElement;
  private userMeshes: THREE.Object3D[] = [];
  private posMap: PosEntry[] = [];

  // Pixels at the right edge of the canvas that are covered by overlay
  // drawers (docs panel, assistant panel, their resizers). Used by
  // resize to setViewOffset on both cameras so the lookAt target
  // projects to the centre of the VISIBLE portion of the canvas
  // instead of the centre of the full canvas — which would put the
  // model behind the drawer. Updated by main.ts via setRightInset.
  private rightInset = 0;
  private ambientLight: THREE.AmbientLight;
  private savedAmbientIntensity: number = 0.8;
  private grid: THREE.LineSegments;
  private gridLabels: THREE.Mesh[] = [];
  private axes: THREE.AxesHelper;
  private axisLabels: THREE.Sprite[] = [];
  private frameCallbacks: Array<(camera: THREE.Camera) => void> = [];
  private meshColor: number;
  private meshMetalness: number;
  private meshRoughness: number;
  private edgeColor: number;
  private edgeOpacity: number;
  private edgeThreshold: number;
  private bgColor: number;
  private bed: string;
  private gridSize: number;
  private gridSpacing: number;

  // Head tracking parallax
  private headTracker: HeadTracker | null = null;

  // Drawing mode state
  private drawingMode = false;
  private drawingOverlays: THREE.Object3D[] = [];
  private drawingOriginalMaterials = new Map<THREE.Mesh, THREE.Material | THREE.Material[]>();
  private hiddenLinesEnabled = false;
  private hiddenLineObjects: THREE.LineSegments[] = [];

  // Wireframe mode state
  private wireframeMode = false;
  private wireframeOriginalMaterials = new Map<THREE.Mesh, THREE.Material | THREE.Material[]>();
  private wireframeLineObjects: THREE.LineSegments[] = [];

  private resizeObserver: ResizeObserver;
  private _visible = true;

  // Keyboard orbital controls
  private keysDown = new Set<string>();
  private viewportFocused = false;
  private controlsOverlay: HTMLElement | null = null;

  // Face-click → source navigation
  private raycaster = new THREE.Raycaster();
  private pickMouseDown: { x: number; y: number; time: number } | null = null;
  private reversePosMap = new Map<number, PosEntry[]>();
  private onFaceClickCb: ((file: string, line: number, col: number) => void) | null = null;
  private lastPickedFaceGroup = -1;
  private lastPickCycleIndex = 0;

  // Dimensioning / measurement state
  private measureMode: 'off' | 'placing' = 'off';
  private hoverReadout = false;
  private pendingSnap: Snap | null = null;
  private pendingMarker: THREE.Object3D | null = null;
  // For chained dimensioning: the point *before* pendingSnap in the current
  // chain, so the next click can build a corner-angle at pendingSnap.
  private chainPrevSnap: Snap | null = null;
  private measurements: Measurement[] = [];
  private measurementGroup = new THREE.Group();
  private hoverMeasurementGroup = new THREE.Group();
  private measurementStyle: MeasurementStyle = {
    lineColor: '#ffcc33',
    labelColor: '#ffcc33',
    format: DEFAULT_MEASUREMENT_FORMAT,
  };
  private onMeasureModeChangeCb: ((mode: 'off' | 'placing', hoverOn: boolean) => void) | null = null;

  constructor(container: HTMLElement, appearance?: ViewerAppearance) {
    this.container = container;

    const bg = appearance?.backgroundColor ?? '#1b2636';
    const meshHex = appearance?.meshColor ?? '#2194ce';
    const gridMajor = appearance?.gridMajorColor ?? '#444444';
    const gridMinor = appearance?.gridMinorColor ?? '#333333';
    this.meshColor = hexToInt(meshHex);
    this.meshMetalness = appearance?.meshMetalness ?? 0.1;
    this.meshRoughness = appearance?.meshRoughness ?? 0.6;
    this.edgeColor = hexToInt(appearance?.edgeColor ?? '#000000');
    this.edgeOpacity = appearance?.edgeOpacity ?? 0.25;
    this.edgeThreshold = appearance?.edgeThreshold ?? 40;
    this.bgColor = hexToInt(bg);
    this.bed = appearance?.bed ?? 'XZ';
    this.gridSize = appearance?.gridSize ?? 250;
    this.gridSpacing = appearance?.gridSpacing ?? 10;

    const canvas = document.createElement('canvas');
    container.appendChild(canvas);

    // Renderer
    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true, preserveDrawingBuffer: true });
    this.renderer.setPixelRatio(window.devicePixelRatio);
    this.renderer.setClearColor(this.bgColor);

    // Scene
    this.scene = new THREE.Scene();

    // Perspective camera (3D mode) — position depends on which plane is the "floor"
    this.perspCamera = new THREE.PerspectiveCamera(60, 1, 0.1, 1000);
    if (this.bed === 'XY') {
      // XY is floor, Z is up
      this.perspCamera.up.set(0, 0, 1);
      this.perspCamera.position.set(20, -20, 15);
    } else if (this.bed === 'YZ') {
      // YZ is floor, X is up
      this.perspCamera.up.set(1, 0, 0);
      this.perspCamera.position.set(15, 20, -20);
    } else {
      // XZ is floor, Y is up (Three.js default)
      this.perspCamera.position.set(20, 15, 20);
    }
    this.perspCamera.lookAt(0, 0, 0);

    // Perspective controls
    this.perspControls = new OrbitControls(this.perspCamera, this.renderer.domElement);
    this.perspControls.enableDamping = true;
    this.perspControls.dampingFactor = 0.1;

    // Orthographic camera (drawing mode)
    this.orthoCamera = new THREE.OrthographicCamera(-10, 10, 10, -10, 0.01, 10000);
    this.orthoCamera.position.set(100, 100, 100);
    this.orthoCamera.lookAt(0, 0, 0);

    // Orthographic controls — disabled until drawing mode activates
    this.orthoControls = new OrbitControls(this.orthoCamera, this.renderer.domElement);
    this.orthoControls.enableDamping = true;
    this.orthoControls.dampingFactor = 0.1;
    this.orthoControls.enabled = false;

    // Default to perspective
    this.activeCamera = this.perspCamera;
    this.activeControls = this.perspControls;

    // Lighting
    this.savedAmbientIntensity = appearance?.ambientIntensity ?? 0.8;
    this.ambientLight = new THREE.AmbientLight(0xffffff, this.savedAmbientIntensity);
    this.scene.add(this.ambientLight);

    const dirLight = new THREE.DirectionalLight(0xffffff, 0.6);
    dirLight.position.set(10, 20, 10);
    this.scene.add(dirLight);

    const dirLight2 = new THREE.DirectionalLight(0xffffff, 0.3);
    dirLight2.position.set(-10, -5, -10);
    this.scene.add(dirLight2);

    // Grid helper — positioned in positive quadrant only
    this.grid = this._createGrid(
      hexToInt(gridMajor),
      hexToInt(gridMinor),
    );
    this.scene.add(this.grid);

    // Axes helper
    this.axes = new THREE.AxesHelper(10);
    this.scene.add(this.axes);

    // X/Y/Z axis labels
    this.axisLabels = this._createAxisLabels();
    for (const lbl of this.axisLabels) this.scene.add(lbl);

    // Dimension overlays live in their own groups so we can clear them wholesale.
    this.scene.add(this.measurementGroup);
    this.scene.add(this.hoverMeasurementGroup);

    // Initial size
    this.resize();

    // Resize handling via ResizeObserver
    this.resizeObserver = new ResizeObserver(() => this.resize());
    this.resizeObserver.observe(this.container);

    // Face-click detection (distinguish click from drag) + keyboard orbital controls
    container.setAttribute('tabindex', '0');
    container.style.outline = 'none';

    const onMouseDown = (e: MouseEvent) => {
      this.pickMouseDown = { x: e.clientX, y: e.clientY, time: Date.now() };
      container.focus();
    };
    const onMouseUp = (e: MouseEvent) => {
      if (!this.pickMouseDown) return;
      const dx = e.clientX - this.pickMouseDown.x;
      const dy = e.clientY - this.pickMouseDown.y;
      const dist = Math.sqrt(dx * dx + dy * dy);
      const elapsed = Date.now() - this.pickMouseDown.time;
      this.pickMouseDown = null;
      if (dist < 5 && elapsed < 300) {
        if (this.measureMode === 'placing' && e.button === 0) {
          this.handleMeasurePick(e);
        } else {
          this.handleFacePick(e);
        }
      }
    };
    const onMouseMove = (e: MouseEvent) => {
      if (this.measureMode === 'placing' || this.hoverReadout) {
        this.updateMeasurementHover(e);
      }
    };
    const onFocus = () => {
      this.viewportFocused = true;
      this.showControlsOverlay();
    };
    const onBlur = () => {
      this.viewportFocused = false;
      this.keysDown.clear();
      this.hideControlsOverlay();
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (!this.viewportFocused) return;
      if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Equal', 'Minus', 'NumpadAdd', 'NumpadSubtract'].includes(e.code)) {
        e.preventDefault();
        this.keysDown.add(e.code);
      }
      if (e.code === 'KeyM' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault();
        this.setMeasureMode(this.measureMode === 'placing' ? 'off' : 'placing');
      } else if (e.code === 'Escape' && this.measureMode === 'placing') {
        e.preventDefault();
        this.cancelMeasureStep();
      }
    };
    const onKeyUp = (e: KeyboardEvent) => {
      this.keysDown.delete(e.code);
    };

    // Right-click / two-finger-click while measuring: treat as Escape. Bound
    // unconditionally (harmless when not measuring) and always consumed in
    // measure mode so the browser context menu doesn't pop.
    const onContextMenu = (e: MouseEvent) => {
      if (this.measureMode !== 'placing') return;
      e.preventDefault();
      this.cancelMeasureStep();
    };

    container.addEventListener('mousedown', onMouseDown);
    container.addEventListener('mouseup', onMouseUp);
    container.addEventListener('mousemove', onMouseMove);
    container.addEventListener('contextmenu', onContextMenu);
    container.addEventListener('focus', onFocus);
    container.addEventListener('blur', onBlur);
    container.addEventListener('keydown', onKeyDown);
    container.addEventListener('keyup', onKeyUp);

    // Start render loop
    this.animate();
  }

  private createMeshMaterial(opts: { vertexColors?: boolean; color?: number; transparent?: boolean }): THREE.MeshStandardMaterial {
    const mat = new THREE.MeshStandardMaterial({
      metalness: this.meshMetalness,
      roughness: this.meshRoughness,
      flatShading: true,
      side: THREE.DoubleSide,
      ...opts,
    });
    if (opts.transparent) {
      // depthWrite=false keeps translucent faces from occluding each other
      // depending on draw order; the trade-off is occasional sort artifacts,
      // which is the standard Three.js approach for non-sorted alpha.
      mat.depthWrite = false;
    }
    return mat;
  }

  loadDecodedMesh(decoded: DecodedMesh): void {
    if (!decoded.vertices || decoded.vertices.length === 0) {
      console.warn('Empty mesh data received');
      return;
    }

    const hasFaceColors = decoded.faceColors && Object.keys(decoded.faceColors).length > 0
      && decoded.faceGroups;
    const useVertexColors = !!decoded.faceGroups;

    let mesh: THREE.Mesh<THREE.BufferGeometry, THREE.Material>;

    if (useVertexColors) {
      // Color-resolution path: both expanded and indexed branches need
      // the same fgId→RGBA logic, with hasTransparency tracked across
      // the whole mesh so the material can opt into transparency once.
      const nTris = decoded.faceGroups!.length;
      const colors = new Float32Array(nTris * 3 * 4);
      const defaultColor = new THREE.Color(this.meshColor);
      type RGBA = [number, number, number, number];
      const colorMap = new Map<number, RGBA>();
      let hasTransparency = false;
      if (hasFaceColors) {
        for (const [id, hex] of Object.entries(decoded.faceColors!)) {
          const rgbHex = hex.length > 7 ? hex.substring(0, 7) : hex;
          const a = hex.length > 7 ? parseInt(hex.substring(7, 9), 16) / 255 : 1;
          if (a < 1) hasTransparency = true;
          const c = new THREE.Color(rgbHex);
          colorMap.set(Number(id), [c.r, c.g, c.b, a]);
        }
      }
      const dr = defaultColor.r, dg = defaultColor.g, db = defaultColor.b;
      for (let t = 0; t < nTris; t++) {
        const fgId = decoded.faceGroups![t];
        const c = hasFaceColors ? colorMap.get(fgId) : undefined;
        const r = c ? c[0] : dr, g = c ? c[1] : dg, b = c ? c[2] : db;
        const a = c ? c[3] : 1;
        const base = t * 12;
        colors[base]     = r; colors[base + 1] = g; colors[base + 2]  = b; colors[base + 3]  = a;
        colors[base + 4] = r; colors[base + 5] = g; colors[base + 6]  = b; colors[base + 7]  = a;
        colors[base + 8] = r; colors[base + 9] = g; colors[base + 10] = b; colors[base + 11] = a;
      }

      const material = this.createMeshMaterial({ vertexColors: true, transparent: hasTransparency });

      if (decoded.expanded) {
        const geometry = new THREE.BufferGeometry();
        geometry.setAttribute('position', new THREE.BufferAttribute(decoded.expanded, 3));
        geometry.setAttribute('color', new THREE.BufferAttribute(colors, 4));
        mesh = new THREE.Mesh<THREE.BufferGeometry, THREE.Material>(geometry, material);
      } else {
        const geometry = new THREE.BufferGeometry();
        geometry.setAttribute('position', new THREE.BufferAttribute(decoded.vertices, 3));
        if (decoded.indices && decoded.indices.length > 0) {
          geometry.setIndex(new THREE.BufferAttribute(decoded.indices, 1));
        }
        const nonIndexed = geometry.toNonIndexed();
        nonIndexed.setAttribute('color', new THREE.BufferAttribute(colors, 4));
        mesh = new THREE.Mesh<THREE.BufferGeometry, THREE.Material>(nonIndexed, material);
        geometry.dispose();
      }
    } else {
      // No face groups — simple material. Either pre-expanded positions, or
      // indexed geometry (debug step meshes and anything the backend hasn't
      // pre-expanded).
      const geometry = new THREE.BufferGeometry();
      if (decoded.expanded) {
        geometry.setAttribute('position', new THREE.BufferAttribute(decoded.expanded, 3));
      } else {
        geometry.setAttribute('position', new THREE.BufferAttribute(decoded.vertices, 3));
        if (decoded.indices && decoded.indices.length > 0) {
          geometry.setIndex(new THREE.BufferAttribute(decoded.indices, 1));
        }
      }
      mesh = new THREE.Mesh<THREE.BufferGeometry, THREE.Material>(geometry,
        this.createMeshMaterial({ color: this.meshColor }));
    }

    // Store raw data for face-group wireframe (needed even if geometry was converted to non-indexed)
    if (decoded.faceGroups && decoded.indices) {
      mesh.userData.fgVertices   = decoded.vertices;
      mesh.userData.fgIndices    = decoded.indices;
      mesh.userData.fgFaceGroups = decoded.faceGroups;
    }

    // Store faceGroups for posMap-based hover highlighting
    if (decoded.faceGroups) {
      mesh.userData.faceGroups = decoded.faceGroups;
      // faceColors needed to identify theme-colored faces on theme change
      mesh.userData.faceColors = decoded.faceColors ?? null;
    }

    // edgeLines feed lazy circular-edge detection (built on first snap).
    if (decoded.edgeLines) {
      mesh.userData.edgeLines = decoded.edgeLines;
    }

    this.scene.add(mesh);
    this.userMeshes.push(mesh);

    // Edge lines: use pre-computed from Go/C++ if available, else compute on CPU
    if (decoded.edgeLines && decoded.edgeLines.length > 0 && !this.drawingMode && this.edgeOpacity > 0) {
      const edgesGeo = new THREE.BufferGeometry();
      edgesGeo.setAttribute('position', new THREE.BufferAttribute(decoded.edgeLines, 3));
      const edgesMat = new THREE.LineBasicMaterial({ color: this.edgeColor, transparent: true, opacity: this.edgeOpacity });
      const edgeLines = new THREE.LineSegments(edgesGeo, edgesMat);
      this.scene.add(edgeLines);
      this.userMeshes.push(edgeLines);
    } else if (!decoded.edgeLines && !this.drawingMode && this.edgeOpacity > 0) {
      const edgesGeo = new THREE.EdgesGeometry(mesh.geometry, this.edgeThreshold);
      const edgesMat = new THREE.LineBasicMaterial({ color: this.edgeColor, transparent: true, opacity: this.edgeOpacity });
      const edgeLines = new THREE.LineSegments(edgesGeo, edgesMat);
      this.scene.add(edgeLines);
      this.userMeshes.push(edgeLines);
    }

    // In drawing mode, replace material and add patent-style overlays immediately
    if (this.drawingMode) {
      this._applyDrawingMaterial(mesh);
    }

    // In wireframe mode, apply wireframe material to newly added meshes
    if (this.wireframeMode) {
      this.wireframeOriginalMaterials.set(mesh, mesh.material);
      (mesh.material as any) = new THREE.MeshBasicMaterial({
        color: this.meshColor,
        wireframe: true,
      });
    }
  }

  /** Set the position map from the evaluator for hover highlighting.
   *  excludeFiles is a set of file paths to exclude from face-click cycling (e.g., stdlib). */
  setPosMap(entries: PosEntry[], excludeFiles?: Set<string>): void {
    this.posMap = entries;
    // Build reverse index: faceGroupID → source positions (multiple entries per face)
    this.reversePosMap.clear();
    this.lastPickedFaceGroup = -1;
    this.lastPickCycleIndex = 0;
    for (const entry of entries) {
      // Skip entries from excluded files (stdlib, etc.)
      if (excludeFiles && excludeFiles.has(entry.file)) continue;
      for (const faceID of entry.faceIDs) {
        let list = this.reversePosMap.get(faceID);
        if (!list) {
          list = [];
          this.reversePosMap.set(faceID, list);
        }
        // Avoid duplicate entries for same file/line/col
        if (!list.some(e => e.file === entry.file && e.line === entry.line && e.col === entry.col)) {
          list.push(entry);
        }
      }
    }
  }

  /** Highlight faces at the given source position by dimming non-matching faces. */
  highlightAtPos(file: string, line: number, col: number): void {
    // Find the closest posMap entry on or before this line/col
    const faceIDSet = this.findFaceIDsAtPos(file, line, col);
    if (!faceIDSet) {
      this.clearHighlight();
      return;
    }

    for (const obj of this.userMeshes) {
      if (!(obj instanceof THREE.Mesh)) continue;
      const mesh = obj as THREE.Mesh<THREE.BufferGeometry, THREE.Material>;
      const faceGroups = mesh.userData.faceGroups as Uint32Array | undefined;
      const colorAttr = mesh.geometry.getAttribute('color') as THREE.BufferAttribute | undefined;
      if (!colorAttr || !faceGroups) continue;

      // Save original colors and material state on first highlight
      if (!mesh.userData.originalColors) {
        mesh.userData.originalColors = (colorAttr.array as Float32Array).slice();
      }
      const mat = mesh.material as THREE.MeshStandardMaterial;
      if (mesh.userData.originalTransparent === undefined) {
        mesh.userData.originalTransparent = mat.transparent;
        mesh.userData.originalOpacity = mat.opacity;
        mesh.userData.originalDepthWrite = mat.depthWrite;
      }

      // Enable transparency for non-highlighted faces
      mat.transparent = true;
      mat.opacity = 1.0;
      mat.depthWrite = true;

      const origColors = mesh.userData.originalColors as Float32Array;
      const colors = colorAttr.array as Float32Array;
      const nTris = faceGroups.length;
      const dimAlpha = 0.12;

      for (let t = 0; t < nTris; t++) {
        const matched = faceIDSet.has(faceGroups[t]);
        for (let v = 0; v < 3; v++) {
          const base = t * 12 + v * 4;
          colors[base + 0] = origColors[base + 0];
          colors[base + 1] = origColors[base + 1];
          colors[base + 2] = origColors[base + 2];
          colors[base + 3] = matched ? 1.0 : dimAlpha;
        }
      }
      colorAttr.needsUpdate = true;
    }
  }

  /** Clear any face highlighting, restoring original colors and material. */
  clearHighlight(): void {
    for (const obj of this.userMeshes) {
      if (!(obj instanceof THREE.Mesh)) continue;
      const mesh = obj as THREE.Mesh<THREE.BufferGeometry, THREE.Material>;
      if (!mesh.userData.originalColors) continue;
      const colorAttr = mesh.geometry.getAttribute('color') as THREE.BufferAttribute | undefined;
      if (!colorAttr) continue;
      (colorAttr.array as Float32Array).set(mesh.userData.originalColors);
      colorAttr.needsUpdate = true;

      // Restore material state
      if (mesh.userData.originalTransparent !== undefined) {
        const mat = mesh.material as THREE.MeshStandardMaterial;
        mat.transparent = mesh.userData.originalTransparent;
        mat.opacity = mesh.userData.originalOpacity;
        // Restore the saved depthWrite — a translucent material is created with
        // depthWrite=false; hardcoding true here left it writing depth after the
        // first highlight, so faces behind it vanished by draw order.
        mat.depthWrite = mesh.userData.originalDepthWrite;
        delete mesh.userData.originalTransparent;
        delete mesh.userData.originalOpacity;
        delete mesh.userData.originalDepthWrite;
      }
    }
  }

  /** Register a callback for face-click → source navigation. */
  setOnFaceClick(cb: (file: string, line: number, col: number) => void): void {
    this.onFaceClickCb = cb;
  }

  /** Handle a face pick (click, not drag) in the viewport. */
  private handleFacePick(e: MouseEvent): void {
    // Clear previous highlight before raycasting so material state is clean
    this.clearHighlight();

    const rc = this.raycastMeshes(e);
    if (!rc) {
      this.onFaceClickCb?.('', 0, 0);
      return;
    }
    const mesh = rc.mesh as THREE.Mesh<THREE.BufferGeometry, THREE.Material>;
    const faceGroups = mesh.userData.faceGroups as Uint32Array | undefined;
    if (!faceGroups || rc.hit.faceIndex == null) {
      return;
    }

    const faceGroupID = faceGroups[rc.hit.faceIndex];
    const entries = this.reversePosMap.get(faceGroupID);

    if (!entries || entries.length === 0) {
      return;
    }

    // Cycle through entries on repeated clicks of the same face group
    if (faceGroupID === this.lastPickedFaceGroup) {
      this.lastPickCycleIndex = (this.lastPickCycleIndex + 1) % entries.length;
    } else {
      this.lastPickedFaceGroup = faceGroupID;
      this.lastPickCycleIndex = 0;
    }

    const entry = entries[this.lastPickCycleIndex];
    // Highlight the matching faces
    this.highlightAtPos(entry.file, entry.line, entry.col);
    // Navigate to source
    this.onFaceClickCb?.(entry.file, entry.line, entry.col);
  }

  // -------------------------------------------------------------------------
  // Measurement / dimensioning
  // -------------------------------------------------------------------------

  /** Register a callback invoked whenever the measure-mode state changes. */
  setOnMeasureModeChange(cb: (mode: 'off' | 'placing', hoverOn: boolean) => void): void {
    this.onMeasureModeChangeCb = cb;
  }

  /** Public: enter or leave measurement placing mode. */
  setMeasureMode(mode: 'off' | 'placing'): void {
    if (this.measureMode === mode) return;
    this.measureMode = mode;
    if (mode === 'off') {
      this.clearPending();
      this.clearHoverMeasurement();
      // Release the snap-glyph cursor so normal orbit/zoom affordances return.
      this.renderer.domElement.style.cursor = '';
    } else {
      // Crosshair while measuring, overridden by a glyph-cursor when a snap
      // target is under the mouse.
      this.setMeasureCursor(null);
    }
    this.onMeasureModeChangeCb?.(mode, this.hoverReadout);
  }

  /** Public: toggle the always-on hover readout (independent of placing mode). */
  setHoverReadout(on: boolean): void {
    if (this.hoverReadout === on) return;
    this.hoverReadout = on;
    if (!on) this.clearHoverMeasurement();
    this.onMeasureModeChangeCb?.(this.measureMode, this.hoverReadout);
  }

  /** Public: place an extents box around all loaded user meshes. */
  showExtents(): void {
    // Use a world-space bounding box so the labels sit on the actual geometry.
    const box = new THREE.Box3();
    let any = false;
    for (const obj of this.userMeshes) {
      if (!(obj instanceof THREE.Mesh)) continue;
      box.expandByObject(obj);
      any = true;
    }
    if (!any || box.isEmpty()) return;
    const m: Measurement = {
      kind: 'extents',
      min: [box.min.x, box.min.y, box.min.z],
      max: [box.max.x, box.max.y, box.max.z],
    };
    this.measurements.push(m);
    const g = buildMeasurementGroup(m, this.measurementStyle, { labelWorldHeight: this.measurementLabelHeight() });
    this.measurementGroup.add(g);
  }

  /** Remove and dispose every child of a measurement overlay group. */
  private static emptyGroup(g: THREE.Group): void {
    while (g.children.length > 0) {
      const c = g.children[0];
      g.remove(c);
      disposeMeasurementGroup(c);
    }
  }

  /** Public: discard all placed dimensions. */
  clearMeasurements(): void {
    this.measurements = [];
    Viewer.emptyGroup(this.measurementGroup);
    this.clearPending();
    this.clearHoverMeasurement();
  }

  private clearPending(): void {
    this.pendingSnap = null;
    this.chainPrevSnap = null;
    if (this.pendingMarker) {
      this.measurementGroup.remove(this.pendingMarker);
      disposeMeasurementGroup(this.pendingMarker);
      this.pendingMarker = null;
    }
  }

  /** Move the pending marker to `s`. Creates it if missing. */
  private setPendingMarker(s: Snap): void {
    if (this.pendingMarker) {
      this.measurementGroup.remove(this.pendingMarker);
      disposeMeasurementGroup(this.pendingMarker);
      this.pendingMarker = null;
    }
    this.pendingMarker = buildPendingMarker(s, {
      color: this.measurementStyle.lineColor,
      worldSize: this.measurementLabelHeight() * 0.15,
    });
    this.measurementGroup.add(this.pendingMarker);
  }

  private clearHoverMeasurement(): void {
    Viewer.emptyGroup(this.hoverMeasurementGroup);
  }

  /** Scale sprite labels so they stay readable across bed sizes. */
  private measurementLabelHeight(): number {
    // Roughly 2% of bed grid — mirrors the scale used for axis labels.
    return Math.max(2, this.gridSize * 0.01);
  }

  /** Run a raycast against loaded meshes and return the first intersection, or null. */
  private raycastMeshes(e: MouseEvent): { hit: THREE.Intersection; mesh: THREE.Mesh; cursorPx: { x: number; y: number }; viewportSize: { w: number; h: number } } | null {
    const rect = this.renderer.domElement.getBoundingClientRect();
    const cursorPx = { x: e.clientX - rect.left, y: e.clientY - rect.top };
    const mouse = new THREE.Vector2(
      (cursorPx.x / rect.width) * 2 - 1,
      -(cursorPx.y / rect.height) * 2 + 1,
    );
    this.raycaster.setFromCamera(mouse, this.activeCamera);
    const meshOnly = this.userMeshes.filter(o => o instanceof THREE.Mesh);
    const intersects = this.raycaster.intersectObjects(meshOnly, false);
    if (intersects.length === 0) return null;
    return {
      hit: intersects[0],
      mesh: intersects[0].object as THREE.Mesh,
      cursorPx,
      viewportSize: { w: rect.width, h: rect.height },
    };
  }

  /** Extract the measurement cache and raw mesh arrays from a hit's mesh. */
  private meshMeasurementData(mesh: THREE.Mesh): {
    vertices: Float32Array;
    indices: Uint32Array;
    faceGroups?: Uint32Array;
    edgeLines?: Float32Array;
    cache: MeasurementCache;
  } | null {
    const vertices = mesh.userData.fgVertices as Float32Array | undefined
      ?? (mesh.geometry.getAttribute('position')?.array as Float32Array | undefined);
    const indices = mesh.userData.fgIndices as Uint32Array | undefined;
    if (!vertices || !indices) return null;
    const faceGroups = mesh.userData.faceGroups as Uint32Array | undefined;
    const edgeLines = mesh.userData.edgeLines as Float32Array | undefined;
    // Build the measurement cache lazily on first snap and memoize it on the mesh.
    // Face planes + circular-edge fitting are only needed while measuring, so they
    // are not computed on every decode. Face planes need per-triangle face groups.
    let cache = mesh.userData.measurementCache as MeasurementCache | undefined;
    if (!cache) {
      if (!faceGroups) return null;
      cache = {
        facePlanes: computeFacePlanes(vertices, indices, faceGroups),
        circularEdges: edgeLines ? detectCircularEdges(edgeLines) : [],
      };
      mesh.userData.measurementCache = cache;
    }
    return { vertices, indices, faceGroups, edgeLines, cache };
  }

  /**
   * Handle a measurement click. Click 1 anchors the chain; each subsequent
   * click commits a linear dim from the previous chain point and, when a
   * prior segment exists, a corner-angle at the chain vertex. Escape (or
   * exiting Measure mode) ends the chain.
   *
   * Radial: click the circle center to anchor, then drag to the same circle's
   * rim (a circleEdge snap) — that pair classifies as radial via
   * buildMeasurement and terminates the chain (it's a complete standalone dim).
   */
  private handleMeasurePick(e: MouseEvent): void {
    const snap = this.snapAtEvent(e);
    if (!snap) return;
    const h = this.measurementLabelHeight();

    // Empty chain: anchor from any snap kind — including circle center.
    if (!this.pendingSnap) {
      this.pendingSnap = snap;
      this.setPendingMarker(snap);
      return;
    }

    // Continuing a chain: build the new segment (linear, or radial if the
    // pair is center+edge of the same circle).
    const m = buildMeasurement(this.pendingSnap, snap);
    if (m) {
      this.measurements.push(m);
      this.measurementGroup.add(buildMeasurementGroup(m, this.measurementStyle, { labelWorldHeight: h }));
    }
    // Radial is a complete terminal measurement — a chain through a center+rim
    // pair into a further linear leg is geometrically confused. End the chain.
    if (m && m.kind === 'radial') {
      this.clearPending();
      this.clearHoverMeasurement();
      return;
    }
    if (this.chainPrevSnap) {
      const corner = buildCornerAngle(this.chainPrevSnap, this.pendingSnap, snap);
      if (corner) {
        this.measurements.push(corner);
        this.measurementGroup.add(buildMeasurementGroup(corner, this.measurementStyle, { labelWorldHeight: h }));
      }
    }
    this.chainPrevSnap = this.pendingSnap;
    this.pendingSnap = snap;
    this.setPendingMarker(snap);
  }

  /** Live hover readout + second-pick preview line. */
  private updateMeasurementHover(e: MouseEvent): void {
    this.clearHoverMeasurement();
    const snap = this.snapAtEvent(e);
    // Reflect the current snap on the cursor itself — the glyph moves with the
    // mouse, so the user never has to hunt for a small on-model marker.
    this.setMeasureCursor(snap?.kind ?? null);
    if (!snap) return;

    // Highlight the geometry feature under the cursor (edge/loop). The glyph
    // is carried by the cursor, so no sprite is added here.
    this.hoverMeasurementGroup.add(
      buildPendingMarker(snap, {
        color: this.measurementStyle.lineColor,
        worldSize: this.measurementLabelHeight() * 0.12,
        sprite: false,
      }),
    );

    if (this.pendingSnap) {
      // Show the pending → hover preview dimension.
      const h = this.measurementLabelHeight();
      const m = buildMeasurement(this.pendingSnap, snap);
      if (m) this.hoverMeasurementGroup.add(
        buildMeasurementGroup(m, this.measurementStyle, { labelWorldHeight: h }),
      );
      // While a chain is in progress and one endpoint sits on an edge, annotate
      // the edge-to-preview-dim angle at that endpoint. Transient (lives on
      // hoverMeasurementGroup) so it disappears on the next pointermove or on
      // click-commit — chained 3+ point corner angles are handled separately.
      if (this.pendingSnap.edge) {
        const arc = buildHoverEdgeAngle(this.pendingSnap, snap, this.measurementStyle, h);
        if (arc) this.hoverMeasurementGroup.add(arc);
      }
      if (snap.edge) {
        const arc = buildHoverEdgeAngle(snap, this.pendingSnap, this.measurementStyle, h);
        if (arc) this.hoverMeasurementGroup.add(arc);
      }
    } else if (this.hoverReadout && snap.kind === 'circleCenter') {
      const m = buildRadial(snap);
      if (m) this.hoverMeasurementGroup.add(
        buildMeasurementGroup(m, this.measurementStyle, { labelWorldHeight: this.measurementLabelHeight() }),
      );
    }
  }

  /**
   * Shared Escape / right-click handler while measuring. If a chain is in
   * progress, end the chain but keep placed dims and stay in Measure mode so
   * the user can immediately start another. Otherwise, exit Measure entirely.
   */
  private cancelMeasureStep(): void {
    if (this.measureMode !== 'placing') return;
    if (this.pendingSnap) {
      this.clearPending();
      this.clearHoverMeasurement();
    } else {
      this.setMeasureMode('off');
    }
  }

  /** Set the canvas cursor to the snap glyph (or plain crosshair) while
   *  measuring. Only touches the DOM when the value would change. */
  private setMeasureCursor(kind: Snap['kind'] | null): void {
    const el = this.renderer.domElement;
    const css = snapCursorCSS(kind, this.measurementStyle.lineColor);
    if (el.style.cursor !== css) el.style.cursor = css;
  }

  /** Raycast + snap resolve in one. Returns null if no mesh is under the cursor. */
  private snapAtEvent(e: MouseEvent): Snap | null {
    const rc = this.raycastMeshes(e);
    if (!rc) return null;
    const data = this.meshMeasurementData(rc.mesh);
    if (!data) return null;
    rc.mesh.updateWorldMatrix(true, false);
    return resolveSnap({
      hit: rc.hit,
      vertices: data.vertices,
      indices: data.indices,
      faceGroups: data.faceGroups,
      edgeLines: data.edgeLines,
      cache: data.cache,
      camera: this.activeCamera,
      cursorPx: rc.cursorPx,
      viewportSize: rc.viewportSize,
      matrixWorld: rc.mesh.matrixWorld,
      pending: this.pendingSnap,
    });
  }

  /** Find the closest posMap entry at or before (line, col) on the same line, filtered by file. */
  private findFaceIDsAtPos(file: string, line: number, col: number): Set<number> | null {
    if (this.posMap.length === 0) return null;

    // Pick the entry on this line closest to col. Prefer the largest col
    // <= cursor; if nothing sits at or before the cursor, take the first
    // entry after it (cursor is to the left of every entry on the line).
    let best: PosEntry | null = null;
    let firstAfter: PosEntry | null = null;
    for (const entry of this.posMap) {
      if (entry.file !== file) continue;
      if (entry.line !== line) continue;
      if (entry.col <= col) {
        if (!best || entry.col > best.col) {
          best = entry;
        }
      } else {
        if (!firstAfter || entry.col < firstAfter.col) {
          firstAfter = entry;
        }
      }
    }
    if (!best) best = firstAfter;
    if (!best || best.faceIDs.length === 0) return null;
    return new Set(best.faceIDs);
  }

  loadDebugStep(step: DebugStepData, binary: ArrayBuffer): void {
    this.clearMeshes();

    const styleMap = STEP_STYLES[step.op] ?? STEP_STYLES._default;

    for (const dm of step.meshes ?? []) {
      if (!dm.mesh) continue;
      const style = styleMap[dm.role] ?? STEP_STYLES._default[dm.role] ?? STEP_STYLES._default.result;
      const decoded = decodeBinaryMesh(binary, dm.mesh);
      if (!decoded.vertices || decoded.vertices.length === 0) continue;

      const geometry = new THREE.BufferGeometry();
      geometry.setAttribute('position', new THREE.BufferAttribute(decoded.vertices, 3));

      if (decoded.indices && decoded.indices.length > 0) {
        geometry.setIndex(new THREE.BufferAttribute(decoded.indices, 1));
      }

      const transparent = style.opacity < 1;
      const color = dm.role === 'result' ? this.meshColor : style.color;
      const material = new THREE.MeshStandardMaterial({
        color,
        opacity: style.opacity,
        transparent,
        depthWrite: !transparent,
        metalness: this.meshMetalness,
        roughness: this.meshRoughness,
        flatShading: true,
        side: THREE.DoubleSide,
      });

      const mesh = new THREE.Mesh(geometry, material);
      this.scene.add(mesh);
      this.userMeshes.push(mesh);
    }

  }

  clearMeshes(): void {
    // Restore original materials before disposing (drawing/wireframe mode may have swapped them)
    this._restoreDrawingMaterials();
    this._clearDrawingOverlays();
    // Dispose wireframe GPU resources before dropping the references. The boundary
    // LineSegments are children of the meshes, which _disposeObject3D does not
    // traverse; and each mesh now holds the swapped-in wireframe material, so the
    // saved original (vertex-colored) material would otherwise leak on every re-eval.
    for (const lines of this.wireframeLineObjects) {
      lines.geometry.dispose();
      (lines.material as THREE.Material).dispose();
    }
    this.wireframeLineObjects = [];
    for (const mat of this.wireframeOriginalMaterials.values()) {
      if (Array.isArray(mat)) {
        mat.forEach((m) => m.dispose());
      } else {
        mat.dispose();
      }
    }
    this.wireframeOriginalMaterials.clear();

    for (const obj of this.userMeshes) {
      this.scene.remove(obj);
      Viewer._disposeObject3D(obj);
    }
    this.userMeshes = [];

    // Clear dimensions — old face/edge IDs don't refer to anything in a new mesh.
    this.clearMeasurements();
    // Also exit placing mode; the next mesh isn't loaded yet and the pending
    // snap (if any) is anchored to geometry that's gone.
    if (this.measureMode === 'placing') {
      this.setMeasureMode('off');
    }
  }

  /**
   * Load a fresh eval result. Owns the full sequence:
   *   clearMeshes → loadDecodedMesh → setPosMap → fitToView.
   * Callers used to drive these calls imperatively from app.ts —
   * forgetting one in a new code path left the viewer in a partial
   * state. Now the viewer is the only place this sequence is spelled
   * out, and the order can't accidentally drift.
   *
   * The mesh is rendered at the world coordinates the kernel returned
   * (the Solid's bounding box). The camera is fit to those coordinates;
   * the geometry itself is not translated.
   *
   * Pass `decoded: null` for a no-mesh result (check-only / empty);
   * the viewer wipes its state without trying to load anything.
   */
  applyEvalResult(
    decoded: DecodedMesh | null,
    posMap: PosEntry[],
    opts?: { excludeFiles?: Set<string>; autofit?: boolean },
  ): void {
    this.clearMeshes();
    if (decoded) {
      this.loadDecodedMesh(decoded);
    }
    this.setPosMap(posMap, opts?.excludeFiles);
    if (decoded && (opts?.autofit ?? true)) {
      this.fitToView();
    }
  }

  /**
   * Wipe all eval-derived state: meshes, pos map, highlights. Used
   * when no tab is open or when the user toggles out of debug.
   * Camera position is preserved so the next load doesn't yank
   * the view.
   */
  reset(): void {
    this.clearMeshes();
    this.setPosMap([]);
    this.clearHighlight();
  }

  applySettings(appearance: ViewerAppearance): void {
    this.bgColor = hexToInt(appearance.backgroundColor);
    if (!this.drawingMode) {
      this.renderer.setClearColor(this.bgColor);
    }

    this.meshColor = hexToInt(appearance.meshColor);
    this.meshMetalness = appearance.meshMetalness ?? 0.1;
    this.meshRoughness = appearance.meshRoughness ?? 0.6;
    this.edgeColor = hexToInt(appearance.edgeColor ?? '#000000');
    this.edgeOpacity = appearance.edgeOpacity ?? 0.25;
    this.edgeThreshold = appearance.edgeThreshold ?? 40;
    this.savedAmbientIntensity = appearance.ambientIntensity ?? 0.8;
    this.ambientLight.intensity = this.savedAmbientIntensity;
    for (const obj of this.userMeshes) {
      if (obj instanceof THREE.Mesh) {
        // Update the stored original material (in drawing mode) or the live material
        const mat = this.drawingOriginalMaterials.get(obj) ?? obj.material;
        if (mat instanceof THREE.MeshStandardMaterial) {
          if (!mat.vertexColors) {
            mat.color.setHex(this.meshColor);
          } else {
            // Vertex-colored mesh — update any faces that use the default theme color
            // (faces with explicit faceColors are left unchanged).
            const faceGroups = obj.userData.faceGroups as Uint32Array | undefined;
            const faceColors = obj.userData.faceColors as Record<string, string> | null | undefined;
            const colorAttr = obj.geometry.getAttribute('color') as THREE.BufferAttribute | undefined;
            if (faceGroups && colorAttr) {
              const newColor = new THREE.Color(this.meshColor);
              const colors = colorAttr.array as Float32Array;
              // Highlight cache, kept in sync in the same pass when a highlight is active.
              const origColors = obj.userData.originalColors as Float32Array | undefined;
              const nTris = faceGroups.length;
              for (let t = 0; t < nTris; t++) {
                if (!faceColors?.[String(faceGroups[t])]) {
                  for (let v = 0; v < 3; v++) {
                    const base = t * 12 + v * 4;
                    colors[base]     = newColor.r;
                    colors[base + 1] = newColor.g;
                    colors[base + 2] = newColor.b;
                    if (origColors) {
                      origColors[base]     = newColor.r;
                      origColors[base + 1] = newColor.g;
                      origColors[base + 2] = newColor.b;
                    }
                  }
                }
              }
              colorAttr.needsUpdate = true;
            }
          }
          mat.metalness = this.meshMetalness;
          mat.roughness = this.meshRoughness;
        }
      } else if (obj instanceof THREE.LineSegments) {
        const mat = obj.material as THREE.LineBasicMaterial;
        mat.color.setHex(this.edgeColor);
        mat.opacity = this.edgeOpacity;
      }
    }

    // Recreate grid with new colors, plane, and size
    this.scene.remove(this.grid);
    this.grid.geometry.dispose();
    (this.grid.material as THREE.Material).dispose();
    this.bed = appearance.bed ?? 'XZ';
    this.gridSize = appearance.gridSize ?? 250;
    this.gridSpacing = appearance.gridSpacing ?? 10;
    this.grid = this._createGrid(hexToInt(appearance.gridMajorColor), hexToInt(appearance.gridMinorColor));
    this._setSceneDecorVisible(!this.drawingMode);
    this.scene.add(this.grid);

    // Theme-derived measurement colors. Snap glyphs share the line color so
    // they read against the same background as the dimension lines.
    this.measurementStyle = {
      lineColor: appearance.measurementLineColor,
      labelColor: appearance.measurementLabelColor,
      format: appearance.measurementFormat,
    };
    this.rebuildMeasurementVisuals();
  }

  /** Rebuild placed measurement visuals from the current style + measurement list. */
  private rebuildMeasurementVisuals(): void {
    Viewer.emptyGroup(this.measurementGroup);
    for (const m of this.measurements) {
      this.measurementGroup.add(buildMeasurementGroup(m, this.measurementStyle, { labelWorldHeight: this.measurementLabelHeight() }));
    }
    if (this.pendingSnap) {
      this.pendingMarker = buildPendingMarker(this.pendingSnap, { color: this.measurementStyle.lineColor, worldSize: this.measurementLabelHeight() * 0.15 });
      this.measurementGroup.add(this.pendingMarker);
    } else {
      this.pendingMarker = null;
    }
  }

  fitToView(): void {
    if (this.userMeshes.length === 0) return;

    if (this.drawingMode) {
      // Re-fit ortho camera maintaining its current view direction
      const dir = new THREE.Vector3();
      this.orthoCamera.getWorldDirection(dir);
      this._fitOrthoToMeshes(dir.negate());
      return;
    }

    const sphere = this._getMeshBoundingSphere();
    if (sphere.radius <= 0) return;

    const fov = this.perspCamera.fov * (Math.PI / 180);
    const distance = (sphere.radius / Math.sin(fov / 2)) * 1.2;

    // Position camera along current viewing direction from sphere center
    const direction = new THREE.Vector3();
    this.perspCamera.getWorldDirection(direction);
    this.perspCamera.position.copy(sphere.center).addScaledVector(direction, -distance);

    this.perspControls.target.copy(sphere.center);
    this.perspControls.update();
  }

  async toggleHeadTracking(deviceId?: string, yOffset?: number): Promise<boolean> {
    if (this.headTracker) {
      this.headTracker.stop();
      this.headTracker = null;
      return false;
    }
    // Assign the field only after start() succeeds. start() self-stops and
    // rethrows on failure, so a thrown error leaves headTracker null and the
    // caller can surface it (camera denied, no device, …) rather than the
    // button showing "on" over dead tracking.
    const tracker = new HeadTracker();
    await tracker.start(this.container, deviceId, yOffset);
    this.headTracker = tracker;
    return true;
  }

  onFrame(cb: (camera: THREE.Camera) => void): void {
    this.frameCallbacks.push(cb);
  }

  /** Orbit `cam` around `target` by mutating the camera→target spherical
   *  coordinates, then re-derive the camera position and re-aim it. */
  private static orbitSpherical(cam: THREE.Camera, target: THREE.Vector3, mutate: (s: THREE.Spherical) => void): void {
    // THREE.Spherical is Y-up, so orbit in a frame where the camera's up-axis is
    // +Y — the Bed setting can make the real up-axis Z (XY bed) or X (YZ bed),
    // and orbiting Y-up there rotates about the wrong axis (azimuth reads as
    // elevation and vice-versa). Rotate the offset into the Y-up frame, apply the
    // spherical change, then rotate back. OrbitControls does its own up handling,
    // so mouse orbit was already correct; this fixes head-tracking and keyboard.
    const q = new THREE.Quaternion().setFromUnitVectors(cam.up.clone().normalize(), new THREE.Vector3(0, 1, 0));
    const offset = new THREE.Vector3().subVectors(cam.position, target).applyQuaternion(q);
    const s = new THREE.Spherical().setFromVector3(offset);
    mutate(s);
    offset.setFromSpherical(s).applyQuaternion(q.invert());
    cam.position.copy(target).add(offset);
    cam.lookAt(target);
  }

  orbitBy(deltaTheta: number, deltaPhi: number): void {
    Viewer.orbitSpherical(this.perspCamera, this.perspControls.target, (s) => {
      s.theta += deltaTheta;
      s.phi = Math.max(0.01, Math.min(Math.PI - 0.01, s.phi + deltaPhi));
    });
    this.perspControls.update();
  }

  isAutoRotating(): boolean {
    return this.perspControls.autoRotate;
  }

  toggleAutoRotate(): boolean {
    if (this.perspControls.autoRotate) {
      this.perspControls.autoRotate = false;
      return false;
    }
    this.fitToView();
    this.perspControls.autoRotate = true;
    this.perspControls.autoRotateSpeed = 4;
    return true;
  }

  toggleGrid(): void {
    const v = !this.grid.visible;
    this.grid.visible = v;
    for (const lbl of this.gridLabels) lbl.visible = v;
  }

  toggleAxes(): void {
    const v = !this.axes.visible;
    this.axes.visible = v;
    for (const lbl of this.axisLabels) lbl.visible = v;
  }

  setVisible(visible: boolean): void {
    this._visible = visible;
    this.renderer.domElement.style.display = visible ? 'block' : 'none';
  }

  /**
   * Test hook: number of user meshes currently attached to the scene.
   * Wails-mocked integration tests assert this becomes non-zero after
   * an eval response with mesh data lands — catches regressions in
   * the decode/load pipeline that would render an empty viewport.
   */
  meshCount(): number {
    return this.userMeshes.length;
  }

  /** Report the pixel width of overlay drawers covering the right edge
   *  of the canvas. The viewer shifts the camera projection so the
   *  lookAt target appears centred in the visible region rather than
   *  the full canvas. Call with 0 to clear the offset. */
  setRightInset(px: number): void {
    const clamped = Math.max(0, Math.floor(px));
    if (this.rightInset === clamped) return;
    this.rightInset = clamped;
    this.resize();
  }

  /** Switch between 3D perspective mode and CAD drawing (orthographic) mode. */
  setDrawingMode(enabled: boolean): void {
    if (this.drawingMode === enabled) return;
    this.drawingMode = enabled;

    if (enabled) {
      // Swap to orthographic camera/controls
      this._activateOrtho();

      // Drawing appearance: paper-white background, no grid/axes
      this.renderer.setClearColor(0xffffff);
      this._setSceneDecorVisible(false);

      // Apply patent-style drawing material to existing meshes
      for (const obj of this.userMeshes) {
        if (obj instanceof THREE.Mesh) {
          this._applyDrawingMaterial(obj);
        }
      }

    } else {
      // Swap back to perspective camera/controls
      this._activatePerspective();

      // Restore original appearance
      this.renderer.setClearColor(this.bgColor);
      this._setSceneDecorVisible(true);
      this.ambientLight.intensity = this.savedAmbientIntensity;

      // Restore mesh materials and remove drawing overlays
      this._restoreDrawingMaterials();
      this._clearDrawingOverlays();
    }
  }

  /** Switch to/from wireframe view (perspective camera, face-group boundary wireframe). */
  setWireframeMode(enabled: boolean): void {
    if (this.wireframeMode === enabled) return;
    this.wireframeMode = enabled;

    if (enabled) {
      for (const obj of this.userMeshes) {
        if (!(obj instanceof THREE.Mesh)) continue;
        // Replace shaded material with a dark unlit surface so lines stand out
        this.wireframeOriginalMaterials.set(obj, obj.material);
        obj.material = new THREE.MeshBasicMaterial({ color: 0x1a1a2e });

        const { fgVertices, fgIndices, fgFaceGroups } = obj.userData;
        if (fgVertices && fgIndices && fgFaceGroups) {
          // Face-group boundary wireframe — only polygon/region edges
          const geo = buildFaceGroupWireframe(fgVertices, fgIndices, fgFaceGroups);
          const mat = new THREE.LineBasicMaterial({ color: this.meshColor });
          const lines = new THREE.LineSegments(geo, mat);
          obj.add(lines); // parented so it follows mesh transform
          this.wireframeLineObjects.push(lines);
        } else {
          // No face-group data (e.g. debug step meshes) — draw every
          // triangle edge.
          const geo = new THREE.WireframeGeometry(obj.geometry);
          const mat = new THREE.LineBasicMaterial({ color: this.meshColor });
          const lines = new THREE.LineSegments(geo, mat);
          obj.add(lines);
          this.wireframeLineObjects.push(lines);
        }
      }
    } else {
      // Remove wireframe lines
      for (const lines of this.wireframeLineObjects) {
        if (lines.parent) lines.parent.remove(lines);
        Viewer._disposeObject3D(lines);
      }
      this.wireframeLineObjects = [];
      // Restore original materials
      for (const [mesh, mat] of this.wireframeOriginalMaterials) {
        (mesh.material as THREE.Material).dispose();
        mesh.material = mat as THREE.Material;
      }
      this.wireframeOriginalMaterials.clear();
    }
  }

  /** Set camera to a named preset view. Works in both perspective and orthographic projection. */
  setViewpoint(vp: DrawingViewpoint): void {
    type VP = { dir: THREE.Vector3; up: THREE.Vector3 };
    const BED_VIEWS: Record<string, Record<DrawingViewpoint, VP>> = {
      XZ: {
        iso:   { dir: new THREE.Vector3( 1,  1,  1).normalize(), up: new THREE.Vector3(0, 1, 0) },
        home:  { dir: new THREE.Vector3( 1,  1,  1).normalize(), up: new THREE.Vector3(0, 1, 0) },
        top:   { dir: new THREE.Vector3( 0,  1,  0),             up: new THREE.Vector3(0, 0,-1) },
        bot:   { dir: new THREE.Vector3( 0, -1,  0),             up: new THREE.Vector3(0, 0, 1) },
        front: { dir: new THREE.Vector3( 0,  0,  1),             up: new THREE.Vector3(0, 1, 0) },
        back:  { dir: new THREE.Vector3( 0,  0, -1),             up: new THREE.Vector3(0, 1, 0) },
        right: { dir: new THREE.Vector3( 1,  0,  0),             up: new THREE.Vector3(0, 1, 0) },
        left:  { dir: new THREE.Vector3(-1,  0,  0),             up: new THREE.Vector3(0, 1, 0) },
      },
      XY: {
        iso:   { dir: new THREE.Vector3( 1,  1,  1).normalize(), up: new THREE.Vector3(0, 0, 1) },
        home:  { dir: new THREE.Vector3( 1,  1,  1).normalize(), up: new THREE.Vector3(0, 0, 1) },
        top:   { dir: new THREE.Vector3( 0,  0,  1),             up: new THREE.Vector3(0, 1, 0) },
        bot:   { dir: new THREE.Vector3( 0,  0, -1),             up: new THREE.Vector3(0,-1, 0) },
        front: { dir: new THREE.Vector3( 0, -1,  0),             up: new THREE.Vector3(0, 0, 1) },
        back:  { dir: new THREE.Vector3( 0,  1,  0),             up: new THREE.Vector3(0, 0, 1) },
        right: { dir: new THREE.Vector3( 1,  0,  0),             up: new THREE.Vector3(0, 0, 1) },
        left:  { dir: new THREE.Vector3(-1,  0,  0),             up: new THREE.Vector3(0, 0, 1) },
      },
      YZ: {
        iso:   { dir: new THREE.Vector3( 1,  1,  1).normalize(), up: new THREE.Vector3(1, 0, 0) },
        home:  { dir: new THREE.Vector3( 1,  1,  1).normalize(), up: new THREE.Vector3(1, 0, 0) },
        top:   { dir: new THREE.Vector3( 1,  0,  0),             up: new THREE.Vector3(0, 0, 1) },
        bot:   { dir: new THREE.Vector3(-1,  0,  0),             up: new THREE.Vector3(0, 0,-1) },
        front: { dir: new THREE.Vector3( 0,  0,  1),             up: new THREE.Vector3(1, 0, 0) },
        back:  { dir: new THREE.Vector3( 0,  0, -1),             up: new THREE.Vector3(1, 0, 0) },
        right: { dir: new THREE.Vector3( 0,  1,  0),             up: new THREE.Vector3(1, 0, 0) },
        left:  { dir: new THREE.Vector3( 0, -1,  0),             up: new THREE.Vector3(1, 0, 0) },
      },
    };
    const { dir, up } = (BED_VIEWS[this.bed] ?? BED_VIEWS.XZ)[vp];

    if (this.activeCamera === this.orthoCamera) {
      this.orthoCamera.up.copy(up);
      this._fitOrthoToMeshes(dir);
    } else {
      if (this.userMeshes.length === 0) return;
      const sphere = this._getMeshBoundingSphere();
      if (sphere.radius <= 0) return;
      const fov = this.perspCamera.fov * (Math.PI / 180);
      const distance = (sphere.radius * 1.3) / Math.sin(fov / 2);
      this.perspCamera.up.copy(up);
      this.perspCamera.position
        .copy(sphere.center)
        .addScaledVector(dir.clone().normalize(), distance);
      this.perspCamera.lookAt(sphere.center);
      this.perspControls.target.copy(sphere.center);
      this.perspControls.update();
    }
  }

  /** Toggle between perspective and orthographic projection without activating drawing mode. */
  setOrthoProjection(enabled: boolean): void {
    if (this.drawingMode) return;
    const isOrtho = this.activeCamera === this.orthoCamera;
    if (enabled === isOrtho) return;

    if (enabled) {
      this._activateOrtho();
      // Initialise ortho frustum from current persp view direction
      const dir = new THREE.Vector3();
      this.perspCamera.getWorldDirection(dir);
      this.orthoCamera.up.copy(this.perspCamera.up);
      this._fitOrthoToMeshes(dir.negate());
    } else {
      this._activatePerspective();
    }
  }

  private _getMeshBoundingSphere(): THREE.Sphere {
    const box = new THREE.Box3();
    for (const mesh of this.userMeshes) box.expandByObject(mesh);
    const sphere = new THREE.Sphere();
    box.getBoundingSphere(sphere);
    return sphere;
  }

  private _activatePerspective(): void {
    this.orthoControls.enabled = false;
    this.perspControls.enabled = true;
    this.activeCamera = this.perspCamera;
    this.activeControls = this.perspControls;
  }

  private _activateOrtho(): void {
    this.perspControls.enabled = false;
    this.orthoControls.enabled = true;
    this.activeCamera = this.orthoCamera;
    this.activeControls = this.orthoControls;
  }

  private static _disposeObject3D(obj: THREE.Object3D): void {
    if (obj instanceof THREE.Mesh) {
      obj.geometry.dispose();
      if (Array.isArray(obj.material)) {
        obj.material.forEach((m: THREE.Material) => m.dispose());
      } else {
        (obj.material as THREE.Material).dispose();
      }
    } else if (obj instanceof THREE.LineSegments || obj instanceof THREE.Points) {
      obj.geometry.dispose();
      (obj.material as THREE.Material).dispose();
    }
  }

  private _setSceneDecorVisible(visible: boolean): void {
    this.grid.visible = visible;
    for (const lbl of this.gridLabels) lbl.visible = visible;
    this.axes.visible = visible;
    for (const lbl of this.axisLabels) lbl.visible = visible;
  }

  private _fitOrthoToMeshes(dir: THREE.Vector3): void {
    if (this.userMeshes.length === 0) return;

    const sphere = this._getMeshBoundingSphere();
    if (sphere.radius <= 0) return;

    const aspect = this.container.clientWidth / this.container.clientHeight;
    const halfSize = sphere.radius * 1.3;

    this.orthoCamera.left = -halfSize * aspect;
    this.orthoCamera.right = halfSize * aspect;
    this.orthoCamera.top = halfSize;
    this.orthoCamera.bottom = -halfSize;
    this.orthoCamera.near = 0.01;
    this.orthoCamera.far = sphere.radius * 20 + 10;
    this.orthoCamera.zoom = 1;
    this.orthoCamera.updateProjectionMatrix();

    const camDist = sphere.radius * 8;
    this.orthoCamera.position
      .copy(sphere.center)
      .addScaledVector(dir.clone().normalize(), camDist);
    this.orthoCamera.lookAt(sphere.center);

    this.orthoControls.target.copy(sphere.center);
    this.orthoControls.update();
  }

  private _addDrawingOverlaysForMesh(mesh: THREE.Mesh): void {
    // Parent overlays to the mesh so they follow its position/rotation exactly.

    // Silhouette outline: back-face rendered slightly larger gives a clean contour
    const outlineMat = new THREE.MeshBasicMaterial({ color: 0x111111, side: THREE.BackSide });
    const outlineMesh = new THREE.Mesh(mesh.geometry, outlineMat);
    outlineMesh.scale.setScalar(1.006);
    mesh.add(outlineMesh);
    this.drawingOverlays.push(outlineMesh);

    // Feature edges only (25° threshold skips smooth tessellation, keeps real corners)
    const edgesGeo = new THREE.EdgesGeometry(mesh.geometry, 25);
    const linesMat = new THREE.LineBasicMaterial({ color: 0x111111 });
    const lines = new THREE.LineSegments(edgesGeo, linesMat);
    mesh.add(lines);
    this.drawingOverlays.push(lines);

    if (this.hiddenLinesEnabled) {
      this._addHiddenLinesForMesh(mesh, edgesGeo);
    }
  }

  private _addHiddenLinesForMesh(mesh: THREE.Mesh, edgesGeo?: THREE.EdgesGeometry): void {
    const geo = edgesGeo ?? new THREE.EdgesGeometry(mesh.geometry, 25);
    const mat = new THREE.LineDashedMaterial({
      color: 0xaaaaaa,
      dashSize: 3,
      gapSize: 2,
    });
    mat.depthFunc = THREE.GreaterDepth;
    mat.depthWrite = false;
    const hiddenLines = new THREE.LineSegments(geo, mat);
    hiddenLines.computeLineDistances();
    mesh.add(hiddenLines);
    this.hiddenLineObjects.push(hiddenLines);
  }

  private _clearHiddenLines(): void {
    for (const obj of this.hiddenLineObjects) {
      if (obj.parent) obj.parent.remove(obj);
      obj.geometry.dispose();
      (obj.material as THREE.Material).dispose();
    }
    this.hiddenLineObjects = [];
  }

  /** Toggle dashed hidden-line display in drawing modes. Returns new state. */
  setHiddenLines(enabled: boolean): void {
    this.hiddenLinesEnabled = enabled;
    if (!this.drawingMode) return;
    if (enabled) {
      for (const obj of this.userMeshes) {
        if (obj instanceof THREE.Mesh) this._addHiddenLinesForMesh(obj);
      }
    } else {
      this._clearHiddenLines();
    }
  }

  private _clearDrawingOverlays(): void {
    this._clearHiddenLines();
    for (const obj of this.drawingOverlays) {
      if (obj.parent) {
        obj.parent.remove(obj);
      }
      if (obj instanceof THREE.Mesh) {
        // Geometry is shared with the original mesh — only dispose the material
        (obj.material as THREE.Material).dispose();
      } else {
        Viewer._disposeObject3D(obj);
      }
    }
    this.drawingOverlays = [];
  }

  /** Create a grid in the positive quadrant for the current plane and size. */
  private _createGrid(majorColor: number, minorColor: number): THREE.LineSegments {
    const size = this.gridSize;
    const spacing = this.gridSpacing;

    // Build line positions at 0, spacing, 2*spacing, …, size
    // Last line is always at exactly `size` (squeezed last cell).
    const stops: number[] = [];
    for (let v = 0; v <= size; v += spacing) stops.push(v);
    if (stops[stops.length - 1] < size) stops.push(size);

    const verts: number[] = [];
    const plane = this.bed;
    for (const s of stops) {
      if (plane === 'XY') {
        // Grid on Z=0, spanning X:[0..size] Y:[0..size]
        verts.push(0, s, 0, size, s, 0);
        verts.push(s, 0, 0, s, size, 0);
      } else if (plane === 'YZ') {
        // Grid on X=0, spanning Y:[0..size] Z:[0..size]
        verts.push(0, s, 0, 0, s, size);
        verts.push(0, 0, s, 0, size, s);
      } else {
        // XZ: Grid on Y=0, spanning X:[0..size] Z:[0..size]
        verts.push(0, 0, s, size, 0, s);
        verts.push(s, 0, 0, s, 0, size);
      }
    }

    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.Float32BufferAttribute(verts, 3));
    const mat = new THREE.LineBasicMaterial({ color: minorColor });
    const grid = new THREE.LineSegments(geo, mat);

    // Size labels centered on each edge
    for (const lbl of this.gridLabels) {
      this.scene.remove(lbl);
      lbl.geometry.dispose();
      (lbl.material as THREE.MeshBasicMaterial).map?.dispose();
      (lbl.material as THREE.Material).dispose();
    }
    this.gridLabels = this._createGridLabels(size, majorColor);

    return grid;
  }

  /** Create flat text meshes showing grid size, one centered on each edge. */
  private _createGridLabels(size: number, color: number): THREE.Mesh[] {
    const text = `${size} mm`;
    const canvas = document.createElement('canvas');
    const ctx = canvas.getContext('2d')!;
    const fontSize = 64;
    ctx.font = `${fontSize}px sans-serif`;
    const metrics = ctx.measureText(text);
    canvas.width = Math.ceil(metrics.width) + 16;
    canvas.height = fontSize + 16;
    ctx.font = `${fontSize}px sans-serif`;
    const r = (color >> 16) & 0xff, g = (color >> 8) & 0xff, b = color & 0xff;
    ctx.fillStyle = `rgb(${r},${g},${b})`;
    ctx.textBaseline = 'middle';
    ctx.textAlign = 'center';
    ctx.fillText(text, canvas.width / 2, canvas.height / 2);

    const texture = new THREE.CanvasTexture(canvas);
    texture.minFilter = THREE.LinearFilter;

    const mid = size / 2;
    const offset = size * 0.04;
    const aspect = canvas.width / canvas.height;
    const h = size * 0.04;
    const w = h * aspect;

    const makeMesh = (): THREE.Mesh => {
      const geo = new THREE.PlaneGeometry(w, h);
      const mat = new THREE.MeshBasicMaterial({
        map: texture,
        transparent: true,
        side: THREE.DoubleSide,
      });
      return new THREE.Mesh(geo, mat);
    };

    const meshes: THREE.Mesh[] = [];

    if (this.bed === 'XY') {
      // Bottom edge along X — face +Z (out of screen)
      const m1 = makeMesh();
      m1.position.set(mid, -offset, 0);
      meshes.push(m1);
      // Left edge along Y — rotated, face +Z
      const m2 = makeMesh();
      m2.rotation.z = -Math.PI / 2;
      m2.position.set(-offset, mid, 0);
      meshes.push(m2);
    } else if (this.bed === 'YZ') {
      // Bottom edge along Z — face +X (outward)
      const m1 = makeMesh();
      m1.rotation.y = Math.PI / 2;
      m1.position.set(0, -offset, mid);
      meshes.push(m1);
      // Left edge along Y — face +X, rotated
      const m2 = makeMesh();
      m2.rotation.set(0, Math.PI / 2, -Math.PI / 2);
      m2.position.set(0, mid, -offset);
      meshes.push(m2);
    } else {
      // XZ: bottom edge along X — face +Y (upward)
      const m1 = makeMesh();
      m1.rotation.x = -Math.PI / 2;
      m1.position.set(mid, 0, -offset);
      meshes.push(m1);
      // Left edge along Z — face +Y, rotated
      const m2 = makeMesh();
      m2.rotation.set(-Math.PI / 2, 0, -Math.PI / 2);
      m2.position.set(-offset, 0, mid);
      meshes.push(m2);
    }

    for (const m of meshes) this.scene.add(m);
    return meshes;
  }

  private _createAxisSprite(text: string, color: string): THREE.Sprite {
    const canvas = document.createElement('canvas');
    canvas.width = 64;
    canvas.height = 64;
    const ctx = canvas.getContext('2d')!;
    ctx.font = 'bold 48px sans-serif';
    ctx.textBaseline = 'middle';
    ctx.textAlign = 'center';
    ctx.fillStyle = color;
    ctx.fillText(text, 32, 32);
    const texture = new THREE.CanvasTexture(canvas);
    texture.minFilter = THREE.LinearFilter;
    const mat = new THREE.SpriteMaterial({ map: texture, transparent: true });
    return new THREE.Sprite(mat);
  }

  private _createAxisLabels(): THREE.Sprite[] {
    const axisLen = 10;
    const pad = 1.8;
    const scale = 2.5;

    const x = this._createAxisSprite('X', '#ff4444');
    x.position.set(axisLen + pad, 0, 0);
    x.scale.setScalar(scale);

    const y = this._createAxisSprite('Y', '#44ff44');
    y.position.set(0, axisLen + pad, 0);
    y.scale.setScalar(scale);

    const z = this._createAxisSprite('Z', '#4488ff');
    z.position.set(0, 0, axisLen + pad);
    z.scale.setScalar(scale);

    return [x, y, z];
  }

  /** Swap a mesh to the patent-style drawing material (stashing the original
   *  for later restore) and attach its drawing overlays. */
  private _applyDrawingMaterial(mesh: THREE.Mesh): void {
    this.drawingOriginalMaterials.set(mesh, mesh.material);
    mesh.material = new THREE.MeshBasicMaterial({
      colorWrite: false,
      side: THREE.FrontSide,
      polygonOffset: true,
      polygonOffsetFactor: 1,
      polygonOffsetUnits: 1,
    });
    this._addDrawingOverlaysForMesh(mesh);
  }

  private _restoreDrawingMaterials(): void {
    for (const [mesh, mat] of this.drawingOriginalMaterials) {
      // Dispose the drawing-mode material we temporarily applied
      if (!Array.isArray(mesh.material)) {
        (mesh.material as THREE.Material).dispose();
      }
      mesh.material = mat;
    }
    this.drawingOriginalMaterials.clear();
  }

  /** Force-resize the renderer to the current container dimensions. Call after moving the container in the DOM. */
  resize(): void {
    const w = this.container.clientWidth;
    const h = this.container.clientHeight;
    if (w === 0 || h === 0) return;

    this.perspCamera.aspect = w / h;

    // Keep ortho frustum aspect ratio in sync (drawing mode or free ortho projection)
    if (this.drawingMode || this.activeCamera === this.orthoCamera) {
      const aspect = w / h;
      const halfH = this.orthoCamera.top;
      this.orthoCamera.left = -halfH * aspect;
      this.orthoCamera.right = halfH * aspect;
    }

    // Apply the visible-area offset to both cameras so a mode swap
    // mid-pose preserves the framing. setViewOffset works by pretending
    // the camera renders the rectangle (dw, 0, w, h) of a virtual
    // (w+dw, h) frustum — the lookAt target sits at the virtual centre
    // ((w+dw)/2, h/2) which lands at canvas x = (w-dw)/2, i.e. the
    // middle of the visible portion to the left of the drawer.
    const dw = Math.min(this.rightInset, Math.max(0, w - 1));
    this.applyViewOffset(this.perspCamera, w, h, dw);
    this.applyViewOffset(this.orthoCamera, w, h, dw);

    this.renderer.setSize(w, h);
  }

  private applyViewOffset(
    cam: THREE.PerspectiveCamera | THREE.OrthographicCamera,
    w: number,
    h: number,
    dw: number,
  ): void {
    if (dw > 0) {
      cam.setViewOffset(w + dw, h, dw, 0, w, h);
    } else {
      cam.clearViewOffset();
    }
    cam.updateProjectionMatrix();
  }

  private applyKeyboardOrbit(): void {
    if (this.keysDown.size === 0) return;
    const rotSpeed = 0.03;
    const zoomSpeed = 0.97;
    Viewer.orbitSpherical(this.activeCamera, this.activeControls.target, (s) => {
      if (this.keysDown.has('ArrowLeft'))  s.theta -= rotSpeed;
      if (this.keysDown.has('ArrowRight')) s.theta += rotSpeed;
      if (this.keysDown.has('ArrowUp'))    s.phi = Math.max(0.01, s.phi - rotSpeed);
      if (this.keysDown.has('ArrowDown'))  s.phi = Math.min(Math.PI - 0.01, s.phi + rotSpeed);
      if (this.keysDown.has('Equal') || this.keysDown.has('NumpadAdd'))
        s.radius *= zoomSpeed;
      if (this.keysDown.has('Minus') || this.keysDown.has('NumpadSubtract'))
        s.radius /= zoomSpeed;
    });
  }

  private showControlsOverlay(): void {
    if (this.controlsOverlay) return;
    const el = document.createElement('div');
    el.className = 'keyboard-controls-overlay';
    el.innerHTML = '<b>Orbit:</b> Arrow Keys &nbsp; <b>Zoom:</b> +/\u2212';
    this.container.appendChild(el);
    this.controlsOverlay = el;
    // Fade out after a few seconds
    setTimeout(() => this.hideControlsOverlay(), 3000);
  }

  private hideControlsOverlay(): void {
    if (!this.controlsOverlay) return;
    this.controlsOverlay.classList.add('fade-out');
    const el = this.controlsOverlay;
    this.controlsOverlay = null;
    // Remove after the CSS fade completes. The .fade-out transition in
    // style.css is 0.4s; add a small buffer so the frame after the fade
    // is still visible before the DOM node goes away.
    setTimeout(() => el.remove(), 450);
  }

  /** Force-render and return a base64 PNG data URL of the current viewport. */
  captureScreenshot(): string {
    this.renderer.render(this.scene, this.activeCamera);
    return this.renderer.domElement.toDataURL('image/png');
  }

  /**
   * Render the scene off-screen from a requested spherical camera pose and
   * return a base64 PNG data URL. The user's camera/controls are not touched.
   *
   * Conventions, normalized to the active bed's up axis:
   *   - `target` is world-space (mm). Defaults to the model bounding-sphere centre.
   *   - `distance` is in mm from target to camera.
   *   - `azimuth` is degrees around `up`, counterclockwise viewed from `+up`.
   *     0° places the camera along the bed's front basis (matches the "front" preset).
   *   - `elevation` is degrees above the horizontal plane.
   *     0° = on the horizon, 90° = overhead (top view), -90° = underside.
   */
  captureScreenshotFromView(opts: {
    azimuth: number;
    elevation: number;
    distance: number;
    target?: { x: number; y: number; z: number };
    width?: number;
    height?: number;
  }): string {
    const { up, front } = this.bedBasis();
    const right = new THREE.Vector3().crossVectors(up, front).normalize();

    let target: THREE.Vector3;
    if (opts.target) {
      target = new THREE.Vector3(opts.target.x, opts.target.y, opts.target.z);
    } else {
      const sphere = this._getMeshBoundingSphere();
      target = sphere.radius > 0 ? sphere.center.clone() : new THREE.Vector3(0, 0, 0);
    }

    const az = THREE.MathUtils.degToRad(opts.azimuth);
    const el = THREE.MathUtils.degToRad(opts.elevation);
    const cosEl = Math.cos(el);
    const offset = new THREE.Vector3()
      .addScaledVector(front, cosEl * Math.cos(az))
      .addScaledVector(right, cosEl * Math.sin(az))
      .addScaledVector(up,    Math.sin(el))
      .multiplyScalar(opts.distance);
    const position = target.clone().add(offset);

    const canvas = this.renderer.domElement;
    const width = Math.max(1, Math.floor(opts.width ?? canvas.width));
    const height = Math.max(1, Math.floor(opts.height ?? canvas.height));

    const camera = new THREE.PerspectiveCamera(
      this.perspCamera.fov,
      width / height,
      this.perspCamera.near,
      Math.max(this.perspCamera.far, opts.distance * 4),
    );
    camera.up.copy(up);
    camera.position.copy(position);
    camera.lookAt(target);

    const rt = new THREE.WebGLRenderTarget(width, height, { type: THREE.UnsignedByteType });
    const prevTarget = this.renderer.getRenderTarget();
    try {
      this.renderer.setRenderTarget(rt);
      this.renderer.clear();
      this.renderer.render(this.scene, camera);
      const pixels = new Uint8Array(width * height * 4);
      this.renderer.readRenderTargetPixels(rt, 0, 0, width, height, pixels);

      // readRenderTargetPixels yields bottom-up rows; flip into a 2D canvas
      // so toDataURL writes a correctly oriented PNG.
      const out = document.createElement('canvas');
      out.width = width;
      out.height = height;
      const ctx = out.getContext('2d');
      if (!ctx) throw new Error('2d context unavailable');
      const img = ctx.createImageData(width, height);
      const rowBytes = width * 4;
      for (let y = 0; y < height; y++) {
        const src = (height - 1 - y) * rowBytes;
        img.data.set(pixels.subarray(src, src + rowBytes), y * rowBytes);
      }
      ctx.putImageData(img, 0, 0);
      return out.toDataURL('image/png');
    } finally {
      this.renderer.setRenderTarget(prevTarget);
      rt.dispose();
    }
  }

  /** Up axis and front-view basis vectors for the active bed. */
  private bedBasis(): { up: THREE.Vector3; front: THREE.Vector3 } {
    switch (this.bed) {
      case 'XY': return { up: new THREE.Vector3(0, 0, 1), front: new THREE.Vector3(0, -1, 0) };
      case 'YZ': return { up: new THREE.Vector3(1, 0, 0), front: new THREE.Vector3(0,  0, 1) };
      default:   return { up: new THREE.Vector3(0, 1, 0), front: new THREE.Vector3(0,  0, 1) };
    }
  }

  private animate(): void {
    requestAnimationFrame(() => this.animate());
    if (!this._visible) return;
    this.applyKeyboardOrbit();
    this.activeControls.update();
    for (const cb of this.frameCallbacks) cb(this.activeCamera);

    if (this.headTracker && this.activeCamera === this.perspCamera) {
      // Save the clean base position so OrbitControls sees unmodified state next frame
      const basePos = this.perspCamera.position.clone();
      const target = this.perspControls.target;
      const offset = this.headTracker.getOffset();

      // Convert to spherical, apply absolute offsets, convert back
      Viewer.orbitSpherical(this.perspCamera, target, (s) => {
        s.theta += offset.azimuth;
        s.phi = Math.max(0.01, Math.min(Math.PI - 0.01, s.phi - offset.elevation));
        s.radius *= offset.dolly;
      });

      // Render with parallax applied
      this.renderer.render(this.scene, this.activeCamera);

      // Restore base position — OrbitControls never sees our modifications
      this.perspCamera.position.copy(basePos);
      this.perspCamera.lookAt(target);
    } else {
      this.renderer.render(this.scene, this.activeCamera);
    }
  }
}
