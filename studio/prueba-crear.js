// Prueba del flujo nuevo de la página: crear el proyecto desde el formulario mínimo (nombre,
// package, icono opcional), que quede en OPFS, que se abra el manifiesto en el editor y que el
// icono viaje a res/drawable. Es lo que la prueba de file:// no puede hacer (no hay OPFS).
//
// uso: APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm node prueba-crear.js
import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { leerZip, ajustesDelManifiesto } from "./proyecto.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const CHROME = process.env.CHROME_BIN || "chromium-browser";

const TMP = process.env.PREFIX ? process.env.PREFIX + "/tmp" : "/tmp";
const WASM = process.env.APKC_WASM || "";
const PUERTO = Number(process.env.STUDIO_CREAR_PORT || 8792);
const CDP = process.env.STUDIO_CREAR_CDP || "9346";
const URL_PAGINA = "http://127.0.0.1:" + PUERTO + "/index.html";

let fallos = 0, hechos = 0;
function comprueba(nombre, cond, detalle) {
  if (cond) { hechos++; console.log("  ok   " + nombre + (detalle ? " — " + detalle : "")); }
  else { fallos++; console.log("  FALLA " + nombre + (detalle ? " — " + detalle : "")); }
}
if (!WASM || !fs.existsSync(WASM)) {
  console.error("falta APKC_WASM apuntando al binario wasm");
  process.exit(2);
}

const esperar = (ms) => new Promise((r) => setTimeout(r, ms));

const servidor = spawn(process.execPath, [path.join(__dirname, "servidor.js"), String(PUERTO)], {
  cwd: __dirname,
  env: { ...process.env, APKC_WASM: path.resolve(WASM) },
  stdio: ["ignore", "pipe", "pipe"],
});
let salidaServidor = "";
servidor.stdout.on("data", (d) => { salidaServidor += d; });
servidor.stderr.on("data", (d) => { salidaServidor += d; });

const PERFIL = path.join(TMP, "crear-chrome-perfil");
fs.rmSync(PERFIL, { recursive: true, force: true });
const navegador = spawn(CHROME, [
  "--headless=new", "--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
  "--no-first-run", "--disable-extensions", "--disable-sync",
  "--user-data-dir=" + PERFIL, "--remote-debugging-port=" + CDP, URL_PAGINA,
], { stdio: ["ignore", "pipe", "pipe"] });
let salidaNavegador = "";
navegador.stderr.on("data", (d) => { salidaNavegador += d; });

function apagar() {
  try { navegador.kill("SIGKILL"); } catch (e) { /* */ }
  try { servidor.kill("SIGKILL"); } catch (e) { /* */ }
}
process.on("exit", apagar);

async function target() {
  for (let i = 0; i < 90; i++) {
    try {
      const lista = await (await fetch("http://127.0.0.1:" + CDP + "/json/list")).json();
      const p = lista.find((t) => t.type === "page" && t.url.startsWith("http://127.0.0.1:" + PUERTO));
      if (p) return p;
    } catch (e) { /* aún no escucha */ }
    await esperar(500);
  }
  throw new Error("chromium no subio el debugger:\n" + salidaNavegador.slice(0, 500) + "\nservidor:\n" + salidaServidor.slice(0, 500));
}

