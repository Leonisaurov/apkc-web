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
    const resaltado = document.getElementById("resalto");
    const numeros = document.getElementById("numeros");
    if (!area) return;
    area.addEventListener("input", () => { EDITOR.pintar(); EDITOR.sincronizar(); marcarSucio(true); });
    area.addEventListener("scroll", EDITOR.sincronizar);
    area.addEventListener("keydown", (e) => {
      if (e.key === "Tab") {
        e.preventDefault();
        const i = area.selectionStart, j = area.selectionEnd;
        area.value = area.value.slice(0, i) + "    " + area.value.slice(j);
        area.selectionStart = area.selectionEnd = i + 4;
        EDITOR.pintar(); marcarSucio(true);
      }
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") { e.preventDefault(); guardar(); }
      if ((e.ctrlKey || e.metaKey) && e.key === "Enter") { e.preventDefault(); ejecutarUltimo(); }
    });
    EDITOR.pintar();
  },

  texto() { return document.getElementById("codigo").value; },
  poner(texto) { document.getElementById("codigo").value = texto; EDITOR.pintar(); marcarSucio(false); },

  pintar() {
    const area = document.getElementById("codigo");
    const resaltado = document.getElementById("resalto");
    const valor = area.value;
    resaltado.innerHTML = resaltar(valor, esXml(valor) ? "xml" : "java") + "\n";
    EDITOR.numeros(valor);
  },

  numeros(valor) {
    const lineas = valor.split("\n").length;
    const ancho = String(lineas).length;
    const cuadro = document.getElementById("numeros");
    let fuera = "";
    for (let i = 1; i <= lineas; i++) fuera += String(i).padStart(ancho, " ") + "\n";
    cuadro.textContent = fuera;
  },

  sincronizar() {
    const area = document.getElementById("codigo");
    document.getElementById("resalto").style.transform =
      `translate(${-area.scrollLeft}px, ${-area.scrollTop}px)`;
    document.getElementById("numeros").style.transform = `translateY(${-area.scrollTop}px)`;
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
async function refrescarArbol() {
  const r = await hospedador.arbol();
  const fuera = document.getElementById("arbol-pestana");
  const prefijo = PROYECTO + "/";
  const nodos = (r.nodos || []).filter((n) => n.ruta.startsWith(prefijo) || n.ruta === PROYECTO);
  fuera.textContent = "";
  if (!nodos.length) {
    fuera.appendChild(document.createTextNode("(vacío: crea el proyecto o escribe un fuente nuevo)"));
    return;
  }
  for (const n of nodos) {
    const nombre = n.ruta === PROYECTO ? PROYECTO : n.ruta.slice(prefijo.length);
    const item = document.createElement("a");
    item.href = "#";
    item.textContent = nombre + (n.dir ? "/" : " (" + n.bytes + " b)");
    item.className = n.dir ? "dir" : "";
    item.onclick = (e) => { e.preventDefault(); if (!n.dir) abrir(n.ruta); };
    fuera.appendChild(item);
    fuera.appendChild(document.createElement("br"));
  }
}

async function abrir(ruta) {
  if (estaSucio() && !(await confirmarDescartes())) return;
  const r = await hospedador.leer(ruta);
  const bytes = r.bytes;
  const texto = bytes ? new TextDecoder().decode(bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes)) : "";
  estadoPestana.ruta = ruta;
  EDITOR.poner(texto);
  document.getElementById("fichero-abierto").textContent = ruta;
}

function marcarSucio(v) { estadoPestana.sucio = v; }
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
  nota("guardado en " + estadoPestana.ruta);
  await refrescarArbol();
}

async function nuevoNombre() {
  const entrada = prompt("Ruta dentro del proyecto", PROYECTO + "/src/com/hola/MainActivity.java");
  return entrada || null;
}

// --- los comandos ------------------------------------------------------------------------
const ULTIMO = { argv: ["create", PROYECTO] };

async function ejecutarUltimo() { await correr(ULTIMO.argv); }

async function correr(argv) {
  ULTIMO.argv = argv;
  const consola = document.getElementById("consola-pestana");
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
  nota("firmando " + origen + " con " + k.alias + " (no sale de esta pestaña)");
  // `sign` escribe `firmado.apk.tmp` junto al destino y no crea la carpeta. Si el proyecto no se
  // compiló nunca, build/ no existe: se crea con un fichero vacío (escribir crea los padres) y se
  // borra antes de firmar.
  await hospedador.escribir(PROYECTO + "/build/.carpeta", new Uint8Array(0));
  await hospedador.limpiar(PROYECTO + "/build/.carpeta");
  const r = await correr(["sign", "--keystore", k.ruta, "--storepass", k.pass,
    "--keyalias", k.alias, "--out", PROYECTO + "/build/firmado.apk", origen]);
  if (r && r.codigo === 0) {
    document.getElementById(cfg.pass).value = "";
    await descargar(PROYECTO + "/build/firmado.apk");
  }
}

