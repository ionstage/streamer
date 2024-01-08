const template = document.createElement('template');

template.innerHTML = `<style>
.link {
  position: absolute;
}

.link-line {
  overflow: visible;
  position: absolute;
}

.link-handle {
  background-color: #bdbdbd;
  border-radius: 7px;
  height: 14px;
  left: -7px;
  position: absolute;
  top: -7px;
  width: 14px;
}

.link-outline {
  display: none;
  overflow: visible;
  position: absolute;
  z-index: -1;
}

:host(.selected) .link-outline {
  display: block;
}

.link-outline > .link-handle {
  border: 2px solid #42a5f5;
  border-radius: 9px;
  left: -9px;
  top: -9px;
}
</style>
<div class="link">
  <svg class="link-line">
    <line x1="0" y1="0" x2="0" y2="0" stroke="#bdbdbd" stroke-width="1" stroke-linecap="round" />
  </svg>
  <div class="link-handle"></div>
  <div class="link-outline">
    <svg class="link-line">
      <line x1="0" y1="0" x2="0" y2="0" stroke="#42a5f5" stroke-width="5" stroke-linecap="round" />
    </svg>
    <div class="link-handle"></div>
    <div class="link-handle"></div>
  </div>
</div>`;

class StreamerLink extends HTMLElement {
  static observedAttributes = ['x1', 'x2', 'y1', 'y2'];

  constructor () {
    super();
    this.attachShadow({ mode: 'open' });
    this.shadowRoot.appendChild(template.content.cloneNode(true));
  }

  attributeChangedCallback(name, oldValue, newValue) {
    const el = this.shadowRoot.lastElementChild;
    const lines = Array.from(el.querySelectorAll('.link-line')).map(el => el.firstElementChild);
    const handles = Array.from(el.querySelectorAll('.link-handle')).slice(0, -1);
    const x1 = Number(this.shadowRoot.host.getAttribute('x1'));
    const x2 = Number(this.shadowRoot.host.getAttribute('x2'));
    const y1 = Number(this.shadowRoot.host.getAttribute('y1'));
    const y2 = Number(this.shadowRoot.host.getAttribute('y2'));
    el.style.transform = `translate(${x1}px, ${y1}px)`;
    lines.forEach(el => {
      el.setAttribute('x2', x2 - x1);
      el.setAttribute('y2', y2 - y1);
    });
    handles.forEach(el => {
      el.style.transform = `translate(${x2 - x1}px, ${y2 - y1}px)`;
    });
  }
}

window.customElements.define('streamer-link', StreamerLink);
