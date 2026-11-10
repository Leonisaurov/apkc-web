import { Historial } from "./historial.js";
import { fusionar } from "./fusion.js";
import { esXml, resaltar } from "./editor.js";
import { CONSOLA, pintarConsola, consolaHtml, textoHtml, enlazarSalidas,
  mostrarCarga, ocultarCarga, conCarga } from "./consola.js";
import { leerZip, zipDelArbol, paqueteDelManifiesto, nombreDeZip,
  ajustesDelManifiesto, conAjustes } from "./proyecto.js";

// La pestaña que hace: editor de fuente + un arbol en OPFS + los comandos de apkc.wasm.
//
// Todo lo que se escribe aquí se queda en el almacenamiento del propio navegador: no hay
// server al que subir el fuente ni el keystore. Lo que sí es nuevo respecto al generador de
// la sección 1 es que el que escribe ficheros ahora es apkc (en wasm), no esta página: la
// página solo edita lo que hay en el árbol y le pasa los comandos al worker.

const PROYECTO = "/proyecto";

const hospedador = (() => {
  // el worker se crea al primer uso, no al cargar el fichero: la prueba de node exige estas
  // funciones puras (el resaltador) y no tiene Worker ni document
  let worker = null, próximo = 1, alSalir = null;
  const pendientes = new Map();
  const asegurar = () => {
    if (worker) return worker;
    worker = new Worker("hospedador.js", { type: "module" });   // module: el hospedador importa su fs con import
    worker.onmessage = (e) => {
      if (e.data && e.data.tipo === "salida") {
        if (alSalir) alSalir(String(e.data.texto || ""));
        return;
      }
      const resolver = pendientes.get(e.data.id);
      if (resolver) { pendientes.delete(e.data.id); resolver(e.data); }
    };
    return worker;
  };
  const pedir = (mensaje) => new Promise((resuelve) => {
    const id = próximo++;
    pendientes.set(id, resuelve);
    asegurar().postMessage({ ...mensaje, id });
  });
  // navigator.storage no existe dentro del worker, así que el mango raíz de OPFS lo baja la
  // página y se lo pasa al hospedador; sin esto el fs no tiene dónde escribir
  let raizPedida = null;
  const bajarRaiz = () => {
    if (raizPedida) return raizPedida;
    raizPedida = (async () => {
      if (!navigator.storage || !navigator.storage.getDirectory) {
        throw new Error("este navegador no expone OPFS (navigator.storage.getDirectory)");
      }
      const raiz = await navigator.storage.getDirectory();
      await pedir({ tipo: "raiz", raiz });
      return raiz;
    })();
    return raizPedida;
  };
  return {
    arbol: async () => { await bajarRaiz(); return pedir({ tipo: "arbol" }); },
    leer: async (ruta) => { await bajarRaiz(); return pedir({ tipo: "leer", ruta }); },
    escribir: async (ruta, datos) => { await bajarRaiz(); return pedir({ tipo: "montar", ruta, datos }); },
    ejecutar: async (argv) => { await bajarRaiz(); return pedir({ tipo: "ejecutar", argv }); },
    limpiar: async (ruta) => { await bajarRaiz(); return pedir({ tipo: "limpiar", ruta }); },
    refrescar: async (ruta) => { await bajarRaiz(); return pedir({ tipo: "refrescar", ruta }); },
    renombrar: async (de, a) => { await bajarRaiz(); return pedir({ tipo: "renombrar", de, a }); },
    cwd: async (ruta) => { await bajarRaiz(); return pedir({ tipo: "cwd", ruta }); },
    sondear: async () => { await bajarRaiz(); return pedir({ tipo: "sondeo" }); },
    // la salida llega por trozos mientras el comando corre; `correr` la pinta sin esperar al final
    escucharSalida(fn) { alSalir = fn; },
    // Go wasm no tiene kill: se termina el worker entero y se recrea en el siguiente comando. El
    // fs vive en OPFS, así que lo escrito hasta aquí no se pierde; el build puede quedar a medias.
    cancelar() {
      if (!worker) return false;
      try { worker.terminate(); } catch (e) { /* ya estaba muerto */ }
      worker = null;
      raizPedida = null;
      for (const [id, resolver] of pendientes) {
        if (pendientes.delete(id)) resolver({ tipo: "error", error: "comando detenido por el usuario", consola: "" });
      }
      return true;
    },
    // leer con el error nombrado: si el fs falla, la página tiene que decir qué falló y no
    // dejar que un objeto de error se cuele como si fueran bytes
    async bytes(ruta) {
      const r = await this.leer(ruta);
      if (r.tipo === "error") throw new Error(ruta + ": " + r.error);
      return r.bytes ? (r.bytes instanceof Uint8Array ? r.bytes : new Uint8Array(r.bytes)) : null;
    },
    async texto(ruta) {
      const b = await this.bytes(ruta);
      return b ? new TextDecoder().decode(b) : "";
    },
  };
})();

const estadoPestana = { ruta: null, nodo: null };

// Varias pestañas, como un editor de verdad: solo el documento activo vive en el textarea.
// Los limpios se descartan de memoria al cambiar y se releen de OPFS al volver; los sucios
// conservan su texto hasta que se guarden o se cierren.
const DOCUMENTOS = { lista: [], activa: null };
const CONFLICTO = { doc: null, datos: null };
const PERSISTENCIA = "apkc-studio/editor-v1";
// una activación lenta no puede pisar a la pestaña que se pulsó después
let activacion = 0;

// Identidad de esta ventana/pestaña del navegador. sessionStorage sobrevive a recargar y no se
// comparte entre ventanas nuevas; el árbol de deshacer se indexa por ella.
const VENTANA = (() => {
  try {
    let id = sessionStorage.getItem("apkc-studio/ventana");
    if (!id) {
      id = "v" + Date.now().toString(36) + Math.random().toString(36).slice(2, 7);
      sessionStorage.setItem("apkc-studio/ventana", id);
    }
    return id;
  } catch (e) { return "v0"; }
})();

let temporizadorHistorial = null;

// Mientras una edición programática usa execCommand, el evento input que dispara no debe
// volver a pintar: la propia edición pinta una sola vez al terminar.
let silencioInput = 0;

