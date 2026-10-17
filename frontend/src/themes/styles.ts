// Page-wide <style> elements. The theme's variables go first and the custom
// CSS after, so custom rules win at equal specificity, whichever loads first.

const THEME_ID = 'vigie-theme';
const CUSTOM_ID = 'vigie-custom-css';

function styleElement(id: string): HTMLStyleElement {
  let el = document.getElementById(id) as HTMLStyleElement | null;
  if (!el) {
    el = document.createElement('style');
    el.id = id;
    const custom = document.getElementById(CUSTOM_ID);
    if (id === THEME_ID && custom) {
      document.head.insertBefore(el, custom);
    } else {
      document.head.appendChild(el);
    }
  }
  return el;
}

export function setThemeCSS(css: string) {
  styleElement(THEME_ID).textContent = css;
}

// textContent, not innerHTML: the CSS cannot close the <style> element.
export function setCustomCSS(css: string) {
  styleElement(CUSTOM_ID).textContent = css;
}
