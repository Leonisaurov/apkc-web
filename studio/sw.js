// Service worker de apkc studio: cachea el shell para recargar sin red. No cachea el jar del SDK,
// los keystores ni el proyecto (OPFS no pasa por HTTP). El binario wasm y wasm_exec.js se intentan
// cachear en la instalación; si el host no los sirve, se cachean la primera vez que se usan.
const VERSION = "apkc-studio-v1";
const CACHE = VERSION;

const SHELL = [
  "./", "./index.html", "./style.css", "./main.js", "./offline.js",
  "./app.js", "./historial.js", "./fusion.js", "./pestana.js", "./vistas.js",
  "./diagnostico.js", "./temas.js", "./apkc.wasm", "./wasm_exec.js",
];

const SOLO_RED = [
  /^\/android-jar(\/|$)/, /^\/android-jar-info$/, /^\/keystore/,
  /^\/apk-ejemplo$/, /^\/aab-ejemplo$/, /^\/apkc-test\.wasm$/,
];

self.addEventListener("install", (evento) => {
  evento.waitUntil(
    caches.open(CACHE)
      .then((cache) => Promise.all(SHELL.map((ruta) =>
        cache.add(ruta).catch(() => { /* un estático que falte no rompe la instalación */ }))))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (evento) => {
  evento.waitUntil(
    caches.keys()
      .then((claves) => Promise.all(claves.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (evento) => {
  const peticion = evento.request;
  if (peticion.method !== "GET") return;
  const url = new URL(peticion.url);
  if (url.origin !== self.location.origin) return;
  if (SOLO_RED.some((patron) => patron.test(url.pathname))) return;

  if (peticion.mode === "navigate") {
    evento.respondWith(
      fetch(peticion)
        .then((respuesta) => {
          if (respuesta && respuesta.ok) {
            const copia = respuesta.clone();
            caches.open(CACHE).then((cache) => cache.put("./index.html", copia)).catch(() => {});
          }
          return respuesta;
        })
        .catch(() => caches.match("./index.html"))
    );
    return;
  }

  evento.respondWith(
    caches.match(peticion, { ignoreSearch: true }).then((cacheada) => {
      if (cacheada) return cacheada;
      return fetch(peticion).then((respuesta) => {
        if (respuesta && respuesta.ok) {
          const copia = respuesta.clone();
          caches.open(CACHE).then((cache) => cache.put(peticion, copia)).catch(() => {});
        }
        return respuesta;
      });
    })
  );
});
