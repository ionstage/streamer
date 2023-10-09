export class Content {
  constructor() {
    this._component = null;
  }

  set component(value) {
    this._component = value;
  }
}

export class ContentComponent {
  constructor() {
    this._delegate = null;
  }

  set delegate(value) {
    this._delegate = value;
  }
}
