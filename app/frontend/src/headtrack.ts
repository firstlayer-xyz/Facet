// headtrack.ts — Webcam-based head tracking for parallax viewport effect.
//
// Absolute mapping: face position directly maps to orbit angle offsets.
// No accumulation, no drift. Face at center = no offset, face left = view from left.

import { FaceDetector, FilesetResolver } from '@mediapipe/tasks-vision';

export interface HeadOffset {
  azimuth: number;  // radians to add to orbit theta
  elevation: number; // radians to add to orbit phi
  dolly: number;     // multiplier: 1 = no change, <1 = closer, >1 = farther
}

export class HeadTracker {
  private video: HTMLVideoElement | null = null;
  private stream: MediaStream | null = null;
  private detector: FaceDetector | null = null;
  private running = false;
  private rafId = 0;

  // Smoothed normalized position (-1..1), 0 = centered
  private smoothX = 0;
  private smoothY = 0;

  // Smoothed face size (fraction of frame width), used for dolly
  private smoothSize = 0;
  private baseSize = 0;
  private calibrated = false;
  private calibrationFrames = 0;
  private calibrationAccum = 0;

  // Detection throttle (15fps)
  private lastDetectTime = 0;
  private readonly detectInterval = 1000 / 15;

  // Smoothing
  private readonly alpha = 0.15;

  // Face-lost decay
  private lastFaceTime = 0;
  private readonly holdMs = 500;
  private readonly decayRate = 0.02;

  // Deadzone — ignore movements smaller than this (normalized units)
  private readonly deadzone = 0.03;

  // Max angular offset in radians
  private readonly maxAzimuth = 1.4;   // left/right
  private readonly maxElevation = 2.0; // up/down (more sensitive)
  // Dolly range
  private readonly dollyScale = 1.0;

  // Camera placement offset — shifts neutral Y down to account for
  // webcam sitting above the screen (0 = centered, 0.3 = 30% down)
  private yZeroOffset = 0.3;

  // Visualization overlay
  private overlay: HTMLCanvasElement | null = null;
  private overlayCtx: CanvasRenderingContext2D | null = null;
  private faceDetected = false;

  async start(container: HTMLElement, deviceId?: string, yOffset?: number): Promise<void> {
    if (yOffset !== undefined) this.yZeroOffset = yOffset;
    try {
      const videoConstraints: MediaTrackConstraints = { width: 320, height: 240 };
      if (deviceId) {
        videoConstraints.deviceId = { exact: deviceId };
      } else {
        videoConstraints.facingMode = 'user';
      }
      this.stream = await navigator.mediaDevices.getUserMedia({
        video: videoConstraints,
      });

      this.video = document.createElement('video');
      this.video.srcObject = this.stream;
      this.video.setAttribute('playsinline', '');
      this.video.style.display = 'none';
      document.body.appendChild(this.video);
      await this.video.play();

      const vision = await FilesetResolver.forVisionTasks(
        'https://cdn.jsdelivr.net/npm/@mediapipe/tasks-vision@latest/wasm',
      );
      this.detector = await FaceDetector.createFromOptions(vision, {
        baseOptions: {
          modelAssetPath: 'https://storage.googleapis.com/mediapipe-models/face_detector/blaze_face_short_range/float16/1/blaze_face_short_range.tflite',
          delegate: 'GPU',
        },
        runningMode: 'VIDEO',
      });

      this.overlay = document.createElement('canvas');
      this.overlay.id = 'head-track-overlay';
      this.overlay.width = 120;
      this.overlay.height = 90;
      container.appendChild(this.overlay);
      this.overlayCtx = this.overlay.getContext('2d');

      this.running = true;
      this.calibrated = false;
      this.calibrationFrames = 0;
      this.calibrationAccum = 0;
      this.lastFaceTime = performance.now();
      this.tick();
    } catch (err) {
      console.error('HeadTracker: failed to start:', err);
      this.stop();
    }
  }

  stop(): void {
    this.running = false;
    if (this.rafId) {
      cancelAnimationFrame(this.rafId);
      this.rafId = 0;
    }
    if (this.detector) {
      this.detector.close();
      this.detector = null;
    }
    if (this.stream) {
      for (const track of this.stream.getTracks()) track.stop();
      this.stream = null;
    }
    if (this.video) {
      this.video.remove();
      this.video = null;
    }
    if (this.overlay) {
      this.overlay.remove();
      this.overlay = null;
      this.overlayCtx = null;
    }
    this.smoothX = 0;
    this.smoothY = 0;
    this.smoothSize = 0;
    this.baseSize = 0;
    this.calibrated = false;
    this.faceDetected = false;
  }

