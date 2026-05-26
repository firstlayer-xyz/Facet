import * as THREE from 'three';

type OrbitFn = (deltaTheta: number, deltaPhi: number) => void;

export class Gnomon {
  private renderer: THREE.WebGLRenderer;
  private camera: THREE.OrthographicCamera;
  private scene: THREE.Scene;

  constructor(container: HTMLElement, orbitBy: OrbitFn) {
    const canvas = document.createElement('canvas');
    canvas.id = 'gnomon';
    container.appendChild(canvas);

    this.renderer = new THREE.WebGLRenderer({ canvas, alpha: true, antialias: true });
    this.renderer.setPixelRatio(window.devicePixelRatio);
    this.renderer.setSize(80, 80);

    this.camera = new THREE.OrthographicCamera(-1.5, 1.5, 1.5, -1.5, 0.1, 20);

    this.scene = new THREE.Scene();
    this.addAxis(new THREE.Vector3(1, 0, 0), 0xff4040, 'X');
    this.addAxis(new THREE.Vector3(0, 1, 0), 0x44cc44, 'Y');
    this.addAxis(new THREE.Vector3(0, 0, 1), 0x4488ff, 'Z');

    this.bindDrag(canvas, orbitBy);
  }

  private makeLabel(text: string, color: string): THREE.Sprite {
    const c = document.createElement('canvas');
    c.width = 64; c.height = 64;
    const ctx = c.getContext('2d')!;
    ctx.font = 'bold 48px monospace';
    ctx.fillStyle = color;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(text, 32, 32);
    const sprite = new THREE.Sprite(
      new THREE.SpriteMaterial({ map: new THREE.CanvasTexture(c), depthTest: false, transparent: true })
    );
    sprite.scale.set(0.58, 0.58, 0.58);
    return sprite;
  }

  private addAxis(dir: THREE.Vector3, color: number, label: string) {
    const hex = '#' + color.toString(16).padStart(6, '0');
    const lineMat = new THREE.LineBasicMaterial({ color, depthTest: false });
    const lineGeom = new THREE.BufferGeometry().setFromPoints([
      new THREE.Vector3(0, 0, 0),
      dir.clone().multiplyScalar(0.82),
    ]);
    this.scene.add(new THREE.Line(lineGeom, lineMat));

    const cone = new THREE.Mesh(
      new THREE.ConeGeometry(0.055, 0.18, 6),
      new THREE.MeshBasicMaterial({ color, depthTest: false })
    );
    cone.position.copy(dir.clone().multiplyScalar(0.91));
    cone.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), dir);
    this.scene.add(cone);

    const sprite = this.makeLabel(label, hex);
    sprite.position.copy(dir.clone().multiplyScalar(1.25));
    this.scene.add(sprite);
  }

  private bindDrag(canvas: HTMLCanvasElement, orbitBy: OrbitFn) {
    const SENSITIVITY = 0.012;
    let dragging = false;
    let lastX = 0, lastY = 0;

    canvas.addEventListener('pointerdown', (e) => {
      dragging = true;
      lastX = e.clientX;
      lastY = e.clientY;
      canvas.setPointerCapture(e.pointerId);
    });

    canvas.addEventListener('pointermove', (e) => {
      if (!dragging) return;
      const dx = e.clientX - lastX;
      const dy = e.clientY - lastY;
      lastX = e.clientX;
      lastY = e.clientY;
      orbitBy(-dx * SENSITIVITY, dy * SENSITIVITY);
    });

    canvas.addEventListener('pointerup', (e) => {
      dragging = false;
      canvas.releasePointerCapture(e.pointerId);
    });
  }

  update(mainCamera: THREE.Camera) {
    const dir = new THREE.Vector3();
    mainCamera.getWorldDirection(dir);
    this.camera.position.copy(dir).negate().multiplyScalar(10);
    this.camera.up.copy(mainCamera.up);
    this.camera.lookAt(0, 0, 0);
    this.renderer.render(this.scene, this.camera);
  }
}
