const TEMPLATE = document.createElement('template');
TEMPLATE.innerHTML = `
  <style>
    :host {
      display: inline-flex;
      vertical-align: middle;
    }

    :host([hidden]) {
      display: none;
    }

    span {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1, 0.25rem);
      font-family: var(--font-mono, ui-monospace, monospace);
      font-size: var(--text-xs, 0.75rem);
      font-weight: 600;
      line-height: 1;
      letter-spacing: 0.02em;
      padding: 0.25rem 0.55rem;
      border-radius: var(--radius-full, 9999px);
      border: 1px solid var(--color-border, rgba(255, 255, 255, 0.08));
      background: var(--color-card, rgba(255, 255, 255, 0.04));
      color: var(--color-fg-secondary, #a1a1aa);
      white-space: nowrap;
    }

    :host([size="md"]) span {
      font-size: var(--text-sm, 0.875rem);
      padding: 0.35rem 0.75rem;
    }

    :host([variant="accent"]) span {
      color: var(--color-accent, #f43f5e);
      border-color: color-mix(in srgb, var(--color-accent, #f43f5e) 35%, transparent);
      background: color-mix(in srgb, var(--color-accent, #f43f5e) 12%, transparent);
    }

    :host([variant="info"]) span {
      color: var(--color-info, #38bdf8);
      border-color: color-mix(in srgb, var(--color-info, #38bdf8) 35%, transparent);
      background: color-mix(in srgb, var(--color-info, #38bdf8) 12%, transparent);
    }

    :host([variant="success"]) span {
      color: var(--color-success, #10b981);
      border-color: color-mix(in srgb, var(--color-success, #10b981) 35%, transparent);
      background: color-mix(in srgb, var(--color-success, #10b981) 12%, transparent);
    }

    :host([variant="warning"]) span {
      color: var(--color-warning, #f59e0b);
      border-color: color-mix(in srgb, var(--color-warning, #f59e0b) 35%, transparent);
      background: color-mix(in srgb, var(--color-warning, #f59e0b) 12%, transparent);
    }
  </style>
  <span part="badge"><slot></slot></span>
`;

/**
 * Status/metadata badge Atom supporting neutral, accent, info, success, and warning variants.
 */
export class UiBadge extends HTMLElement {
  static observedAttributes = ['variant', 'size'];

  constructor() {
    super();
    const shadow = this.attachShadow({ mode: 'open' });
    shadow.appendChild(TEMPLATE.content.cloneNode(true));
  }
}

if (!customElements.get('ui-badge')) {
  customElements.define('ui-badge', UiBadge);
}
