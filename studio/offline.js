// Registro del service worker. Solo en http(s): file:// no tiene SW y los módulos tampoco.
if ("serviceWorker" in navigator && location.protocol !== "file:") {
  window.addEventListener("load", () => {
    navigator.serviceWorker.register("./sw.js").catch((e) => {
      // no es un fallo de la app: si el host no sirve sw.js, la página sigue en línea
      console.warn("sin service worker:", e && e.message ? e.message : e);
    });
  });
}