// --- el editor, sin dependencias ---------------------------------------------------------
// Un textarea encima de un <pre> resaltado: es la única forma de tener color sin arrastrar
// un editor de terceros, y el precio es cuidar que las dos capas metricen igual (misma fuente,
// mismo whitespace, mismo scroll).
const EDITOR = {
  inicializar() {
    const area = document.getElementById("codigo");
    if (!area) return;
    area.addEventListener("input", () => {
      if (silencioInput) return;
      EDITOR.pintar();
      EDITOR.sincronizar();
      actualizarCursor();
      marcarSucio(true);
      programarHistorial();
    });
    area.addEventListener("scroll", EDITOR.sincronizar);
    area.addEventListener("blur", volcarHistorial);
    area.addEventListener("keydown", (e) => {
      if (e.isComposing) return;
      if ((e.ctrlKey || e.metaKey) && !e.altKey) {
        const tecla = (e.key || "").toLowerCase();
        if (tecla === "z") {
          e.preventDefault();
          if (e.shiftKey) rehacer(); else deshacer();
          return;
        }
        if (tecla === "y") {
          e.preventDefault();
          rehacer();
          return;
        }
      }
      if (e.key === "Tab" && !e.ctrlKey && !e.altKey && !e.metaKey) {
        e.preventDefault();
        EDITOR.indentar(e.shiftKey ? -1 : 1);
        return;
      }
      if (e.key === "Enter" && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault();
        EDITOR.entrar();
      }
    });
    area.addEventListener("keyup", actualizarCursor);
    area.addEventListener("click", actualizarCursor);
    area.addEventListener("focus", actualizarCursor);
    document.addEventListener("selectionchange", () => {
      if (document.activeElement === area) actualizarCursor();
    });

    // la rueda sobre la columna de números no puede mover el textarea (es otro elemento):
    // se reenvía, y el clic en el margen devuelve el foco al código sin perder la posición
    const marco = document.querySelector(".editor");
    if (marco) {
      marco.addEventListener("wheel", (e) => {
        if (e.target === area || e.ctrlKey || e.metaKey) return;
        const escala = e.deltaMode === 1 ? 16 : (e.deltaMode === 2 ? area.clientHeight : 1);
        area.scrollTop += e.deltaY * escala;
        area.scrollLeft += e.deltaX * escala;
        EDITOR.sincronizar();
        e.preventDefault();
      }, { passive: false });
      marco.addEventListener("mousedown", (e) => {
        if (e.target === area || e.button !== 0) return;
        e.preventDefault();
        area.focus();
      });
      // el panel inferior cambia el alto del editor: lo que estaba visible sigue siéndolo
      if (typeof ResizeObserver === "function") {
        new ResizeObserver(() => EDITOR.sincronizar()).observe(marco);
      }
    }
    EDITOR.pintar();
    actualizarCursor();
  },

  texto() { return document.getElementById("codigo").value; },

  poner(texto) {
    const area = document.getElementById("codigo");
    area.value = texto;
    area.setSelectionRange(0, 0);
    area.scrollTop = 0;
    area.scrollLeft = 0;
    EDITOR.pintar();
    EDITOR.sincronizar();
    actualizarCursor();
    marcarSucio(false);
  },

  pintar() {
    const area = document.getElementById("codigo");
    const resaltado = document.getElementById("resalto");
    if (!area || !resaltado) return;
    const valor = area.value;
    resaltado.innerHTML = resaltar(valor, esXml(valor) ? "xml" : "java") + "\n";
    EDITOR.numeros(valor);
  },

  numeros(valor) {
    const lineas = valor.split("\n").length;
    const ancho = Math.max(3, String(lineas).length);
    const cuadro = document.getElementById("numeros");
    if (!cuadro) return;
    let fuera = "";
    for (let i = 1; i <= lineas; i++) fuera += String(i).padStart(ancho, " ") + "\n";
    cuadro.textContent = fuera;
    cuadro.style.width = "calc(" + ancho + "ch + 18px)";
  },

  // Las dos capas de solo lectura tienen su propio scroll y se les copia el del textarea.
  // Antes se trasladaban con transform: el <pre> recortaba su contenido al alto visible y
  // al trasladarlo hacia arriba no quedaba nada pintado, así que a partir de la primera
  // pantalla el editor se veía vacío y parecía que no hacía scroll (§141).
  sincronizar() {
    const area = document.getElementById("codigo");
    const resaltado = document.getElementById("resalto");
    const numeros = document.getElementById("numeros");
    if (!area) return;
    if (resaltado) {
      if (resaltado.scrollTop !== area.scrollTop) resaltado.scrollTop = area.scrollTop;
      if (resaltado.scrollLeft !== area.scrollLeft) resaltado.scrollLeft = area.scrollLeft;
    }
    if (numeros && numeros.scrollTop !== area.scrollTop) numeros.scrollTop = area.scrollTop;
  },

  // Todo cambio programático pasa por aquí. Se usa execCommand("insertText") porque es la
  // única forma de que Tab, Shift+Tab y Enter entren en el historial nativo de deshacer;
  // setRangeText deja el textarea sin Ctrl+Z. Si el navegador no lo soporta, se cae a
  // setRangeText: el texto queda igual, solo se pierde el deshacer de esa edición.
  cambiar(desde, hasta, texto, seleccion) {
    const area = document.getElementById("codigo");
    let hecho = false;
    // execCommand actúa sobre el elemento con foco: si la barra de búsqueda lo tiene, el texto
    // se perdía sin error y el reemplazo decía «3 reemplazadas» sin cambiar nada.
    if (document.activeElement !== area) area.focus();
    if (typeof document.execCommand === "function") {
      if (area.selectionStart !== desde || area.selectionEnd !== hasta) {
        area.setSelectionRange(desde, hasta);
      }
      silencioInput++;
      try { hecho = document.execCommand("insertText", false, texto); }
      finally { silencioInput--; }
    }
    if (!hecho) area.setRangeText(texto, desde, hasta, "preserve");
    if (seleccion) area.setSelectionRange(seleccion[0], seleccion[1]);
    EDITOR.pintar();
    EDITOR.sincronizar();
    actualizarCursor();
    marcarSucio(true);
    programarHistorial();
  },

  // Tab y Shift+Tab: con selección indenta o desindenta las líneas; sin selección, Tab
  // inserta cuatro espacios en el cursor y Shift+Tab quita la sangría de su línea.
  indentar(delta) {
    const area = document.getElementById("codigo");
    const v = area.value;
    const a = area.selectionStart, b = area.selectionEnd;

    if (a === b) {
      if (delta > 0) {
        EDITOR.cambiar(a, b, "    ", [a + 4, a + 4]);
        return;
      }
      const ini = v.lastIndexOf("\n", a - 1) + 1;
      let finLinea = v.indexOf("\n", a);
      if (finLinea === -1) finLinea = v.length;
      const m = /^( {1,4}|\t)/.exec(v.slice(ini, finLinea));
      if (!m) return;
      const corte = m[0].length;
      const cursor = Math.max(ini, a - corte);
      EDITOR.cambiar(ini, ini + corte, "", [cursor, cursor]);
      return;
    }

    const ini = v.lastIndexOf("\n", a - 1) + 1;
    let fin = v.indexOf("\n", b);
    if (fin === -1) fin = v.length;
    // una selección que termina justo al empezar una línea no incluye esa línea
    if (b > a && v[b - 1] === "\n") fin = b - 1;
    let primera = 0, total = 0;
    const nuevas = v.slice(ini, fin).split("\n").map((linea, i) => {
      let cambio = 0;
      if (delta > 0) cambio = 4;
      else {
        const m = /^( {1,4}|\t)/.exec(linea);
        if (m) cambio = -m[0].length;
      }
      if (i === 0) primera = cambio;
      total += cambio;
      return delta > 0 ? "    " + linea : linea.slice(Math.max(0, -cambio));
    });
    if (!total) return;
    const nuevoA = Math.max(ini, a + primera);
    const nuevoB = Math.max(nuevoA, b + total);
    EDITOR.cambiar(ini, fin, nuevas.join("\n"), [nuevoA, nuevoB]);
  },

  // Enter conserva la sangría de la línea y añade un nivel si se acaba de abrir una llave.
  entrar() {
    const area = document.getElementById("codigo");
    const v = area.value;
    const a = area.selectionStart, b = area.selectionEnd;
    const ini = v.lastIndexOf("\n", a - 1) + 1;
    const linea = v.slice(ini, a);
    const sangria = (/^[ \t]*/.exec(linea) || [""])[0];
    const extra = /\{[ \t]*$/.test(linea) ? "    " : "";
    const insertar = "\n" + sangria + extra;
    EDITOR.cambiar(a, b, insertar, [a + insertar.length, a + insertar.length]);
  },
};

// Ln/Col del cursor, en la barra de estado. Se cuenta sobre el valor real, no sobre el HTML
// resaltado: el resaltado tiene spans y no sirve para medir posiciones.
function actualizarCursor() {
  const area = document.getElementById("codigo");
  const salida = document.getElementById("estado-cursor");
  if (!area || !salida) return;
  const antes = area.value.slice(0, area.selectionStart);
  const lineas = antes.split("\n");
  salida.textContent = "Ln " + lineas.length + ", Col " + (lineas[lineas.length - 1].length + 1);
}

// El resaltado y esXml viven en editor.js.

// --- el árbol y los ficheros --------------------------------------------------------------
// El árbol empieza cerrado: solo se guardan las carpetas abiertas, no las plegadas. Antes el
// conjunto vacío significaba «todo abierto» y un proyecto con src/ y res/ se comía la pantalla.
const ARBOL = { abiertos: new Set() };

// En táctil el arrastre nativo compite con el menú de mantener pulsado; el arrastre se ofrece
// donde hay ratón o lápiz, y en el resto se mueve con «Mover a…».
const ARRASTRABLE = typeof window !== "undefined" && typeof window.matchMedia === "function"
  ? window.matchMedia("(pointer: fine)").matches
  : false;

function relativa(ruta) {
  return ruta === PROYECTO ? "" : (ruta.startsWith(PROYECTO + "/") ? ruta.slice(PROYECTO.length + 1) : ruta);
}

// --- subir ficheros del dispositivo ---------------------------------------------------------
// El botón del explorador y el del menú Archivo usan la carpeta del fichero abierto; el menú de
// una carpeta del árbol permite elegir otra. Los bytes van directos a OPFS: no pasan por ningún
// servidor, igual que el resto del proyecto.
const SUBIDA = { destino: null };

function carpetaDe(ruta) {
  const i = String(ruta || "").lastIndexOf("/");
  return i <= 0 ? "/" : ruta.slice(0, i);
}

function carpetaActual() {
  return estadoPestana.ruta ? carpetaDe(estadoPestana.ruta) : PROYECTO;
}

// webkitRelativePath existe cuando se elige una carpeta: conserva los subdirectorios y descarta
// los tramos que intentarían salir del proyecto.
function nombreDeSubida(archivo) {
  const original = String(archivo.webkitRelativePath || archivo.name || "archivo").replace(/\\/g, "/");
  const partes = original.split("/").filter((p) => p && p !== "." && p !== "..");
  return partes.length ? partes.join("/") : "archivo";
}

function pedirSubida(destino, esCarpeta) {
  SUBIDA.destino = destino || carpetaActual();
  const entrada = document.getElementById(esCarpeta ? "subir-carpeta" : "subir-ficheros");
  if (!entrada) { nota("este navegador no expone el selector de archivos"); return; }
  entrada.value = "";
  entrada.click();
}

async function subirSeleccion(entrada) {
  const archivos = [...(entrada.files || [])];
  if (!archivos.length) return;
  const destino = SUBIDA.destino || carpetaActual();
  const rutas = archivos.map((archivo) => destino + "/" + nombreDeSubida(archivo));
  const choques = [];
  for (const ruta of rutas) {
    const r = await hospedador.leer(ruta);
    if (r && r.bytes) choques.push(ruta);
  }
  const donde = relativa(destino) || "la raíz del proyecto";
  if (choques.length && !window.confirm("Ya existen " + choques.length + " archivo(s) en " +
      donde + ". ¿Reemplazarlos?")) return;

  try {
    await conCarga("Subiendo " + archivos.length + " archivo(s)…", () => conBloqueo(async () => {
      for (let i = 0; i < archivos.length; i++) {
        const datos = new Uint8Array(await archivos[i].arrayBuffer());
        const escrito = await hospedador.escribir(rutas[i], datos);
        if (escrito && escrito.tipo === "error") throw new Error(escrito.error);
        try { localStorage.setItem(claveRevision(rutas[i]), nuevaRevision()); } catch (e) { /* sin revisión */ }
      }
    }));
  } finally {
    SUBIDA.destino = null;
  }
  publicar({ tipo: "mutacion", accion: "subir", rutas });
  await refrescarArbol();
  nota(archivos.length + (archivos.length === 1 ? " archivo subido a " : " archivos subidos a ") + donde);
  // con un solo fichero de texto, dejarlo abierto ahorra el paso de buscarlo en el árbol
  if (rutas.length === 1 && !estaSucio()) {
    if (await abrir(rutas[0])) {
      const vista = document.querySelector('[data-vista="pestana"]');
      if (vista && document.body.dataset.vista !== "pestana") vista.click();
    }
  }
}

// --- exportar e importar el proyecto ---------------------------------------------------------
// El zip conserva rutas y bytes tal cual. Al importar se reemplaza `/proyecto` entero, así que
// la confirmación dice cuántos ficheros entran y si hay cambios sin guardar.
function descargarBytes(nombre, bytes, tipo) {
  const enlace = document.createElement("a");
  const url = URL.createObjectURL(new Blob([bytes], { type: tipo || "application/octet-stream" }));
  enlace.href = url;
  enlace.download = nombre;
  enlace.click();
  setTimeout(() => URL.revokeObjectURL(url), 2000);
}

async function exportarProyecto() {
  return conCarga("Empaquetando el proyecto…", async () => {
    try {
      const arbol = await hospedador.arbol();
      const nodos = (arbol.nodos || []).filter(
        (n) => n.ruta === PROYECTO || n.ruta.startsWith(PROYECTO + "/"));
      const manifiesto = await hospedador.texto(PROYECTO + "/AndroidManifest.xml");
      if (!manifiesto) {
        nota("no hay proyecto que exportar: crea uno en Inicio");
        return;
      }
      const entradas = [];
      for (const n of nodos) {
        if (n.ruta === PROYECTO) continue;
        const rel = n.ruta.slice(PROYECTO.length + 1);
        if (n.dir) {
          entradas.push([rel + "/", new Uint8Array(0)]);
          continue;
        }
        const bytes = await hospedador.bytes(n.ruta);
        if (bytes) entradas.push([rel, bytes]);
      }
      const zip = zipDelArbol(entradas);
      descargarBytes(nombreDeZip(paqueteDelManifiesto(manifiesto)), zip, "application/zip");
      nota("proyecto exportado: " + entradas.filter((e) => !e[0].endsWith("/")).length +
        " ficheros, " + zip.length + " bytes");
    } catch (e) {
      nota("no se pudo exportar: " + (e && e.message ? e.message : e));
    }
  });
}

async function importarProyecto(archivo) {
  if (!archivo) return;
  return conCarga("Leyendo " + archivo.name + "…", async () => {
    try {
      const entradas = await leerZip(new Uint8Array(await archivo.arrayBuffer()));
      const manifiesto = entradas.find((e) => !e.dir && e.ruta === "AndroidManifest.xml");
      if (!manifiesto) throw new Error("el zip no trae AndroidManifest.xml en la raíz");
      const ficheros = entradas.filter((e) => !e.dir);
      const sucios = hayDocumentosSucios();
      if (!window.confirm("Importar «" + archivo.name + "» reemplaza el proyecto actual (" +
          ficheros.length + " ficheros)" + (sucios ? " y hay cambios sin guardar" : "") +
          ". Exporta antes si quieres conservarlo. ¿Seguir?")) return;
      await conBloqueo(async () => {
        await hospedador.limpiar(PROYECTO);
        for (const entrada of entradas) {
          if (entrada.dir) continue;
          const ruta = PROYECTO + "/" + entrada.ruta;
          const r = await hospedador.escribir(ruta, entrada.datos);
          if (r && r.tipo === "error") throw new Error(r.error);
          try { localStorage.setItem(claveRevision(ruta), nuevaRevision()); } catch (e) { /* sin revisión */ }
        }
      });
      limpiarEstadoEditor();
      await refrescarArbol();
      publicar({ tipo: "proyecto", accion: "importar" });
      nota("proyecto importado: " + ficheros.length + " ficheros");
      const primerJava = ficheros.find((e) => /\.java$/.test(e.ruta));
      if (primerJava && await abrir(PROYECTO + "/" + primerJava.ruta)) {
        const vista = document.querySelector('[data-vista="pestana"]');
        if (vista && document.body.dataset.vista !== "pestana") vista.click();
      }
    } catch (e) {
      nota("no se pudo importar: " + (e && e.message ? e.message : e));
    }
  });
}

// --- documentos abiertos -------------------------------------------------------------------
function documentoActivo() {
  return DOCUMENTOS.lista.find((doc) => doc.ruta === DOCUMENTOS.activa) || null;
}

function documentoDe(ruta) {
  return DOCUMENTOS.lista.find((doc) => doc.ruta === ruta) || null;
}

function hayDocumentosSucios() {
  return DOCUMENTOS.lista.some((doc) => doc.sucio) ||
    (!!estadoPestana.sucio && !documentoActivo());
}

function leerPersistencia() {
  try {
    let propio = sessionStorage.getItem(PERSISTENCIA);
    if (!propio) {
      // migración única desde la versión que guardaba la vista en localStorage
      const viejo = localStorage.getItem(PERSISTENCIA);
      if (viejo) {
        sessionStorage.setItem(PERSISTENCIA, viejo);
        localStorage.removeItem(PERSISTENCIA);
        propio = viejo;
      }
    }
    const dato = JSON.parse(propio || "{}");
    return dato && typeof dato === "object" ? dato : {};
  } catch (e) { return {}; }
}

// Estado de esta ventana, no compartido: pestañas, activa y carpetas abiertas. El proyecto y el
// árbol de deshacer viven aparte (OPFS e IndexedDB), así dos ventanas no se pisan la vista.
function guardarPersistencia() {
  try {
    sessionStorage.setItem(PERSISTENCIA, JSON.stringify({
      tabs: DOCUMENTOS.lista.map((doc) => doc.ruta),
      activa: DOCUMENTOS.activa,
      abiertos: [...ARBOL.abiertos],
    }));
  } catch (e) { /* sin persistencia, la página sigue */ }
}

// --- coordinación entre ventanas del mismo origen -----------------------------------------
const REV_PREF = "apkc-studio/rev/";
const canal = typeof window !== "undefined" && typeof BroadcastChannel === "function"
  ? new BroadcastChannel("apkc-studio")
  : null;

async function conBloqueo(tarea) {
  if (typeof navigator !== "undefined" && navigator.locks &&
      typeof navigator.locks.request === "function") {
    return navigator.locks.request("apkc-studio:/proyecto", { mode: "exclusive" }, tarea);
  }
  return tarea();
}

function claveRevision(ruta) {
  return REV_PREF + encodeURIComponent(ruta);
}
function revisionActual(ruta) {
  try { return localStorage.getItem(claveRevision(ruta)); } catch (e) { return null; }
}
function nuevaRevision() {
  return Date.now().toString(36) + "-" + Math.random().toString(36).slice(2, 8);
}
function publicar(mensaje) {
  if (!canal) return;
  try { canal.postMessage({ ...mensaje, origen: VENTANA }); } catch (e) { /* sin canal */ }
}

function ocultarConflicto() {
  CONFLICTO.doc = null;
  CONFLICTO.datos = null;
  const aviso = document.getElementById("conflicto-aviso");
  if (aviso) aviso.hidden = true;
}

function mostrarConflicto(doc, datos) {
  CONFLICTO.doc = doc;
  CONFLICTO.datos = datos;
  const texto = document.getElementById("conflicto-texto");
  const aviso = document.getElementById("conflicto-aviso");
  if (texto) {
    texto.textContent = "Otra ventana cambió " + relativa(doc.ruta) +
      "; las mismas líneas cambiaron en las dos.";
  }
  if (aviso) aviso.hidden = false;
}

function aplicarVersionRemota(doc, datos) {
  doc.texto = datos.suyo;
  doc.base = datos.suyo;
  doc.revision = datos.revision;
  doc.sucio = false;
  doc.conflicto = null;
  if (DOCUMENTOS.activa === doc.ruta) {
    EDITOR.poner(datos.suyo);
    marcarSucio(false);
    actualizarAccionesFichero();
  }
  ocultarConflicto();
  renderPestanas();
  renderHistorial();
  guardarPersistencia();
  nota("usada la versión de la otra ventana en " + relativa(doc.ruta));
}

function mantenerVersionPropia(doc) {
  const datos = doc && doc.conflicto ? doc.conflicto : CONFLICTO.datos;
  if (!doc || !datos) return;
  doc.base = datos.suyo;
  doc.revision = datos.revision;
  doc.sucio = true;
  doc.conflicto = null;
  ocultarConflicto();
  marcarSucio(true);
  renderPestanas();
  nota("mantienes tu versión; guarda para sobrescribir");
}

function insertarMarcasConflicto(doc) {
  const datos = doc && doc.conflicto ? doc.conflicto : CONFLICTO.datos;
  if (!doc || !datos) return;
  const nuestro = DOCUMENTOS.activa === doc.ruta ? EDITOR.texto() : datos.nuestro;
  const marcas = "<<<<<<< esta ventana\n" + nuestro +
    (nuestro.endsWith("\n") ? "" : "\n") + "=======\n" + datos.suyo +
    (datos.suyo.endsWith("\n") ? "" : "\n") + ">>>>>>> otra ventana\n";
  doc.texto = marcas;
  doc.base = datos.suyo;
  doc.revision = datos.revision;
  doc.sucio = true;
  doc.conflicto = null;
  if (DOCUMENTOS.activa === doc.ruta) {
    EDITOR.poner(marcas);
    marcarSucio(true);
  }
  ocultarConflicto();
  renderHistorial();
  nota("marcas de conflicto insertadas; edita y guarda");
}

async function recargarDocumentoRemoto(doc, revision) {
  try {
    await hospedador.refrescar(doc.ruta);
    const bytes = await hospedador.bytes(doc.ruta);
    if (!bytes) return;
    const suyo = new TextDecoder().decode(bytes);
    const rev = revision || revisionActual(doc.ruta);
    const activo = DOCUMENTOS.activa === doc.ruta;
    const nuestro = activo ? EDITOR.texto() : doc.texto;

    if (!doc.sucio || nuestro === suyo) {
      doc.texto = activo ? suyo : null;
      doc.base = suyo;
      doc.revision = rev;
      doc.sucio = false;
      doc.conflicto = null;
      if (activo) {
        EDITOR.poner(suyo);
        marcarSucio(false);
        await asegurarHistorial(doc, suyo);
        renderHistorial();
        actualizarAccionesFichero();
      }
      nota("otra ventana guardó " + relativa(doc.ruta) + "; recargado");
      return;
    }

    const base = doc.base === undefined || doc.base === null ? null : doc.base;
    if (base === null) {
      doc.conflicto = { base: suyo, suyo, nuestro, revision: rev };
      if (activo) mostrarConflicto(doc, doc.conflicto);
      else nota("otra ventana guardó " + relativa(doc.ruta) + "; hay conflicto");
      return;
    }

    const fusion = fusionar(base, nuestro, suyo);
    doc.revision = rev;
    if (fusion.estado === "suyo" || fusion.estado === "igual") {
      aplicarVersionRemota(doc, { suyo, revision: rev });
      return;
    }
    if (fusion.estado === "nuestro") {
      doc.base = suyo;
      nota("otra ventana guardó " + relativa(doc.ruta) + " sin cambios respecto a la base");
      return;
    }
    if (fusion.estado === "auto") {
      doc.texto = fusion.texto;
      doc.base = suyo;
      doc.sucio = true;
      doc.conflicto = null;
      if (activo) {
        EDITOR.poner(fusion.texto);
        marcarSucio(true);
        await asegurarHistorial(doc, fusion.texto);
        renderHistorial();
        actualizarAccionesFichero();
      }
      nota("cambios fusionados automáticamente en " + relativa(doc.ruta) + "; guarda para publicarlos");
      return;
    }

    doc.conflicto = { base, suyo, nuestro, revision: rev };
    if (activo) mostrarConflicto(doc, doc.conflicto);
    else nota("otra ventana guardó " + relativa(doc.ruta) + "; hay conflicto");
  } catch (e) {
    nota("no se pudo recargar " + relativa(doc.ruta) + ": " + (e && e.message ? e.message : e));
  }
}

async function aplicarBorradoRemoto(ruta) {
  const conservar = [];
  for (const doc of DOCUMENTOS.lista) {
    if (!(doc.ruta === ruta || doc.ruta.startsWith(ruta + "/"))) {
      conservar.push(doc);
      continue;
    }
    if (doc.sucio) {
      doc.remotoEliminado = true;
      conservar.push(doc);
    } else {
      Historial.borrar(VENTANA, doc.ruta).catch(() => {});
    }
  }
  DOCUMENTOS.lista = conservar;
  if (!DOCUMENTOS.lista.some((doc) => doc.ruta === DOCUMENTOS.activa)) {
    DOCUMENTOS.activa = null;
    estadoPestana.ruta = null;
    const vecino = DOCUMENTOS.lista[0];
    if (vecino) await activarDocumento(vecino.ruta);
    else limpiarSinDocumento();
  } else {
    renderPestanas();
    renderHistorial();
    actualizarAccionesFichero();
    guardarPersistencia();
  }
  nota("otra ventana borró " + relativa(ruta));
}

async function recibirRemoto(msg) {
  if (!msg || msg.origen === VENTANA) return;
  if (msg.tipo === "guardado") {
    const doc = documentoDe(msg.ruta);
    if (doc) await recargarDocumentoRemoto(doc, msg.revision);
    await hospedador.refrescar(PROYECTO).catch(() => {});
    await refrescarArbol();
    return;
  }
  if (msg.tipo === "mutacion") {
    if (msg.accion === "renombrar" || msg.accion === "mover") {
      await hospedador.refrescar(msg.origen).catch(() => {});
      await hospedador.refrescar(msg.destino).catch(() => {});
      remapearDocumentos(msg.origen, msg.destino);
      remapearAbiertos(msg.origen, msg.destino);
      revelarRuta(estadoPestana.ruta);
    } else if (msg.accion === "borrar") {
      await aplicarBorradoRemoto(msg.ruta);
    } else if (msg.accion === "subir" || msg.accion === "crear") {
      for (const ruta of msg.rutas || []) {
        await hospedador.refrescar(ruta).catch(() => {});
        const doc = documentoDe(ruta);
        if (!doc) continue;
        await recargarDocumentoRemoto(doc, revisionActual(ruta));
      }
    }
    await hospedador.refrescar(PROYECTO).catch(() => {});
    await refrescarArbol();
    return;
  }
  if (msg.tipo === "proyecto") {
    await hospedador.refrescar(PROYECTO).catch(() => {});
    for (const doc of DOCUMENTOS.lista) {
      if (doc.sucio) doc.remotoEliminado = true;
      else doc.texto = null;
    }
    await refrescarArbol();
    nota("otra ventana creó un proyecto; el árbol se ha actualizado");
  }
}

function falloDocumento(e) {
  nota("fallo: " + (e && e.message ? e.message : e));
  console.error(e);
}

if (canal) {
  canal.addEventListener("message", (evento) => {
    if (evento.data && evento.data.origen !== VENTANA) {
      recibirRemoto(evento.data).catch(falloDocumento);
    }
  });
}

function renderPestanas() {
  const caja = document.getElementById("pestanas-archivo");
  if (!caja) return;
  caja.textContent = "";
  if (!DOCUMENTOS.lista.length) {
    const tab = document.createElement("div");
    tab.className = "editor-tab activa";
    const icono = document.createElement("span");
    icono.className = "editor-tab-icono";
    icono.setAttribute("aria-hidden", "true");
    const nombre = document.createElement("span");
    nombre.id = "fichero-abierto";
    nombre.textContent = "Sin fichero";
    const marca = document.createElement("span");
    marca.id = "marca-sucio";
    marca.className = "marca-sucio";
    marca.hidden = true;
    marca.setAttribute("aria-label", "cambios sin guardar");
    marca.textContent = "●";
    tab.append(icono, nombre, marca);
    caja.appendChild(tab);
    return;
  }
  for (const doc of DOCUMENTOS.lista) {
    const activa = doc.ruta === DOCUMENTOS.activa;
    const tab = document.createElement("div");
    tab.className = "editor-tab" + (activa ? " activa" : "");
    tab.dataset.ruta = doc.ruta;
    tab.title = doc.ruta;
    tab.setAttribute("role", "tab");
    tab.setAttribute("aria-selected", String(activa));

    const icono = document.createElement("span");
    icono.className = "editor-tab-icono";
    icono.setAttribute("aria-hidden", "true");
    const nombre = document.createElement("span");
    if (activa) nombre.id = "fichero-abierto";
    else nombre.className = "editor-tab-nombre";
    nombre.textContent = relativa(doc.ruta);
    const marca = document.createElement("span");
    marca.className = "marca-sucio";
    if (activa) marca.id = "marca-sucio";
    marca.hidden = !doc.sucio;
    marca.setAttribute("aria-label", "cambios sin guardar");
    marca.textContent = "●";
    const cerrar = document.createElement("button");
    cerrar.type = "button";
    cerrar.className = "editor-tab-cerrar";
    cerrar.title = "Cerrar " + relativa(doc.ruta);
    cerrar.setAttribute("aria-label", cerrar.title);
    cerrar.textContent = "×";
    cerrar.addEventListener("click", (e) => {
      e.stopPropagation();
      cerrarDocumento(doc.ruta).catch(falloDocumento);
    });

    tab.append(icono, nombre, marca, cerrar);
    tab.addEventListener("click", () => { abrir(doc.ruta).catch(falloDocumento); });
    tab.addEventListener("auxclick", (e) => {
      if (e.button !== 1) return;
      e.preventDefault();
      cerrarDocumento(doc.ruta).catch(falloDocumento);
    });
    caja.appendChild(tab);
  }
  const activa = caja.querySelector(".editor-tab.activa");
  if (activa && typeof activa.scrollIntoView === "function") {
    activa.scrollIntoView({ block: "nearest", inline: "nearest" });
  }
}

function pintarMarcaSucio() {
  const doc = documentoActivo();
  const marca = document.getElementById("marca-sucio");
  if (marca) marca.hidden = !(doc && doc.sucio);
}

function guardarEnMemoria() {
  const doc = documentoActivo();
  if (!doc) return;
  volcarHistorial();
  const area = document.getElementById("codigo");
  doc.texto = EDITOR.texto();
  if (area) {
    doc.cursor = [area.selectionStart, area.selectionEnd];
    doc.scroll = [area.scrollTop, area.scrollLeft];
  }
  // un documento limpio se puede releer de OPFS: no hace falta retener su texto
  if (!doc.sucio) doc.texto = null;
}

// --- historial de deshacer ---------------------------------------------------------------
function historialDe(doc) {
  return doc && doc.hist ? doc.hist : null;
}

async function asegurarHistorial(doc, texto) {
  if (!doc) return null;
  if (!doc.hist) {
    let arbol = await Historial.cargar(VENTANA, doc.ruta);
    if (!arbol) {
      arbol = Historial.crear(doc.ruta, texto);
    } else {
      arbol.cache = Historial.textoDe(arbol, arbol.actual);
      // el fichero cambió en otra ventana o en OPFS desde la última sesión
      if (arbol.cache !== texto) {
        doc.recuperable = arbol.actual;
        Historial.agregar(arbol, texto, { autor: "remoto" });
      }
    }
    doc.hist = arbol;
  } else {
    doc.hist.ruta = doc.ruta;
    const actual = Historial.textoDe(doc.hist, doc.hist.actual);
    if (actual !== texto) Historial.agregar(doc.hist, texto, { autor: "remoto" });
  }
  return doc.hist;
}

function volcarHistorial() {
  if (temporizadorHistorial) {
    clearTimeout(temporizadorHistorial);
    temporizadorHistorial = null;
  }
  const doc = documentoActivo();
  const arbol = historialDe(doc);
  if (!doc || !arbol) return;
  const area = document.getElementById("codigo");
  const id = Historial.agregar(arbol, EDITOR.texto(), {
    seleccion: area ? [area.selectionStart, area.selectionEnd] : [0, 0],
    scroll: area ? [area.scrollTop, area.scrollLeft] : [0, 0],
  });
  if (id) {
    Historial.guardar(VENTANA, arbol).catch(() => {});
    renderHistorial();
  }
}

function programarHistorial() {
  if (temporizadorHistorial) clearTimeout(temporizadorHistorial);
  temporizadorHistorial = setTimeout(volcarHistorial, 500);
}

function aplicarNodoHistorial(nodo) {
  const doc = documentoActivo();
  if (!doc || !doc.hist || !nodo) return;
  const texto = Historial.textoDe(doc.hist, nodo.id);
  EDITOR.poner(texto);
  const area = document.getElementById("codigo");
  if (area && nodo.a) area.setSelectionRange(nodo.a[0], nodo.a[1]);
  if (area && nodo.s) {
    area.scrollTop = nodo.s[0];
    area.scrollLeft = nodo.s[1];
    EDITOR.sincronizar();
  }
  marcarSucio(true);
  doc.texto = texto;
  renderHistorial();
  Historial.guardar(VENTANA, doc.hist).catch(() => {});
}

function deshacer() {
  volcarHistorial();
  const doc = documentoActivo();
  if (!doc || !doc.hist) return false;
  const nodo = Historial.deshacer(doc.hist);
  if (!nodo) return false;
  aplicarNodoHistorial(nodo);
  return true;
}

function rehacer() {
  volcarHistorial();
  const doc = documentoActivo();
  if (!doc || !doc.hist) return false;
  const nodo = Historial.rehacer(doc.hist);
  if (!nodo) return false;
  aplicarNodoHistorial(nodo);
  return true;
}

function restaurarNodoHistorial(id) {
  volcarHistorial();
  const doc = documentoActivo();
  if (!doc || !doc.hist) return;
  const nodo = Historial.restaurar(doc.hist, id);
  if (nodo) aplicarNodoHistorial(nodo);
}

function limpiarHistorial() {
  const doc = documentoActivo();
  if (!doc || !doc.hist) return;
  if (!window.confirm("¿Borrar el historial de " + relativa(doc.ruta) + "?")) return;
  doc.hist = Historial.crear(doc.ruta, EDITOR.texto());
  Historial.guardar(VENTANA, doc.hist).catch(() => {});
  renderHistorial();
}

function renderHistorial() {
  const caja = document.getElementById("arbol-historial");
  const resumen = document.getElementById("historial-resumen");
  const botonDeshacer = document.getElementById("historial-deshacer");
  const botonRehacer = document.getElementById("historial-rehacer");
  if (!caja || !resumen) return;
  caja.textContent = "";
  const doc = documentoActivo();
  const arbol = historialDe(doc);
  if (!doc || !arbol) {
    resumen.textContent = "Sin historial";
    if (botonDeshacer) botonDeshacer.disabled = true;
    if (botonRehacer) botonRehacer.disabled = true;
    return;
  }
  const nodos = Historial.recorrer(arbol);
  const actual = arbol.nodos[arbol.actual];
  resumen.textContent = nodos.length + " versiones · " + relativa(doc.ruta);
  if (botonDeshacer) botonDeshacer.disabled = !actual || !actual.p;
  if (botonRehacer) botonRehacer.disabled = !actual || !actual.h.length;
  for (const nodo of nodos) {
    const fila = document.createElement("button");
    fila.type = "button";
    fila.className = "historial-nodo" + (nodo.id === arbol.actual ? " actual" : "");
    fila.style.paddingLeft = (8 + nodo.nivel * 16) + "px";
    fila.dataset.id = nodo.id;
    fila.setAttribute("role", "treeitem");
    fila.setAttribute("aria-level", String(nodo.nivel + 1));
    if (nodo.id === arbol.actual) fila.setAttribute("aria-current", "true");

    const hora = document.createElement("span");
    hora.className = "historial-hora";
    const fecha = new Date(nodo.ts);
    hora.textContent = fecha.toLocaleTimeString("es", { hour: "2-digit", minute: "2-digit", second: "2-digit" });
    const autor = document.createElement("span");
    autor.className = "historial-autor";
    autor.textContent = nodo.au === "remoto" ? "remoto" : "local";
    const texto = document.createElement("span");
    texto.className = "historial-resumen";
    const delta = nodo.d ? " · +" + nodo.d.s.length + "/-" + nodo.d.b : (nodo.p ? " · checkpoint" : "");
    texto.textContent = (nodo.nivel ? "· " : "● ") + (nodo.p ? delta.replace(/^ · /, "") : "inicio");
    fila.append(hora, autor, texto);
    fila.addEventListener("click", () => restaurarNodoHistorial(nodo.id));
    caja.appendChild(fila);
  }
}

function limpiarSinDocumento() {
  DOCUMENTOS.activa = null;
  estadoPestana.ruta = null;
  estadoPestana.sucio = false;
  EDITOR.poner("");
  renderPestanas();
  renderHistorial();
  actualizarAccionesFichero();
  guardarPersistencia();
}

function quitarDocumento(doc) {
  const i = DOCUMENTOS.lista.indexOf(doc);
  if (i >= 0) DOCUMENTOS.lista.splice(i, 1);
  if (DOCUMENTOS.activa !== doc.ruta) {
    renderPestanas();
    guardarPersistencia();
    return false;
  }
  DOCUMENTOS.activa = null;
  estadoPestana.ruta = null;
  renderPestanas();
  const vecino = DOCUMENTOS.lista[Math.min(i, DOCUMENTOS.lista.length - 1)];
  if (vecino) return activarDocumento(vecino.ruta);
  limpiarSinDocumento();
  return false;
}

async function activarDocumento(ruta) {
  const doc = documentoDe(ruta);
  if (!doc) return false;
  if (DOCUMENTOS.activa === ruta) { renderPestanas(); return true; }
  const turno = ++activacion;
  guardarEnMemoria();
  DOCUMENTOS.activa = ruta;
  estadoPestana.ruta = ruta;
  revelarRuta(ruta);

  if (doc.texto === null) {
    // una pestaña limpia inactiva puede tener una versión vieja en el espejo del worker; solo
    // se invalida si otra ventana publicó una revisión distinta (no en el flujo local de subida)
    const revisionDisco = revisionActual(ruta);
    if (doc.revision && revisionDisco && doc.revision !== revisionDisco) {
      await hospedador.refrescar(ruta).catch(() => {});
    }
    let bytes;
    try {
      bytes = await conCarga("Abriendo " + relativa(ruta) + "…", () => hospedador.bytes(ruta));
    } catch (e) {
      if (turno !== activacion) return false;
      nota("no se pudo abrir " + ruta + ": " + (e && e.message ? e.message : e));
      return quitarDocumento(doc);
    }
    if (turno !== activacion) return false;
    if (!bytes) {
      nota("no existe " + ruta);
      return quitarDocumento(doc);
    }
    if (bytes.includes(0)) {
      nota(ruta + " parece un binario; el editor solo abre texto");
      return quitarDocumento(doc);
    }
    doc.texto = new TextDecoder().decode(bytes);
    doc.base = doc.texto;
    doc.revision = revisionActual(ruta) || doc.revision || null;
  }

  await asegurarHistorial(doc, doc.texto);
  // EDITOR.poner marca limpio; el estado sucio de la pestaña hay que restaurarlo después
  const sucio = doc.sucio;
  EDITOR.poner(doc.texto);
  const area = document.getElementById("codigo");
  if (area && doc.cursor) area.setSelectionRange(doc.cursor[0], doc.cursor[1]);
  if (area && doc.scroll) {
    area.scrollTop = doc.scroll[0];
    area.scrollLeft = doc.scroll[1];
    EDITOR.sincronizar();
  }
  marcarSucio(sucio);
  renderPestanas();
  renderHistorial();
  if (doc.conflicto) mostrarConflicto(doc, doc.conflicto);
  else ocultarConflicto();
  if (doc.remotoEliminado) {
    nota("este fichero fue borrado en otra ventana; guardarlo lo recrea");
  }
  if (doc.recuperable) {
    nota("hay cambios sin guardar de la sesión anterior; ábrelos en Historial");
    doc.recuperable = null;
  }
  actualizarAccionesFichero();
  await refrescarArbol();
  guardarPersistencia();
  return true;
}

async function cerrarDocumento(ruta) {
  const doc = documentoDe(ruta);
  if (!doc) return;
  if (doc.ruta === DOCUMENTOS.activa) guardarEnMemoria();
  if (doc.sucio && !confirmarDescartes()) return;
  const i = DOCUMENTOS.lista.indexOf(doc);
  DOCUMENTOS.lista.splice(i, 1);
  if (DOCUMENTOS.activa === ruta) {
    DOCUMENTOS.activa = null;
    estadoPestana.ruta = null;
    const vecino = DOCUMENTOS.lista[Math.min(i, DOCUMENTOS.lista.length - 1)];
    if (vecino) await activarDocumento(vecino.ruta);
    else limpiarSinDocumento();
  } else {
    renderPestanas();
    guardarPersistencia();
  }
}

function remapearDocumentos(origen, destino) {
  for (const doc of DOCUMENTOS.lista) {
    const viejo = doc.ruta;
    if (doc.ruta === origen) doc.ruta = destino;
    else if (doc.ruta.startsWith(origen + "/")) doc.ruta = destino + doc.ruta.slice(origen.length);
    if (doc.ruta !== viejo && doc.hist) doc.hist.ruta = doc.ruta;
  }
  if (DOCUMENTOS.activa === origen) DOCUMENTOS.activa = destino;
  else if (DOCUMENTOS.activa && DOCUMENTOS.activa.startsWith(origen + "/")) {
    DOCUMENTOS.activa = destino + DOCUMENTOS.activa.slice(origen.length);
  }
  estadoPestana.ruta = DOCUMENTOS.activa;
  const historiasAbiertas = DOCUMENTOS.lista.filter((doc) => doc.hist).map((doc) => doc.hist);
  Historial.renombrarBajo(VENTANA, origen, destino)
    .then(() => Promise.all(historiasAbiertas.map((hist) => Historial.guardar(VENTANA, hist))))
    .catch(() => {});
  renderPestanas();
  renderHistorial();
  actualizarAccionesFichero();
  guardarPersistencia();
}

async function quitarDocumentosBajo(ruta) {
  const indiceActiva = DOCUMENTOS.lista.findIndex((doc) => doc.ruta === DOCUMENTOS.activa);
  for (const doc of DOCUMENTOS.lista) {
    if (doc.ruta === ruta || doc.ruta.startsWith(ruta + "/")) {
      Historial.borrar(VENTANA, doc.ruta).catch(() => {});
    }
  }
  DOCUMENTOS.lista = DOCUMENTOS.lista.filter((doc) =>
    !(doc.ruta === ruta || doc.ruta.startsWith(ruta + "/")));
  if (!DOCUMENTOS.lista.some((doc) => doc.ruta === DOCUMENTOS.activa)) {
    DOCUMENTOS.activa = null;
    estadoPestana.ruta = null;
    const vecino = DOCUMENTOS.lista[Math.min(indiceActiva, DOCUMENTOS.lista.length - 1)];
    if (vecino) await activarDocumento(vecino.ruta);
    else limpiarSinDocumento();
  } else {
    renderPestanas();
    guardarPersistencia();
  }
}

// crear un proyecto borra /proyecto: las pestañas viejas no pueden quedar apuntando a la nada
function limpiarEstadoEditor() {
  Historial.limpiar(VENTANA).catch(() => {});
  DOCUMENTOS.lista = [];
  DOCUMENTOS.activa = null;
  estadoPestana.ruta = null;
  estadoPestana.sucio = false;
  ARBOL.abiertos.clear();
  EDITOR.poner("");
  renderPestanas();
  renderHistorial();
  actualizarAccionesFichero();
  guardarPersistencia();
}

function cerrarMenuArchivo() {
  const menu = document.getElementById("menu-archivo");
  if (menu) menu.hidden = true;
}

function abrirMenuArchivo(x, y, nodo) {
  const menu = document.getElementById("menu-archivo");
  if (!menu) return;
  menu.textContent = "";
  const añadir = (texto, accion, peligro) => {
    const b = document.createElement("button");
    b.type = "button";
    b.setAttribute("role", "menuitem");
    b.textContent = texto;
    if (peligro) b.className = "peligro";
    b.onclick = () => { cerrarMenuArchivo(); accion(); };
    menu.appendChild(b);
  };
  if (!nodo.dir) {
    añadir("Abrir", async () => {
      if (await abrir(nodo.ruta)) {
        const vista = document.querySelector('[data-vista="pestana"]');
        if (vista && document.body.dataset.vista !== "pestana") vista.click();
      }
    });
    añadir("Descargar", () => descargarRuta(nodo.ruta));
  } else {
    añadir("Nuevo fichero aquí", () => nuevoFichero(nodo.ruta));
    añadir("Subir archivos aquí", () => pedirSubida(nodo.ruta, false));
    añadir("Subir carpeta aquí", () => pedirSubida(nodo.ruta, true));
  }
  añadir("Mover a…", async () => {
    const carpeta = await pedirCarpetaDestino();
    if (carpeta) await moverRuta(nodo.ruta, carpeta);
  });
  añadir("Renombrar", () => renombrarRuta(nodo.ruta));
  añadir("Borrar", () => borrarRuta(nodo.ruta), true);
  menu.hidden = false;
  menu.style.left = "0px";
  menu.style.top = "0px";
  const caja = menu.getBoundingClientRect();
  menu.style.left = Math.max(6, Math.min(x, window.innerWidth - caja.width - 6)) + "px";
  menu.style.top = Math.max(6, Math.min(y, window.innerHeight - caja.height - 6)) + "px";
}

function conectarMenu(fila, nodo) {
  fila.addEventListener("contextmenu", (e) => {
    e.preventDefault();
    abrirMenuArchivo(e.clientX, e.clientY, nodo);
  });
  // Long press en táctil: el contexto nativo de Android no es fiable y aquí queremos un menú
  // propio con renombrar, borrar y descargar.
  let temporizador = null, inicio = null;
  const cancelar = () => {
    if (temporizador) clearTimeout(temporizador);
    temporizador = null;
    inicio = null;
  };
  fila.addEventListener("pointerdown", (e) => {
    if (e.pointerType !== "touch") return;
    inicio = { x: e.clientX, y: e.clientY };
    temporizador = setTimeout(() => {
      temporizador = null;
      fila.dataset.menuLargo = "1";
      abrirMenuArchivo(inicio.x, inicio.y, nodo);
    }, 550);
  });
  fila.addEventListener("pointermove", (e) => {
    if (!inicio) return;
    if (Math.abs(e.clientX - inicio.x) > 8 || Math.abs(e.clientY - inicio.y) > 8) cancelar();
  });
  fila.addEventListener("pointerup", cancelar);
  fila.addEventListener("pointercancel", cancelar);

  if (ARRASTRABLE) {
    fila.draggable = true;
    fila.addEventListener("dragstart", (e) => {
      if (!e.dataTransfer) return;
      e.dataTransfer.setData("text/plain", nodo.ruta);
      e.dataTransfer.effectAllowed = "move";
      fila.classList.add("arrastrando");
    });
    fila.addEventListener("dragend", () => fila.classList.remove("arrastrando"));
    if (nodo.dir) {
      fila.addEventListener("dragover", (e) => {
        if (!Array.from(e.dataTransfer.types || []).includes("text/plain")) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
        fila.classList.add("soltar");
      });
      fila.addEventListener("dragleave", () => fila.classList.remove("soltar"));
      fila.addEventListener("drop", (e) => {
        e.preventDefault();
        fila.classList.remove("soltar");
        const origen = e.dataTransfer.getData("text/plain");
        if (origen) moverRuta(origen, nodo.ruta);
      });
    }
  }
}

async function renombrarRuta(ruta) {
  const nombre = ruta.split("/").pop();
  const nuevo = prompt("Nuevo nombre de " + nombre, nombre);
  if (!nuevo || nuevo === nombre) return;
  if (nuevo === "." || nuevo === ".." || nuevo.includes("/")) {
    nota("el nombre no puede llevar / ni ser . o ..");
    return;
  }
  const base = ruta.slice(0, ruta.lastIndexOf("/"));
  const destino = base + "/" + nuevo;
  const r = await conBloqueo(() => hospedador.renombrar(ruta, destino));
  if (r.tipo === "error") { nota("no se pudo renombrar: " + r.error); return; }
  publicar({ tipo: "mutacion", accion: "renombrar", origen: ruta, destino: destino });
  remapearDocumentos(ruta, destino);
  remapearAbiertos(ruta, destino);
  revelarRuta(estadoPestana.ruta);
  actualizarAccionesFichero();
  await refrescarArbol();
  nota("renombrado a " + relativa(destino));
}

async function borrarRuta(ruta) {
  if (!window.confirm("¿Borrar " + relativa(ruta) + "?")) return;
  const r = await conBloqueo(() => hospedador.limpiar(ruta));
  if (r.tipo === "error") { nota("no se pudo borrar: " + r.error); return; }
  publicar({ tipo: "mutacion", accion: "borrar", ruta: ruta });
  for (const abierta of [...ARBOL.abiertos]) {
    if (abierta === ruta || abierta.startsWith(ruta + "/")) ARBOL.abiertos.delete(abierta);
  }
  await quitarDocumentosBajo(ruta);
  await refrescarArbol();
  nota("borrado " + relativa(ruta));
}

// --- plegar, desplegar y mover ------------------------------------------------------------
function revelarRuta(ruta) {
  if (!ruta || !ruta.startsWith(PROYECTO + "/")) return;
  const partes = relativa(ruta).split("/");
  partes.pop();
  let actual = PROYECTO;
  for (const parte of partes) {
    actual += "/" + parte;
    ARBOL.abiertos.add(actual);
  }
}

function remapearAbiertos(origen, destino) {
  for (const ruta of [...ARBOL.abiertos]) {
    if (ruta === origen || ruta.startsWith(origen + "/")) {
      ARBOL.abiertos.delete(ruta);
      ARBOL.abiertos.add(destino + ruta.slice(origen.length));
    }
  }
}

function plegarTodo() {
  ARBOL.abiertos.clear();
  guardarPersistencia();
  refrescarArbol();
}

async function desplegarTodo() {
  let r;
  try { r = await hospedador.arbol(); } catch (e) { r = { nodos: [] }; }
  for (const nodo of r.nodos || []) {
    if (nodo.dir && nodo.ruta.startsWith(PROYECTO + "/")) ARBOL.abiertos.add(nodo.ruta);
  }
  guardarPersistencia();
  await refrescarArbol();
}

async function pedirCarpetaDestino() {
  const entrada = prompt("Carpeta destino dentro del proyecto", PROYECTO);
  if (entrada === null) return null;
  let carpeta = entrada.trim().replace(/\\/g, "/").replace(/\/{2,}/g, "/");
  if (!carpeta.startsWith("/")) carpeta = PROYECTO + "/" + carpeta;
  carpeta = normalizarRuta(carpeta);
  if (carpeta !== PROYECTO && !carpeta.startsWith(PROYECTO + "/")) {
    nota("la carpeta tiene que quedar dentro de /proyecto");
    return null;
  }
  const arbol = await hospedador.arbol();
  if (!(arbol.nodos || []).some((n) => n.dir && n.ruta === carpeta)) {
    nota("no existe la carpeta " + carpeta);
    return null;
  }
  return carpeta;
}

async function moverRuta(origen, carpeta) {
  if (!origen || !carpeta) return;
  carpeta = normalizarRuta(carpeta);
  // el arrastre no puede sacar nada del árbol que se ve
  if (!origen.startsWith(PROYECTO + "/")) return;
  if (carpeta !== PROYECTO && !carpeta.startsWith(PROYECTO + "/")) return;
  const destino = carpeta + "/" + origen.split("/").pop();
  if (destino === origen) return;
  if (destino.startsWith(origen + "/")) {
    nota("no se puede mover una carpeta dentro de sí misma");
    return;
  }
  const r = await conBloqueo(() => hospedador.renombrar(origen, destino));
  if (r.tipo === "error") { nota("no se pudo mover: " + r.error); return; }
  publicar({ tipo: "mutacion", accion: "mover", origen: origen, destino: destino });
  remapearDocumentos(origen, destino);
  remapearAbiertos(origen, destino);
  revelarRuta(estadoPestana.ruta);
  actualizarAccionesFichero();
  await refrescarArbol();
  nota("movido a " + relativa(destino));
}

async function refrescarArbol() {
  const fuera = document.getElementById("arbol-pestana");
  if (!fuera) return;
  let r;
  try {
    r = await hospedador.arbol();
  } catch (e) {
    r = { nodos: [] };
  }
  const nodos = (r.nodos || []).filter((n) => n.ruta.startsWith(PROYECTO + "/"));
  const entradaFiltro = document.getElementById("filtro-arbol");
  const filtro = entradaFiltro ? entradaFiltro.value.trim().toLowerCase() : "";
  let visibles = null;
  if (filtro) {
    // se queda lo que coincide y la cadena de padres, para que el árbol siga teniendo forma
    visibles = new Set();
    for (const n of nodos) {
      if (!relativa(n.ruta).toLowerCase().includes(filtro)) continue;
      visibles.add(n.ruta);
      let padre = n.ruta;
      while (padre.includes("/")) {
        padre = padre.slice(0, padre.lastIndexOf("/"));
        if (padre === PROYECTO || padre.length <= PROYECTO.length) break;
        visibles.add(padre);
      }
    }
  }
  const mostrados = filtro ? nodos.filter((n) => visibles.has(n.ruta)) : nodos;
  const scroll = fuera.scrollTop;
  fuera.textContent = "";
  if (!mostrados.length && filtro) {
    const vacio = document.createElement("div");
    vacio.className = "vacio";
    vacio.textContent = "Ningún fichero coincide con «" + entradaFiltro.value.trim() + "».";
    fuera.appendChild(vacio);
    return;
  }
  if (!nodos.length) {
    const vacio = document.createElement("div");
    vacio.className = "vacio";
    vacio.append("El proyecto está vacío. ");
    const ir = document.createElement("a");
    ir.href = "#crear";
    ir.textContent = "Crea uno en Inicio";
    ir.onclick = (e) => {
      e.preventDefault();
      const boton = document.querySelector('[data-vista="crear"]');
      if (boton) boton.click();
    };
    vacio.appendChild(ir);
    vacio.append(".");
    fuera.appendChild(vacio);
    fuera.scrollTop = 0;
    return;
  }

  // agrupa por carpeta: el explorador tiene que leerse como un árbol, no como una lista de rutas
  const hijos = new Map();
  for (const n of mostrados) {
    const partes = relativa(n.ruta).split("/");
    const padre = partes.length > 1 ? PROYECTO + "/" + partes.slice(0, -1).join("/") : PROYECTO;
    if (!hijos.has(padre)) hijos.set(padre, []);
    hijos.get(padre).push({ ...n, nombre: partes[partes.length - 1] });
  }
  for (const lista of hijos.values()) {
    lista.sort((x, y) => (Number(y.dir) - Number(x.dir)) || x.nombre.localeCompare(y.nombre));
  }

  const pintar = (padre, profundidad) => {
    for (const n of hijos.get(padre) || []) {
      const rel = relativa(n.ruta);
      const generado = rel === "build" || rel.startsWith("build/");
      const abierta = n.dir && (filtro ? true : ARBOL.abiertos.has(n.ruta));
      const fila = document.createElement("a");
      fila.href = "#";
      fila.dataset.ruta = n.ruta;
      fila.title = rel + (generado ? " · generado por apkc; se regenera en cada compilación" : "");
      fila.style.paddingLeft = (8 + profundidad * 12) + "px";
      fila.className = (n.dir ? "dir" : "archivo") + (generado ? " generado" : "");
      if (!n.dir) {
        const ext = (n.nombre.match(/\.([A-Za-z0-9]+)$/) || [])[1];
        if (ext) fila.classList.add("tipo-" + ext.toLowerCase());
      }
      fila.setAttribute("role", "treeitem");
      fila.setAttribute("aria-level", String(profundidad + 1));
      if (n.dir) fila.setAttribute("aria-expanded", String(abierta));
      if (n.ruta === estadoPestana.ruta) {
        fila.classList.add("activo");
        fila.setAttribute("aria-selected", "true");
      }

      const flecha = document.createElement("span");
      flecha.className = "arbol-flecha " +
        (n.dir ? (abierta ? "abierta" : "") : "hoja");
      fila.appendChild(flecha);

      const icono = document.createElement("span");
      icono.className = "arbol-icono " + (n.dir ? "dir" : "archivo");
      fila.appendChild(icono);

      const nombre = document.createElement("span");
      nombre.className = "nombre";
      nombre.textContent = n.nombre;
      fila.appendChild(nombre);
      if (generado && rel === "build") {
        const etiqueta = document.createElement("span");
        etiqueta.className = "arbol-generado";
        etiqueta.textContent = "generado";
        fila.appendChild(etiqueta);
      }

      if (!n.dir) {
        const bajar = document.createElement("span");
        bajar.className = "arbol-descargar";
        bajar.textContent = "↓";
        bajar.title = "Descargar " + relativa(n.ruta);
        bajar.setAttribute("role", "button");
        bajar.setAttribute("aria-label", "Descargar " + relativa(n.ruta));
        fila.appendChild(bajar);
      }

      if (n.dir) {
        fila.onclick = (e) => {
          e.preventDefault();
          if (fila.dataset.menuLargo) { delete fila.dataset.menuLargo; return; }
          if (ARBOL.abiertos.has(n.ruta)) ARBOL.abiertos.delete(n.ruta);
          else ARBOL.abiertos.add(n.ruta);
          guardarPersistencia();
          refrescarArbol();
        };
      } else {
        fila.onclick = async (e) => {
          e.preventDefault();
          if (fila.dataset.menuLargo) { delete fila.dataset.menuLargo; return; }
          if (e.target.closest(".arbol-descargar")) {
            await descargarRuta(n.ruta);
            return;
          }
          if (await abrir(n.ruta)) {
            const vista = document.querySelector('[data-vista="pestana"]');
            if (vista && document.body.dataset.vista !== "pestana") vista.click();
          }
        };
      }
      conectarMenu(fila, n);
      fuera.appendChild(fila);
      if (n.dir && abierta) pintar(n.ruta, profundidad + 1);
    }
  };
  pintar(PROYECTO, 0);
  fuera.scrollTop = scroll;

  // el fichero abierto puede quedar fuera de la parte visible del explorador: se acerca
  // solo lo justo, sin saltar al principio ni mover la página entera
  const activo = fuera.querySelector("a.activo");
  if (activo) {
    const r = activo.getBoundingClientRect(), c = fuera.getBoundingClientRect();
    if (r.top < c.top) fuera.scrollTop -= c.top - r.top;
    else if (r.bottom > c.bottom) fuera.scrollTop += r.bottom - c.bottom;
  }
}

// Abrir ya no descarta nada: cada fichero tiene su pestaña y su texto vive en ella. El contenido
// de una pestaña limpia no se lee hasta que se activa, así restaurar diez ficheros no lee diez.
async function abrir(ruta, yaConfirmado) {
  if (!ruta) return false;
  if (documentoDe(ruta)) return activarDocumento(ruta);
  DOCUMENTOS.lista.push({ ruta, texto: null, sucio: false, cursor: null, scroll: null });
  renderPestanas();
  return activarDocumento(ruta);
}

// Selecciona una línea del fichero abierto. El número viene de la consola (`ruta:línea`) y se
// limita al tamaño real: un enlace no puede dejar el cursor fuera del texto.
function irALinea(numero) {
  const area = document.getElementById("codigo");
  if (!area) return;
  const lineas = area.value.split("\n");
  const n = Math.max(1, Math.min(Number(numero) || 1, lineas.length));
  let desde = 0;
  for (let i = 0; i < n - 1; i++) desde += lineas[i].length + 1;
  area.focus();
  area.setSelectionRange(desde, desde + lineas[n - 1].length);
  const alto = parseFloat(getComputedStyle(area).lineHeight) || 18;
  area.scrollTop = Math.max(0, (n - 3) * alto);
  EDITOR.sincronizar();
  actualizarCursor();
}

async function saltarALinea(ruta, numero) {
  const limpia = String(ruta || "").replace(/^\.\//, "");
  if (!limpia) return;
  const absoluta = limpia.startsWith("/") ? limpia : PROYECTO + "/" + limpia;
  if (!(await abrir(absoluta))) return;
  irALinea(numero);
}

// --- buscar, reemplazar e ir a línea --------------------------------------------------------
// La barra es del editor, no del navegador: buscar dentro de un fuente no puede abrir el panel
// de búsqueda de la página, que no conoce el resaltado ni el undo tree.
function abrirBusqueda(reemplazo) {
  const barra = document.getElementById("buscar-barra");
  if (!barra) return;
  barra.hidden = false;
  const campo = document.getElementById("buscar-texto");
  const area = document.getElementById("codigo");
  for (const id of ["reemplazar-texto", "reemplazar-uno", "reemplazar-todos"]) {
    const el = document.getElementById(id);
    if (el) el.hidden = !reemplazo;
  }
  if (campo) {
    if (area && area.selectionEnd > area.selectionStart) {
      const elegido = area.value.slice(area.selectionStart, area.selectionEnd);
      if (elegido && !elegido.includes("\n")) campo.value = elegido;
    }
    campo.focus();
    campo.select();
  }
}

function cerrarBusqueda() {
  const barra = document.getElementById("buscar-barra");
  if (barra) barra.hidden = true;
  const area = document.getElementById("codigo");
  if (area) area.focus();
}

function buscarSiguiente(atras) {
  const area = document.getElementById("codigo");
  const campo = document.getElementById("buscar-texto");
  const estado = document.getElementById("buscar-estado");
  if (!area || !campo) return false;
  const aguja = campo.value;
  if (!aguja) return false;
  const pajar = area.value.toLowerCase();
  const busca = aguja.toLowerCase();
  const desde = atras ? Math.max(0, area.selectionStart - 1) : area.selectionEnd;
  let i = atras ? pajar.lastIndexOf(busca, desde) : pajar.indexOf(busca, desde);
  if (i < 0) i = atras ? pajar.lastIndexOf(busca) : pajar.indexOf(busca);
  if (i < 0) {
    if (estado) estado.textContent = "sin coincidencias";
    return false;
  }
  area.focus();
  area.setSelectionRange(i, i + aguja.length);
  const linea = area.value.slice(0, i).split("\n").length;
  const alto = parseFloat(getComputedStyle(area).lineHeight) || 18;
  area.scrollTop = Math.max(0, (linea - 3) * alto);
  EDITOR.sincronizar();
  actualizarCursor();
  if (estado) estado.textContent = "línea " + linea;
  return true;
}

function reemplazarUno() {
  const area = document.getElementById("codigo");
  const aguja = (document.getElementById("buscar-texto") || {}).value || "";
  const nuevo = (document.getElementById("reemplazar-texto") || {}).value || "";
  if (!area || !aguja) return;
  const elegido = area.value.slice(area.selectionStart, area.selectionEnd);
  if (elegido.toLowerCase() !== aguja.toLowerCase()) {
    if (!buscarSiguiente(false)) return;
  }
  const desde = area.selectionStart, hasta = area.selectionEnd;
  EDITOR.cambiar(desde, hasta, nuevo, [desde + nuevo.length, desde + nuevo.length]);
  buscarSiguiente(false);
}

function reemplazarTodos() {
  const area = document.getElementById("codigo");
  const aguja = (document.getElementById("buscar-texto") || {}).value || "";
  const nuevo = (document.getElementById("reemplazar-texto") || {}).value || "";
  const estado = document.getElementById("buscar-estado");
  if (!area || !aguja) return;
  const patron = new RegExp(aguja.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), "gi");
  const cuantos = (area.value.match(patron) || []).length;
  if (!cuantos) {
    if (estado) estado.textContent = "sin coincidencias";
    return;
  }
  EDITOR.cambiar(0, area.value.length, area.value.replace(patron, nuevo));
  if (estado) estado.textContent = cuantos + " reemplazadas";
  actualizarCursor();
}

function irALineaDesdeCuadro() {
  const n = window.prompt("Ir a la línea", "1");
  if (n && /^\d+$/.test(n.trim())) irALinea(Number(n));
}

function marcarSucio(v) {
  const doc = documentoActivo();
  if (doc) doc.sucio = !!v;
  estadoPestana.sucio = !!v;
  pintarMarcaSucio();
}
function estaSucio() {
  const doc = documentoActivo();
  return doc ? !!doc.sucio : !!estadoPestana.sucio;
}

// Las acciones del menú Archivo que necesitan un fichero abierto se deshabilitan en vez de
// contestar «abre uno primero» cuando no lo hay.
function actualizarAccionesFichero() {
  const hay = !!documentoActivo() || !!estadoPestana.ruta;
  for (const id of ["renombrar-fichero", "borrar-fichero"]) {
    const boton = document.getElementById(id);
    if (boton) boton.disabled = !hay;
  }
  const descargar = document.getElementById("descargar-fichero");
  if (descargar) descargar.hidden = !hay;
}
function confirmarDescartes() { return window.confirm("Hay cambios sin guardar. ¿Descartarlos?"); }

async function guardar() {
  let doc = documentoActivo();
  if (!doc) {
    // compatibilidad con quien fija estadoPestana.ruta a mano (las pruebas lo hacen) y con el
    // primer guardado de un buffer sin nombre
    const ruta = estadoPestana.ruta || await nuevoNombre();
    if (!ruta) return false;
    const previo = await hospedador.leer(ruta);
    if (previo && previo.bytes && !window.confirm("Ya existe " + relativa(ruta) + ". ¿Reemplazarlo?")) return false;
    doc = { ruta, texto: EDITOR.texto(), sucio: true, cursor: null, scroll: null,
      hist: Historial.crear(ruta, EDITOR.texto()) };
    DOCUMENTOS.lista.push(doc);
    DOCUMENTOS.activa = ruta;
    estadoPestana.ruta = ruta;
    revelarRuta(ruta);
    renderPestanas();
  }
  if (!doc.hist) doc.hist = Historial.crear(doc.ruta, EDITOR.texto());
  volcarHistorial();
  const ruta = doc.ruta;
  const guardado = EDITOR.texto();
  const anterior = doc.revision;
  const revisionGuardada = revisionActual(ruta);
  if (anterior && revisionGuardada && anterior !== revisionGuardada &&
      !window.confirm("Otra ventana guardó " + relativa(ruta) + ". ¿Sobrescribir?")) {
    return false;
  }
  const revision = nuevaRevision();
  try {
    await conBloqueo(async () => {
      const escrito = await hospedador.escribir(ruta, new TextEncoder().encode(guardado));
      if (escrito && escrito.tipo === "error") throw new Error(escrito.error);
      try { localStorage.setItem(claveRevision(ruta), revision); } catch (e) { /* sin revisión */ }
    });
  } catch (e) {
    nota("no se pudo guardar " + ruta + ": " + (e && e.message ? e.message : e));
    return false;
  }
  doc.revision = revision;
  doc.base = guardado;
  // si se siguió escribiendo o se cambió de pestaña mientras el worker guardaba, el documento
  // solo queda limpio si el texto en memoria sigue siendo el que se escribió
  const actual = documentoDe(ruta);
  if (actual) {
    const texto = DOCUMENTOS.activa === ruta ? EDITOR.texto() : actual.texto;
    if (texto === guardado) {
      actual.sucio = false;
      if (DOCUMENTOS.activa === ruta) actual.texto = null;
    }
    if (DOCUMENTOS.activa === ruta) marcarSucio(!!actual.sucio);
  }
  renderPestanas();
  renderHistorial();
  actualizarAccionesFichero();
  Historial.guardar(VENTANA, doc.hist).catch(() => {});
  publicar({ tipo: "guardado", ruta, revision });
  nota("guardado en " + ruta);
  await refrescarArbol();
  guardarPersistencia();
  return true;
}

// El nombre por defecto sale del package que la persona escribió en Inicio, y cualquier ruta
// relativa se ancla a /proyecto: así "Nuevo" no puede escribir fuera del árbol que se ve.
function paqueteDelFormulario() {
  const campo = document.querySelector('#form [name="package"]');
  const paquete = campo ? String(campo.value || "").trim().toLowerCase() : "";
  return /^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$/.test(paquete) ? paquete : "com.mi.app";
}

function normalizarRuta(ruta) {
  const partes = [];
  for (const parte of ruta.split("/")) {
    if (!parte || parte === ".") continue;
    if (parte === "..") { partes.pop(); continue; }
    partes.push(parte);
  }
  return "/" + partes.join("/");
}

async function nuevoNombre(carpeta) {
  const base = carpeta || (PROYECTO + "/src/" + paqueteDelFormulario().replace(/\./g, "/"));
  const sugerida = base + "/" + (carpeta ? "nuevo.txt" : "MainActivity.java");
  const entrada = prompt("Ruta dentro del proyecto", sugerida);
  if (entrada === null) return null;
  let ruta = entrada.trim().replace(/\\/g, "/").replace(/\/{2,}/g, "/");
  if (!ruta.startsWith("/")) ruta = PROYECTO + "/" + ruta;
  ruta = normalizarRuta(ruta);
  if (!ruta.startsWith(PROYECTO + "/")) {
    nota("la ruta tiene que quedar dentro de /proyecto");
    return null;
  }
  return ruta;
}

// Crear un fichero desde el menú Archivo o desde el menú de una carpeta del árbol. La ruta se
// pide siempre, pero el valor por defecto cambia: MainActivity.java en el package del formulario,
// o nuevo.txt dentro de la carpeta sobre la que se pulsó.
async function nuevoFichero(carpeta) {
  const nombre = await nuevoNombre(carpeta);
  if (!nombre) return;
  const previo = await hospedador.leer(nombre);
  if (previo && previo.bytes && !window.confirm("Ya existe " + relativa(nombre) + ". ¿Reemplazarlo?")) return;
  // el fichero nace vacío en OPFS para que el explorador y el estado «sin cambios» digan
  // la verdad: un buffer suelto que no está en el árbol confunde más que ayuda
  try {
    await conBloqueo(async () => {
      const r = await hospedador.escribir(nombre, new Uint8Array(0));
      if (r && r.tipo === "error") throw new Error(r.error);
      try { localStorage.setItem(claveRevision(nombre), nuevaRevision()); } catch (e) { /* sin revisión */ }
    });
  } catch (e) {
    nota("no se pudo crear " + relativa(nombre) + ": " + (e && e.message ? e.message : e));
    return;
  }
  await abrir(nombre);
  const codigo = document.getElementById("codigo");
  if (codigo) codigo.focus();
  publicar({ tipo: "mutacion", accion: "crear", rutas: [nombre] });
  nota("creado " + relativa(nombre));
  guardarPersistencia();
}

// --- el panel inferior -------------------------------------------------------------------
function mostrarPanel(nombre) {
  const panel = document.getElementById("panel-inferior");
  if (!panel) return;
  panel.hidden = false;
  for (const b of document.querySelectorAll(".panel-tab")) {
    const activo = b.dataset.panel === nombre;
    b.classList.toggle("activo", activo);
    b.setAttribute("aria-selected", String(activo));
  }
  for (const v of document.querySelectorAll("#panel-inferior .panel-vista")) {
    v.classList.toggle("activa", v.id === "panel-" + nombre);
  }
  if (nombre === "historial") renderHistorial();
  const alternar = document.getElementById("alternar-panel");
  if (alternar) alternar.setAttribute("aria-expanded", "true");
}

function alternarPanel(nombre) {
  const panel = document.getElementById("panel-inferior");
  if (!panel) return;
  if (nombre) { mostrarPanel(nombre); return; }
  panel.hidden = !panel.hidden;
  const alternar = document.getElementById("alternar-panel");
  if (alternar) alternar.setAttribute("aria-expanded", String(!panel.hidden));
  if (!panel.hidden) {
    const activa = document.querySelector(".panel-tab.activo");
    if (activa) mostrarPanel(activa.dataset.panel);
  }
}

function cerrarPanel() {
  const panel = document.getElementById("panel-inferior");
  if (panel) panel.hidden = true;
  const alternar = document.getElementById("alternar-panel");
  if (alternar) alternar.setAttribute("aria-expanded", "false");
}

// --- los comandos ------------------------------------------------------------------------
const ULTIMO = { argv: ["create", PROYECTO] };

async function ejecutarUltimo() { await correr(ULTIMO.argv); }

async function correr(argv) {
  ULTIMO.argv = argv;
  // la salida vive en el panel del editor: un atajo lanzado desde Inicio no puede dejarla oculta
  const vistaEditor = document.querySelector('[data-vista="pestana"]');
  if (vistaEditor && document.body.dataset.vista !== "pestana") vistaEditor.click();
  mostrarPanel("salida");
  CONSOLA.escribir("$ apkc " + argv.join(" ") + "\n");
  const ocupado = document.getElementById("ocupado");
  const botonDetener = document.getElementById("detener-comando");
  if (ocupado) { ocupado.hidden = false; ocupado.textContent = "apkc trabajando…"; }
  if (botonDetener) botonDetener.hidden = false;
  let r;
  let recibido = 0;
  // la salida llega trozo a trozo: se pinta ya y la última línea hace de fase en la barra, sin
  // inventar porcentajes que apkc no imprime
  hospedador.escucharSalida((texto) => {
    recibido += texto.length;
    CONSOLA.escribir(texto);
    const lineas = texto.replace(/\u001b\[[0-9;?]*[A-Za-z]/g, "").split("\n").filter((l) => l.trim());
    const ultima = lineas[lineas.length - 1];
    if (ocupado && ultima) ocupado.textContent = "apkc: " + ultima.trim().slice(0, 70);
  });
  try {
    r = await hospedador.ejecutar(argv);
  } catch (e) {
    // si el worker no arranca, el aviso de «trabajando» no puede quedarse para siempre
    r = { tipo: "error", error: String((e && e.message) || e), consola: "" };
  } finally {
    hospedador.escucharSalida(null);
    if (ocupado) ocupado.hidden = true;
    if (botonDetener) botonDetener.hidden = true;
  }
  const completa = String(r.consola || "");
  CONSOLA.escribir(completa.slice(recibido) + (r.error ? "FALLO DEL HOSPEDADOR: " + r.error + "\n" : ""));
  if (r.tipo === "terminado") {
    const codigo = document.getElementById("ultimo-codigo");
    if (codigo) codigo.textContent = String(r.codigo);
  }
  await refrescarArbol();
  return r;
}

// Go wasm no se puede matar por dentro: se termina el worker y el siguiente comando arranca
// otro. Es honesto avisar de que el build puede quedar a medias.
function detener() {
  if (!hospedador.cancelar()) return;
  CONSOLA.escribir("\ncomando detenido; el build puede quedar a medias (apkc clean para empezar de cero).\n");
  const codigo = document.getElementById("ultimo-codigo");
  if (codigo) codigo.textContent = "(detenido)";
  const ocupado = document.getElementById("ocupado");
  if (ocupado) ocupado.hidden = true;
  const boton = document.getElementById("detener-comando");
  if (boton) boton.hidden = true;
}

function nota(texto) {
  // la página se puede recortar: sin el aviso, un fallo de OPFS no puede reventar el manejador
  const n = document.getElementById("estado-pestana");
  if (n) n.textContent = texto;
}

async function firmar() {
  const cfg = KEYSTORES.firmar;
  const k = await prepararKeystore(cfg);
  if (!k) return;
  const entrada = document.getElementById("apk-a-firmar");
  const origen = (entrada.value || PROYECTO + "/build/app.apk").trim();
  const esAab = /\.aab$/i.test(origen);
  const destino = PROYECTO + "/build/firmado." + (esAab ? "aab" : "apk");
  const v4 = document.getElementById("v4-firmar").checked;
  nota("firmando " + origen + " con " + k.alias + " (no sale de esta pestaña)");
  // `sign` escribe `firmado.*.tmp` junto al destino y no crea la carpeta. Si el proyecto no se
  // compiló nunca, build/ no existe: se crea con un fichero vacío (escribir crea los padres) y se
  // borra antes de firmar.
  await hospedador.escribir(PROYECTO + "/build/.carpeta", new Uint8Array(0));
  await hospedador.limpiar(PROYECTO + "/build/.carpeta");
  const argv = ["sign", "--keystore", k.ruta, "--storepass", k.pass,
    "--keyalias", k.alias, "--v4=" + (v4 ? "true" : "false")];
  if (document.getElementById("usar-rotacion").checked) {
    if (esAab) { nota("un .aab no lleva bloque v3/v3.1: la rotación es para apk"); return; }
    const passV = document.getElementById("storepass-viejo").value;
    const aliasV = (document.getElementById("keyalias-viejo").value || "androiddebugkey").trim();
    const rotMin = (document.getElementById("rotation-min-sdk").value || "33").trim();
    if (!passV) { nota("falta la contraseña del keystore viejo para la rotación"); return; }
    if (!(await hospedador.leer(RUTA_LINAJE)).bytes) {
      nota("crea el linaje antes de firmar con rotación");
      return;
    }
    argv.push("--lineage", RUTA_LINAJE, "--old-keystore", RUTA_VIEJO,
      "--old-storepass", passV, "--old-keyalias", aliasV, "--rotation-min-sdk", rotMin);
  }
  argv.push("--out", destino, origen);
  const r = await correr(argv);
  // la contraseña se limpia también cuando falla: un campo con la clave abierta en una pestaña
  // que se queda delante no es un detalle de estilo
  const campoPass = document.getElementById(cfg.pass);
  if (campoPass) campoPass.value = "";
  const campoViejo = document.getElementById("storepass-viejo");
  if (campoViejo) campoViejo.value = "";
  if (r && r.codigo === 0) {
    const v = await correr(["verify", destino]);
    await descargar(destino);
    await ofrecerDescarga(destino + ".idsig", "descargar-idsig");
    if (v && v.codigo === 0) nota("firmado y verificado: " + destino.split("/").pop());
  }
}

// Descargar una entrada cualquiera del explorador sin abrirla en el editor: vale también
// para un PNG, un .apk o un .idsig que no son texto.
async function descargarRuta(ruta) {
  const nombre = ruta.split("/").pop();
  return conCarga("Preparando " + nombre + "…", async () => {
    try {
      const bytes = await hospedador.bytes(ruta);
      if (!bytes) { nota("no existe " + ruta); return; }
      const enlace = document.createElement("a");
      enlace.href = URL.createObjectURL(new Blob([bytes]));
      enlace.download = nombre;
      enlace.click();
      setTimeout(() => URL.revokeObjectURL(enlace.href), 2000);
      nota("descarga lista: " + enlace.download + " (" + bytes.length + " bytes)");
    } catch (e) {
      nota("no se pudo descargar " + ruta + ": " + (e && e.message ? e.message : e));
    }
  });
}

// Los enlaces de descarga se reutilizan en cada compilación: guardar el blob anterior y
// revocarlo evita que una sesión larga acumule apks en memoria.
function ponerBlob(enlace, bytes, tipo) {
  if (!enlace) return null;
  if (enlace.dataset.blob) URL.revokeObjectURL(enlace.dataset.blob);
  const url = URL.createObjectURL(new Blob([bytes], { type: tipo }));
  enlace.dataset.blob = url;
  enlace.href = url;
  return url;
}

async function descargar(ruta, ancla) {
  const nombre = ruta.split("/").pop();
  return conCarga("Preparando " + nombre + "…", async () => {
    const r = await hospedador.leer(ruta);
    if (!r.bytes) { nota("no hay nada en " + ruta); return; }
    const bytes = r.bytes instanceof Uint8Array ? r.bytes : new Uint8Array(r.bytes);
    const enlace = document.getElementById(ancla || "descargar-apk");
    if (!enlace) return;
    const tipo = /\.aab$/i.test(ruta) ? "application/octet-stream" : "application/vnd.android.package-archive";
    ponerBlob(enlace, bytes, tipo);
    enlace.download = nombre;
    enlace.hidden = false;
    if (ancla === "descargar-fichero") {
      enlace.textContent = "Descargar";
    } else {
      enlace.textContent = enlace.download + " (" + bytes.length + " bytes)";
      const barra = document.getElementById("panel-descargas");
      if (barra) barra.hidden = false;
    }
    nota("descarga lista: " + enlace.download + " (" + bytes.length + " bytes)");
  });
}

// El .idsig viaja al lado del apk. Si no existe (un .aab, o --v4=false) el enlace se
// esconde, para que no quede un botón que no lleva a nada.
async function ofrecerDescarga(ruta, ancla) {
  const nombre = ruta.split("/").pop();
  return conCarga("Preparando " + nombre + "…", async () => {
    const enlace = document.getElementById(ancla);
    if (!enlace) return;
    const r = await hospedador.leer(ruta);
    if (!r.bytes) {
      enlace.hidden = true;
      if (enlace.dataset.blob) { URL.revokeObjectURL(enlace.dataset.blob); delete enlace.dataset.blob; }
      enlace.removeAttribute("href");
      return;
    }
    const bytes = r.bytes instanceof Uint8Array ? r.bytes : new Uint8Array(r.bytes);
    ponerBlob(enlace, bytes, "application/octet-stream");
    enlace.download = nombre;
    enlace.textContent = enlace.download + " (" + bytes.length + " bytes)";
    enlace.hidden = false;
    const barra = document.getElementById("panel-descargas");
    if (barra) barra.hidden = false;
  });
}

// --- el build sin procesos: recursos, Java, dex y firma, en este mismo binario ------------
const RUTA_JAR = "/sdk/android.jar";
const RUTA_KEYSTORE = "/sdk/release.keystore";
const RUTA_VIEJO = "/keystore-viejo";
const RUTA_LINAJE = "/lineage.bin";

// API de compilación: el selector visible mantiene sincronizados los dos campos que ya usaba
// `apkc build` (y que las pruebas siguen fijando a mano).
const API_NOMBRES = {
  27: "8.1", 28: "9", 29: "10", 30: "11", 31: "12", 32: "12L",
  33: "13", 34: "14", 35: "15",
};
const API_PREF = "apkc-studio/api";

function sincronizarApi(guardar) {
  const selector = document.getElementById("api-version");
  const version = document.getElementById("sdk-version");
  const codename = document.getElementById("codename");
  if (!selector) return;
  if (version) version.value = selector.value;
  if (codename) codename.value = API_NOMBRES[selector.value] || "";
  if (guardar) {
    try { localStorage.setItem(API_PREF, selector.value); } catch (e) { /* sin preferencia */ }
  }
}

// Lo que el servidor local sabe del SDK. En una página servida de forma estática no hay
// /android-jar-info: el camino es el paquete oficial de Google, el .zip o el .jar.
const JAR_SDK = { disponible: false, consultado: false, version: null, bytes: 0 };

// Un keystore por flujo: el del build queda en OPFS (`/sdk/release.keystore`) y el de `sign` en
// `/keystore`, las rutas que ya usaban los botones. Lo nuevo es la contraseña, el alias que se
// elige entre los que el fichero declara, y el certificado que se enseña antes de firmar.
const KEYSTORES = {
  compilar: { fichero: "keystore-compilar", pass: "storepass-compilar", alias: "keyalias-compilar",
              estado: "estado-keystore-compilar", ruta: RUTA_KEYSTORE },
  firmar: { fichero: "keystore", pass: "storepass-firmar", alias: "keyalias-firmar",
            estado: "estado-keystore-firmar", ruta: "/keystore" },
};

function actualizarEstadoClave(tiene) {
  const detalle = document.getElementById("detalle-release");
  const badge = document.getElementById("clave-badge");
  const boton = document.getElementById("crear-clave-release");
  if (detalle) {
    detalle.classList.toggle("jar-listo", !!tiene);
    if (tiene) detalle.open = false;
  }
  if (badge) {
    badge.hidden = !tiene;
    badge.textContent = tiene ? "listo" : "";
  }
  if (boton) {
    boton.textContent = tiene ? "Cambiar clave de release" : "Crear clave de release";
    boton.classList.toggle("primario", !tiene);
  }
}

async function decirEstadoClave() {
  try {
    const r = await hospedador.leer(RUTA_KEYSTORE);
    actualizarEstadoClave(!!(r && r.bytes));
    return !!(r && r.bytes);
  } catch (e) {
    actualizarEstadoClave(false);
    return false;
  }
}

let claveDescargaUrl = null;

function cerrarDialogoClave() {
  const dialogo = document.getElementById("dialogo-clave");
  if (!dialogo) return;
  if (typeof dialogo.close === "function") dialogo.close();
  else dialogo.hidden = true;
}

function abrirDialogoClave() {
  const dialogo = document.getElementById("dialogo-clave");
  if (!dialogo) return;
  const form = document.getElementById("clave-paso-form");
  const listo = document.getElementById("clave-paso-listo");
  if (form) form.hidden = false;
  if (listo) listo.hidden = true;
  const aviso = document.getElementById("clave-aviso");
  const guardada = document.getElementById("clave-guardada");
  const generar = document.getElementById("clave-generar");
  const terminar = document.getElementById("clave-terminar");
  const estado = document.getElementById("clave-estado");
  if (aviso) aviso.checked = false;
  if (guardada) guardada.checked = false;
  if (generar) generar.disabled = true;
  if (terminar) terminar.disabled = true;
  if (estado) estado.textContent = "";
  for (const id of ["clave-pass", "clave-pass2"]) {
    const campo = document.getElementById(id);
    if (campo) campo.value = "";
  }
  if (typeof dialogo.showModal === "function") dialogo.showModal();
  else dialogo.hidden = false;
}

async function generarClaveRelease() {
  const valor = (id) => {
    const campo = document.getElementById(id);
    return campo ? campo.value.trim() : "";
  };
  const alias = valor("clave-alias") || "release";
  const pass = document.getElementById("clave-pass") ? document.getElementById("clave-pass").value : "";
  const pass2 = document.getElementById("clave-pass2") ? document.getElementById("clave-pass2").value : "";
  const cn = valor("clave-cn") || "Release";
  const org = valor("clave-org");
  const country = (valor("clave-pais") || "ES").toUpperCase();
  const estado = document.getElementById("clave-estado");
  const aviso = document.getElementById("clave-aviso");
  const decir = (texto) => { if (estado) estado.textContent = texto; };

  if (!aviso || !aviso.checked) { decir("marca el aviso antes de generar la clave"); return; }
  if (!/^[A-Za-z0-9._-]{1,64}$/.test(alias)) {
    decir("el alias solo puede llevar letras, números, punto, guion o guion bajo"); return;
  }
  if (pass.length < 8) { decir("la contraseña debe tener al menos 8 caracteres"); return; }
  if (pass !== pass2) { decir("las dos contraseñas no coinciden"); return; }
  if (!/^[A-Za-z]{2}$/.test(country)) { decir("el país tiene que ser dos letras, por ejemplo ES"); return; }
  const previo = await hospedador.leer(RUTA_KEYSTORE);
  if (previo && previo.bytes &&
      !window.confirm("Ya hay una clave de release en esta pestaña. ¿Reemplazarla? La anterior dejará de servir para actualizar la app.")) {
    return;
  }

  decir("generando la clave…");
  try {
    const r = await conBloqueo(() => correr([
      "create-keystore", "--out", RUTA_KEYSTORE,
      "--storepass", pass, "--keyalias", alias,
      "--common-name", cn, "--organization", org, "--country", country,
      "--force",
    ]));
    if (!r || r.codigo !== 0) {
      decir("no se pudo generar la clave; mira la consola");
      return;
    }
    const bytes = await hospedador.bytes(RUTA_KEYSTORE);
    if (!bytes || bytes.length < 1000) {
      decir("la clave no quedó en OPFS");
      return;
    }
    const campoPass = document.getElementById("storepass-compilar");
    if (campoPass) campoPass.value = pass;
    const select = document.getElementById("keyalias-compilar");
    if (select) {
      select.innerHTML = "";
      select.add(new Option(alias, alias, true, true));
    }
    actualizarEstadoClave(true);

    if (claveDescargaUrl) URL.revokeObjectURL(claveDescargaUrl);
    claveDescargaUrl = URL.createObjectURL(new Blob([bytes], { type: "application/octet-stream" }));
    const enlace = document.getElementById("clave-descargar");
    if (enlace) {
      enlace.href = claveDescargaUrl;
      enlace.download = alias + ".jks";
      enlace.click();
    }
    const form = document.getElementById("clave-paso-form");
    const listo = document.getElementById("clave-paso-listo");
    if (form) form.hidden = true;
    if (listo) listo.hidden = false;
    for (const id of ["clave-pass", "clave-pass2"]) {
      const campo = document.getElementById(id);
      if (campo) campo.value = "";
    }
    decir("");
  } catch (e) {
    decir("no se pudo generar la clave: " + (e && e.message ? e.message : e));
  }
}

function estadoKeystore(cfg, texto) {
  document.getElementById(cfg.estado).textContent = texto;
}

// Escribe el fichero elegido en la ruta de la pestaña. Si no hay uno elegido, vale el que ya
// está guardado: así el botón funciona después de recargar y la prueba puede montarlo.
async function guardarKeystoreElegido(cfg) {
  const entrada = document.getElementById(cfg.fichero);
  if (entrada && entrada.files.length) {
    const datos = new Uint8Array(await entrada.files[0].arrayBuffer());
    await hospedador.escribir(cfg.ruta, datos);
    return true;
  }
  return !!(await hospedador.leer(cfg.ruta)).bytes;
}

// Inspecciona con el mismo binario que firma: alias, sujeto, huella y validez. Llena el
// <select> y, si el fichero tiene una sola clave, la deja elegida.
async function inspeccionarKeystore(cfg) {
  if (!(await guardarKeystoreElegido(cfg))) {
    estadoKeystore(cfg, "elige primero un keystore (.p12 o .jks)");
    return null;
  }
  const pass = document.getElementById(cfg.pass).value;
  if (!pass) { estadoKeystore(cfg, "escribe la contraseña del keystore"); return null; }
  const r = await correr(["keystore", "--keystore", cfg.ruta, "--storepass", pass]);
  const m = /keystore-json: (\{.*\})/.exec(r.consola || "");
  if (!m) {
    const ultima = (r.consola || "").trim().split("\n").pop() || "sin salida";
    estadoKeystore(cfg, "no se pudo leer el keystore: " + ultima);
    return null;
  }
  let info;
  try { info = JSON.parse(m[1]); } catch (e) {
    estadoKeystore(cfg, "el keystore no devolvió un JSON legible");
    return null;
  }
  const select = document.getElementById(cfg.alias);
  select.innerHTML = "";
  if (info.keys === 1 && info.entries.length === 1) {
    const e = info.entries[0];
    select.add(new Option(e.alias, e.alias, true, true));
  } else {
    select.add(new Option("— elige el alias —", ""));
    for (const e of info.entries) select.add(new Option(e.alias, e.alias));
  }
  const e = info.entries.find((x) => x.alias === select.value) || info.entries[0];
  if (!e) { estadoKeystore(cfg, "el keystore no tiene ninguna clave con la que firmar"); return null; }
  const huella = e.sha256 ? " · SHA-256 " + e.sha256 : " · sin certificado";
  const validez = e.notAfter ? " · hasta " + e.notAfter : "";
  estadoKeystore(cfg, info.keys + " clave(s) · " + e.alias + " · " + e.subject + huella + validez);
  if (cfg === KEYSTORES.compilar) actualizarEstadoClave(true);
  return { info, elegida: e };
}

// Lo que firma: inspecciona siempre (para enseñar el certificado y exigir alias si hay varias
// claves) y devuelve las credenciales. La contraseña solo vive en el campo y en esta llamada.
async function prepararKeystore(cfg) {
  const v = await inspeccionarKeystore(cfg);
  if (!v) return null;
  const alias = document.getElementById(cfg.alias).value;
  if (!alias) { estadoKeystore(cfg, "elige el alias con el que firmar"); return null; }
  return { ruta: cfg.ruta, pass: document.getElementById(cfg.pass).value, alias: alias };
}

// «olvidar»: borra el fichero de OPFS y el estado de la página. La contraseña no se guardó.
async function olvidarKeystore(cfg) {
  await hospedador.limpiar(cfg.ruta);
  if (cfg === KEYSTORES.compilar) actualizarEstadoClave(false);
  document.getElementById(cfg.pass).value = "";
  if (document.getElementById(cfg.fichero)) document.getElementById(cfg.fichero).value = "";
  const select = document.getElementById(cfg.alias);
  select.innerHTML = "";
  select.add(new Option("— pulsa «ver certificado» —", ""));
  estadoKeystore(cfg, "keystore olvidado: se borró de esta pestaña");
}

function estadoLinaje(texto) {
  const n = document.getElementById("estado-linaje");
  if (n) n.textContent = texto;
}

// Crea el linaje con la clave nueva ya inspeccionada (la de arriba) y la vieja de este bloque.
async function crearLinaje() {
  const cfg = KEYSTORES.firmar;
  const k = await prepararKeystore(cfg);
  if (!k) return;
  const entrada = document.getElementById("keystore-viejo");
  if (entrada.files.length) {
    const datos = new Uint8Array(await entrada.files[0].arrayBuffer());
    await hospedador.escribir(RUTA_VIEJO, datos);
  } else if (!(await hospedador.leer(RUTA_VIEJO)).bytes) {
    estadoLinaje("elige el keystore viejo (.p12 o .jks)");
    return;
  }
  const pass = document.getElementById("storepass-viejo").value;
  if (!pass) { estadoLinaje("escribe la contraseña del keystore viejo"); return; }
  const alias = (document.getElementById("keyalias-viejo").value || "androiddebugkey").trim();
  const r = await correr(["rotate", "--old-keystore", RUTA_VIEJO, "--old-storepass", pass,
    "--old-keyalias", alias, "--new-keystore", k.ruta, "--new-storepass", k.pass,
    "--new-keyalias", k.alias, "--out", RUTA_LINAJE]);
  if (r && r.codigo === 0) {
    estadoLinaje("linaje listo: la vieja firma v1/v2 y " + k.alias + " firma v3/v3.1");
  }
}

async function olvidarLinaje() {
  await hospedador.limpiar(RUTA_LINAJE);
  await hospedador.limpiar(RUTA_VIEJO);
  if (document.getElementById("keystore-viejo")) document.getElementById("keystore-viejo").value = "";
  document.getElementById("storepass-viejo").value = "";
  document.getElementById("usar-rotacion").checked = false;
  estadoLinaje("sin linaje");
}

// El paquete oficial de una plataforma es un .zip con `android-34/android.jar` dentro. Se lee
// el directorio central del zip, se toma esa entrada y se descomprime con DecompressionStream:
// así una página estática no obliga a descomprimir a mano. Si lo elegido ya es un .jar, no hay
// entrada anidada y se usa tal cual.
async function jarDeSeleccion(datos) {
  if (!datos || datos.length < 1000 || datos[0] !== 0x50 || datos[1] !== 0x4b) {
    throw new Error("el fichero no es un .jar ni un .zip");
  }
  const anidado = await extraerJarDeZip(datos);
  const jar = anidado || datos;
  if (jar.length < 1000 || jar[0] !== 0x50 || jar[1] !== 0x4b) {
    throw new Error("lo elegido no contiene un android.jar (ni es un .jar)");
  }
  return jar;
}

async function extraerJarDeZip(datos) {
  const dv = new DataView(datos.buffer, datos.byteOffset, datos.byteLength);
  const desde = Math.max(0, datos.length - 65557);
  let eocd = -1;
  for (let i = datos.length - 22; i >= desde; i--) {
    if (dv.getUint32(i, true) === 0x06054b50) { eocd = i; break; }
  }
  if (eocd < 0) throw new Error("el zip no tiene fin de directorio (EOCD)");
  const entradas = dv.getUint16(eocd + 10, true);
  const inicioCentral = dv.getUint32(eocd + 16, true);
  if (entradas === 0xffff || inicioCentral === 0xffffffff) {
    throw new Error("zip64 no soportado: descomprime el paquete a mano y elige android.jar");
  }
  const dec = new TextDecoder();
  let p = inicioCentral;
  for (let i = 0; i < entradas; i++) {
    if (dv.getUint32(p, true) !== 0x02014b50) throw new Error("zip: entrada central " + i + " corrupta");
    const metodo = dv.getUint16(p + 10, true);
    const csize = dv.getUint32(p + 20, true);
    const usize = dv.getUint32(p + 24, true);
    const nlen = dv.getUint16(p + 28, true);
    const elen = dv.getUint16(p + 30, true);
    const clen = dv.getUint16(p + 32, true);
    const local = dv.getUint32(p + 42, true);
    const nombre = dec.decode(datos.subarray(p + 46, p + 46 + nlen));
    if (/(^|\/)android\.jar$/i.test(nombre)) {
      if (dv.getUint32(local, true) !== 0x04034b50) throw new Error("zip: cabecera local de " + nombre + " corrupta");
      const inicioDatos = local + 30 + dv.getUint16(local + 26, true) + dv.getUint16(local + 28, true);
      const comprimido = datos.subarray(inicioDatos, inicioDatos + csize);
      let salida;
      if (metodo === 0) salida = comprimido.slice();
      else if (metodo === 8) {
        if (typeof DecompressionStream !== "function") {
          throw new Error("este navegador no descomprime deflate: descomprime el zip a mano");
        }
        const flujo = new Blob([comprimido]).stream().pipeThrough(new DecompressionStream("deflate-raw"));
        salida = new Uint8Array(await new Response(flujo).arrayBuffer());
      } else {
        throw new Error("zip: método de compresión " + metodo + " no soportado");
      }
      if (salida.length !== usize) {
        throw new Error("zip: android.jar descomprimido mide " + salida.length + ", se esperaban " + usize);
      }
      return salida;
    }
    p += 46 + nlen + elen + clen;
  }
  return null;
}

async function ponerJarDeUrl(url) {
  // la ruta del servidor local, no de un CDN: la página solo habla con su propio origen, y eso es
  // lo que comprueba la prueba de la pestaña contando las peticiones
  return conCarga("Descargando android.jar…", async () => {
    const respuesta = await fetch(url);
    if (!respuesta.ok) throw new Error("el android.jar de " + url + " respondió " + respuesta.status);
    const datos = new Uint8Array(await respuesta.arrayBuffer());
    // un 200 con una página de error no puede acabar en OPFS como si fuera el jar
    if (datos.length < 1000 || datos[0] !== 0x50 || datos[1] !== 0x4b) {
      throw new Error("lo que devolvió " + url + " no es un jar (cabecera " +
        Array.from(datos.slice(0, 2)).join(",") + ")");
    }
    await hospedador.escribir(RUTA_JAR, datos);
    await decirEstadoJar();
    return datos.length;
  });
}

// El servidor busca el SDK local y sirve su android.jar en /android-jar; la pestaña lo pide a
// su propio origen. La licencia de la SDK no permite versionarlo en el repo, pero sí leerlo de
// una instalación oficial que ya esté en la máquina.
async function traerJarDelSdk(silencioso) {
  try {
    const n = await ponerJarDeUrl("/android-jar");
    nota("android.jar del SDK local guardado en OPFS (" + n + " bytes)");
    return true;
  } catch (e) {
    if (!silencioso) {
      const motivo = e && e.message ? e.message : String(e);
      nota("no se pudo traer el android.jar: " + motivo +
        ". Elige el fichero del SDK o ejecuta tools/entorno.sh jar");
    }
    return false;
  }
}

// Pinta si el servidor ve un SDK local, sin bajar el jar entero: es un JSON pequeño y deja claro
// si el botón «Traer del SDK local» va a poder hacer algo.
async function consultarJarDelSdk() {
  const aviso = document.getElementById("estado-jar-sdk");
  const boton = document.getElementById("traer-android-jar");
  const sinSdk = "sin servidor local de SDK: descarga el paquete oficial o elige el .jar/.zip";
  if (!aviso) return;
  try {
    const respuesta = await fetch("/android-jar-info");
    if (!respuesta.ok) throw new Error("HTTP " + respuesta.status);
    const info = await respuesta.json();
    JAR_SDK.disponible = !!info.disponible;
    JAR_SDK.version = info.version || null;
    JAR_SDK.bytes = info.bytes || 0;
    aviso.textContent = JAR_SDK.disponible
      ? "el servidor ve el android.jar del SDK local: API " + info.version + ", " + info.bytes + " bytes"
      : sinSdk;
  } catch (e) {
    JAR_SDK.disponible = false;
    aviso.textContent = sinSdk;
  } finally {
    JAR_SDK.consultado = true;
    if (boton) boton.hidden = !JAR_SDK.disponible;
    const usar = document.getElementById("sdk-usar-servidor");
    if (usar) usar.hidden = !JAR_SDK.disponible;
  }
}

async function usarJarElegido() {
  const entrada = document.getElementById("android-jar");
  if (!entrada.files.length) { nota("elige el android.jar o el .zip oficial primero"); return false; }
  const archivo = entrada.files[0];
  return conCarga("Leyendo " + archivo.name + "…", async () => {
    const datos = new Uint8Array(await archivo.arrayBuffer());
    const jar = await jarDeSeleccion(datos);
    await hospedador.escribir(RUTA_JAR, jar);
    await decirEstadoJar();
    nota("android.jar guardado en esta pestaña (" + jar.length + " bytes); no sube a ningún sitio");
    return true;
  });
}

function actualizarEstadoSdk(tiene, bytes) {
  const detalle = document.getElementById("detalle-sdk");
  const badge = document.getElementById("sdk-badge");
  const boton = document.getElementById("abrir-dialogo-sdk");
  if (detalle) {
    detalle.classList.toggle("jar-listo", !!tiene);
    if (tiene) detalle.open = false;          // ya está: que no siga llamando la atención
  }
  if (badge) {
    badge.hidden = !tiene;
    badge.textContent = tiene ? "listo" : "";
  }
  if (boton) {
    boton.textContent = tiene ? "Cambiar android.jar" : "Obtener el android.jar";
    boton.classList.toggle("primario", !tiene);
  }
}

async function decirEstadoJar() {
  const aviso = document.getElementById("estado-jar");
  if (!aviso) return false;
  try {
    const r = await hospedador.leer(RUTA_JAR);
    if (r.bytes) {
      actualizarEstadoSdk(true, r.bytes.length);
      aviso.textContent = "android.jar listo en esta pestaña (" + r.bytes.length + " bytes)";
      return true;
    }
  } catch (e) {
    actualizarEstadoSdk(false);
    aviso.textContent = "no se puede leer OPFS en este contexto: " + (e && e.message ? e.message : e);
    return false;
  }
  actualizarEstadoSdk(false);
  aviso.textContent = "sin android.jar en esta pestaña todavía";
  return false;
}

async function asegurarJarDelProyecto() {
  if (await decirEstadoJar()) return true;
  const entrada = document.getElementById("android-jar");
  // si la persona ya eligió un fichero, ese manda; si no, solo se prueba el SDK local cuando
  // el servidor ha dicho que existe (en una página estática ese endpoint no está)
  if (entrada && entrada.files.length) {
    if (await usarJarElegido()) return true;
    nota("no se pudo leer el android.jar elegido");
    return false;
  }
  if (!JAR_SDK.consultado) await consultarJarDelSdk();
  if (JAR_SDK.disponible && await traerJarDelSdk(true)) return true;
  nota("hace falta el android.jar: acepta los términos, descarga el paquete oficial y elige el .zip o el .jar en Compilar");
  mostrarPanel("compilar");
  const detalle = document.getElementById("detalle-sdk");
  if (detalle) detalle.open = true;
  return false;
}

// La clave de depuración vive en OPFS y se crea una sola vez. No es una clave de release:
// alias androiddebugkey, contraseña android y CN=Android Debug, lo que espera cualquier
// instalación de desarrollo. Se conserva para que una apk debug se pueda actualizar encima
// de la anterior.
const DEBUG_KEY = { ruta: "/sdk/debug.keystore", pass: "android", alias: "androiddebugkey" };

async function asegurarDebugKeystore() {
  const existente = await hospedador.leer(DEBUG_KEY.ruta);
  if (existente.bytes) { decirEstadoDepuracion(); return DEBUG_KEY; }
  nota("creando la clave de depuración en esta pestaña…");
  const r = await correr(["debug-keystore", "--out", DEBUG_KEY.ruta,
    "--storepass", DEBUG_KEY.pass, "--keyalias", DEBUG_KEY.alias]);
  if (!r || r.codigo !== 0) {
    nota("no se pudo crear la clave de depuración; revisa la salida");
    return null;
  }
  if (!(await hospedador.leer(DEBUG_KEY.ruta)).bytes) {
    nota("la clave de depuración no quedó en OPFS");
    return null;
  }
  decirEstadoDepuracion();
  nota("clave de depuración creada: androiddebugkey · CN=Android Debug");
  return DEBUG_KEY;
}

// La clave de depuración también se puede sacar de la pestaña: es la misma que usa Android
// (`androiddebugkey` / `android`) y sirve para actualizar una instalación de prueba.
async function decirEstadoDepuracion() {
  const boton = document.getElementById("descargar-debug-keystore");
  if (!boton) return false;
  try {
    const r = await hospedador.leer(DEBUG_KEY.ruta);
    const hay = !!(r && r.bytes);
    boton.hidden = !hay;
    return hay;
  } catch (e) {
    boton.hidden = true;
    return false;
  }
}

async function compilarCon(k, tipo) {
  const v4 = document.getElementById("v4-compilar").checked;
  const version = (document.getElementById("sdk-version").value || "34").trim();
  const codename = (document.getElementById("codename").value || "14").trim();
  await hospedador.cwd(PROYECTO);
  const r = await correr(["build", "--linker", "go", "--compiler", "go",
    "--android-jar", RUTA_JAR, "--sdk-version", version, "--codename", codename,
    "--keystore", k.ruta, "--storepass", k.pass, "--keyalias", k.alias,
    "--v4=" + (v4 ? "true" : "false")]);
  // la contraseña de release no se queda escrita aunque el build falle
  if (tipo !== "debug") {
    const campoPass = document.getElementById(KEYSTORES.compilar.pass);
    if (campoPass) campoPass.value = "";
  }
  if (r && r.codigo === 0) {
    await descargar(PROYECTO + "/build/app.apk", "descargar-apk-compilada");
    await ofrecerDescarga(PROYECTO + "/build/app.apk.idsig", "descargar-idsig-compilada");
    await verificarLoCompilado();
  }
  return r;
}

async function compilarDebug() {
  if (!(await asegurarJarDelProyecto())) return;
  const k = await asegurarDebugKeystore();
  if (!k) return;
  nota("compilando debug con la clave de esta pestaña (no hace falta keystore)");
  return compilarCon(k, "debug");
}

async function compilarDelTodo() {
  if (!(await asegurarJarDelProyecto())) return;
  const detalle = document.getElementById("detalle-release");
  if (detalle) detalle.open = true;
  const k = await prepararKeystore(KEYSTORES.compilar);
  if (!k) return;
  return compilarCon(k, "release");
}

// Verificar lo que se compiló aquí, con el lector de firmas del propio binario: la vuelta al apk se
// hace sobre el fs de la pestaña, sin apksigner y sin red. Si el verificador dice mal, su motivo es la
// única frase que le interesa a quien espera una apk utilizable.
async function verificarLoCompilado() {
  const aviso = document.getElementById("estado-verificacion");
  await hospedador.cwd(PROYECTO);
  const r = await correr(["verify", "build/app.apk"]);
  const lineas = String(r.consola || "")
    .replace(/\u001b\[[0-9]*m/g, "")
    .split("\n")
    .map((l) => l.replace("[verify] ", "").trim())
    .filter((l) => l.startsWith("v1 (JAR signing)") || l.startsWith("v2 (APK Signature Scheme v2)") ||
      l.startsWith("certificado") || l.startsWith("par desconocido"));
  if (aviso) aviso.textContent = lineas.length ? lineas.join(" · ") : "apkc no devolvió ningún veredicto";
  return r;
}

// --- ajustes del proyecto ------------------------------------------------------------------
const RUTA_MANIFIESTO = PROYECTO + "/AndroidManifest.xml";

async function leerAjustes(silencioso) {
  const aviso = document.getElementById("estado-ajustes");
  const texto = await hospedador.texto(RUTA_MANIFIESTO);
  if (!texto) {
    if (aviso) aviso.textContent = "no hay AndroidManifest.xml todavía";
    return null;
  }
  const a = ajustesDelManifiesto(texto);
  const poner = (id, valor) => {
    const campo = document.getElementById(id);
    if (campo && valor !== undefined && valor !== "") campo.value = valor;
  };
  poner("ajustes-minsdk", a.minSdk || "21");
  poner("ajustes-targetsdk", a.targetSdk || "34");
  poner("ajustes-versioncode", a.versionCode || "1");
  poner("ajustes-versionname", a.versionName || "0.1");
  if (aviso && !silencioso) {
    aviso.textContent = "leído del manifiesto: minSdk " + (a.minSdk || "?") +
      " · targetSdk " + (a.targetSdk || "?") + " · " + (a.versionName || "?") +
      " (" + (a.versionCode || "?") + ")";
  }
  return a;
}

async function guardarAjustes() {
  const valor = (id) => {
    const campo = document.getElementById(id);
    return campo ? campo.value : "";
  };
  const aviso = document.getElementById("estado-ajustes");
  try {
    const texto = await hospedador.texto(RUTA_MANIFIESTO);
    if (!texto) { nota("no hay AndroidManifest.xml que ajustar"); return; }
    const nuevo = conAjustes(texto, {
      minSdk: valor("ajustes-minsdk"),
      targetSdk: valor("ajustes-targetsdk"),
      versionCode: valor("ajustes-versioncode"),
      versionName: valor("ajustes-versionname"),
    });
    await conBloqueo(async () => {
      const r = await hospedador.escribir(RUTA_MANIFIESTO, new TextEncoder().encode(nuevo));
      if (r && r.tipo === "error") throw new Error(r.error);
    });
    try { localStorage.setItem(claveRevision(RUTA_MANIFIESTO), nuevaRevision()); } catch (e) { /* sin revisión */ }
    const doc = documentoDe(RUTA_MANIFIESTO);
    if (doc) {
      doc.texto = nuevo;
      doc.sucio = false;
      if (DOCUMENTOS.activa === RUTA_MANIFIESTO) EDITOR.poner(nuevo);
      renderPestanas();
    }
    await refrescarArbol();
    publicar({ tipo: "mutacion", accion: "guardar", rutas: [RUTA_MANIFIESTO] });
    const target = Number(valor("ajustes-targetsdk"));
    const politica = target < 34
      ? " · targetSdk " + target + " está por detrás de la política actual de Google Play"
      : "";
    if (aviso) aviso.textContent = "guardado en AndroidManifest.xml" + politica;
    nota("ajustes guardados en el manifiesto" + politica);
  } catch (e) {
    if (aviso) aviso.textContent = "no se pudo guardar: " + (e && e.message ? e.message : e);
  }
}

// --- arranque ----------------------------------------------------------------------------
let listo = false;
async function iniciarPestana() {
  try {
    const r = await hospedador.arbol();
    listo = true;
    const nodos = r.nodos || [];
    const guardado = leerPersistencia();
    const aislacion = document.getElementById("aislacion");
    if (aislacion) aislacion.textContent = r.aislada ? "sí" : "no (hay que servir por http con COOP/COEP)";

    // restaurar árbol y pestañas antes de pintar; las pestañas se crean vacías y solo la activa
    // se lee de OPFS
    for (const ruta of guardado.abiertos || []) {
      if (nodos.some((n) => n.dir && n.ruta === ruta)) ARBOL.abiertos.add(ruta);
    }
    const rutas = [...new Set((guardado.tabs || []).filter((ruta) =>
      nodos.some((n) => !n.dir && n.ruta === ruta)))];
    DOCUMENTOS.lista = rutas.map((ruta) => ({ ruta, texto: null, sucio: false, cursor: null, scroll: null }));
    DOCUMENTOS.activa = null;
    renderPestanas();
    await refrescarArbol();

    const manifest = nodos.find((n) => n.ruta === PROYECTO + "/AndroidManifest.xml");
    const activa = rutas.includes(guardado.activa) ? guardado.activa : rutas[0];
    if (activa) {
      await activarDocumento(activa);
      const editor = document.querySelector('[data-vista="pestana"]');
      if (editor) editor.click();
    } else if (manifest) {
      await abrir(PROYECTO + "/AndroidManifest.xml");
      // si ya había proyecto en OPFS, se entra directamente al editor en vez de dejar a la
      // persona en Inicio mirando una pantalla que ya no le toca
      const editor = document.querySelector('[data-vista="pestana"]');
      if (editor) editor.click();
    } else {
      limpiarSinDocumento();
      const codigo = document.getElementById("codigo");
      if (codigo) codigo.placeholder = "Abre un fichero del explorador o crea un proyecto en Inicio.";
    }
  } catch (e) {
    const aislacion = document.getElementById("aislacion");
    if (aislacion) aislacion.textContent = "no disponible";
    // no borrar la persistencia por un fallo transitorio: el proyecto puede volver al recargar
    DOCUMENTOS.lista = [];
    DOCUMENTOS.activa = null;
    estadoPestana.ruta = null;
    estadoPestana.sucio = false;
    EDITOR.poner("");
    renderPestanas();
    actualizarAccionesFichero();
    const codigo = document.getElementById("codigo");
    if (codigo) codigo.placeholder = "Abre un fichero del explorador o crea un proyecto en Inicio.";
    nota("sin OPFS: " + (e && e.message ? e.message : e));
  }
}

if (typeof document !== "undefined") {
  document.addEventListener("DOMContentLoaded", () => {
    if (!document.getElementById("codigo")) return;
    // la página se puede recortar: cada botón se engancha solo si está en el DOM. El envoltorio
    // recoge las promesas rechazadas: un fallo de OPFS tiene que salir como aviso en la barra,
    // no como un unhandledrejection mudo con la página aparentemente viva.
    const mostrarFallo = (e) => {
      const texto = e && e.message ? e.message : String(e);
      nota("fallo: " + texto);
      console.error(e);
    };
    const en = (id, fn) => {
      const el = document.getElementById(id);
      if (!el) return;
      el.addEventListener("click", (e) => {
        try {
          const r = fn(e);
          if (r && typeof r.catch === "function") r.catch(mostrarFallo);
        } catch (err) {
          mostrarFallo(err);
        }
      });
    };
    EDITOR.inicializar();
    actualizarAccionesFichero();

    // soltar en el hueco del explorador mueve a la raíz del proyecto
    const cajaArbol = document.getElementById("arbol-pestana");
    if (cajaArbol && ARRASTRABLE) {
      cajaArbol.addEventListener("dragover", (e) => {
        if (e.target !== cajaArbol) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
        cajaArbol.classList.add("soltar-raiz");
      });
      cajaArbol.addEventListener("dragleave", (e) => {
        if (e.target === cajaArbol) cajaArbol.classList.remove("soltar-raiz");
      });
      cajaArbol.addEventListener("drop", (e) => {
        if (e.target !== cajaArbol) return;
        e.preventDefault();
        cajaArbol.classList.remove("soltar-raiz");
        const origen = e.dataTransfer.getData("text/plain");
        if (origen) moverRuta(origen, PROYECTO);
      });
    }

    const selectorApi = document.getElementById("api-version");
    if (selectorApi) {
      try {
        const pref = localStorage.getItem(API_PREF);
        if (pref && API_NOMBRES[pref]) selectorApi.value = pref;
      } catch (e) { /* sin preferencia */ }
      selectorApi.addEventListener("change", () => sincronizarApi(true));
      sincronizarApi(false);
    }

    const menuEditor = document.getElementById("menu-editor");
    if (menuEditor) {
      // cualquier acción del menú lo cierra; el <details> no guarda estado en JS
      menuEditor.addEventListener("click", (e) => {
        if (e.target.closest(".menu-editor-lista")) menuEditor.open = false;
      });
    }
    document.addEventListener("pointerdown", (e) => {
      const menu = document.getElementById("menu-archivo");
      if (menu && !menu.hidden && !menu.contains(e.target)) cerrarMenuArchivo();
      if (menuEditor && menuEditor.open && !menuEditor.contains(e.target)) menuEditor.open = false;
    });
    document.addEventListener("keydown", (e) => {
      if (e.key === "Escape") {
        cerrarMenuArchivo();
        if (menuEditor) menuEditor.open = false;
      }
    });
    window.addEventListener("scroll", cerrarMenuArchivo, true);
    window.addEventListener("resize", cerrarMenuArchivo);

    // El jar es de Google: no lo distribuimos ni lo descargamos por el usuario. El diálogo es un
    // acuse informado; los términos se aceptan en el sitio de Google al abrir sus enlaces.
    const dialogoSdk = document.getElementById("dialogo-sdk");
    const entiendo = document.getElementById("sdk-entiendo");
    const enlaceTerminos = document.getElementById("sdk-terminos");
    const enlaceDescarga = document.getElementById("sdk-descarga");
    const usarServidor = document.getElementById("sdk-usar-servidor");
    if (dialogoSdk && entiendo) {
      const reflejarAcuse = () => {
        const listo = !!entiendo.checked;
        for (const enlace of [enlaceTerminos, enlaceDescarga]) {
          if (!enlace) continue;
          enlace.setAttribute("aria-disabled", String(!listo));
          if (listo) enlace.removeAttribute("tabindex");
          else enlace.tabIndex = -1;
        }
      };
      const abrirDialogo = () => {
        entiendo.checked = false;
        reflejarAcuse();
        if (usarServidor) usarServidor.hidden = !JAR_SDK.disponible;
        if (typeof dialogoSdk.showModal === "function") dialogoSdk.showModal();
        else dialogoSdk.hidden = false;
        entiendo.focus();
      };
      const cerrarDialogo = () => {
        if (typeof dialogoSdk.close === "function") dialogoSdk.close();
        else dialogoSdk.hidden = true;
      };
      entiendo.addEventListener("change", reflejarAcuse);
      for (const enlace of [enlaceTerminos, enlaceDescarga]) {
        if (!enlace) continue;
        enlace.addEventListener("click", (e) => {
          if (!entiendo.checked) {
            e.preventDefault();
            nota("marca el acuse del SDK antes de abrir la página de Google");
            entiendo.focus();
          }
        });
      }
      en("abrir-dialogo-sdk", abrirDialogo);
      en("sdk-cerrar", cerrarDialogo);
      en("sdk-elegir-archivo", () => {
        cerrarDialogo();
        const entrada = document.getElementById("android-jar");
        if (entrada) entrada.click();
      });
      if (usarServidor) {
        en("sdk-usar-servidor", () => {
          cerrarDialogo();
          Promise.resolve(traerJarDelSdk(false)).catch((e) => nota(String(e && e.message || e)));
        });
      }
      reflejarAcuse();
    }

    en("crear-clave-release", abrirDialogoClave);
    en("clave-cancelar", cerrarDialogoClave);
    en("clave-generar", generarClaveRelease);
    en("clave-terminar", cerrarDialogoClave);
    const avisoClave = document.getElementById("clave-aviso");
    const generarClave = document.getElementById("clave-generar");
    if (avisoClave && generarClave) {
      avisoClave.addEventListener("change", () => {
        generarClave.disabled = !avisoClave.checked;
      });
    }
    const guardadaClave = document.getElementById("clave-guardada");
    const terminarClave = document.getElementById("clave-terminar");
    if (guardadaClave && terminarClave) {
      guardadaClave.addEventListener("change", () => {
        terminarClave.disabled = !guardadaClave.checked;
      });
    }

    for (const boton of document.querySelectorAll(".panel-tab")) {
      boton.addEventListener("click", () => mostrarPanel(boton.dataset.panel));
    }
    en("alternar-panel", () => {
      const panel = document.getElementById("panel-inferior");
      if (panel && panel.hidden) mostrarPanel("compilar");
      else cerrarPanel();
    });
    en("cerrar-panel", cerrarPanel);
    en("abrir-historial", () => mostrarPanel("historial"));
    en("conflicto-mio", () => mantenerVersionPropia(CONFLICTO.doc));
    en("conflicto-remoto", () => {
      const doc = CONFLICTO.doc;
      const datos = doc && doc.conflicto ? doc.conflicto : CONFLICTO.datos;
      if (doc && datos) aplicarVersionRemota(doc, datos);
    });
    en("conflicto-marcas", () => insertarMarcasConflicto(CONFLICTO.doc));
    en("historial-deshacer", deshacer);
    en("historial-rehacer", rehacer);
    en("historial-limpiar", limpiarHistorial);
    en("guardar-fichero", guardar);
    en("nuevo-fichero", () => nuevoFichero());
    en("subir-ficheros-boton", () => pedirSubida(carpetaActual(), false));
    en("exportar-proyecto", exportarProyecto);
    en("importar-proyecto", () => {
      const entrada = document.getElementById("importar-archivo");
      if (!entrada) { nota("este navegador no expone el selector de archivos"); return; }
      entrada.value = "";
      entrada.click();
    });
    const importarEntrada = document.getElementById("importar-archivo");
    if (importarEntrada) {
      importarEntrada.addEventListener("change", () => {
        importarProyecto(importarEntrada.files && importarEntrada.files[0]).catch(mostrarFallo);
      });
    }
    en("subir-arbol", () => pedirSubida(carpetaActual(), false));
    en("renombrar-fichero", () => estadoPestana.ruta
      ? renombrarRuta(estadoPestana.ruta)
      : nota("abre un fichero antes de renombrarlo"));
    en("borrar-fichero", () => estadoPestana.ruta
      ? borrarRuta(estadoPestana.ruta)
      : nota("abre un fichero antes de borrarlo"));

    // los dos selectores de archivo: uno normal y otro de carpeta (webkitdirectory conserva
    // los subdirectorios en webkitRelativePath)
    for (const id of ["subir-ficheros", "subir-carpeta"]) {
      const entrada = document.getElementById(id);
      if (entrada) entrada.addEventListener("change", () => {
        subirSeleccion(entrada).catch(mostrarFallo);
      });
    }
    en("firmar-aqui", firmar);
    en("crear-linaje", crearLinaje);
    en("olvidar-linaje", olvidarLinaje);
    en("ver-keystore-firmar", () => inspeccionarKeystore(KEYSTORES.firmar));
    en("olvidar-keystore-firmar", () => olvidarKeystore(KEYSTORES.firmar));
    en("ver-keystore-compilar", () => inspeccionarKeystore(KEYSTORES.compilar));
    en("olvidar-keystore-compilar", () => olvidarKeystore(KEYSTORES.compilar));
    en("usar-android-jar", async () => { await usarJarElegido(); });
    en("traer-android-jar", () => traerJarDelSdk(false));
    en("compilar-debug", compilarDebug);
    en("compilar-apk", compilarDelTodo);
    en("verificar-apk", verificarLoCompilado);
    en("descargar-debug-keystore", () => descargarRuta(DEBUG_KEY.ruta));
    en("leer-ajustes", () => leerAjustes());
    en("guardar-ajustes", guardarAjustes);
    en("detener-comando", detener);
    const consolaPestana = document.getElementById("consola-pestana");
    if (consolaPestana) {
      consolaPestana.addEventListener("click", (e) => {
        const enlace = e.target && e.target.closest ? e.target.closest("a.salto") : null;
        if (!enlace) return;
        e.preventDefault();
        saltarALinea(enlace.dataset.ruta, enlace.dataset.linea).catch(mostrarFallo);
      });
    }
    const filtroArbol = document.getElementById("filtro-arbol");
    if (filtroArbol) {
      filtroArbol.addEventListener("input", () => { refrescarArbol().catch(() => {}); });
    }
    en("buscar-siguiente", () => buscarSiguiente(false));
    en("buscar-anterior", () => buscarSiguiente(true));
    en("reemplazar-uno", reemplazarUno);
    en("reemplazar-todos", reemplazarTodos);
    en("buscar-cerrar", cerrarBusqueda);
    const campoBuscar = document.getElementById("buscar-texto");
    if (campoBuscar) {
      campoBuscar.addEventListener("keydown", (e) => {
        if (e.key === "Enter") { e.preventDefault(); buscarSiguiente(e.shiftKey); }
      });
    }
    const detalleAjustes = document.getElementById("detalle-ajustes");
    if (detalleAjustes) {
      detalleAjustes.addEventListener("toggle", () => {
        if (detalleAjustes.open) leerAjustes(true).catch(() => {});
      });
    }
    en("plegar-arbol", plegarTodo);
    en("desplegar-arbol", desplegarTodo);
    en("recargar-arbol", refrescarArbol);
    // Los enlaces de la barra ya llevan un href blob cuando están visibles: se deja que el
    // navegador siga el enlace. Interceptarlos y llamar a descargar() otra vez solo preparaba
    // el blob y no descargaba nada.
    en("descargar-fichero", (e) => {
      if (!estadoPestana.ruta) { e.preventDefault(); nota("no hay un fichero abierto"); return; }
      const enlace = e.currentTarget;
      // el buffer abierto, no lo último guardado: si hay cambios sin guardar, se descargan igual
      ponerBlob(enlace, EDITOR.texto(), "text/plain;charset=utf-8");
      enlace.download = estadoPestana.ruta.split("/").pop();
    });

    // Los atajos no dependen de que el foco esté en el textarea: Ctrl+S guarda también con el
    // cursor en el panel, y Ctrl+` cierra el panel desde cualquier sitio.
    document.addEventListener("keydown", (e) => {
      if (e.isComposing || !(e.ctrlKey || e.metaKey) || e.altKey) return;
      const tecla = (e.key || "").toLowerCase();
      if (tecla === "s") { e.preventDefault(); Promise.resolve(guardar()).catch(mostrarFallo); }
      else if (tecla === "p") {
        e.preventDefault();
        const filtro = document.getElementById("filtro-arbol");
        if (filtro) { filtro.focus(); filtro.select(); }
      }
      else if (tecla === "f") { e.preventDefault(); abrirBusqueda(false); }
      else if (tecla === "h") { e.preventDefault(); abrirBusqueda(true); }
      else if (tecla === "g") { e.preventDefault(); irALineaDesdeCuadro(); }
      else if (e.key === "Enter") { e.preventDefault(); Promise.resolve(ejecutarUltimo()).catch(mostrarFallo); }
      else if (e.key === "`" || e.code === "Backquote") { e.preventDefault(); alternarPanel(); }
    });
    document.addEventListener("keydown", (e) => {
      if (e.key !== "Escape") return;
      const barra = document.getElementById("buscar-barra");
      if (barra && !barra.hidden) { e.preventDefault(); cerrarBusqueda(); }
    });
    window.addEventListener("beforeunload", (e) => {
      volcarHistorial();
      if (!hayDocumentosSucios()) return;
      e.preventDefault();
      e.returnValue = "";
    });

    decirEstadoJar();
    decirEstadoClave();
    decirEstadoDepuracion();
    consultarJarDelSdk();
    iniciarPestana();
  });
}

export {
  PROYECTO, hospedador, estadoPestana, DOCUMENTOS, VENTANA, ARBOL,
  EDITOR, CONSOLA, abrir, guardar, correr, compilarDelTodo, compilarDebug,
  mostrarPanel, alternarPanel, marcarSucio, deshacer, rehacer,
  refrescarArbol, estaSucio, hayDocumentosSucios, confirmarDescartes,
  limpiarEstadoEditor, ponerJarDeUrl, usarJarElegido, cerrarMenuArchivo,
  pedirSubida, subirSeleccion, renderHistorial, restaurarNodoHistorial,
  limpiarHistorial, resaltar, esXml, textoHtml, consolaHtml,
  jarDeSeleccion, extraerJarDeZip, sincronizarApi, moverRuta, revelarRuta,
  verificarLoCompilado, inspeccionarKeystore, crearLinaje, guardarPersistencia,
  documentoActivo, documentoDe, volcarHistorial, programarHistorial,
  aplicarNodoHistorial, historialDe, conCarga, mostrarCarga, ocultarCarga,
  pintarConsola, KEYSTORES, DEBUG_KEY, RUTA_JAR, RUTA_LINAJE, RUTA_VIEJO,
  firmar, asegurarDebugKeystore, compilarCon, nuevoFichero,
  renombrarRuta, borrarRuta, abrirMenuArchivo, decirEstadoJar,
  consultarJarDelSdk, traerJarDelSdk, asegurarJarDelProyecto,
  conBloqueo, publicar, recibirRemoto, decirEstadoClave,
  actualizarEstadoClave, abrirDialogoClave, generarClaveRelease,
  exportarProyecto, importarProyecto, descargarBytes,
  leerAjustes, guardarAjustes, irALinea, saltarALinea, detener, enlazarSalidas,
  decirEstadoDepuracion, abrirBusqueda, cerrarBusqueda, buscarSiguiente,
  reemplazarUno, reemplazarTodos, irALineaDesdeCuadro,
};

// Puente de compatibilidad para las pruebas de navegador y la consola de desarrollo.
// El código de la página importa los símbolos; estos globales no son la fuente de verdad.
if (typeof window !== "undefined") {
  Object.assign(window, {
    PROYECTO, hospedador, estadoPestana, DOCUMENTOS, VENTANA, ARBOL,
    EDITOR, CONSOLA, abrir, guardar, correr, compilarDelTodo, compilarDebug,
    mostrarPanel, alternarPanel, marcarSucio, deshacer, rehacer,
    refrescarArbol, estaSucio, hayDocumentosSucios, confirmarDescartes,
    limpiarEstadoEditor, ponerJarDeUrl, usarJarElegido, cerrarMenuArchivo,
    pedirSubida, subirSeleccion, renderHistorial, restaurarNodoHistorial,
    limpiarHistorial, resaltar, esXml, textoHtml, consolaHtml,
    jarDeSeleccion, extraerJarDeZip, sincronizarApi, Historial, moverRuta,
    revelarRuta, verificarLoCompilado, inspeccionarKeystore, crearLinaje,
    guardarPersistencia, documentoActivo, documentoDe, volcarHistorial,
    programarHistorial, aplicarNodoHistorial, historialDe, conCarga,
    mostrarCarga, ocultarCarga, pintarConsola, KEYSTORES, DEBUG_KEY,
    RUTA_JAR, RUTA_LINAJE, RUTA_VIEJO, firmar,
    asegurarDebugKeystore, compilarCon, nuevoFichero, renombrarRuta,
    borrarRuta, abrirMenuArchivo, decirEstadoJar, consultarJarDelSdk,
    traerJarDelSdk, asegurarJarDelProyecto, conBloqueo, publicar, recibirRemoto,
    decirEstadoClave, actualizarEstadoClave, abrirDialogoClave, generarClaveRelease,
    exportarProyecto, importarProyecto, descargarBytes,
    leerAjustes, guardarAjustes, irALinea, saltarALinea, detener, enlazarSalidas,
    decirEstadoDepuracion, abrirBusqueda, cerrarBusqueda, buscarSiguiente,
    reemplazarUno, reemplazarTodos, irALineaDesdeCuadro,
  });
  // Superficie completa para las pruebas CDP, que evalúan los nombres antiguos. No se usa desde
  // el código de la página; los imports son la vía normal.
  Object.assign(window, {
    actualizarCursor, relativa, carpetaDe, carpetaActual, nombreDeSubida,
    pedirSubida, subirSeleccion, hayDocumentosSucios, leerPersistencia,
    falloDocumento, renderPestanas, pintarMarcaSucio, guardarEnMemoria,
    historialDe, asegurarHistorial, volcarHistorial, programarHistorial,
    aplicarNodoHistorial, restaurarNodoHistorial, limpiarHistorial,
    renderHistorial, limpiarSinDocumento, quitarDocumento, activarDocumento,
    cerrarDocumento, remapearDocumentos, quitarDocumentosBajo,
    limpiarEstadoEditor, conectarMenu, remapearAbiertos, plegarTodo,
    desplegarTodo, pedirCarpetaDestino, actualizarAccionesFichero,
    paqueteDelFormulario, normalizarRuta, nuevoNombre, cerrarPanel,
    ejecutarUltimo, nota, descargarRuta, ponerBlob, descargar,
    ofrecerDescarga, estadoKeystore, guardarKeystoreElegido,
    inspeccionarKeystore, prepararKeystore, olvidarKeystore, estadoLinaje,
    olvidarLinaje, iniciarPestana, PERSISTENCIA, SUBIDA, API_NOMBRES,
    API_PREF, JAR_SDK, RUTA_KEYSTORE, ARRASTRABLE,
  });
}
