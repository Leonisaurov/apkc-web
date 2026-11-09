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

// --- el editor, sin dependencias ---------------------------------------------------------
// Un textarea encima de un <pre> resaltado: es la única forma de tener color sin arrastrar
// un editor de terceros, y el precio es cuidar que las dos capas metricen igual (misma fuente,
// mismo whitespace, mismo scroll).
const EDITOR = {
  inicializar() {
    const area = document.getElementById("codigo");
    if (!area) return;
    area.addEventListener("input", () => { EDITOR.pintar(); EDITOR.sincronizar(); marcarSucio(true); });
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
        return;
      }
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") { e.preventDefault(); guardar(); }
      if ((e.ctrlKey || e.metaKey) && e.key === "Enter") { e.preventDefault(); ejecutarUltimo(); }
      if ((e.ctrlKey || e.metaKey) && (e.key === "`" || e.code === "Backquote")) { e.preventDefault(); alternarPanel(); }
    });
    EDITOR.pintar();
  },

  texto() { return document.getElementById("codigo").value; },

  poner(texto) {
    const area = document.getElementById("codigo");
    area.value = texto;
    area.scrollTop = 0;
    area.scrollLeft = 0;
    EDITOR.pintar();
    EDITOR.sincronizar();
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

  sincronizar() {
    const area = document.getElementById("codigo");
    const resaltado = document.getElementById("resalto");
    const numeros = document.getElementById("numeros");
    if (!area) return;
    if (resaltado) resaltado.style.transform = `translate(${-area.scrollLeft}px, ${-area.scrollTop}px)`;
    if (numeros) numeros.style.transform = `translateY(${-area.scrollTop}px)`;
  },

  // Tab y Shift+Tab sobre una o varias líneas: es lo mínimo que se espera de un editor de
  // código y lo que el textarea no trae de fábrica.
  indentar(delta) {
    const area = document.getElementById("codigo");
    const v = area.value;
    const a = area.selectionStart, b = area.selectionEnd;
    const ini = v.lastIndexOf("\n", a - 1) + 1;
    let fin = v.indexOf("\n", b);
    if (fin === -1) fin = v.length;
    const lineas = v.slice(ini, fin).split("\n");
    let cambio = 0;
    const nuevas = lineas.map((linea) => {
      if (delta > 0) {
        cambio += 4;
        return "    " + linea;
      }
      const m = /^( {1,4}|\t)/.exec(linea);
      if (!m) return linea;
      cambio -= m[0].length;
      return linea.slice(m[0].length);
    });
    if (!cambio) return;
    area.value = v.slice(0, ini) + nuevas.join("\n") + v.slice(fin);
    if (a === b) {
      area.selectionStart = area.selectionEnd = Math.max(ini, a + cambio);
    } else {
      area.selectionStart = Math.max(ini, a + (delta > 0 ? 4 : 0));
      area.selectionEnd = Math.max(area.selectionStart, b + cambio);
    }
    EDITOR.pintar();
    EDITOR.sincronizar();
    marcarSucio(true);
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
    area.value = v.slice(0, a) + insertar + v.slice(b);
    area.selectionStart = area.selectionEnd = a + insertar.length;
    EDITOR.pintar();
    EDITOR.sincronizar();
    marcarSucio(true);
  },
};

function esXml(texto) { return /<\?xml|<manifest|<LinearLayout|<[A-Za-z][\w.]*\s*>/.test(texto.slice(0, 400)); }

const PALABRAS_JAVA = "package|import|public|private|protected|class|extends|implements|interface|void|return|if|else|for|while|switch|case|new|this|super|static|final|int|long|float|double|boolean|byte|char|try|catch|finally|throw|throws|null|true|false|@Override";

// Un solo pasado de regex por línea, no una cadena de replace: encadenar replace sobre el HTML
// que el paso anterior ya inyectó coloreaba los nombres de las propias clases CSS (`class`
// dentro de `<span class=...>`), que es exactamente el fallo que hacía del resaltador un
// decorado roto en cuanto había un comentario.
// Los literales no cruzan de línea: si un fichero se queda sin cerrar una comilla, un `[^]*?`
// se comería el resto del archivo y el editor quedaría gris de ahí en adelante.
const PAST_JAVA = new RegExp("(\\/\\/[^\\n]*|/\\*[^]*?\\*/|&quot;[^\\n]*?&quot;)|\\b(" + PALABRAS_JAVA + ")\\b", "g");
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
  return escapado.replace(PAST_JAVA, (todo, literal, palabra) => {
    if (literal) {
      const clase = literal.startsWith("&quot;") ? "cadena" : "comentario";
      return '<span class="' + clase + '">' + literal + "</span>";
    }
    if (palabra) return '<span class="palabra">' + palabra + "</span>";
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
        fila.onclick = (e) => { e.preventDefault(); abrir(n.ruta); };
      }
      fuera.appendChild(fila);
      if (n.dir && !ARBOL.plegados.has(n.ruta)) pintar(n.ruta, profundidad + 1);
    }
  };
  pintar(PROYECTO, 0);
  fuera.scrollTop = scroll;
}

