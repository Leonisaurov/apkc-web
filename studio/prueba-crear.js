// Prueba del flujo nuevo de la página: crear el proyecto desde el formulario mínimo (nombre,
// package, icono opcional), que quede en OPFS, que se abra el manifiesto en el editor y que el
// icono viaje a res/drawable. Es lo que la prueba de file:// no puede hacer (no hay OPFS).
//
// uso: APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm node prueba-crear.js
import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

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
const navegador = spawn("chromium-browser", [
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

  // subir un archivo del dispositivo: el mismo camino que usa el botón del explorador, sin
  // pasar por ningún servidor. Con el manifiesto abierto, el destino es /proyecto.
  const subida = await evaluar(`(async () => {
    const entrada = document.getElementById("subir-ficheros");
    const dt = new DataTransfer();
    dt.items.add(new File(["hola desde el dispositivo\\n"], "nota.txt", { type: "text/plain" }));
    entrada.files = dt.files;
    entrada.dispatchEvent(new Event("change", { bubbles: true }));
    for (let i = 0; i < 100; i++) {
      if (/archivo subido/.test(document.getElementById("estado-pestana").textContent)) break;
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

  ws.close();
  apagar();
  console.log(fallos ? "crear: " + fallos + " problemas" : "crear: todo en orden (" + hechos + " comprobaciones)");
  process.exit(fallos ? 1 : 0);
})().catch((e) => {
  console.log("  FALLO: " + e.message);
  apagar();
  process.exit(1);
});
