const TEMPLATE = document.createElement('template');
TEMPLATE.innerHTML = `
  <style>
    :host {
      display: block;
    }

    :host([hidden]) {
      display: none;
    }

    input {
      box-sizing: border-box;
      width: 100%;
      font-family: var(--font-sans, system-ui, sans-serif);
      font-size: var(--text-sm, 0.875rem);
      line-height: var(--leading-normal, 1.5);
      color: var(--color-fg, #ededed);
      background: var(--color-surface, #111116);
      border: 1px solid color-mix(in srgb, var(--color-border-hover, rgba(255, 255, 255, 0.22)) 75%, transparent);
      border-radius: var(--radius-md, 0.5rem);
      padding: 0 var(--space-3, 0.75rem);
      height: 2.25rem;
      transition:
        border-color var(--duration-fast, 120ms) var(--ease-standard, ease),
        box-shadow var(--duration-fast, 120ms) var(--ease-standard, ease);
    }

    input::placeholder {
      color: var(--color-fg-muted, #8e8e98);
    }

    input:hover:not(:disabled) {
      border-color: var(--color-border-hover, rgba(255, 255, 255, 0.22));
    }

    input:focus-visible {
      outline: 2px solid var(--color-focus-ring, #f43f5e);
      outline-offset: 2px;
      border-color: var(--color-focus-ring, #f43f5e);
    }

    :host([invalid]) input {
      border-color: var(--color-danger, #f87171);
    }

    :host([disabled]) input {
      opacity: 0.55;
      cursor: not-allowed;
    }

    @media (prefers-reduced-motion: reduce) {
      input {
        transition: none;
      }
    }
  </style>
  <input part="input" />
`;

/**
 * Text input Atom encapsulating native <input> with reactive value,
 * invalid/disabled/readonly states, and ui-input / ui-change events.
 */
export class UiInput extends HTMLElement {
  static observedAttributes = [
    'type', 'value', 'placeholder', 'disabled', 'readonly', 'required', 'invalid', 'name', 'aria-label',
  ];

  #input;

  constructor() {
    super();
    const shadow = this.attachShadow({ mode: 'open', delegatesFocus: true });
    shadow.appendChild(TEMPLATE.content.cloneNode(true));
    this.#input = shadow.querySelector('input');

    this.#input.addEventListener('input', () => {
      this.dispatchEvent(new CustomEvent('ui-input', {
        bubbles: true,
        composed: true,
        detail: { value: this.#input.value },
      }));
    });

    this.#input.addEventListener('change', () => {
      this.dispatchEvent(new CustomEvent('ui-change', {
        bubbles: true,
        composed: true,
        detail: { value: this.#input.value },
      }));
    });
  }

  connectedCallback() {
    this.#sync();
  }

  attributeChangedCallback(name, _oldVal, newVal) {
    if (name === 'value' && this.#input && this.#input.value !== (newVal ?? '')) {
      this.#input.value = newVal ?? '';
      return;
    }
    this.#sync();
  }

  get value() {
    return this.#input ? this.#input.value : (this.getAttribute('value') ?? '');
  }

  set value(val) {
    const next = val == null ? '' : String(val);
    if (this.#input) {
      this.#input.value = next;
    }
    this.setAttribute('value', next);
  }

  get disabled() {
    return this.hasAttribute('disabled');
  }

  set disabled(val) {
    this.toggleAttribute('disabled', Boolean(val));
  }

  get invalid() {
    return this.hasAttribute('invalid');
  }

  set invalid(val) {
    this.toggleAttribute('invalid', Boolean(val));
  }

  #sync() {
    if (!this.#input) return;
    this.#input.type = this.getAttribute('type') || 'text';
    this.#input.placeholder = this.getAttribute('placeholder') || '';
    this.#input.name = this.getAttribute('name') || '';
    this.#input.disabled = this.disabled;
    this.#input.readOnly = this.hasAttribute('readonly');
    this.#input.required = this.hasAttribute('required');
    this.#input.setAttribute('aria-invalid', String(this.invalid));
    if (this.hasAttribute('value') && this.#input.value !== this.getAttribute('value')) {
      this.#input.value = this.getAttribute('value') ?? '';
    }
    const ariaLabel = this.getAttribute('aria-label');
    if (ariaLabel) {
      this.#input.setAttribute('aria-label', ariaLabel);
    } else {
      this.#input.removeAttribute('aria-label');
    }
  }
}

if (!customElements.get('ui-input')) {
  customElements.define('ui-input', UiInput);
}
