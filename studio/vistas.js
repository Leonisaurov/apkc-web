// El marco de la página: dos vistas (crear y editor), y el botón que escribe el proyecto
// generado en OPFS y abre sus ficheros. La lógica de apkc.wasm y del editor sigue en pestana.js;
// aquí solo se decide qué se ve y se siembra el árbol con buildProject.

(() => {
  const NOMBRES = { crear: "Inicio", pestana: "Editor" };

  function mostrar(vista) {
    if (!document.getElementById(vista)) vista = "crear";
    for (const seccion of document.querySelectorAll("#vistas > .vista")) {
      seccion.classList.toggle("activa", seccion.id === vista);
    }
    for (const boton of document.querySelectorAll("[data-vista]")) {
      const activo = boton.dataset.vista === vista;
      if (boton.classList.contains("pestana-vista")) boton.classList.toggle("activa", activo);
      else boton.classList.toggle("activo", activo);
    }
    document.body.dataset.vista = vista;
    const estado = document.getElementById("estado-general");
    if (estado) estado.textContent = NOMBRES[vista] || vista;
    const vistas = document.getElementById("vistas");
    if (vistas) {
      vistas.classList.toggle("modo-editor", vista === "pestana");
      vistas.scrollTop = 0;
    }
  }

  for (const boton of document.querySelectorAll("[data-vista]")) {
    boton.addEventListener("click", () => {
      mostrar(boton.dataset.vista);
      try { history.replaceState(null, "", "#" + boton.dataset.vista); } catch (e) { /* file:// sin historial */ }
    });
  }

  const inicial = (location.hash || "").replace(/^#/, "");
  mostrar(inicial || "crear");
  window.addEventListener("hashchange", () => mostrar((location.hash || "").replace(/^#/, "")));

  // el nombre de la app en la barra de título y el package en el explorador
  const form = document.getElementById("form");
  const titulo = document.getElementById("titulo-proyecto");
  const proyecto = document.getElementById("lateral-proyecto");
  function reflejar() {
    if (!form) return;
    const datos = new FormData(form);
    const nombre = String(datos.get("name") || "").trim() || "Mi app";
    const paquete = String(datos.get("package") || "").trim();
    if (titulo) titulo.textContent = nombre;
    if (proyecto) proyecto.textContent = paquete || "MI APP";
    document.title = nombre + " — apkc studio";
  }
  if (form) {
    form.addEventListener("input", reflejar);
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      const boton = document.getElementById("crear-proyecto");
      if (boton) boton.click();
    });
    reflejar();
  }

  // el estado que escribe la pestaña se ve también en la barra de pestañas
  const estadoPestana = document.getElementById("estado-pestana");
  const estadoCab = document.getElementById("estado-pestana-cab");
  if (estadoPestana && estadoCab) {
    const copiar = () => { estadoCab.textContent = estadoPestana.textContent; };
    new MutationObserver(copiar).observe(estadoPestana, { childList: true, characterData: true, subtree: true });
    copiar();
  }

  const aviso = document.getElementById("estado-crear");
  const texto = (n) => new TextEncoder().encode(n);
  const botonCrear = document.getElementById("crear-proyecto");

  async function sembrar() {
    if (!form) return;
    const datos = new FormData(form);
    const nombre = String(datos.get("name") || "").trim();
    const paquete = String(datos.get("package") || "").trim().toLowerCase();
    if (!nombre) return "Escribe el nombre de la app.";
    if (!/^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$/.test(paquete)) {
      return "El package tiene que ser minúsculas con al menos dos tramos: com.mi.app";
    }

    const estado = {
      name: nombre,
      package: paquete,
      minsdk: "21",
      targetsdk: "33",
      hello: "Hello World!",
      button: false,
      buttonLabel: "Press me",
      toast: "Clicked!",
      kotlin: false,
      libs: false,
      native: false,
      assets: false,
      device: false,
    };
    const p = buildProject(estado);

    const icono = document.getElementById("icono");
    let png = null;
    if (icono && icono.files && icono.files.length) {
      const b = new Uint8Array(await icono.files[0].arrayBuffer());
      if (!(b[0] === 0x89 && b[1] === 0x50 && b[2] === 0x4e && b[3] === 0x47)) {
        return "El icono tiene que ser un PNG.";
      }
      png = b;
      p.files["AndroidManifest.xml"] = p.files["AndroidManifest.xml"]
        .replace("<application ", '<application android:icon="@drawable/icono" ');
    }

    // se borra el proyecto anterior: un /proyecto viejo se traga el create siguiente
    await hospedador.limpiar(PROYECTO);
    for (const [ruta, contenido] of Object.entries(p.files)) {
      await hospedador.escribir(PROYECTO + "/" + ruta, texto(contenido));
    }
    for (const d of p.dirs) {
      // un fichero vacío y su borrado dejan el directorio creado, también en OPFS
      await hospedador.escribir(PROYECTO + "/" + d + "/.keep", new Uint8Array(0));
      await hospedador.limpiar(PROYECTO + "/" + d + "/.keep");
    }
    if (png) await hospedador.escribir(PROYECTO + "/res/drawable/icono.png", png);

    await refrescarArbol();
    await abrir(PROYECTO + "/AndroidManifest.xml");
    mostrar("pestana");
    try { history.replaceState(null, "", "#pestana"); } catch (e) { /* */ }
    return "proyecto creado; abre un fichero del árbol y edítalo";
  }

  if (botonCrear) {
    botonCrear.addEventListener("click", async () => {
      if (aviso) { aviso.textContent = "creando…"; }
      try {
        const r = await sembrar();
        if (aviso) aviso.textContent = r || "";
      } catch (e) {
        if (aviso) {
          aviso.textContent = "no se pudo crear aquí: " + (e && e.message ? e.message : e) +
            ". Sirve la página por http (servidor.js) para usar OPFS.";
        }
      }
    });
  }
})();