(async () => {
  const t = await target();
  const ws = new WebSocket(t.webSocketDebuggerUrl);
  await new Promise((res, rej) => { ws.onopen = res; ws.onerror = () => rej(new Error("websocket fallo")); });
  let id = 0;
  const colas = new Map();
  ws.addEventListener("message", (ev) => {
    const m = JSON.parse(ev.data);
    if (m.id && colas.has(m.id)) { const { res } = colas.get(m.id); colas.delete(m.id); res(m); }
  });
  const exec = (method, params) => new Promise((res) => {
    const n = ++id;
    colas.set(n, { res });
    ws.send(JSON.stringify({ id: n, method, params: params || {} }));
  });
  const evaluar = async (expresion) => {
    const r = await exec("Runtime.evaluate", { expression: expresion, awaitPromise: true, returnByValue: true });
    if (r.result && r.result.exceptionDetails) {
      throw new Error("excepcion en la pagina: " + JSON.stringify(r.result.exceptionDetails.exception || r.result.exceptionDetails).slice(0, 300));
    }
    return r.result && r.result.result ? r.result.result.value : undefined;
  };

  // segunda pestaña del navegador, mismo origen y mismo OPFS, otra sesión de CDP
  async function abrirSegundaPagina() {
    const respuesta = await fetch("http://127.0.0.1:" + CDP + "/json/new?" + encodeURIComponent(URL_PAGINA), { method: "PUT" });
    const t2 = await respuesta.json();
    const ws2 = new WebSocket(t2.webSocketDebuggerUrl);
    await new Promise((res, rej) => { ws2.onopen = res; ws2.onerror = () => rej(new Error("websocket 2 fallo")); });
    let id2 = 0;
    const colas2 = new Map();
    ws2.addEventListener("message", (ev) => {
      const m = JSON.parse(ev.data);
      if (m.id && colas2.has(m.id)) { const { res } = colas2.get(m.id); colas2.delete(m.id); res(m); }
    });
    const exec2 = (method, params) => new Promise((res) => {
      const n = ++id2;
      colas2.set(n, { res });
      ws2.send(JSON.stringify({ id: n, method, params: params || {} }));
    });
    const evaluar2 = async (expresion) => {
      const r = await exec2("Runtime.evaluate", { expression: expresion, awaitPromise: true, returnByValue: true });
      if (r.result && r.result.exceptionDetails) {
        throw new Error("excepcion en la segunda pagina: " + JSON.stringify(r.result.exceptionDetails.exception || r.result.exceptionDetails).slice(0, 300));
      }
      return r.result && r.result.result ? r.result.result.value : undefined;
    };
    await exec2("Runtime.enable");
    for (let i = 0; i < 60; i++) {
      try {
        if (await evaluar2("typeof abrir === 'function' && !!document.getElementById('codigo')")) break;
      } catch (e) { /* aún carga */ }
      await esperar(300);
    }
    return { ws: ws2, evaluar: evaluar2 };
  }

  await exec("Runtime.enable");
  for (let i = 0; i < 60; i++) {
    if (await evaluar("typeof hospedador === 'object' && typeof buildProject === 'function' && !!document.getElementById('crear-proyecto')")) break;
    await esperar(300);
  }

  // un PNG de 1x1 válido, para el icono opcional
  const png = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=";
  const crear = await evaluar(`(async () => {
    const nombre = document.querySelector('[name="name"]');
    const paquete = document.querySelector('[name="package"]');
    nombre.value = "Creada";
    paquete.value = "com.creada.app";
    const bytes = Uint8Array.from(atob(${JSON.stringify(png)}), (c) => c.charCodeAt(0));
    const dt = new DataTransfer();
    dt.items.add(new File([bytes], "icono.png", { type: "image/png" }));
    document.getElementById("icono").files = dt.files;
    document.getElementById("crear-proyecto").click();
    for (let i = 0; i < 100; i++) {
      const t = document.getElementById("estado-crear").textContent;
      if (/proyecto creado|no se pudo/.test(t)) break;
      await new Promise((ok) => setTimeout(ok, 150));
    }
    const texto = () => document.getElementById("arbol-pestana").textContent;
    const espera = async (cond) => {
      for (let i = 0; i < 60; i++) {
        if (cond()) return true;
        await new Promise((ok) => setTimeout(ok, 100));
      }
      return false;
    };
    // el árbol arranca cerrado: las hojas profundas no están en el DOM hasta desplegar
    const empiezaCerrado = !/MainActivity\\.java/.test(texto());
    document.getElementById("desplegar-arbol").click();
    const desplegado = await espera(() => /MainActivity\\.java/.test(texto()));
    document.getElementById("plegar-arbol").click();
    const plegado = await espera(() => !/MainActivity\\.java/.test(texto()));
    document.getElementById("desplegar-arbol").click();
    await espera(() => /MainActivity\\.java/.test(texto()));
    return {
      aviso: document.getElementById("estado-crear").textContent,
      editor: document.getElementById("pestana").classList.contains("activa"),
      abierto: document.getElementById("fichero-abierto").textContent,
      arbol: texto(),
      rutas: [...document.querySelectorAll("#arbol-pestana a")].map((a) => a.dataset.ruta || ""),
      codigo: document.getElementById("codigo").value,
      titulo: document.getElementById("titulo-proyecto").textContent,
      empiezaCerrado, desplegado, plegado,
    };
  })()`);

  comprueba("el botón crea el proyecto", /proyecto creado/.test(String(crear.aviso)), String(crear.aviso).slice(0, 160));
  comprueba("la pestaña Editor queda delante", crear.editor === true, String(crear.editor));
  comprueba("el manifiesto se abre en el editor", /AndroidManifest\.xml$/.test(String(crear.abierto)), String(crear.abierto));
  const rutasArbol = (crear.rutas || []).join(" ");
  comprueba("el árbol trae el package pedido", String(crear.arbol).includes("MainActivity.java") &&
    /\/com\/creada\/app/.test(rutasArbol), rutasArbol.replace(/ /g, " ").slice(0, 180));
  comprueba("el manifiesto lleva el package y el icono",
    /package="com\.creada\.app"/.test(String(crear.codigo)) && /@drawable\/icono/.test(String(crear.codigo)),
    String(crear.codigo).split("\n").filter((l) => /package=|icon=/.test(l)).join(" ").slice(0, 180));
  comprueba("el proyecto nace con API 34 por defecto",
    /android:targetSdkVersion="34"/.test(String(crear.codigo)),
    String(crear.codigo).split("\n").find((l) => /targetSdkVersion/.test(l)) || "sin targetSdkVersion");
  comprueba("el icono está en el árbol", /res\/drawable\/icono\.png/.test(rutasArbol),
    rutasArbol.slice(0, 180));
  comprueba("el árbol empieza cerrado", crear.empiezaCerrado === true,
    String(crear.empiezaCerrado));
  comprueba("el botón desplegar abre todas las carpetas", crear.desplegado === true,
    String(crear.desplegado));
  comprueba("el botón contraer las vuelve a cerrar", crear.plegado === true,
    String(crear.plegado));

  // build/ se ve, pero atenuado y etiquetado: es salida de apkc y se regenera. El filtro deja
  // el fichero buscado y la cadena de carpetas que lo contiene.
  const filtroArbol = await evaluar(`(async () => {
    await hospedador.escribir("/proyecto/build/app.apk", new Uint8Array([1, 2, 3]));
    await refrescarArbol();
    const build = document.querySelector('#arbol-pestana a[data-ruta="/proyecto/build"]');
    const generado = build ? { clase: build.className, texto: build.textContent } : null;
    const filtro = document.getElementById("filtro-arbol");
    filtro.value = "MainActivity";
    filtro.dispatchEvent(new Event("input", { bubbles: true }));
    await refrescarArbol();
    for (let i = 0; i < 40; i++) {
      if (document.querySelector('#arbol-pestana a[data-ruta$="MainActivity.java"]')) break;
      await new Promise((ok) => setTimeout(ok, 100));
    }
    const rutas = [...document.querySelectorAll("#arbol-pestana a")].map((a) => a.dataset.ruta || "");
    const manifiesto = rutas.includes("/proyecto/AndroidManifest.xml");
    const valor = filtro.value;
    filtro.value = "";
    filtro.dispatchEvent(new Event("input", { bubbles: true }));
    for (let i = 0; i < 40; i++) {
      if (document.querySelector('#arbol-pestana a[data-ruta="/proyecto/build"]')) break;
      await new Promise((ok) => setTimeout(ok, 100));
    }
    await hospedador.limpiar("/proyecto/build");
    await refrescarArbol();
    return { generado, rutas, manifiesto, valor };
  })()`);
  comprueba("build/ se ve atenuado y con la etiqueta generado",
    filtroArbol.generado && /generado/.test(String(filtroArbol.generado.clase)) &&
    /generado/.test(String(filtroArbol.generado.texto)),
    JSON.stringify(filtroArbol.generado));
  comprueba("el filtro deja el fichero buscado y sus carpetas, no el resto",
    filtroArbol.rutas.some((r) => /\/MainActivity\.java$/.test(r)) &&
    filtroArbol.rutas.includes("/proyecto/src") && filtroArbol.manifiesto === false,
    JSON.stringify(filtroArbol.rutas).slice(0, 260));

  // subir un archivo del dispositivo: el mismo camino que usa el botón del explorador, sin
  // pasar por ningún servidor. Con el manifiesto abierto, el destino es /proyecto.
  const subida = await evaluar(`(async () => {
    const entrada = document.getElementById("subir-ficheros");
    const dt = new DataTransfer();
    dt.items.add(new File(["hola desde el dispositivo\\n"], "nota.txt", { type: "text/plain" }));
    entrada.files = dt.files;
    entrada.dispatchEvent(new Event("change", { bubbles: true }));
    for (let i = 0; i < 100; i++) {
      if (/archivo subido/.test(document.getElementById("estado-pestana").textContent) &&
          /nota\.txt/.test(document.getElementById("fichero-abierto").textContent)) break;
      await new Promise((ok) => setTimeout(ok, 100));
    }
    const bytes = await hospedador.bytes("/proyecto/nota.txt");
    return {
      aviso: document.getElementById("estado-pestana").textContent,
      texto: bytes ? new TextDecoder().decode(bytes) : null,
      abierto: document.getElementById("fichero-abierto").textContent,
      rutas: [...document.querySelectorAll("#arbol-pestana a")].map((a) => a.dataset.ruta || ""),
    };
  })()`);
  comprueba("subir un archivo lo escribe en OPFS",
    subida.texto === "hola desde el dispositivo\n", JSON.stringify(subida).slice(0, 220));
  comprueba("el archivo subido aparece en el árbol y se abre",
    /nota\.txt$/.test(String(subida.abierto)) && subida.rutas.includes("/proyecto/nota.txt"),
    JSON.stringify({ abierto: subida.abierto, rutas: subida.rutas }).slice(0, 220));

  // mover el fichero abierto a res/: el contenido viaja, la pestaña sigue el cambio y el árbol
  // se reabre en la carpeta de destino
  const movimiento = await evaluar(`(async () => {
    await moverRuta("/proyecto/nota.txt", "/proyecto/res");
    const nuevo = await hospedador.bytes("/proyecto/res/nota.txt");
    const viejo = await hospedador.bytes("/proyecto/nota.txt");
    return {
      texto: nuevo ? new TextDecoder().decode(nuevo) : null,
      viejo: !!viejo,
      abierto: document.getElementById("fichero-abierto").textContent,
      rutas: [...document.querySelectorAll("#arbol-pestana a")].map((a) => a.dataset.ruta || ""),
    };
  })()`);
  comprueba("mover un archivo lo cambia de carpeta y conserva el contenido",
    movimiento.texto === "hola desde el dispositivo\n" && !movimiento.viejo,
    JSON.stringify(movimiento).slice(0, 220));
  comprueba("la pestaña y el árbol siguen al archivo movido",
    movimiento.abierto === "res/nota.txt" && movimiento.rutas.includes("/proyecto/res/nota.txt"),
    JSON.stringify({ abierto: movimiento.abierto, rutas: movimiento.rutas }).slice(0, 220));

  // exportar e importar: el zip sale con el manifiesto y un binario intacto, y al importarlo
  // se repone lo que se había borrado. Es la prueba de que el proyecto no depende del navegador.
  const exportado = await evaluar(`(async () => {
    const bin = new Uint8Array([137, 80, 78, 71, 0, 255, 10, 20, 30]);
    await hospedador.escribir("/proyecto/res/icono-prueba.png", bin);
    let blob = null;
    const crear = URL.createObjectURL;
    const clic = HTMLAnchorElement.prototype.click;
    URL.createObjectURL = (b) => { blob = b; return "blob:prueba"; };
    HTMLAnchorElement.prototype.click = function () {};
    try {
      await exportarProyecto();
    } finally {
      URL.createObjectURL = crear;
      HTMLAnchorElement.prototype.click = clic;
    }
    const bytes = new Uint8Array(await blob.arrayBuffer());
    await hospedador.limpiar("/proyecto/res/nota.txt");
    await hospedador.limpiar("/proyecto/res/icono-prueba.png");
    return {
      zip: btoa(String.fromCharCode(...bytes)),
      antes: {
        nota: !!(await hospedador.bytes("/proyecto/res/nota.txt")),
        png: !!(await hospedador.bytes("/proyecto/res/icono-prueba.png")),
      },
    };
  })()`);
  const entradasZip = await leerZip(new Uint8Array(Buffer.from(exportado.zip, "base64")));
  const pngZip = entradasZip.find((e) => e.ruta === "res/icono-prueba.png");
  comprueba("exportar el proyecto produce un zip con manifiesto y binarios",
    entradasZip.some((e) => e.ruta === "AndroidManifest.xml") &&
    pngZip && Array.from(pngZip.datos).join(",") === "137,80,78,71,0,255,10,20,30" &&
    exportado.antes.nota === false && exportado.antes.png === false,
    JSON.stringify({ entradas: entradasZip.length, png: pngZip ? pngZip.datos.length : 0,
      antes: exportado.antes }));

  const importado = await evaluar(`(async () => {
    const bytes = Uint8Array.from(atob(${JSON.stringify(exportado.zip)}), (c) => c.charCodeAt(0));
    const confirmar = window.confirm;
    window.confirm = () => true;
    try {
      await importarProyecto(new File([bytes], "proyecto.zip", { type: "application/zip" }));
    } finally {
      window.confirm = confirmar;
    }
    const nota = await hospedador.bytes("/proyecto/res/nota.txt");
    const png = await hospedador.bytes("/proyecto/res/icono-prueba.png");
    const aviso = document.getElementById("estado-pestana").textContent;
    // la importación cierra las pestañas del proyecto anterior; se dejan las dos que tenía la
    // prueba para que las comprobaciones siguientes midan lo mismo que antes
    limpiarEstadoEditor();
    await abrir("/proyecto/AndroidManifest.xml");
    await abrir("/proyecto/res/nota.txt");
    return {
      nota: nota ? new TextDecoder().decode(nota) : null,
      png: png ? Array.from(png) : null,
      aviso,
    };
  })()`);
  comprueba("importar el zip repone los ficheros y los binarios del proyecto",
    importado.nota === "hola desde el dispositivo\n" &&
    importado.png && importado.png.join(",") === "137,80,78,71,0,255,10,20,30" &&
    /proyecto importado/.test(String(importado.aviso)),
    JSON.stringify(importado).slice(0, 260));

  // ajustes: se leen del manifiesto y se guardan en el propio XML, sin fichero paralelo
  const ajustes = await evaluar(`(async () => {
    await leerAjustes(true);
    const antes = {
      min: document.getElementById("ajustes-minsdk").value,
      target: document.getElementById("ajustes-targetsdk").value,
      code: document.getElementById("ajustes-versioncode").value,
      nombre: document.getElementById("ajustes-versionname").value,
    };
    document.getElementById("ajustes-minsdk").value = "24";
    document.getElementById("ajustes-targetsdk").value = "35";
    document.getElementById("ajustes-versioncode").value = "7";
    document.getElementById("ajustes-versionname").value = "1.0";
    await guardarAjustes();
    return {
      antes,
      texto: await hospedador.texto("/proyecto/AndroidManifest.xml"),
      aviso: document.getElementById("estado-ajustes").textContent,
    };
  })()`);
  const leidos = ajustesDelManifiesto(ajustes.texto);
  comprueba("los ajustes se leen del manifiesto y se guardan en él",
    ajustes.antes.min === "21" && ajustes.antes.target === "34" &&
    leidos.minSdk === "24" && leidos.targetSdk === "35" &&
    leidos.versionCode === "7" && leidos.versionName === "1.0" &&
    !/maxSdkVersion/.test(ajustes.texto) && /guardado/.test(String(ajustes.aviso)),
    JSON.stringify({ antes: ajustes.antes, leidos, aviso: ajustes.aviso }));

  // la clave de depuración se crea en la pestaña y se puede descargar, igual que la de release
  const debugKey = await evaluar(`(async () => {
    const k = await asegurarDebugKeystore();
    const bytes = await hospedador.bytes("/sdk/debug.keystore");
    return {
      alias: k && k.alias,
      bytes: bytes ? bytes.length : 0,
      visible: !document.getElementById("descargar-debug-keystore").hidden,
    };
  })()`);
  comprueba("la clave de depuración se genera y se ofrece para descargar",
    debugKey.alias === "androiddebugkey" && debugKey.bytes > 1000 && debugKey.visible === true,
    JSON.stringify(debugKey));

  const politica = await evaluar(`(async () => {
    document.getElementById("ajustes-targetsdk").value = "30";
    await guardarAjustes();
    return {
      aviso: document.getElementById("estado-ajustes").textContent,
      texto: await hospedador.texto("/proyecto/AndroidManifest.xml"),
    };
  })()`);
  comprueba("bajar targetSdk avisa de la política de Google Play",
    /Google Play/.test(String(politica.aviso)) && /android:targetSdkVersion="30"/.test(politica.texto),
    JSON.stringify(politica).slice(0, 260));

  // varias pestañas: abrir otro fichero no cierra el anterior, y al volver se relee de OPFS
  const pestanas = await evaluar(`(async () => {
    await abrir("/proyecto/res/layout/main.xml");
    const tres = document.querySelectorAll("#pestanas-archivo .editor-tab").length;
    const enMain = document.getElementById("fichero-abierto").textContent;
    await abrir("/proyecto/AndroidManifest.xml");
    const enManifiesto = document.getElementById("fichero-abierto").textContent;
    await abrir("/proyecto/res/nota.txt");
    const principal = DOCUMENTOS.lista.find((d) => d.ruta === "/proyecto/res/layout/main.xml");
    return {
      tres, enMain, enManifiesto,
      activo: document.getElementById("fichero-abierto").textContent,
      mainSinCargar: principal ? principal.texto === null : "sin documento",
      rutas: [...document.querySelectorAll("#pestanas-archivo .editor-tab")].map((t) => t.dataset.ruta || ""),
    };
  })()`);
  comprueba("abrir varios ficheros crea varias pestañas",
    pestanas.tres === 3 && pestanas.rutas.includes("/proyecto/res/layout/main.xml"),
    JSON.stringify(pestanas).slice(0, 260));
  comprueba("cambiar de pestaña activa el fichero correcto",
    pestanas.enMain === "res/layout/main.xml" && pestanas.enManifiesto === "AndroidManifest.xml" &&
    pestanas.activo === "res/nota.txt",
    JSON.stringify(pestanas).slice(0, 260));
  comprueba("una pestaña limpia inactiva se descarga de memoria",
    pestanas.mainSinCargar === true, String(pestanas.mainSinCargar));

  // recargar: vuelven las pestañas y las carpetas abiertas, pero solo se lee la activa
  // se deja un estado distinguible: src abierta y res/layout cerrada, con la activa en res/
  await evaluar(`(() => {
    ARBOL.abiertos.clear();
    ARBOL.abiertos.add("/proyecto/src");
    guardarPersistencia();
    return true;
  })()`);
  await exec("Page.enable");
  await exec("Page.navigate", { url: URL_PAGINA });
  for (let i = 0; i < 80; i++) {
    try {
      if (await evaluar("typeof abrir === 'function' && !!document.getElementById('pestanas-archivo')")) break;
    } catch (e) { /* aún carga */ }
    await esperar(150);
  }
  const trasRecarga = await evaluar(`(async () => {
    for (let i = 0; i < 80; i++) {
      if (DOCUMENTOS.lista.length >= 3 && document.getElementById("codigo").value) break;
      await new Promise((ok) => setTimeout(ok, 100));
    }
    const main = DOCUMENTOS.lista.find((d) => d.ruta === "/proyecto/res/layout/main.xml");
    return {
      pestanas: document.querySelectorAll("#pestanas-archivo .editor-tab").length,
      activo: document.getElementById("fichero-abierto").textContent,
      codigo: document.getElementById("codigo").value,
      mainSinCargar: main ? main.texto === null : "sin documento",
      resAbierta: ARBOL.abiertos.has("/proyecto/res"),
      srcAbierta: ARBOL.abiertos.has("/proyecto/src"),
      layoutCerrada: !ARBOL.abiertos.has("/proyecto/res/layout"),
    };
  })()`);
  comprueba("al recargar vuelven las pestañas y la activa",
    trasRecarga.pestanas === 3 && trasRecarga.activo === "res/nota.txt" &&
    trasRecarga.codigo === "hola desde el dispositivo\n",
    JSON.stringify(trasRecarga).slice(0, 260));
  comprueba("al recargar se restaura el árbol y la carga sigue siendo diferida",
    trasRecarga.resAbierta === true && trasRecarga.srcAbierta === true &&
    trasRecarga.layoutCerrada === true && trasRecarga.mainSinCargar === true,
    JSON.stringify({ res: trasRecarga.resAbierta, src: trasRecarga.srcAbierta,
      layout: trasRecarga.layoutCerrada, main: trasRecarga.mainSinCargar }));

  // undo tree: dos versiones, deshacer y ramificar; además se persiste en IndexedDB
  const undo = await evaluar(`(async () => {
    const doc = documentoActivo();
    const area = document.getElementById("codigo");
    const teclear = (texto) => {
      area.value = texto;
      area.dispatchEvent(new Event("input", { bubbles: true }));
    };
    const esperar = (ms) => new Promise((ok) => setTimeout(ok, ms));
    teclear("version 1\\n"); await esperar(700);
    teclear("version 2\\n"); await esperar(700);
    const antes = Historial.recorrer(doc.hist).length;
    area.dispatchEvent(new KeyboardEvent("keydown", {
      key: "z", ctrlKey: true, bubbles: true, cancelable: true,
    }));
    const trasDeshacer = area.value;
    teclear("rama nueva\\n"); await esperar(700);
    const padre = doc.hist.nodos[doc.hist.nodos[doc.hist.actual].p];
    const ramas = padre ? padre.h.length : 0;
    const total = Historial.recorrer(doc.hist).length;
    mostrarPanel("historial");
    const filas = document.querySelectorAll("#arbol-historial .historial-nodo").length;
    await esperar(300);
    const guardado = await Historial.cargar(VENTANA, doc.ruta);
    return {
      antes, trasDeshacer, ramas, total, filas,
      persistido: guardado ? Historial.recorrer(guardado).length : 0,
      activo: document.getElementById("historial-resumen").textContent,
    };
  })()`);
  comprueba("deshacer crea un árbol con ramas",
    undo.antes >= 3 && undo.trasDeshacer === "version 1\n" && undo.ramas === 2,
    JSON.stringify(undo).slice(0, 260));
  comprueba("el historial se pinta en su panel",
    undo.total >= 4 && undo.filas >= 4 && /versiones/.test(String(undo.activo)),
    JSON.stringify({ total: undo.total, filas: undo.filas, activo: undo.activo }).slice(0, 240));
  comprueba("el historial queda persistido en IndexedDB",
    undo.persistido >= 4, String(undo.persistido));

  // 2 ventanas del mismo origen: BroadcastChannel avisa y el espejo del worker se refresca
  const segunda = await abrirSegundaPagina();
  const enSegunda = await segunda.evaluar(`(async () => {
    await abrir("/proyecto/res/nota.txt");
    return document.getElementById("codigo").value;
  })()`);
  comprueba("la segunda ventana abre el mismo fichero de OPFS",
    enSegunda === "hola desde el dispositivo\n", JSON.stringify(enSegunda).slice(0, 160));

  await evaluar(`(async () => {
    const area = document.getElementById("codigo");
    area.value = "desde A\\n";
    area.dispatchEvent(new Event("input", { bubbles: true }));
    await guardar();
    return true;
  })()`);
  let recibidoEnB = null;
  for (let i = 0; i < 60; i++) {
    recibidoEnB = await segunda.evaluar(`document.getElementById("codigo").value`);
    if (recibidoEnB === "desde A\n") break;
    await esperar(200);
  }
  comprueba("la segunda ventana recibe el guardado de la primera",
    recibidoEnB === "desde A\n", JSON.stringify(recibidoEnB).slice(0, 160));

  await segunda.evaluar(`(async () => {
    const area = document.getElementById("codigo");
    area.value = "desde B\\n";
    area.dispatchEvent(new Event("input", { bubbles: true }));
    await guardar();
    return true;
  })()`);
  let recibidoEnA = null;
  for (let i = 0; i < 60; i++) {
    recibidoEnA = await evaluar(`document.getElementById("codigo").value`);
    if (recibidoEnA === "desde B\n") break;
    await esperar(200);
  }
  comprueba("la primera ventana recibe la sobrescritura de la segunda",
    recibidoEnA === "desde B\n", JSON.stringify(recibidoEnA).slice(0, 160));

  // conflicto real: A tiene cambios sucios y B guarda otra versión de la misma línea
  await evaluar(`(() => {
    const area = document.getElementById("codigo");
    area.value = "local A\\n";
    area.dispatchEvent(new Event("input", { bubbles: true }));
    return true;
  })()`);
  await segunda.evaluar(`(async () => {
    const area = document.getElementById("codigo");
    area.value = "remoto B\\n";
    area.dispatchEvent(new Event("input", { bubbles: true }));
    await guardar();
    return true;
  })()`);
  let conflicto = null;
  for (let i = 0; i < 60; i++) {
    conflicto = await evaluar(`({
      visible: !document.getElementById("conflicto-aviso").hidden,
      texto: document.getElementById("conflicto-texto").textContent,
    })`);
    if (conflicto.visible) break;
    await esperar(200);
  }
  comprueba("las dos ventanas que tocan la misma línea generan conflicto",
    conflicto.visible === true && /conflicto|cambiaron/.test(String(conflicto.texto)),
    JSON.stringify(conflicto).slice(0, 220));
  await evaluar(`(() => { document.getElementById("conflicto-remoto").click(); return true; })()`);
  const resuelto = await evaluar(`({
    texto: document.getElementById("codigo").value,
    aviso: !document.getElementById("conflicto-aviso").hidden,
  })`);
  comprueba("se puede elegir la versión remota del conflicto",
    resuelto.texto === "remoto B\n" && resuelto.aviso === false, JSON.stringify(resuelto));
  segunda.ws.close();

  // plantilla Contador: se crea desde el formulario y deja el layout y la actividad que el
  // compilador del subconjunto ya sabe leer (campo, anónima y concatenación)
  const contador = await evaluar(`(async () => {
    document.querySelector('[data-vista="crear"]').click();
    document.getElementById("plantilla").value = "contador";
    const confirmar = window.confirm;
    window.confirm = () => true;
    document.getElementById("crear-proyecto").click();
    for (let i = 0; i < 120; i++) {
      if (/proyecto creado/.test(document.getElementById("estado-crear").textContent)) break;
      await new Promise((ok) => setTimeout(ok, 100));
    }
    window.confirm = confirmar;
    const arbol = await hospedador.arbol();
    const java = (arbol.nodos || []).find((n) => /MainActivity\\.java$/.test(n.ruta));
    const layout = (arbol.nodos || []).find((n) => /res\\/layout\\/main\\.xml$/.test(n.ruta));
    return {
      java: java ? await hospedador.texto(java.ruta) : "",
      layout: layout ? await hospedador.texto(layout.ruta) : "",
      aviso: document.getElementById("estado-crear").textContent,
    };
  })()`);
  comprueba("la plantilla Contador crea su actividad y su layout",
    /TextView total;/.test(contador.java) && /Pulsos: /.test(contador.java) &&
    /@\+id\/total/.test(contador.layout) && /@\+id\/pulsar/.test(contador.layout),
    JSON.stringify({ java: contador.java.slice(0, 60), layout: contador.layout.slice(0, 60), aviso: contador.aviso }));

  ws.close();
  apagar();
  console.log(fallos ? "crear: " + fallos + " problemas" : "crear: todo en orden (" + hechos + " comprobaciones)");
  process.exit(fallos ? 1 : 0);
})().catch((e) => {
  console.log("  FALLO: " + e.message);
  apagar();
  process.exit(1);
});
