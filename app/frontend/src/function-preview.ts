// function-preview.ts — Bar for editing constrained params of the selected entry point function.

export interface ParamConstraint {
  kind: string;
  min?: any;
  max?: any;
  step?: any;
  exclusive?: boolean;
  values?: any[];
}

export interface ParamEntry {
  name: string;
  type: string;
  hasDefault: boolean;
  default: any;
  unit?: string;
  constraint?: ParamConstraint;
}

export interface EntryPoint {
  name: string;
  signature: string;
  params: ParamEntry[];
  libPath: string;
  libVar: string;
  doc: string;
}

export interface FunctionPreviewCallbacks {
  onOverrideChange(overrides: Record<string, any>): void;
}

// Common unit options for unconstrained Length and Angle params.
// Each entry is [displayName, factor-to-canonical-unit].
// Length canonical unit = mm; Angle canonical unit = deg.
const LENGTH_UNITS: [string, number][] = [
  ['mm', 1],
  ['cm', 10],
  ['m', 1000],
  ['in', 25.4],
  ['ft', 304.8],
];
const ANGLE_UNITS: [string, number][] = [
  ['deg', 1],
  ['rad', 180 / Math.PI],
];

export class FunctionPreview {
  private panel: HTMLElement;
  private selected: EntryPoint | null = null;
  private overrides: Record<string, any> = {};
  private unitSelections: Record<string, string> = {};
  private callbacks: FunctionPreviewCallbacks;

  private container: HTMLElement;
  private resizeObserver: ResizeObserver;

  constructor(container: HTMLElement, callbacks: FunctionPreviewCallbacks) {
    this.callbacks = callbacks;
    this.container = container;

    this.panel = document.createElement('div');
    this.panel.id = 'fn-preview-bar';
    this.panel.style.display = 'none';
    container.appendChild(this.panel);

    // Keep --fn-bar-h in sync with the panel's actual rendered height
    this.resizeObserver = new ResizeObserver(() => this.syncBarHeight());
    this.resizeObserver.observe(this.panel);
  }

  get element(): HTMLElement { return this.panel; }

  getSelected(): EntryPoint | null { return this.selected; }

  /** Previous param defaults — used to detect when a default changes across re-renders. */
  private prevDefaults: Record<string, any> = {};

  /** Clear all overrides and stored defaults (e.g. when user explicitly switches function). */
  resetOverrides() {
    this.overrides = {};
    this.prevDefaults = {};
    this.unitSelections = {};
  }

  /** Render the parameter UI for the given function (or hide if null).
   *  Returns the reconciled overrides so the caller can push them to the backend. */
  updateUI(entry: EntryPoint | null): Record<string, any> {
    this.selected = entry;
    this.reconcileOverrides(entry);
    this.render();
    return { ...this.overrides };
  }

  private render() {
    this.panel.innerHTML = '';
    if (!this.selected) {
      this.panel.style.display = 'none';
      this.syncBarHeight();
      return;
    }
    this.panel.style.display = 'flex';

    // Function name header — only show for non-Main functions (library calls, etc.)
    if (this.selected.name !== 'Main') {
      const header = document.createElement('div');
      const nameBadge = document.createElement('span');
      nameBadge.className = 'fn-preview-name';
      nameBadge.textContent = (this.selected.libVar
        ? `${this.selected.libVar}.`
        : '') + this.selected.name;
      header.appendChild(nameBadge);

      if (this.selected.params.length === 0) {
        const noArgs = document.createElement('span');
        noArgs.className = 'fn-preview-noargs';
        noArgs.textContent = '( )';
        header.appendChild(noArgs);
      }
      this.panel.appendChild(header);
    }

    // Params grid
    const configurableParams = this.selected.params.filter(p => p.constraint);
    if (configurableParams.length > 0) {
      const grid = document.createElement('div');
      grid.className = 'fn-preview-params-grid';
      for (const param of configurableParams) {
        grid.appendChild(this.makeParamInput(param));
      }
      this.panel.appendChild(grid);
    }

    // Ensure --fn-bar-h is set immediately (ResizeObserver may fire async)
    requestAnimationFrame(() => this.syncBarHeight());
  }