async function descargar(ruta, ancla) {
  const r = await hospedador.leer(ruta);
  if (!r.bytes) { nota("no hay nada en " + ruta); return; }
  const bytes = r.bytes instanceof Uint8Array ? r.bytes : new Uint8Array(r.bytes);
  const enlace = document.getElementById(ancla || "descargar-apk");
  enlace.href = URL.createObjectURL(new Blob([bytes], { type: "application/vnd.android.package-archive" }));
  enlace.download = ruta.split("/").pop();
  nota("descarga lista: " + enlace.download + " (" + bytes.length + " bytes)");
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
  const r = await hospedador.leer(RUTA_JAR);
  if (r.bytes) {
    aviso.textContent = "hay un android.jar de " + r.bytes.length + " bytes en esta pestaña (" + RUTA_JAR + ")";
    return true;
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
  const version = (document.getElementById("sdk-version").value || "34").trim();
  const codename = (document.getElementById("codename").value || "14").trim();
  await hospedador.cwd(PROYECTO);
  const r = await correr(["build", "--linker", "go", "--compiler", "go",
    "--android-jar", RUTA_JAR, "--sdk-version", version, "--codename", codename,
    "--keystore", k.ruta, "--storepass", k.pass, "--keyalias", k.alias]);
  if (r && r.codigo === 0) {
    document.getElementById(KEYSTORES.compilar.pass).value = "";
    await descargar(PROYECTO + "/build/app.apk", "descargar-apk-compilada");
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
  const r = await hospedador.arbol();
  listo = true;
  document.getElementById("aislacion").textContent = r.aislada ? "sí" : "no (hay que servir por http con COOP/COEP)";
  await refrescarArbol();
  const manifest = (r.nodos || []).find((n) => n.ruta === PROYECTO + "/AndroidManifest.xml");
  if (manifest) await abrir(PROYECTO + "/AndroidManifest.xml");
  else EDITOR.poner("// Escribe aquí MainActivity.java y pulsa guardar,\n// o crea el proyecto con apkc en el botón de arriba.\n");
}

if (typeof document !== "undefined") {
  document.addEventListener("DOMContentLoaded", () => {
    if (!document.getElementById("codigo")) return;
    EDITOR.inicializar();
    document.getElementById("crear-aqui").addEventListener("click", crearProyecto);
    document.getElementById("guardar-fichero").addEventListener("click" , guardar);
    document.getElementById("nuevo-fichero").addEventListener("click", async () => {
      const nombre = await nuevoNombre();
      if (nombre) { estadoPestana.ruta = nombre; EDITOR.poner(""); document.getElementById("fichero-abierto").textContent = nombre; }
    });
    document.getElementById("firmar-aqui").addEventListener("click", firmar);
    document.getElementById("ver-keystore-firmar").addEventListener("click", () => inspeccionarKeystore(KEYSTORES.firmar));
    document.getElementById("olvidar-keystore-firmar").addEventListener("click", () => olvidarKeystore(KEYSTORES.firmar));
    document.getElementById("ver-keystore-compilar").addEventListener("click", () => inspeccionarKeystore(KEYSTORES.compilar));
    document.getElementById("olvidar-keystore-compilar").addEventListener("click", () => olvidarKeystore(KEYSTORES.compilar));
    document.getElementById("compilar-aqui").addEventListener("click", intentarCompilar);
    document.getElementById("usar-android-jar").addEventListener("click", async () => { await usarJarElegido(); });
    document.getElementById("compilar-apk").addEventListener("click", compilarDelTodo);
    document.getElementById("verificar-apk").addEventListener("click", verificarLoCompilado);
    document.getElementById("recargar-arbol").addEventListener("click", refrescarArbol);
    document.getElementById("descargar-apk").addEventListener("click", (e) => {
      e.preventDefault();
      descargar((document.getElementById("apk-a-firmar").value || PROYECTO + "/build/firmado.apk").trim());
    });
    document.getElementById("descargar-apk-compilada").addEventListener("click", (e) => {
      e.preventDefault();
      descargar(PROYECTO + "/build/app.apk", "descargar-apk-compilada");
    });
    decirEstadoJar();
    iniciarPestana();
  });
}

if (typeof module !== "undefined") module.exports = { resaltar, esXml, textoHtml };
