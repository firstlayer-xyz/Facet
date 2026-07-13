// In-app screen recorder for the automation system. `canvas` mode captures the
// WebGL viewer canvas via captureStream + MediaRecorder (which works because the
// renderer keeps preserveDrawingBuffer). `page` mode (full UI) is pending the
// getDisplayMedia-in-WKWebView spike and is added by Task 7 — until then it is a
// loud error, not a silent fallback to canvas.

export type RecordMode = 'canvas' | 'page';

export interface RecordOpts {
  mode: RecordMode;
  fps?: number;
}

export class Recorder {
  private rec: MediaRecorder | null = null;
  private chunks: Blob[] = [];

  constructor(private readonly canvas: HTMLCanvasElement) {}

  async start(opts: RecordOpts): Promise<void> {
    if (this.rec) throw new Error('recorder: already recording');
    if (opts.mode !== 'canvas') {
      throw new Error(
        `recorder: mode '${opts.mode}' not yet supported (page capture pending getDisplayMedia spike)`,
      );
    }
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
