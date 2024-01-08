const template = document.createElement('template');

template.innerHTML = `<style>
.node {
  display: grid;
  grid-template-rows: 32px auto;
  position: absolute;
}

.node-header {
  background-color: #f5f5f5;
  border: 1px solid #bdbdbd;
  border-radius: 8px 8px 0 0;
  position: relative;
}

.node-title {
  box-sizing: border-box;
  color: #9e9e9e;
  line-height: 30px;
  overflow: hidden;
  padding: 0 12px;
  position: absolute;
  text-overflow: ellipsis;
  width: 100%;
}

.node-port {
  background-color: #bdbdbd;
  border-radius: 7px;
  height: 14px;
  position: absolute;
  width: 14px;
}

.node-port.left {
  left: -7.5px;
  top: 8px;
}

.node-port.right {
  right: -7.5px;
  top: 8px;
}

.node-content {
  background-color: #fff;
  border: 1px solid #bdbdbd;
  border-radius: 0 0 8px 8px;
  border-top: 0;
  overflow: hidden;
}

.node-component {
  background-color: #212121;
  height: 100%;
  width: 100%;
}

.node-outline {
  border: 2px solid #42a5f5;
  border-radius: 10px;
  display: none;
  height: 100%;
  left: -2px;
  position: absolute;
  top: -2px;
  width: 100%;
  z-index: -1;
}

:host(.selected) .node-outline {
  display: block;
}

.node-outline > .node-port {
  background-color: #42a5f5;
  border-radius: 9px;
  height: 18px;
  width: 18px;
}

.node-outline > .node-port.left {
  left: -8.5px;
  top: 7px;
}

.node-outline > .node-port.right {
  right: -8.5px;
  top: 7px;
}
</style>
<div class="node">
  <div class="node-header">
    <div class="node-title"><slot name="title"></slot></div>
    <div class="node-port left"></div>
    <div class="node-port right"></div>
  </div>
  <div class="node-content">
    <div class="node-component">
      <slot name="component"></slot>
    </div>
  </div>
  <div class="node-outline">
    <div class="node-port left"></div>
    <div class="node-port right"></div>
  </div>
</div>`;

class StreamerNode extends HTMLElement {
  static observedAttributes = ['x', 'y', 'width', 'height'];

  constructor () {
    super();
    this.attachShadow({ mode: 'open' });
    this.shadowRoot.appendChild(template.content.cloneNode(true));
  }

  attributeChangedCallback(name, oldValue, newValue) {
    const el = this.shadowRoot.lastElementChild;
    switch (name) {
      case 'x':
        const y = this.shadowRoot.host.getAttribute('y');
        el.style.transform = `translate(${newValue}px, ${y}px)`;
        break;
      case 'y':
        const x = this.shadowRoot.host.getAttribute('x');
        el.style.transform = `translate(${x}px, ${newValue}px)`;
        break;
      case 'width':
        el.style.width = newValue + 'px';
        break;
      case 'height':
        el.style.height = newValue + 'px';
        break;
    }
  }
}

window.customElements.define('streamer-node', StreamerNode);
