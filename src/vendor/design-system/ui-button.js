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
      justify-content: center;
      gap: var(--space-2, 0.5rem);
      font-family: var(--font-sans, system-ui, sans-serif);
      font-weight: 600;
      line-height: 1;
      border-radius: var(--radius-md, 0.5rem);
      border: 1px solid transparent;
      cursor: pointer;
      user-select: none;
      white-space: nowrap;
      transition:
        background-color var(--duration-fast, 120ms) var(--ease-standard, ease),
        border-color var(--duration-fast, 120ms) var(--ease-standard, ease),
        color var(--duration-fast, 120ms) var(--ease-standard, ease),
        opacity var(--duration-fast, 120ms) var(--ease-standard, ease);
    }

    /* Sizes */
    :host([size="sm"]) button {
      font-size: var(--text-xs, 0.75rem);
      padding: var(--space-1, 0.25rem) var(--space-3, 0.75rem);
      min-height: 1.875rem;
    }

    :host(:not([size])) button,
    :host([size="md"]) button {
      font-size: var(--text-sm, 0.875rem);
      padding: var(--space-2, 0.5rem) var(--space-4, 1rem);
      min-height: 2.25rem;
    }

    :host([size="lg"]) button {
      font-size: var(--text-md, 1rem);
      padding: var(--space-3, 0.75rem) var(--space-5, 1.25rem);
      min-height: 2.75rem;
    }

    /* Variants */
    :host(:not([variant])) button,
    :host([variant="primary"]) button {
      background: var(--color-accent, #f43f5e);
      color: var(--color-accent-fg, #08080b);
    }

    :host(:not([variant])) button:hover:not(:disabled):not([aria-busy="true"]),
    :host([variant="primary"]) button:hover:not(:disabled):not([aria-busy="true"]) {
      background: var(--color-accent-hover, #fb7185);
    }

    :host([variant="secondary"]) button {
      background: var(--color-card, rgba(255, 255, 255, 0.04));
      color: var(--color-fg, #ededed);
      border-color: var(--color-border, rgba(255, 255, 255, 0.08));
    }

    :host([variant="secondary"]) button:hover:not(:disabled):not([aria-busy="true"]) {
      background: var(--color-card-hover, rgba(255, 255, 255, 0.07));
      border-color: var(--color-border-hover, rgba(255, 255, 255, 0.22));
    }

    :host([variant="ghost"]) button {
      background: transparent;
      color: var(--color-fg-secondary, #a1a1aa);
    }

    :host([variant="ghost"]) button:hover:not(:disabled):not([aria-busy="true"]) {
      background: var(--color-card, rgba(255, 255, 255, 0.04));
      color: var(--color-fg, #ededed);
    }

    :host([variant="surface"]) button {
      background: var(--color-surface, #111116);
      color: var(--color-fg, #ededed);
      border-color: var(--color-border-hover, rgba(255, 255, 255, 0.22));
      box-shadow: var(--shadow-sm, 0 1px 2px rgba(0, 0, 0, 0.35));
    }

    :host([variant="danger"]) button {
      background: var(--color-danger, #f87171);
      color: var(--color-danger-fg, #08080b);
    }

    /* Toggle button that is on */
    :host([aria-pressed="true"]:not([variant="danger"])) button {
      background: color-mix(in srgb, var(--color-accent, #f43f5e) 18%, transparent);
      border-color: color-mix(in srgb, var(--color-accent, #f43f5e) 50%, transparent);
      color: var(--color-fg, #ededed);
    }

    :host([aria-pressed="true"]:not([variant="danger"])) button:hover:not(:disabled):not([aria-busy="true"]) {
      background: color-mix(in srgb, var(--color-accent, #f43f5e) 28%, transparent);
    }

    /* Focus ring */
    button:focus-visible {
      outline: 2px solid var(--color-focus-ring, #f43f5e);
      outline-offset: 2px;
    }

    /* Disabled & busy states */
    :host([disabled]) button,
    :host([busy]) button {
      opacity: 0.55;
      cursor: not-allowed;
    }

    .spinner {
      display: none;
      width: 0.875em;
      height: 0.875em;
      border: 2px solid currentColor;
      border-right-color: transparent;
      border-radius: 9999px;
      animation: ui-spin 650ms linear infinite;
    }

    :host([busy]) .spinner {
      display: inline-block;
    }

    @keyframes ui-spin {
      to { transform: rotate(360deg); }
    }

    @media (prefers-reduced-motion: reduce) {
      button {
        transition: none;
      }
      .spinner {
        animation: none;
        border-right-color: currentColor;
        opacity: 0.7;
      }
    }
  </style>
  <button part="button" type="button">
    <span class="spinner" aria-hidden="true"></span>
    <slot></slot>
  </button>
`;

/**
 * Interactive button Atom encapsulating native <button> with variant, size,
 * disabled, and busy state handling.
 */
export class UiButton extends HTMLElement {
  static observedAttributes = ['disabled', 'busy', 'type', 'variant', 'size', 'aria-label', 'aria-pressed'];

  #btn;

  constructor() {
    super();
    const shadow = this.attachShadow({ mode: 'open', delegatesFocus: true });
    shadow.appendChild(TEMPLATE.content.cloneNode(true));
    this.#btn = shadow.querySelector('button');

    this.#btn.addEventListener('click', (event) => {
      if (this.disabled || this.busy) {
        event.preventDefault();
        event.stopImmediatePropagation();
        return;
      }
      this.dispatchEvent(new CustomEvent('ui-press', {
        bubbles: true,
        composed: true,
        detail: { variant: this.variant },
      }));
    });
  }

  connectedCallback() {
    this.#sync();
  }

  attributeChangedCallback() {
    this.#sync();
  }

  get disabled() {
    return this.hasAttribute('disabled');
  }

  set disabled(val) {
    this.toggleAttribute('disabled', Boolean(val));
  }

  get busy() {
    return this.hasAttribute('busy');
  }

  set busy(val) {
    this.toggleAttribute('busy', Boolean(val));
  }

  get variant() {
    return this.getAttribute('variant') || 'primary';
  }

  set variant(val) {
    this.setAttribute('variant', val);
  }

  #sync() {
    if (!this.#btn) return;
    const isDisabled = this.disabled;
    const isBusy = this.busy;
    this.#btn.type = this.getAttribute('type') || 'button';
    this.#btn.disabled = isDisabled;
    this.#btn.setAttribute('aria-disabled', String(isDisabled || isBusy));
    this.#btn.setAttribute('aria-busy', String(isBusy));
    const ariaLabel = this.getAttribute('aria-label');
    if (ariaLabel) {
      this.#btn.setAttribute('aria-label', ariaLabel);
    } else {
      this.#btn.removeAttribute('aria-label');
    }
    const pressed = this.getAttribute('aria-pressed');
    if (pressed === null) {
      this.#btn.removeAttribute('aria-pressed');
    } else {
      this.#btn.setAttribute('aria-pressed', String(pressed === 'true'));
    }
  }
}

if (!customElements.get('ui-button')) {
  customElements.define('ui-button', UiButton);
}