  /** Returns absolute angle offsets and dolly factor based on current face position. */
  getOffset(): HeadOffset {
    let dolly = 1;
    if (this.calibrated && this.baseSize > 0) {
      const sizeRatio = this.smoothSize / this.baseSize;
      // Face bigger → closer → dolly < 1 (reduce radius)
      dolly = 1 - (sizeRatio - 1) * this.dollyScale;
      dolly = Math.max(0.5, Math.min(1.6, dolly));
    }
    return {
      azimuth: this.smoothX * this.maxAzimuth,
      elevation: this.smoothY * this.maxElevation,
      dolly,
    };
  }

  private tick(): void {
    if (!this.running) return;
    this.rafId = requestAnimationFrame(() => this.tick());

    const now = performance.now();
    if (now - this.lastDetectTime < this.detectInterval) return;
    if (!this.video || !this.detector || this.video.readyState < 2) return;

    this.lastDetectTime = now;
    let result;
    try {
      result = this.detector.detectForVideo(this.video, now);
    } catch {
      return; // skip frame on detection failure
    }

    if (result.detections.length > 0 && result.detections[0].boundingBox) {
      const bbox = result.detections[0].boundingBox;
      const vw = this.video.videoWidth;
      const vh = this.video.videoHeight;

      // Normalize face center to -1..1 (mirrored X so moving right -> positive)
      // Y is shifted by yZeroOffset to account for webcam above screen —
      // moves the neutral point down so looking straight ahead reads as ~0.
      const rawX = -((bbox.originX + bbox.width / 2) / vw - 0.5) * 2;
      const rawY = -((bbox.originY + bbox.height / 2) / vh - (0.5 + this.yZeroOffset)) * 2;
      const rawSize = bbox.width / vw;

      const dx = rawX - this.smoothX;
      const dy = rawY - this.smoothY;
      if (Math.abs(dx) > this.deadzone) this.smoothX += this.alpha * dx;
      if (Math.abs(dy) > this.deadzone) this.smoothY += this.alpha * dy;
      const ds = rawSize - this.smoothSize;
      if (Math.abs(ds) > this.deadzone * 0.5) this.smoothSize += this.alpha * 0.5 * ds;

      // Calibrate baseline size from first 15 frames
      if (!this.calibrated) {
        this.calibrationAccum += rawSize;
        this.calibrationFrames++;
        if (this.calibrationFrames >= 15) {
          this.baseSize = this.calibrationAccum / this.calibrationFrames;
          this.smoothSize = this.baseSize;
          this.calibrated = true;
        }
      }

      this.lastFaceTime = now;
      this.faceDetected = true;
    } else {
      this.faceDetected = false;
      if (now - this.lastFaceTime > this.holdMs) {
        this.smoothX *= (1 - this.decayRate);
        this.smoothY *= (1 - this.decayRate);
        if (this.calibrated) {
          this.smoothSize += this.decayRate * (this.baseSize - this.smoothSize);
        }
      }
    }

    this.drawOverlay();
  }

  private drawOverlay(): void {
    const ctx = this.overlayCtx;
    if (!ctx || !this.overlay) return;
    const w = this.overlay.width;
    const h = this.overlay.height;

    ctx.clearRect(0, 0, w, h);

    ctx.fillStyle = 'rgba(0, 0, 0, 0.5)';
    ctx.fillRect(0, 0, w, h);

    ctx.strokeStyle = this.faceDetected ? '#4a9eff' : '#555';
    ctx.lineWidth = 1.5;
    ctx.strokeRect(1, 1, w - 2, h - 2);

    // Crosshair at center
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.15)';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(w / 2, 0);
    ctx.lineTo(w / 2, h);
    ctx.moveTo(0, h / 2);
    ctx.lineTo(w, h / 2);
    ctx.stroke();

    // Face X marker
    const fx = w / 2 + (this.smoothX * w * 0.4);
    const fy = h / 2 - (this.smoothY * h * 0.4);

    let s = 6;
    if (this.calibrated && this.baseSize > 0) {
      const sizeRatio = this.smoothSize / this.baseSize;
      s = Math.max(4, Math.min(12, 6 * sizeRatio));
    }

    ctx.strokeStyle = this.faceDetected ? '#4a9eff' : '#666';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(fx - s, fy - s);
    ctx.lineTo(fx + s, fy + s);
    ctx.moveTo(fx + s, fy - s);
    ctx.lineTo(fx - s, fy + s);
    ctx.stroke();
  }
}
