// layout.js
// - Подключает общий header/footer из статических фрагментов
// - Подключает "шаблон" (Bootswatch темы + Bootstrap JS + Bootstrap Icons) централизованно
// - Переключение темы (Zephyr/Darkly) одной кнопкой в header
// - Подсвечивает активный пункт меню

const UI = {
  storageKey: "ui-theme-mode",
  themes: {
    light: {
      name: "zephyr",
      cssHref:
        "https://cdn.jsdelivr.net/npm/bootswatch@5.3.8/dist/zephyr/bootstrap.min.css"
    },
    dark: {
      name: "darkly",
      cssHref:
        "https://cdn.jsdelivr.net/npm/bootswatch@5.3.8/dist/darkly/bootstrap.min.css"
    }
  },
  bootstrap: {
    // CSS берём из Bootswatch; нужен только JS bundle для dropdown/collapse/offcanvas.
    jsSrc:
      "https://cdn.jsdelivr.net/npm/bootstrap@5.3.8/dist/js/bootstrap.bundle.min.js",
    jsIntegrity:
      "sha384-FKyoEForCGlyvwx9Hj09JcYn3nv7wiPVlz7YYwJrWVcXK/BmnVDxM+D2scQbITxI"
  },
  icons: {
    cssHref:
      "https://cdn.jsdelivr.net/npm/bootstrap-icons@1.13.1/font/bootstrap-icons.min.css"
  }
};

const THEME_LINK_ID = "app-theme-css";

function getSavedMode() {
  const m = localStorage.getItem(UI.storageKey);
  return m === "dark" || m === "light" ? m : "light";
}

function setSavedMode(mode) {
  localStorage.setItem(UI.storageKey, mode);
}

function ensureThemeCss(mode) {
  const theme = UI.themes[mode] || UI.themes.light;
  let link = document.getElementById(THEME_LINK_ID);
  if (!link) {
    link = document.createElement("link");
    link.id = THEME_LINK_ID;
    link.rel = "stylesheet";
    document.head.appendChild(link);
  }
  if (link.getAttribute("href") !== theme.cssHref) {
    link.setAttribute("href", theme.cssHref);
  }

  // Для совместимости с компонентами BS 5.3 color-modes.
  document.documentElement.setAttribute("data-bs-theme", mode);
}

function ensureBootstrapJs() {
  const hasJs = Array.from(document.querySelectorAll("script")).some(
    (s) => (s.getAttribute("src") || "") === UI.bootstrap.jsSrc
  );
  if (!hasJs) {
    const s = document.createElement("script");
    s.src = UI.bootstrap.jsSrc;
    s.integrity = UI.bootstrap.jsIntegrity;
    s.crossOrigin = "anonymous";
    s.async = false;
    document.head.appendChild(s);
  }
}

function ensureBootstrapIcons() {
  const hasCss = Array.from(document.querySelectorAll('link[rel="stylesheet"]')).some(
    (l) => (l.getAttribute("href") || "") === UI.icons.cssHref
  );
  if (!hasCss) {
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = UI.icons.cssHref;
    document.head.appendChild(link);
  }
}

async function includeFragment(selector, url) {
  const host = document.querySelector(selector);
  if (!host) return null;
  try {
    const resp = await fetch(url, { cache: "no-store" });
    if (!resp.ok) throw new Error("HTTP " + resp.status);
    const html = await resp.text();
    host.innerHTML = html;
    return host;
  } catch (e) {
    console.error("includeFragment error for", url, e);
    return null;
  }
}

function normalizePath(p) {
  if (!p) return "/";
  const clean = p.split("?")[0].split("#")[0];
  if (clean === "" || clean === "/index.html") return "/";
  return clean;
}

function markActiveNav() {
  const current = normalizePath(window.location.pathname);

  const links = Array.from(
    document.querySelectorAll("nav a.nav-link, nav a.dropdown-item")
  );

  let activeLink = null;
  for (const a of links) {
    const href = a.getAttribute("href");
    if (!href) continue;
    const path = normalizePath(href);
    if (path === current) {
      activeLink = a;
      break;
    }
  }

  if (!activeLink) return;

  activeLink.classList.add("active");
  activeLink.setAttribute("aria-current", "page");
}

function updateThemeToggleUI(mode) {
  const btn = document.getElementById("theme-toggle");
  if (!btn) return;

  const icon = btn.querySelector("i");
  const isDark = mode === "dark";

  if (icon) {
    icon.className = isDark ? "bi bi-sun" : "bi bi-moon-stars";
  }

  btn.title = isDark ? "Светлая тема" : "Тёмная тема";
  btn.setAttribute("aria-label", btn.title);
  btn.setAttribute("data-mode", mode);
}

function wireThemeToggle() {
  const btn = document.getElementById("theme-toggle");
  if (!btn) return;

  btn.addEventListener("click", () => {
    const current = getSavedMode();
    const next = current === "dark" ? "light" : "dark";
    setSavedMode(next);
    ensureThemeCss(next);
    updateThemeToggleUI(next);
  });

  updateThemeToggleUI(getSavedMode());
}

// Применяем тему как можно раньше (скрипт обычно подключен в <head defer>)
ensureThemeCss(getSavedMode());
ensureBootstrapJs();
ensureBootstrapIcons();

document.addEventListener("DOMContentLoaded", async () => {
  // Чуть более "приложенческий" лэйаут по умолчанию
  document.body.classList.add("d-flex", "flex-column", "min-vh-100");

  await includeFragment('[data-include="header"]', "/header.html");
  await includeFragment('[data-include="footer"]', "/footer.html");

  markActiveNav();
  wireThemeToggle();
});
