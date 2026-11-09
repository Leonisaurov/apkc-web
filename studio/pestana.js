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
  let worker = null, próximo = 1;
  const pendientes = new Map();
  const asegurar = () => {
    if (worker) return worker;
    worker = new Worker("hospedador.js", { type: "module" });   // module: el hospedador importa su fs con import
    worker.onmessage = (e) => {
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
    cwd: async (ruta) => { await bajarRaiz(); return pedir({ tipo: "cwd", ruta }); },
    sondear: async () => { await bajarRaiz(); return pedir({ tipo: "sondeo" }); },
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
    });
    area.addEventListener("scroll", EDITOR.sincronizar);
    area.addEventListener("keydown", (e) => {
      if (e.isComposing) return;
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

// Un fuente Java puede llevar genéricos (`List<String>`, `Map<K, V>`) y la heurística vieja
// los tomaba por etiquetas XML: el fichero entero se pintaba con las reglas equivocadas. Lo
// que distingue a un recurso Android es que, quitando BOM, espacios y comentarios, empieza
// por una etiqueta; un .java empieza por package, import, un comentario o una anotación.
function esXml(texto) {
  const t = texto.slice(0, 400);
  const limpio = t.replace(/^\uFEFF/, "").trimStart().replace(/^(?:<!--[^]*?-->\s*)+/, "");
  if (/^<\?xml/i.test(limpio)) return true;
  return /^<\/?[A-Za-z][\w.:-]*/.test(limpio);
}

const PALABRAS_JAVA = "abstract|assert|break|byte|case|catch|char|class|const|continue|default|do|double|else|enum|extends|final|finally|float|for|goto|if|implements|import|instanceof|int|interface|long|native|new|package|private|protected|public|record|return|sealed|short|static|strictfp|super|switch|synchronized|this|throw|throws|transient|try|var|void|volatile|while|yield|null|true|false";

// Un solo pasado de regex por línea, no una cadena de replace: encadenar replace sobre el HTML
// que el paso anterior ya inyectó coloreaba los nombres de las propias clases CSS (`class`
// dentro de `<span class=...>`), que es exactamente el fallo que hacía del resaltador un
// decorado roto en cuanto había un comentario.
// Los literales no cruzan de línea: si un fichero se queda sin cerrar una comilla, un `[^]*?`
// se comería el resto del archivo y el editor quedaría gris de ahí en adelante.
const PAST_JAVA = new RegExp(
  "(\\/\\/[^\\n]*|/\\*[^]*?\\*/|&quot;[^\\n]*?&quot;|'[^'\\n]*')" +
  "|(@[A-Za-z_]\\w*)" +
  "|\\b(" + PALABRAS_JAVA + ")\\b" +
  "|\\b(0[xX][0-9a-fA-F]+|\\d+(?:\\.\\d+)?[fFdDlL]?)\\b", "g");
const PAST_XML = /(&lt;!--[^]*?--&gt;)|(&lt;\/?)([\w.:-]+)|(&quot;[^\n]*?&quot;)|([\w.:-]+)(?==)/g;

function resaltar(texto, modo) {
  const escapado = textoHtml(texto);
  if (modo === "xml") {
    return escapado.replace(PAST_XML, (todo, comentario, apertura, etiqueta, valor, atributo) => {
      if (comentario) return '<span class="comentario">' + comentario + "</span>";
      if (etiqueta !== undefined) return apertura + '<span class="etiqueta">' + etiqueta + "</span>";
      if (valor) return '<span class="cadena">' + valor + "</span>";
      if (atributo) return '<span class="atributo">' + atributo + "</span>";
      return todo;
    });
  }
  return escapado.replace(PAST_JAVA, (todo, literal, anotacion, palabra, numero) => {
    if (literal) {
      const clase = literal.startsWith("&quot;") || literal.startsWith("'") ? "cadena" : "comentario";
      return '<span class="' + clase + '">' + literal + "</span>";
    }
    if (anotacion) return '<span class="anotacion">' + anotacion + "</span>";
    if (palabra) return '<span class="palabra">' + palabra + "</span>";
    if (numero) return '<span class="numero">' + numero + "</span>";
    return todo;
  });
}

function textoHtml(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

// --- el árbol y los ficheros --------------------------------------------------------------
const ARBOL = { plegados: new Set() };

function relativa(ruta) {
  return ruta === PROYECTO ? "" : (ruta.startsWith(PROYECTO + "/") ? ruta.slice(PROYECTO.length + 1) : ruta);
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
  const scroll = fuera.scrollTop;
  fuera.textContent = "";
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
  for (const n of nodos) {
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
      const fila = document.createElement("a");
      fila.href = "#";
      fila.dataset.ruta = n.ruta;
      fila.title = relativa(n.ruta);
      fila.style.paddingLeft = (8 + profundidad * 12) + "px";
      fila.className = n.dir ? "dir" : "archivo";
      if (n.ruta === estadoPestana.ruta) fila.classList.add("activo");

      const flecha = document.createElement("span");
      flecha.className = "arbol-flecha " +
        (n.dir ? (ARBOL.plegados.has(n.ruta) ? "" : "abierta") : "hoja");
      fila.appendChild(flecha);

      const icono = document.createElement("span");
      icono.className = "arbol-icono " + (n.dir ? "dir" : "archivo");
      fila.appendChild(icono);

      const nombre = document.createElement("span");
      nombre.className = "nombre";
      nombre.textContent = n.nombre;
      fila.appendChild(nombre);

      if (n.dir) {
        fila.onclick = (e) => {
          e.preventDefault();
          if (ARBOL.plegados.has(n.ruta)) ARBOL.plegados.delete(n.ruta);
          else ARBOL.plegados.add(n.ruta);
          refrescarArbol();
        };
      } else {
        fila.onclick = async (e) => {
          e.preventDefault();
          if (await abrir(n.ruta)) {
            const vista = document.querySelector('[data-vista="pestana"]');
            if (vista && document.body.dataset.vista !== "pestana") vista.click();
          }
        };
      }
      fuera.appendChild(fila);
      if (n.dir && !ARBOL.plegados.has(n.ruta)) pintar(n.ruta, profundidad + 1);
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

async function abrir(ruta, yaConfirmado) {
  if (!yaConfirmado && estaSucio() && !(await confirmarDescartes())) return false;
  let texto;
  try {
    const bytes = await hospedador.bytes(ruta);
    if (!bytes) { nota("no existe " + ruta); return false; }
    // un PNG del árbol no se abre como si fuera fuente: guardarlo después lo corrompería
    if (bytes.includes(0)) { nota(ruta + " parece un binario; el editor solo abre texto"); return false; }
    texto = new TextDecoder().decode(bytes);
  } catch (e) {
    nota("no se pudo abrir " + ruta + ": " + (e && e.message ? e.message : e));
    return false;
  }
  estadoPestana.ruta = ruta;
  EDITOR.poner(texto);
  const nombre = document.getElementById("fichero-abierto");
  if (nombre) {
    nombre.textContent = relativa(ruta) || ruta;
    nombre.title = ruta;
  }
  await refrescarArbol();
  return true;
}

function marcarSucio(v) {
  estadoPestana.sucio = v;
  const marca = document.getElementById("marca-sucio");
  if (marca) marca.hidden = !v;
}
function estaSucio() { return !!estadoPestana.sucio; }
function confirmarDescartes() { return window.confirm("Hay cambios sin guardar. ¿Descartarlos?"); }

async function guardar() {
  let recien = false;
  if (!estadoPestana.ruta) {
    const nombre = await nuevoNombre();
    if (!nombre) return false;
    estadoPestana.ruta = nombre;
    recien = true;
  }
  if (recien) {
    const previo = await hospedador.bytes(estadoPestana.ruta);
    if (previo !== null && !window.confirm("Ya existe " + relativa(estadoPestana.ruta) + ". ¿Reemplazarlo?")) {
      estadoPestana.ruta = null;
      return false;
    }
  }
  const guardado = EDITOR.texto();
  try {
    await hospedador.escribir(estadoPestana.ruta, new TextEncoder().encode(guardado));
  } catch (e) {
    nota("no se pudo guardar " + estadoPestana.ruta + ": " + (e && e.message ? e.message : e));
    return false;
  }
  // si se siguió escribiendo mientras el worker guardaba, el fichero tiene lo viejo y el
  // documento sigue sucio; marcarlo limpio perdería esos cambios al cerrar la pestaña
  if (EDITOR.texto() === guardado) marcarSucio(false);
  const abierto = document.getElementById("fichero-abierto");
  if (abierto) {
    abierto.textContent = relativa(estadoPestana.ruta);
    abierto.title = estadoPestana.ruta;
  }
  nota("guardado en " + estadoPestana.ruta);
  await refrescarArbol();
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

async function nuevoNombre() {
  const sugerida = PROYECTO + "/src/" + paqueteDelFormulario().replace(/\./g, "/") + "/MainActivity.java";
  const entrada = prompt("Ruta dentro del proyecto", sugerida);
  if (entrada === null) return null;
  let ruta = entrada.trim().replace(/\\/g, "/").replace(/\/{2,}/g, "/");
  if (!ruta.startsWith("/")) ruta = PROYECTO + "/" + ruta;
  ruta = normalizarRuta(ruta);
  if (!ruta.startsWith(PROYECTO + "/")) return null;
  return ruta;
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
  const consola = document.getElementById("consola-pestana");
  mostrarPanel("salida");
  if (consola) consola.textContent += "$ apkc " + argv.join(" ") + "\n";
  const ocupado = document.getElementById("ocupado");
  if (ocupado) ocupado.hidden = false;
  let r;
  try {
    r = await hospedador.ejecutar(argv);
  } catch (e) {
    // si el worker no arranca, el aviso de «trabajando» no puede quedarse para siempre
    r = { tipo: "error", error: String((e && e.message) || e), consola: "" };
  } finally {
    if (ocupado) ocupado.hidden = true;
  }
  if (consola) {
    consola.textContent += (r.consola || "") + (r.error ? "FALLO DEL HOSPEDADOR: " + r.error + "\n" : "");
    if (consola.textContent.length > 200000) consola.textContent = consola.textContent.slice(-200000);
    consola.scrollTop = consola.scrollHeight;
  }
  if (r.tipo === "terminado") {
    const codigo = document.getElementById("ultimo-codigo");
    if (codigo) codigo.textContent = String(r.codigo);
  }
  await refrescarArbol();
  return r;
}

function nota(texto) {
  const n = document.getElementById("estado-pestana");
  n.textContent = texto;
}

async function crearProyecto() {
  await correr(["create", PROYECTO]);
  const r = await hospedador.leer(PROYECTO + "/AndroidManifest.xml");
  if (r.bytes) await abrir(PROYECTO + "/AndroidManifest.xml");
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
  if (r && r.codigo === 0) {
    const v = await correr(["verify", destino]);
    document.getElementById(cfg.pass).value = "";
    if (document.getElementById("storepass-viejo")) document.getElementById("storepass-viejo").value = "";
    await descargar(destino);
    await ofrecerDescarga(destino + ".idsig", "descargar-idsig");
    if (v && v.codigo === 0) nota("firmado y verificado: " + destino.split("/").pop());
  }
}

async function descargar(ruta, ancla) {
  const r = await hospedador.leer(ruta);
  if (!r.bytes) { nota("no hay nada en " + ruta); return; }
  const bytes = r.bytes instanceof Uint8Array ? r.bytes : new Uint8Array(r.bytes);
  const enlace = document.getElementById(ancla || "descargar-apk");
  const tipo = /\.aab$/i.test(ruta) ? "application/octet-stream" : "application/vnd.android.package-archive";
  enlace.href = URL.createObjectURL(new Blob([bytes], { type: tipo }));
  enlace.download = ruta.split("/").pop();
  nota("descarga lista: " + enlace.download + " (" + bytes.length + " bytes)");
}

// El .idsig viaja al lado del apk. Si no existe (un .aab, o --v4=false) el enlace se
// esconde, para que no quede un botón que no lleva a nada.
async function ofrecerDescarga(ruta, ancla) {
  const enlace = document.getElementById(ancla);
  const r = await hospedador.leer(ruta);
  if (!r.bytes) { enlace.hidden = true; enlace.removeAttribute("href"); return; }
  const bytes = r.bytes instanceof Uint8Array ? r.bytes : new Uint8Array(r.bytes);
  enlace.href = URL.createObjectURL(new Blob([bytes], { type: "application/octet-stream" }));
  enlace.download = ruta.split("/").pop();
  enlace.hidden = false;
}

async function intentarCompilar() {
  // build no toma ruta: trabaja en el directorio del proceso, así que hay que meterse en el
  // proyecto antes; si no, apkc contesta "esto no es un proyecto apkc" y el muro no se ve
  await hospedador.cwd(PROYECTO);
  const r = await correr(["build"]);
  if (r && r.codigo !== 0) {
    document.getElementById("consola-pestana").textContent +=
      "\nEse fallo es el muro conocido: js/wasm no tiene fork/exec, así que aapt2/javac/d8 no pueden arrancar.\n" +
      "Lo que sí corre aquí entero es `build --linker go --compiler go`: el botón de arriba, que pide el android.jar.\n";
  }
}

// --- el build sin procesos: recursos, Java, dex y firma, en este mismo binario ------------
const RUTA_JAR = "/sdk/android.jar";
const RUTA_KEYSTORE = "/sdk/keystore.p12";
const RUTA_VIEJO = "/keystore-viejo";
const RUTA_LINAJE = "/lineage.bin";

// Un keystore por flujo: el del build queda en OPFS (`/sdk/keystore.p12`) y el de `sign` en
// `/keystore`, las rutas que ya usaban los botones. Lo nuevo es la contraseña, el alias que se
// elige entre los que el fichero declara, y el certificado que se enseña antes de firmar.
const KEYSTORES = {
  compilar: { fichero: "keystore-compilar", pass: "storepass-compilar", alias: "keyalias-compilar",
              estado: "estado-keystore-compilar", ruta: RUTA_KEYSTORE },
  firmar: { fichero: "keystore", pass: "storepass-firmar", alias: "keyalias-firmar",
            estado: "estado-keystore-firmar", ruta: "/keystore" },
};

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

async function ponerJarDeUrl(url) {
  // la ruta del servidor local, no de un CDN: la página solo habla con su propio origen, y eso es
  // lo que comprueba la prueba de la pestaña contando las peticiones
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
  if (!aviso) return;
  try {
    const respuesta = await fetch("/android-jar-info");
    if (!respuesta.ok) throw new Error("HTTP " + respuesta.status);
    const info = await respuesta.json();
    aviso.textContent = info.disponible
      ? "el servidor ve el android.jar del SDK local: API " + info.version + ", " + info.bytes + " bytes"
      : "el servidor no encontró un SDK local; elige el fichero o ejecuta tools/entorno.sh jar";
  } catch (e) {
    aviso.textContent = "este servidor no expone /android-jar-info; usa el selector de fichero";
  }
}

async function usarJarElegido() {
  const entrada = document.getElementById("android-jar");
  if (!entrada.files.length) { nota("elige el android.jar primero"); return false; }
  const datos = new Uint8Array(await entrada.files[0].arrayBuffer());
  await hospedador.escribir(RUTA_JAR, datos);
  await decirEstadoJar();
  nota("android.jar guardado en esta pestaña (" + datos.length + " bytes); no sube a ningún sitio");
  return true;
}

async function decirEstadoJar() {
  const aviso = document.getElementById("estado-jar");
  if (!aviso) return false;
  try {
    const r = await hospedador.leer(RUTA_JAR);
    if (r.bytes) {
      aviso.textContent = "hay un android.jar de " + r.bytes.length + " bytes en esta pestaña (" + RUTA_JAR + ")";
      return true;
    }
  } catch (e) {
    aviso.textContent = "no se puede leer OPFS en este contexto: " + (e && e.message ? e.message : e);
    return false;
  }
  aviso.textContent = "sin android.jar en esta pestaña todavía";
  return false;
}

async function compilarDelTodo() {
  if (!(await decirEstadoJar())) {
    const entrada = document.getElementById("android-jar");
    // si la persona ya eligió un fichero, ese manda; si no, se prueba el SDK local del origen
    if (entrada && entrada.files.length) {
      if (!(await usarJarElegido())) { nota("no se pudo leer el android.jar elegido"); return; }
    } else if (!(await traerJarDelSdk(true)) && !(await usarJarElegido())) {
      nota("hace falta el android.jar del API 34: no hay SDK local en el servidor y no elegiste fichero; ejecuta tools/entorno.sh jar o pásalo con el selector");
      return;
    }
  }
  const k = await prepararKeystore(KEYSTORES.compilar);
  if (!k) return;
  const v4 = document.getElementById("v4-compilar").checked;
  const version = (document.getElementById("sdk-version").value || "34").trim();
  const codename = (document.getElementById("codename").value || "14").trim();
  await hospedador.cwd(PROYECTO);
  const r = await correr(["build", "--linker", "go", "--compiler", "go",
    "--android-jar", RUTA_JAR, "--sdk-version", version, "--codename", codename,
    "--keystore", k.ruta, "--storepass", k.pass, "--keyalias", k.alias,
    "--v4=" + (v4 ? "true" : "false")]);
  if (r && r.codigo === 0) {
    document.getElementById(KEYSTORES.compilar.pass).value = "";
    await descargar(PROYECTO + "/build/app.apk", "descargar-apk-compilada");
    await ofrecerDescarga(PROYECTO + "/build/app.apk.idsig", "descargar-idsig-compilada");
    await verificarLoCompilado();
  }
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
  aviso.textContent = lineas.length ? lineas.join(" · ") : "apkc no devolvió ningún veredicto";
  return r;
}

// --- arranque ----------------------------------------------------------------------------
let listo = false;
async function iniciarPestana() {
  try {
    const r = await hospedador.arbol();
    listo = true;
    const aislacion = document.getElementById("aislacion");
    if (aislacion) aislacion.textContent = r.aislada ? "sí" : "no (hay que servir por http con COOP/COEP)";
    await refrescarArbol();
    const manifest = (r.nodos || []).find((n) => n.ruta === PROYECTO + "/AndroidManifest.xml");
    if (manifest) {
      await abrir(PROYECTO + "/AndroidManifest.xml");
      // si ya había proyecto en OPFS, se entra directamente al editor en vez de dejar a la
      // persona en Inicio mirando una pantalla que ya no le toca
      const editor = document.querySelector('[data-vista="pestana"]');
      if (editor) editor.click();
    } else {
      EDITOR.poner("");
      const abierto = document.getElementById("fichero-abierto");
      if (abierto) abierto.textContent = "Sin fichero";
      const codigo = document.getElementById("codigo");
      if (codigo) codigo.placeholder = "Abre un fichero del explorador o crea un proyecto en Inicio.";
    }
  } catch (e) {
    const aislacion = document.getElementById("aislacion");
    if (aislacion) aislacion.textContent = "no disponible";
    const abierto = document.getElementById("fichero-abierto");
    if (abierto) abierto.textContent = "Sin fichero";
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
    for (const boton of document.querySelectorAll(".panel-tab")) {
      boton.addEventListener("click", () => mostrarPanel(boton.dataset.panel));
    }
    en("alternar-panel", () => alternarPanel());
    en("cerrar-panel", cerrarPanel);
    en("guardar-fichero", guardar);
    en("nuevo-fichero", async () => {
      if (estaSucio() && !(await confirmarDescartes())) return;
      const nombre = await nuevoNombre();
      if (!nombre) return;
      const previo = await hospedador.bytes(nombre);
      if (previo !== null && !window.confirm("Ya existe " + relativa(nombre) + ". ¿Reemplazarlo?")) return;
      // el fichero nace vacío en OPFS para que el explorador y el estado «sin cambios» digan
      // la verdad: un buffer suelto que no está en el árbol confunde más que ayuda
      await hospedador.escribir(nombre, new Uint8Array(0));
      estadoPestana.ruta = nombre;
      EDITOR.poner("");
      const abierto = document.getElementById("fichero-abierto");
      if (abierto) {
        abierto.textContent = relativa(nombre);
        abierto.title = nombre;
      }
      const codigo = document.getElementById("codigo");
      if (codigo) codigo.focus();
      await refrescarArbol();
    });
    en("firmar-aqui", firmar);
    en("crear-linaje", crearLinaje);
    en("olvidar-linaje", olvidarLinaje);
    en("ver-keystore-firmar", () => inspeccionarKeystore(KEYSTORES.firmar));
    en("olvidar-keystore-firmar", () => olvidarKeystore(KEYSTORES.firmar));
    en("ver-keystore-compilar", () => inspeccionarKeystore(KEYSTORES.compilar));
    en("olvidar-keystore-compilar", () => olvidarKeystore(KEYSTORES.compilar));
    en("usar-android-jar", async () => { await usarJarElegido(); });
    en("traer-android-jar", () => traerJarDelSdk(false));
    en("compilar-apk", compilarDelTodo);
    en("verificar-apk", verificarLoCompilado);
    en("recargar-arbol", refrescarArbol);
    en("descargar-apk", (e) => {
      e.preventDefault();
      descargar((document.getElementById("apk-a-firmar").value || PROYECTO + "/build/firmado.apk").trim());
    });
    en("descargar-apk-compilada", (e) => {
      e.preventDefault();
      descargar(PROYECTO + "/build/app.apk", "descargar-apk-compilada");
    });

    // Los atajos no dependen de que el foco esté en el textarea: Ctrl+S guarda también con el
    // cursor en el panel, y Ctrl+` cierra el panel desde cualquier sitio.
    document.addEventListener("keydown", (e) => {
      if (e.isComposing || !(e.ctrlKey || e.metaKey) || e.altKey) return;
      const tecla = (e.key || "").toLowerCase();
      if (tecla === "s") { e.preventDefault(); Promise.resolve(guardar()).catch(mostrarFallo); }
      else if (e.key === "Enter") { e.preventDefault(); Promise.resolve(ejecutarUltimo()).catch(mostrarFallo); }
      else if (e.key === "`" || e.code === "Backquote") { e.preventDefault(); alternarPanel(); }
    });
    window.addEventListener("beforeunload", (e) => {
      if (!estaSucio()) return;
      e.preventDefault();
      e.returnValue = "";
    });

    decirEstadoJar();
    consultarJarDelSdk();
    iniciarPestana();
  });
}

if (typeof module !== "undefined") module.exports = { resaltar, esXml, textoHtml };
