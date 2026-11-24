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

    .wrap {
      display: inline-flex;
      align-items: center;
      gap: var(--space-2, 0.5rem);
      font-family: var(--font-sans, system-ui, sans-serif);
      font-size: var(--text-sm, 0.875rem);
      line-height: var(--leading-normal, 1.5);
      color: var(--color-fg, #ededed);
      cursor: pointer;
      user-select: none;
    }

    button {
      position: relative;
      display: inline-flex;
      align-items: center;
      box-sizing: border-box;
      width: 2.35rem;
      height: 1.35rem;
      padding: 2px;
      margin: 0;
      border-radius: var(--radius-full, 9999px);
      border: 1px solid var(--color-border-hover, rgba(255, 255, 255, 0.22));
      background: var(--color-card-hover, rgba(255, 255, 255, 0.07));
      cursor: pointer;
      flex-shrink: 0;
      transition:
        background-color var(--duration-fast, 120ms) var(--ease-standard, ease),
        border-color var(--duration-fast, 120ms) var(--ease-standard, ease);
    }

    :host([size="sm"]) button {
      width: 1.95rem;
      height: 1.1rem;
    }

    .thumb {
      width: 0.95rem;
      height: 0.95rem;
      border-radius: var(--radius-full, 9999px);
      background: #ffffff;
      box-shadow: 0 1px 3px rgba(0, 0, 0, 0.35);
      transform: translateX(0);
      transition:
        transform var(--duration-fast, 120ms) var(--ease-standard, ease),
        background-color var(--duration-fast, 120ms) var(--ease-standard, ease);
    }

    :host([size="sm"]) .thumb {
      width: 0.75rem;
      height: 0.75rem;
    }

    button[aria-checked="true"] {
      background: var(--color-accent, #f43f5e);
      border-color: var(--color-accent, #f43f5e);
    }

    button[aria-checked="true"] .thumb {
      transform: translateX(1rem);
      background: #ffffff;
    }

    :host([size="sm"]) button[aria-checked="true"] .thumb {
      transform: translateX(0.85rem);
    }

    button:focus-visible {
      outline: 2px solid var(--color-focus-ring, #f43f5e);
      outline-offset: 2px;
    }

    :host([disabled]) .wrap,
    :host([disabled]) button {
      opacity: 0.55;
      cursor: not-allowed;
    }

    @media (prefers-reduced-motion: reduce) {
      button,
      .thumb {
        transition: none;
      }
    }
  </style>
  <span class="wrap" part="label">
    <button part="switch" type="button" role="switch" aria-checked="false">
      <span class="thumb" part="thumb" aria-hidden="true"></span>
    </button>
    <span class="text"><slot></slot></span>
  </span>
`;

/**
 * Boolean toggle switch Atom exposing role="switch" and aria-checked semantics.
 */
export class UiSwitch extends HTMLElement {
  static observedAttributes = ['checked', 'disabled', 'size', 'name', 'value', 'aria-label'];

  #btn;
  #wrap;

  constructor() {
    super();
    const shadow = this.attachShadow({ mode: 'open', delegatesFocus: true });
    shadow.appendChild(TEMPLATE.content.cloneNode(true));
    this.#btn = shadow.querySelector('button');
    this.#wrap = shadow.querySelector('.wrap');

    this.#wrap.addEventListener('click', (event) => {
      if (this.disabled) {
        event.preventDefault();
        return;
      }
      this.checked = !this.checked;
      this.dispatchEvent(new CustomEvent('ui-change', {
        bubbles: true,
        composed: true,
        detail: { checked: this.checked, value: this.value },
      }));
    });
  }

  connectedCallback() {
    this.#sync();
  }

  attributeChangedCallback() {
    this.#sync();
  }

  get checked() {
    return this.hasAttribute('checked');
  }

  set checked(val) {
    this.toggleAttribute('checked', Boolean(val));
  }

  get disabled() {
    return this.hasAttribute('disabled');
  }

  set disabled(val) {
    this.toggleAttribute('disabled', Boolean(val));
  }

  get value() {
    return this.getAttribute('value') || 'on';
  }

  set value(val) {
    this.setAttribute('value', val == null ? 'on' : String(val));
  }

  #sync() {
    if (!this.#btn) return;
    this.#btn.setAttribute('aria-checked', this.checked ? 'true' : 'false');
    if (this.disabled) {
      this.#btn.setAttribute('aria-disabled', 'true');
    } else {
      this.#btn.removeAttribute('aria-disabled');
    }
    const ariaLabel = this.getAttribute('aria-label') || this.textContent?.trim() || 'Toggle';
    this.#btn.setAttribute('aria-label', ariaLabel);
  }
}

if (!customElements.get('ui-switch')) {
  customElements.define('ui-switch', UiSwitch);
}
