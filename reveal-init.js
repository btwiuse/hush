const HUSH_ENV = {
  TERM: 'xterm-256color',
  COLORTERM: 'truecolor',
  CLICOLOR_FORCE: '1',
  WANIX: '/opfs/wanix',
  HOME: '/opfs/home',
  PATH: '/opfs/wanix/go/bin:/opfs/wanix',
  CRUSH_CORE_UTILS: '1',
  DO_NOT_TRACK: '1',
  CRUSH_DISABLE_PROVIDER_AUTO_UPDATE: '1',
  TERM_WINCH: '/winch',
  GOROOT: '/go',
  GOPATH: '/opfs/gopath',
  GOTOOLDIR: '/opfs/wanix/go/pkg/tool/js_wasm',
  LOCATION: window.location.pathname,
};

let tabIdCounter = 0;
let activeTabId = null;
let homeTabId = 0;
let systemReady = false;
const tabs = new Map();

const system = document.querySelector('wanix-system');
const tabsContainer = document.getElementById('tabs-container');
const newTabBtn = document.getElementById('new-tab-btn');
const homeContent = document.getElementById('home-content');

// --- Reveal.js init ---
let deck = null;
function initReveal() {
  if (typeof Reveal === 'undefined') { requestAnimationFrame(initReveal); return; }
  if (deck) return;
  deck = new Reveal(homeContent.querySelector('.reveal'), {
    hash: false,
    controls: true,
    progress: true,
    center: true,
    transition: 'slide',
    backgroundTransition: 'fade',
    keyboard: true,
    keyboardCondition: () => !['INPUT', 'TEXTAREA', 'BUTTON'].includes(document.activeElement.tagName),
    overview: true,
    touch: true,
  });
  deck.initialize();
}
window.addEventListener('DOMContentLoaded', initReveal);

// --- Config (localStorage) ---
const CONFIG_KEY = 'gear-shell-config';
const DEFAULT_CONFIG = { cmd: 'hush', env: '', autoOpen: false };

      function loadConfig() {
        try {
          const saved = JSON.parse(localStorage.getItem(CONFIG_KEY));
          return { ...DEFAULT_CONFIG, ...saved };
        } catch { return { ...DEFAULT_CONFIG }; }
      }
function saveConfig(cfg) {
  localStorage.setItem(CONFIG_KEY, JSON.stringify(cfg));
}

      function populateConfigForm() {
        const cfg = loadConfig();
        document.getElementById('cfg-cmd').value = cfg.cmd;
        document.getElementById('cfg-env').value = cfg.env;
        document.getElementById('cfg-auto-open').checked = !!cfg.autoOpen;
      }

      document.getElementById('cfg-save').addEventListener('click', () => {
        saveConfig({
          cmd: document.getElementById('cfg-cmd').value.trim() || 'hush',
          env: document.getElementById('cfg-env').value,
          autoOpen: document.getElementById('cfg-auto-open').checked,
        });
  const s = document.getElementById('cfg-status');
  s.textContent = 'Saved!';
  s.style.color = '#3fb950';
  setTimeout(() => { s.textContent = ''; }, 2000);
});

document.getElementById('cfg-reset').addEventListener('click', () => {
  localStorage.removeItem(CONFIG_KEY);
  populateConfigForm();
  const s = document.getElementById('cfg-status');
  s.textContent = 'Reset to defaults.';
  s.style.color = '#8b949e';
  setTimeout(() => { s.textContent = ''; }, 2000);
});

populateConfigForm();

function buildEnv() {
  const cfg = loadConfig();
  const env = { ...HUSH_ENV };
  if (cfg.env.trim()) {
    for (const line of cfg.env.split('\n')) {
      const trimmed = line.trim();
      if (trimmed) {
        const [key, ...rest] = trimmed.split('=');
        if (key) {
          env[key] = rest.join('=');
        }
      }
    }
  }
  return Object.entries(env).map(([k, v]) => `${k}=${v}`).join(' ');
}

// --- Home tab (tab-0): always present, no terminal, not closable ---
{
  const tabEl = document.createElement('div');
  tabEl.className = 'tab';
  tabEl.dataset.tabId = 'home';

  const icon = document.createElement('img');
  icon.style.width = '28px';
  icon.style.height = '28px';
  icon.style.borderRadius = '4px';
  icon.style.display = 'block';
  icon.src = 'logo-home.png';
  icon.alt = '';
  icon.draggable = false;
  tabEl.appendChild(icon);
  tabEl.style.cssText = 'padding:0;width:36px;height:36px;display:flex;align-items:center;justify-content:center';

  tabEl.addEventListener('click', () => showHome());
  tabsContainer.appendChild(tabEl);

  tabs.set(homeTabId, { tabEl, isHome: true });
}