async function abrir(ruta) {
  if (estaSucio() && !(await confirmarDescartes())) return;
  const r = await hospedador.leer(ruta);
  const bytes = r.bytes;
  const texto = bytes ? new TextDecoder().decode(bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes)) : "";
  estadoPestana.ruta = ruta;
  EDITOR.poner(texto);
  const nombre = document.getElementById("fichero-abierto");
  if (nombre) {
    nombre.textContent = relativa(ruta) || ruta;
    nombre.title = ruta;
  }
  await refrescarArbol();
}

function marcarSucio(v) {
  estadoPestana.sucio = v;
  const marca = document.getElementById("marca-sucio");
  if (marca) marca.hidden = !v;
}
function estaSucio() { return !!estadoPestana.sucio; }
function confirmarDescartes() { return window.confirm("Hay cambios sin guardar. ¿Descartarlos?"); }

async function guardar() {
  if (!estadoPestana.ruta) {
    const nombre = await nuevoNombre();
    if (!nombre) return;
    estadoPestana.ruta = nombre;
  }
  await hospedador.escribir(estadoPestana.ruta, new TextEncoder().encode(EDITOR.texto()));
  marcarSucio(false);
  const abierto = document.getElementById("fichero-abierto");
  if (abierto) {
    abierto.textContent = relativa(estadoPestana.ruta);
    abierto.title = estadoPestana.ruta;
  }
  nota("guardado en " + estadoPestana.ruta);
  await refrescarArbol();
}

async function nuevoNombre() {
  const entrada = prompt("Ruta dentro del proyecto", PROYECTO + "/src/com/hola/MainActivity.java");
  return entrada || null;
}

// --- el panel inferior -------------------------------------------------------------------
function mostrarPanel(nombre) {
  const panel = document.getElementById("panel-inferior");
  if (!panel) return;
  panel.hidden = false;
  for (const b of document.querySelectorAll(".panel-tab")) b.classList.toggle("activo", b.dataset.panel === nombre);
  for (const v of document.querySelectorAll("#panel-inferior .panel-vista")) {
    v.classList.toggle("activa", v.id === "panel-" + nombre);
  }
  const alternar = document.getElementById("alternar-panel");
  if (alternar) alternar.setAttribute("aria-expanded", "true");
  const activa = document.getElementById("panel-" + nombre);
  if (activa) activa.scrollTop = 0;
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
  const consola = document.getElementById("consola-pestana");
  mostrarPanel("salida");
  consola.textContent += "$ apkc " + argv.join(" ") + "\n";
  document.getElementById("ocupado").hidden = false;
  const r = await hospedador.ejecutar(argv);
  document.getElementById("ocupado").hidden = true;
  consola.textContent += (r.consola || "") + (r.error ? "FALLO DEL HOSPEDADOR: " + r.error + "\n" : "");
  consola.scrollTop = consola.scrollHeight;
  if (r.tipo === "terminado") document.getElementById("ultimo-codigo").textContent = String(r.codigo);
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
  await hospedador.escribir(RUTA_JAR, datos);
  await decirEstadoJar();
  return datos.length;
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
    if (!(await usarJarElegido())) { nota("hace falta el android.jar del API 34 para compilar aquí"); return; }
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
    // la página se puede recortar: cada botón se engancha solo si está en el DOM
    const en = (id, fn) => {
      const el = document.getElementById(id);
      if (el) el.addEventListener("click", fn);
    };
    EDITOR.inicializar();
    for (const boton of document.querySelectorAll(".panel-tab")) {
      boton.addEventListener("click", () => mostrarPanel(boton.dataset.panel));
    }
    en("alternar-panel", () => alternarPanel());
    en("cerrar-panel", cerrarPanel);
    en("guardar-fichero", guardar);
    en("nuevo-fichero", async () => {
      const nombre = await nuevoNombre();
      if (!nombre) return;
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
    decirEstadoJar();
    iniciarPestana();
  });
}

if (typeof module !== "undefined") module.exports = { resaltar, esXml, textoHtml };
