export class Content {
  constructor() {
    this._objects = [];
    this._component = null;
  }

  set component(value) {
    value.delegate = new ContentComponentDelegateImpl(this);
    this._component = value;
  }

  start() { /* TODO */ }
}

export class ContentComponent {
  constructor() {
    this._delegate = null;
  }

  set delegate(value) {
    this._delegate = value;
  }
}

export class ContentComponentDelegate { /* TODO */ }

class ContentComponentDelegateImpl extends ContentComponentDelegate {
  constructor(content) {
    super();
    this._content = content;
  }
}
