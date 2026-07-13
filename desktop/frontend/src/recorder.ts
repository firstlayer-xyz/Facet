// In-app canvas recorder. Captures the WebGL viewer canvas via captureStream +
// MediaRecorder (which works because the renderer keeps preserveDrawingBuffer).
// This is the `canvas` capture surface only; whole-window (`page`) recording is
// done natively on the Go side (ScreenCaptureKit) — see automation.ts.

export interface CanvasRecordOpts {
  fps?: number;
}

export class Recorder {
  private rec: MediaRecorder | null = null;
  private chunks: Blob[] = [];

  constructor(private readonly canvas: HTMLCanvasElement) {}

  start(opts: CanvasRecordOpts): void {
    if (this.rec) throw new Error('recorder: already recording');
    const stream = this.canvas.captureStream(opts.fps ?? 30);
    this.chunks = [];
    this.rec = new MediaRecorder(stream, { mimeType: 'video/webm' });
    this.rec.ondataavailable = (e) => {
      if (e.data.size) this.chunks.push(e.data);
    };
    this.rec.start();
  }

  stop(): Promise<Blob> {
    const rec = this.rec;
    if (!rec) throw new Error('recorder: not recording');
    return new Promise((resolve) => {
      rec.onstop = () => {
        this.rec = null;
        resolve(new Blob(this.chunks, { type: 'video/webm' }));
      };
      rec.stop();
    });
  }
}

/** Read a Blob as a base64 data URL for handing to Go's SaveRecording. */
export function blobToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const fr = new FileReader();
    fr.onload = () => resolve(String(fr.result));
    fr.onerror = () => reject(fr.error);
    fr.readAsDataURL(blob);
  });
}