  private makeParamInput(param: EntryPoint['params'][0]): HTMLElement {
    const group = document.createElement('div');
    group.className = 'fn-preview-param';

    const label = document.createElement('label');
    label.className = 'fn-preview-param-label';
    label.textContent = param.name;
    group.appendChild(label);

    const c = param.constraint;

    if (param.type === 'Bool') {
      const input = document.createElement('input');
      input.type = 'checkbox';
      input.className = 'fn-preview-checkbox';
      const cur = this.overrides[param.name];
      input.checked = cur != null ? !!cur : (param.default === true);
      input.addEventListener('change', () => {
        this.overrides[param.name] = input.checked;
        this.applyOverrides();
      });
      group.appendChild(input);
    } else if (c?.kind === 'enum' && c.values?.length) {
      const select = document.createElement('select');
      select.className = 'fn-preview-select';
      for (const v of c.values) {
        const opt = document.createElement('option');
        opt.value = String(v);
        opt.textContent = String(v);
        select.appendChild(opt);
      }
      const cur = this.overrides[param.name] ?? param.default;
      if (cur != null) select.value = String(cur);
      select.addEventListener('change', () => {
        const val = parseFloat(select.value);
        this.overrides[param.name] = isNaN(val) ? select.value : val;
        this.applyOverrides();
      });
      group.appendChild(select);
    } else if (c?.kind === 'range' && c.min != null && c.max != null) {
      const min = Number(c.min);
      const max = Number(c.max);
      const step = c.step != null ? Number(c.step) : (max - min) / 100;
      const cur = this.overrides[param.name] ?? param.default ?? (min + max) / 2;

      const slider = document.createElement('input');
      slider.type = 'range';
      slider.className = 'fn-preview-slider';
      slider.min = String(min);
      slider.max = String(max);
      slider.step = String(step);
      slider.value = String(cur);

      const numInput = document.createElement('input');
      numInput.type = 'number';
      numInput.className = 'fn-preview-input fn-preview-input-sm';
      numInput.min = String(min);
      numInput.max = String(max);
      numInput.step = String(step);
      numInput.value = String(cur);

      slider.addEventListener('input', () => {
        numInput.value = slider.value;
        this.overrides[param.name] = parseFloat(slider.value);
        this.applyOverrides();
      });
      numInput.addEventListener('input', () => {
        slider.value = numInput.value;
        this.overrides[param.name] = parseFloat(numInput.value) || 0;
        this.applyOverrides();
      });

      group.appendChild(slider);
      group.appendChild(numInput);
    } else if (param.type === 'String') {
      const input = document.createElement('input');
      input.type = 'text';
      input.className = 'fn-preview-input';
      input.value = this.overrides[param.name] ?? param.default ?? '';
      input.addEventListener('input', () => {
        this.overrides[param.name] = input.value;
        this.applyOverrides();
      });
      group.appendChild(input);
    } else if (!c && (param.type === 'Length' || param.type === 'Angle')) {
      const unitOptions = param.type === 'Length' ? LENGTH_UNITS : ANGLE_UNITS;
      const selectedUnitName = this.unitSelections[param.name] ?? unitOptions[0][0];
      const unitFactor = () => unitOptions.find(u => u[0] === unitSelect.value)?.[1] ?? 1;
      const canonicalValue = this.overrides[param.name] as number ?? (param.default != null ? Number(param.default) : null);

      const numInput = document.createElement('input');
      numInput.type = 'number';
      numInput.className = 'fn-preview-input fn-preview-input-sm';
      numInput.step = 'any';
      const initialFactor = unitOptions.find(u => u[0] === selectedUnitName)?.[1] ?? 1;
      numInput.value = canonicalValue != null ? String(+((canonicalValue / initialFactor).toPrecision(6))) : '';
      numInput.placeholder = '0';

      const unitSelect = document.createElement('select');
      unitSelect.className = 'fn-preview-select fn-preview-unit-select';
      for (const [name] of unitOptions) {
        const opt = document.createElement('option');
        opt.value = name;
        opt.textContent = name;
        unitSelect.appendChild(opt);
      }
      unitSelect.value = selectedUnitName;

      numInput.addEventListener('input', () => {
        this.overrides[param.name] = (parseFloat(numInput.value) || 0) * unitFactor();
        this.applyOverrides();
      });
      unitSelect.addEventListener('change', () => {
        this.unitSelections[param.name] = unitSelect.value;
        const canonical = typeof this.overrides[param.name] === 'number'
          ? this.overrides[param.name] as number
          : canonicalValue ?? 0;
        numInput.value = String(+((canonical / unitFactor()).toPrecision(6)));
        // Re-apply so backend stays in sync with the canonical value (unchanged)
        this.applyOverrides();
      });

      group.appendChild(numInput);
      group.appendChild(unitSelect);
    } else {
      const input = document.createElement('input');
      input.type = 'number';
      input.className = 'fn-preview-input';
      input.step = 'any';
      input.value = String(this.overrides[param.name] ?? param.default ?? 0);
      input.addEventListener('input', () => {
        this.overrides[param.name] = parseFloat(input.value) || 0;
        this.applyOverrides();
      });
      group.appendChild(input);
    }

    if (param.unit) {
      const unit = document.createElement('span');
      unit.className = 'fn-preview-unit';
      unit.textContent = param.unit;
      group.appendChild(unit);
    }

    return group;
  }

  /**
   * Preserve overrides across re-renders unless:
   * - The param no longer exists
   * - The param's default value changed (user edited the default in code)
   * - The override is now outside the constraint range / not in the enum
   */
  private reconcileOverrides(entry: EntryPoint | null) {
    if (!entry) {
      this.overrides = {};
      this.prevDefaults = {};
      return;
    }

    const newDefaults: Record<string, any> = {};
    const kept: Record<string, any> = {};

    for (const p of entry.params) {
      newDefaults[p.name] = p.default;
      const prev = this.overrides[p.name];
      if (prev == null) {
        // Initialize required params so the backend can call them immediately.
        if (!p.hasDefault) {
          if (p.type === 'Length') kept[p.name] = 10; // 10 mm
          else if (p.type === 'Angle') kept[p.name] = 45; // 45 deg
          else kept[p.name] = 0;
        }
        continue; // no existing override to preserve
      }

      // Drop if default changed (user edited the code default)
      if (this.prevDefaults[p.name] !== undefined && p.default !== this.prevDefaults[p.name]) continue;

      // Drop if out of constraint range
      const c = p.constraint;
      if (c?.kind === 'range' && c.min != null && c.max != null) {
        const v = Number(prev);
        if (v < Number(c.min) || v > Number(c.max)) continue;
      }
      // Drop if not in enum values
      if (c?.kind === 'enum' && c.values?.length) {
        if (!c.values.some(v => String(v) === String(prev))) continue;
      }

      kept[p.name] = prev;
    }

    this.overrides = kept;
    this.prevDefaults = newDefaults;
  }

  private syncBarHeight() {
    if (this.panel.style.display === 'none') {
      this.container.style.removeProperty('--fn-bar-h');
    } else {
      this.container.style.setProperty('--fn-bar-h', `${this.panel.offsetHeight}px`);
    }
  }

  private applyOverrides() {
    this.callbacks.onOverrideChange({ ...this.overrides });
  }
}
