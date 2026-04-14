import * as THREE from 'three';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';
import { HeadTracker } from './headtrack';
import { hexToInt } from './color';
import { decodeBinaryMesh, buildFaceGroupWireframe } from './mesh-decode';
import type { DecodedMesh, DebugStepData } from './mesh-decode';

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
  bed: string;
  gridSize: number;
  gridSpacing: number;
}

/** PosEntry maps a source position to the face IDs produced there. */
interface PosEntry {
  file: string;  // "" = main, "folder/name" = library path
  line: number;
  col: number;
  faceIDs: number[];
}


export type DrawingViewpoint = 'iso' | 'top' | 'front' | 'right' | 'left';

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
  _transform: {
    input: { color: 0x888888, opacity: 0.2 },
    result: { color: 0x4488ff, opacity: 1.0 },
  },
  _default: {
    result: { color: 0x4488ff, opacity: 1.0 },
    input: { color: 0x888888, opacity: 0.2 },
  },
};

const TRANSFORM_OPS = new Set(['Translate', 'Rotate', 'Scale', 'Mirror']);

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
  private ambientLight: THREE.AmbientLight;
  private savedAmbientIntensity: number = 0.8;
  private grid: THREE.LineSegments;
  private gridLabels: THREE.Mesh[] = [];
  private axes: THREE.AxesHelper;
  private axisLabels: THREE.Sprite[] = [];
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
  private parallaxEnabled = false;

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

  // Bound event handlers for cleanup in dispose()
  private onMouseDown: (e: MouseEvent) => void;
  private onMouseUp: (e: MouseEvent) => void;
  private onFocus: () => void;
  private onBlur: () => void;
  private onKeyDown: (e: KeyboardEvent) => void;
  private onKeyUp: (e: KeyboardEvent) => void;

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

    // Initial size
    this.onResize();

    // Resize handling via ResizeObserver
    this.resizeObserver = new ResizeObserver(() => this.onResize());
    this.resizeObserver.observe(this.container);

    // Face-click detection (distinguish click from drag) + keyboard orbital controls
    container.setAttribute('tabindex', '0');
    container.style.outline = 'none';

    this.onMouseDown = (e: MouseEvent) => {
      this.pickMouseDown = { x: e.clientX, y: e.clientY, time: Date.now() };
      container.focus();
    };
    this.onMouseUp = (e: MouseEvent) => {
      if (!this.pickMouseDown) return;
      const dx = e.clientX - this.pickMouseDown.x;
      const dy = e.clientY - this.pickMouseDown.y;
      const dist = Math.sqrt(dx * dx + dy * dy);
      const elapsed = Date.now() - this.pickMouseDown.time;
      this.pickMouseDown = null;
      if (dist < 5 && elapsed < 300) {
        this.handleFacePick(e);
      }
    };
    this.onFocus = () => {
      this.viewportFocused = true;
      this.showControlsOverlay();
    };
    this.onBlur = () => {
      this.viewportFocused = false;
      this.keysDown.clear();
      this.hideControlsOverlay();
    };
    this.onKeyDown = (e: KeyboardEvent) => {
      if (!this.viewportFocused) return;
      if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Equal', 'Minus', 'NumpadAdd', 'NumpadSubtract'].includes(e.code)) {
        e.preventDefault();
        this.keysDown.add(e.code);
      }
    };
    this.onKeyUp = (e: KeyboardEvent) => {
      this.keysDown.delete(e.code);
    };

    container.addEventListener('mousedown', this.onMouseDown);
    container.addEventListener('mouseup', this.onMouseUp);
    container.addEventListener('focus', this.onFocus);
    container.addEventListener('blur', this.onBlur);
    container.addEventListener('keydown', this.onKeyDown);
    container.addEventListener('keyup', this.onKeyUp);

    // Start render loop
    this.animate();
  }

  private createMeshMaterial(opts: { vertexColors?: boolean; color?: number }): THREE.MeshStandardMaterial {
    return new THREE.MeshStandardMaterial({
      metalness: this.meshMetalness,
      roughness: this.meshRoughness,
      flatShading: true,
      side: THREE.DoubleSide,
      ...opts,
    });
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

    if (decoded.expanded && useVertexColors) {
      // Pre-expanded positions but need to compute colors on frontend
      const geometry = new THREE.BufferGeometry();
      geometry.setAttribute('position', new THREE.BufferAttribute(decoded.expanded, 3));

      const nTris = decoded.faceGroups!.length;
      const colors = new Float32Array(nTris * 3 * 4);
      const defaultColor = new THREE.Color(this.meshColor);
      const colorMap = new Map<number, [number, number, number]>();
      if (hasFaceColors) {
        for (const [id, hex] of Object.entries(decoded.faceColors!)) {
          const key = hex.length > 7 ? hex.substring(0, 7) : hex;
          const c = new THREE.Color(key);
          colorMap.set(Number(id), [c.r, c.g, c.b]);
        }
      }
      const dr = defaultColor.r, dg = defaultColor.g, db = defaultColor.b;
      for (let t = 0; t < nTris; t++) {
        const fgId = decoded.faceGroups![t];
        const c = hasFaceColors ? colorMap.get(fgId) : undefined;
        const r = c ? c[0] : dr, g = c ? c[1] : dg, b = c ? c[2] : db;
        const base = t * 12;
        colors[base]     = r; colors[base + 1] = g; colors[base + 2]  = b; colors[base + 3]  = 1;
        colors[base + 4] = r; colors[base + 5] = g; colors[base + 6]  = b; colors[base + 7]  = 1;
        colors[base + 8] = r; colors[base + 9] = g; colors[base + 10] = b; colors[base + 11] = 1;
      }
      geometry.setAttribute('color', new THREE.BufferAttribute(colors, 4));
      mesh = new THREE.Mesh<THREE.BufferGeometry, THREE.Material>(geometry,
        this.createMeshMaterial({ vertexColors: true }));
    } else if (decoded.expanded) {
      // Pre-expanded but no face groups — simple material
      const geometry = new THREE.BufferGeometry();
      geometry.setAttribute('position', new THREE.BufferAttribute(decoded.expanded, 3));
      mesh = new THREE.Mesh<THREE.BufferGeometry, THREE.Material>(geometry,
        this.createMeshMaterial({ color: this.meshColor }));
    } else {
      // Fallback: indexed geometry (old path for debug step meshes, etc.)
      const geometry = new THREE.BufferGeometry();
      geometry.setAttribute('position', new THREE.BufferAttribute(decoded.vertices, 3));
      if (decoded.indices && decoded.indices.length > 0) {
        geometry.setIndex(new THREE.BufferAttribute(decoded.indices, 1));
      }
      if (useVertexColors) {
        const nonIndexed = geometry.toNonIndexed();
        const nTris = decoded.faceGroups!.length;
        const colors = new Float32Array(nTris * 3 * 4);
        const defaultColor = new THREE.Color(this.meshColor);
        const colorMap = new Map<string, THREE.Color>();
        if (hasFaceColors) {
          for (const hex of Object.values(decoded.faceColors!)) {
            const key = hex.length > 7 ? hex.substring(0, 7) : hex;
            if (!colorMap.has(key)) colorMap.set(key, new THREE.Color(key));
          }
        }
        for (let t = 0; t < nTris; t++) {
          const fgId = String(decoded.faceGroups![t]);
          const hex = hasFaceColors ? decoded.faceColors![fgId] : undefined;
          let r: number, g: number, b: number;
          if (hex) {
            const key = hex.length > 7 ? hex.substring(0, 7) : hex;
            const c = colorMap.get(key)!;
            r = c.r; g = c.g; b = c.b;
          } else {
            r = defaultColor.r; g = defaultColor.g; b = defaultColor.b;
          }
          for (let v = 0; v < 3; v++) {
            const base = t * 12 + v * 4;
            colors[base] = r; colors[base+1] = g; colors[base+2] = b; colors[base+3] = 1.0;
          }
        }
        nonIndexed.setAttribute('color', new THREE.BufferAttribute(colors, 4));
        mesh = new THREE.Mesh<THREE.BufferGeometry, THREE.Material>(nonIndexed,
          this.createMeshMaterial({ vertexColors: true }));
        geometry.dispose();
      } else {
        mesh = new THREE.Mesh<THREE.BufferGeometry, THREE.Material>(geometry,
          this.createMeshMaterial({ color: this.meshColor }));
      }
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
      this.drawingOriginalMaterials.set(mesh, mesh.material);
      (mesh.material as any) = new THREE.MeshBasicMaterial({
        colorWrite: false,
        side: THREE.FrontSide,
        polygonOffset: true,
        polygonOffsetFactor: 1,
        polygonOffsetUnits: 1,
      });
      this._addDrawingOverlaysForMesh(mesh);
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
        mat.depthWrite = true;
        delete mesh.userData.originalTransparent;
        delete mesh.userData.originalOpacity;
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

    const rect = this.renderer.domElement.getBoundingClientRect();
    const mouse = new THREE.Vector2(
      ((e.clientX - rect.left) / rect.width) * 2 - 1,
      -((e.clientY - rect.top) / rect.height) * 2 + 1,
    );
    this.raycaster.setFromCamera(mouse, this.activeCamera);
    const meshOnly = this.userMeshes.filter(o => o instanceof THREE.Mesh);
    const intersects = this.raycaster.intersectObjects(meshOnly, false);


    if (intersects.length === 0) {
      this.onFaceClickCb?.('', 0, 0);
      return;
    }

    const hit = intersects[0];
    const mesh = hit.object as THREE.Mesh<THREE.BufferGeometry, THREE.Material>;
    const faceGroups = mesh.userData.faceGroups as Uint32Array | undefined;
    if (!faceGroups || hit.faceIndex == null) {
      return;
    }

    const faceGroupID = faceGroups[hit.faceIndex];
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

  /** Find the closest posMap entry at or before (line, col) on the same line, filtered by file. */
  private findFaceIDsAtPos(file: string, line: number, col: number): Set<number> | null {
    if (this.posMap.length === 0) return null;

    // Find entries on this line in the same file, pick the one with largest col <= col
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
        // Track the first entry after col as fallback
        if (!firstAfter || entry.col < firstAfter.col) {
          firstAfter = entry;
        }
      }
    }

    // Fall back to first entry after cursor if none before
    if (!best) best = firstAfter;
    if (!best || best.faceIDs.length === 0) return null;
    return new Set(best.faceIDs);
  }

  loadDebugStep(step: DebugStepData, binary: ArrayBuffer): void {
    this.clearMeshes();

    const styleMap = STEP_STYLES[step.op]
      ?? (TRANSFORM_OPS.has(step.op) ? STEP_STYLES._transform : STEP_STYLES._default);

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
      const material = new THREE.MeshStandardMaterial({
        color: style.color,
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
    // Wireframe lines are children of meshes — they get removed with the mesh, just clear state
    this.wireframeLineObjects = [];
    this.wireframeOriginalMaterials.clear();

    for (const obj of this.userMeshes) {
      this.scene.remove(obj);
      Viewer._disposeObject3D(obj);
    }
    this.userMeshes = [];
  }

  /** Center all user meshes on the bed. Bed-plane axes are centered; the "up" axis min is at 0. */
  centerOnBed(): void {
    if (this.userMeshes.length === 0) return;

    // Compute combined bounding box of all user meshes
    const box = new THREE.Box3();
    for (const obj of this.userMeshes) {
      box.expandByObject(obj);
    }
    if (box.isEmpty()) return;

    const bedCenter = this.gridSize / 2;
    const center = new THREE.Vector3();
    box.getCenter(center);

    const offset = new THREE.Vector3();
    if (this.bed === 'XY') {
      // Bed axes: X, Y; up: Z
      offset.set(bedCenter - center.x, bedCenter - center.y, -box.min.z);
    } else if (this.bed === 'YZ') {
      // Bed axes: Y, Z; up: X
      offset.set(-box.min.x, bedCenter - center.y, bedCenter - center.z);
    } else {
      // XZ: Bed axes: X, Z; up: Y
      offset.set(bedCenter - center.x, -box.min.y, bedCenter - center.z);
    }

    for (const obj of this.userMeshes) {
      obj.position.add(offset);
    }
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
    if (this.parallaxEnabled) {
      this.parallaxEnabled = false;
      if (this.headTracker) {
        this.headTracker.stop();
        this.headTracker = null;
      }
      return false;
    }
    this.headTracker = new HeadTracker();
    await this.headTracker.start(this.container, deviceId, yOffset);
    this.parallaxEnabled = true;
    return true;
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

  dispose(): void {
    this.container.removeEventListener('mousedown', this.onMouseDown);
    this.container.removeEventListener('mouseup', this.onMouseUp);
    this.container.removeEventListener('focus', this.onFocus);
    this.container.removeEventListener('blur', this.onBlur);
    this.container.removeEventListener('keydown', this.onKeyDown);
    this.container.removeEventListener('keyup', this.onKeyUp);
    this.resizeObserver.disconnect();
    this.renderer.dispose();
  }

  /** Force-resize the renderer to the current container dimensions. Call after moving the container in the DOM. */
  resize(): void {
    this.onResize();
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
          this.drawingOriginalMaterials.set(obj, obj.material);
          obj.material = new THREE.MeshBasicMaterial({
            colorWrite: false,
            side: THREE.FrontSide,
            polygonOffset: true,
            polygonOffsetFactor: 1,
            polygonOffsetUnits: 1,
          });
          this._addDrawingOverlaysForMesh(obj);
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
          // Fallback: all triangle edges
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
        top:   { dir: new THREE.Vector3( 0,  1,  0),             up: new THREE.Vector3(0, 0,-1) },
        front: { dir: new THREE.Vector3( 0,  0,  1),             up: new THREE.Vector3(0, 1, 0) },
        right: { dir: new THREE.Vector3( 1,  0,  0),             up: new THREE.Vector3(0, 1, 0) },
        left:  { dir: new THREE.Vector3(-1,  0,  0),             up: new THREE.Vector3(0, 1, 0) },
      },
      XY: {
        iso:   { dir: new THREE.Vector3( 1,  1,  1).normalize(), up: new THREE.Vector3(0, 0, 1) },
        top:   { dir: new THREE.Vector3( 0,  0,  1),             up: new THREE.Vector3(0, 1, 0) },
        front: { dir: new THREE.Vector3( 0, -1,  0),             up: new THREE.Vector3(0, 0, 1) },
        right: { dir: new THREE.Vector3( 1,  0,  0),             up: new THREE.Vector3(0, 0, 1) },
        left:  { dir: new THREE.Vector3(-1,  0,  0),             up: new THREE.Vector3(0, 0, 1) },
      },
      YZ: {
        iso:   { dir: new THREE.Vector3( 1,  1,  1).normalize(), up: new THREE.Vector3(1, 0, 0) },
        top:   { dir: new THREE.Vector3( 1,  0,  0),             up: new THREE.Vector3(0, 0, 1) },
        front: { dir: new THREE.Vector3( 0,  0,  1),             up: new THREE.Vector3(1, 0, 0) },
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

  private onResize(): void {
    const w = this.container.clientWidth;
    const h = this.container.clientHeight;
    if (w === 0 || h === 0) return;

    this.perspCamera.aspect = w / h;
    this.perspCamera.updateProjectionMatrix();

    // Keep ortho frustum aspect ratio in sync (drawing mode or free ortho projection)
    if (this.drawingMode || this.activeCamera === this.orthoCamera) {
      const aspect = w / h;
      const halfH = this.orthoCamera.top;
      this.orthoCamera.left = -halfH * aspect;
      this.orthoCamera.right = halfH * aspect;
      this.orthoCamera.updateProjectionMatrix();
    }

    this.renderer.setSize(w, h);
  }

  private applyKeyboardOrbit(): void {
    if (this.keysDown.size === 0) return;
    const target = this.activeControls.target;
    const cam = this.activeCamera;
    const delta = new THREE.Vector3().subVectors(cam.position, target);
    const spherical = new THREE.Spherical().setFromVector3(delta);
    const rotSpeed = 0.03;
    const zoomSpeed = 0.97;

    if (this.keysDown.has('ArrowLeft'))  spherical.theta -= rotSpeed;
    if (this.keysDown.has('ArrowRight')) spherical.theta += rotSpeed;
    if (this.keysDown.has('ArrowUp'))    spherical.phi = Math.max(0.01, spherical.phi - rotSpeed);
    if (this.keysDown.has('ArrowDown'))  spherical.phi = Math.min(Math.PI - 0.01, spherical.phi + rotSpeed);
    if (this.keysDown.has('Equal') || this.keysDown.has('NumpadAdd'))
      spherical.radius *= zoomSpeed;
    if (this.keysDown.has('Minus') || this.keysDown.has('NumpadSubtract'))
      spherical.radius /= zoomSpeed;

    cam.position.setFromSpherical(spherical).add(target);
    cam.lookAt(target);
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
    el.addEventListener('transitionend', () => el.remove());
    // Fallback removal in case transitionend doesn't fire
    setTimeout(() => { if (el.parentNode) el.remove(); }, 500);
  }

  /** Force-render and return a base64 PNG data URL of the current viewport. */
  captureScreenshot(): string {
    this.renderer.render(this.scene, this.activeCamera);
    return this.renderer.domElement.toDataURL('image/png');
  }

  private animate(): void {
    requestAnimationFrame(() => this.animate());
    if (!this._visible) return;
    this.applyKeyboardOrbit();
    this.activeControls.update();

    if (this.parallaxEnabled && this.headTracker && this.activeCamera === this.perspCamera) {
      // Save the clean base position so OrbitControls sees unmodified state next frame
      const basePos = this.perspCamera.position.clone();
      const target = this.perspControls.target;
      const offset = this.headTracker.getOffset();

      // Convert to spherical, apply absolute offsets, convert back
      const delta = new THREE.Vector3().subVectors(basePos, target);
      const spherical = new THREE.Spherical().setFromVector3(delta);
      spherical.theta += offset.azimuth;
      spherical.phi = Math.max(0.01, Math.min(Math.PI - 0.01,
        spherical.phi - offset.elevation));
      spherical.radius *= offset.dolly;
      this.perspCamera.position.setFromSpherical(spherical).add(target);
      this.perspCamera.lookAt(target);

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
