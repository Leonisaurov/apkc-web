// Temas de interfaz y de terminal. Se aplican en <html> con data-tema/data-tema-consola y las
// variables de style.css hacen el resto; no hay dependencias ni hojas externas.
(() => {
  const CLAVE_UI = "apkc-studio/tema";
  const CLAVE_TERM = "apkc-studio/tema-consola";

  const UI = {
    apkc: "Oscuro",
    catppuccin: "Catppuccin Mocha",
    "rose-pine": "Rosé Pine",
    "tokyo-night": "Tokyo Night",
    gruvbox: "Gruvbox",
  };
  const TERM = {
    catppuccin: "Catppuccin Mocha",
    "rose-pine": "Rosé Pine",
    gruvbox: "Gruvbox",
    "solarized-dark": "Solarized Dark",
    dracula: "Dracula",
  };

  function leer(clave, permitidos, porDefecto) {
    try {
      const valor = localStorage.getItem(clave);
      return valor && permitidos[valor] ? valor : porDefecto;
    } catch (e) {
      return porDefecto;
    }
  }

  function aplicar() {
    const ui = leer(CLAVE_UI, UI, "apkc");
    const term = leer(CLAVE_TERM, TERM, "catppuccin");
    document.documentElement.dataset.tema = ui;
    document.documentElement.dataset.temaConsola = term;
  }

  aplicar();

  document.addEventListener("DOMContentLoaded", () => {
    const selUI = document.getElementById("tema-ui");
    const selTerm = document.getElementById("tema-consola");
    if (!selUI || !selTerm) return;
    for (const [id, nombre] of Object.entries(UI)) selUI.add(new Option(nombre, id));
    for (const [id, nombre] of Object.entries(TERM)) selTerm.add(new Option(nombre, id));
    selUI.value = document.documentElement.dataset.tema;
    selTerm.value = document.documentElement.dataset.temaConsola;

    selUI.addEventListener("change", () => {
      document.documentElement.dataset.tema = selUI.value;
      try { localStorage.setItem(CLAVE_UI, selUI.value); } catch (e) { /* sin persistencia */ }
    });
    selTerm.addEventListener("change", () => {
      document.documentElement.dataset.temaConsola = selTerm.value;
      try { localStorage.setItem(CLAVE_TERM, selTerm.value); } catch (e) { /* sin persistencia */ }
    });

    // el menú de tema se cierra al elegir o al hacer clic fuera
    const menu = document.getElementById("menu-apariencia");
    if (menu) {
      menu.addEventListener("change", () => { menu.open = false; });
      document.addEventListener("pointerdown", (e) => {
        if (menu.open && !menu.contains(e.target)) menu.open = false;
      });
      document.addEventListener("keydown", (e) => {
        if (e.key === "Escape") menu.open = false;
      });
    }
  });
})();
