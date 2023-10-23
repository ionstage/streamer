import { Content } from './core/content.js';
import { WebContentComponent } from './infra/web-content-component.js';

function main() {
  const content = new Content();
  content.component = new WebContentComponent(document.querySelector('.content'));
  content.start();
}

main();