system.addEventListener('ready', () => {
  systemReady = true;
});

function showHome() {
  if (activeTabId !== null && tabs.has(activeTabId)) {
    const prev = tabs.get(activeTabId);
    if (prev.wrapperEl) prev.wrapperEl.classList.add('hidden');
    prev.tabEl.classList.remove('active');
  }
  activeTabId = homeTabId;
  homeContent.classList.remove('hidden');
  tabs.get(homeTabId).tabEl.classList.add('active');
  if (deck) deck.sync();
}

function hideHome() {
  if (activeTabId !== homeTabId) return;
  homeContent.classList.add('hidden');
  tabs.get(homeTabId).tabEl.classList.remove('active');
  activeTabId = null;
}

function wake(el) {
  if (el._awake) queueMicrotask(() => el._awake());
}

      async function createTab({ activate = true } = {}) {
        hideHome();

        const id = ++tabIdCounter;
        const cfg = loadConfig();

        const wrapper = document.createElement('div');
  wrapper.className = 'term-wrapper';

  const task = document.createElement('wanix-task');
  task.id = `repl-${id}`;
  task.setAttribute('cmd', cfg.cmd || 'hush');
  task.setAttribute('type', 'gojs');
  task.setAttribute('env', buildEnv());
  task.setAttribute('term', '');
  task.setAttribute('start', '');

  const winchBind = document.createElement('wanix-bind');
  winchBind.setAttribute('dst', 'winch');
  winchBind.setAttribute('src', '#task/self/term/winch');
  task.appendChild(winchBind);

  const term = document.createElement('wanix-term');
  term.setAttribute('raw', '');
  term.setAttribute('no-scrollbar', '');
  term.setAttribute('path', `#task/repl-${id}/term`);

  wrapper.appendChild(task);
  wrapper.appendChild(term);
  system.appendChild(wrapper);

  if (systemReady) {
    wake(task);
    wake(term);
  }

  const tabEl = document.createElement('div');
  tabEl.className = 'tab';
  tabEl.dataset.tabId = id;

  const label = document.createElement('span');
  label.textContent = `Term ${id}`;
  tabEl.appendChild(label);

  const closeBtn = document.createElement('span');
  closeBtn.className = 'close';
  closeBtn.textContent = '\u2715';
  closeBtn.title = 'Close tab';
  closeBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    closeTab(id);
  });
  tabEl.appendChild(closeBtn);

  tabEl.addEventListener('click', () => activateTab(id));
  tabsContainer.appendChild(tabEl);

  tabs.set(id, { taskEl: task, termEl: term, wrapperEl: wrapper, tabEl, isHome: false });

  if (activate) {
    activateTab(id);
  } else {
    wrapper.classList.add('hidden');
  }

  return id;
}

function activateTab(id) {
  if (activeTabId === id) return;
  hideHome();
  if (activeTabId !== null && activeTabId !== homeTabId && tabs.has(activeTabId)) {
    const prev = tabs.get(activeTabId);
    if (prev.wrapperEl) prev.wrapperEl.classList.add('hidden');
    prev.tabEl.classList.remove('active');
  }
  const entry = tabs.get(id);
  if (!entry) return;
  if (entry.wrapperEl) entry.wrapperEl.classList.remove('hidden');
  entry.tabEl.classList.add('active');
  activeTabId = id;
  if (entry.termEl && entry.termEl._term && entry.termEl._fitAddon) {
    setTimeout(() => {
      entry.termEl._fitAddon.fit();
      entry.termEl._term.focus();
    }, 0);
  }
}

function closeTab(id) {
  if (id === homeTabId) return;
  if (tabs.size <= 1) return;
  const entry = tabs.get(id);
  if (!entry) return;
  const wasActive = activeTabId === id;
  entry.wrapperEl.remove();
  entry.tabEl.remove();
  tabs.delete(id);
  if (wasActive) {
    if (tabs.size > 0) {
      const keys = [...tabs.keys()];
      const lastKey = keys[keys.length - 1];
      if (lastKey === homeTabId) {
        showHome();
      } else {
        activateTab(lastKey);
      }
    } else {
      showHome();
    }
  }
}

newTabBtn.addEventListener('click', () => createTab({ activate: true }));

// Start on Home
showHome();

// Auto-open terminal on page load if enabled
system.addEventListener('ready', () => {
  const cfg = loadConfig();
  if (cfg.autoOpen) {
    createTab({ activate: true });
  }
});
