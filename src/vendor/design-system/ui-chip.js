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

    button {
      all: unset;
      box-sizing: border-box;
      display: inline-flex;
      align-items: center;
      gap: var(--space-1, 0.25rem);
      font-family: var(--font-sans, system-ui, sans-serif);
      font-size: var(--text-xs, 0.75rem);
      font-weight: 500;
      line-height: 1;
      padding: 0.4rem 0.75rem;
      border-radius: var(--radius-full, 9999px);
      border: 1px solid var(--color-border, rgba(255, 255, 255, 0.08));
      background: var(--color-surface, #111116);
      color: var(--color-fg-secondary, #a1a1aa);
      cursor: pointer;
      user-select: none;
      white-space: nowrap;
      transition:
        background-color var(--duration-fast, 120ms) var(--ease-standard, ease),
        border-color var(--duration-fast, 120ms) var(--ease-standard, ease),
        color var(--duration-fast, 120ms) var(--ease-standard, ease);
    }

    button:hover:not(:disabled) {
      color: var(--color-fg, #ededed);
      border-color: var(--color-border-hover, rgba(255, 255, 255, 0.22));
    }

    :host([pressed]) button {
      background: var(--color-fg, #ededed);
      color: var(--color-bg, #08080b);
      border-color: var(--color-fg, #ededed);
    }

    :host([variant="accent"]) button {
      color: var(--color-accent, #f43f5e);
      border-color: color-mix(in srgb, var(--color-accent, #f43f5e) 50%, transparent);
    }

    :host([variant="accent"][pressed]) button {
      background: var(--color-accent, #f43f5e);
      color: var(--color-accent-fg, #08080b);
      border-color: var(--color-accent, #f43f5e);
    }

    button:focus-visible {
      outline: 2px solid var(--color-focus-ring, #f43f5e);
      outline-offset: 2px;
    }

    :host([disabled]) button {
      opacity: 0.5;
      cursor: not-allowed;
    }

    @media (prefers-reduced-motion: reduce) {
      button { transition: none; }
    }
  </style>
  <button part="chip" type="button" aria-pressed="false"><slot></slot></button>
`;

/**
 * Toggleable filter chip Atom with `pressed` state (`aria-pressed`) and `ui-toggle` event.
 */
export class UiChip extends HTMLElement {
  static observedAttributes = ['pressed', 'disabled', 'variant'];

  #btn;

  constructor() {
    super();
    const shadow = this.attachShadow({ mode: 'open', delegatesFocus: true });
    shadow.appendChild(TEMPLATE.content.cloneNode(true));
    this.#btn = shadow.querySelector('button');

    this.#btn.addEventListener('click', () => {
      if (this.disabled) return;
      this.pressed = !this.pressed;
      this.dispatchEvent(new CustomEvent('ui-toggle', {
        bubbles: true,
        composed: true,
        detail: { pressed: this.pressed },
      }));
    });
  }

  connectedCallback() {
    this.#sync();
  }

  attributeChangedCallback() {
    this.#sync();
  }

  get pressed() {
    return this.hasAttribute('pressed');
  }

  set pressed(val) {
    this.toggleAttribute('pressed', Boolean(val));
  }

  get disabled() {
    return this.hasAttribute('disabled');
  }

  set disabled(val) {
    this.toggleAttribute('disabled', Boolean(val));
  }

  #sync() {
    if (!this.#btn) return;
    this.#btn.disabled = this.disabled;
    this.#btn.setAttribute('aria-pressed', String(this.pressed));
  }
}

if (!customElements.get('ui-chip')) {
  customElements.define('ui-chip', UiChip);
}
