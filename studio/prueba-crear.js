// Prueba del flujo nuevo de la página: crear el proyecto desde el formulario mínimo (nombre,
// package, icono opcional), que quede en OPFS, que se abra el manifiesto en el editor y que el
// icono viaje a res/drawable. Es lo que la prueba de file:// no puede hacer (no hay OPFS).
//
// uso: APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm node prueba-crear.js
const { spawn } = require("child_process");
const fs = require("fs");
const path = require("path");

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
    return {
      aviso: document.getElementById("estado-crear").textContent,
      editor: document.getElementById("pestana").classList.contains("activa"),
      abierto: document.getElementById("fichero-abierto").textContent,
      arbol: document.getElementById("arbol-pestana").textContent,
      rutas: [...document.querySelectorAll("#arbol-pestana a")].map((a) => a.dataset.ruta || ""),
      codigo: document.getElementById("codigo").value,
      titulo: document.getElementById("titulo-proyecto").textContent,
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
  comprueba("el icono está en el árbol", /res\/drawable\/icono\.png/.test(rutasArbol),
    rutasArbol.slice(0, 180));

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

  ws.close();
  apagar();
  console.log(fallos ? "crear: " + fallos + " problemas" : "crear: todo en orden (" + hechos + " comprobaciones)");
  process.exit(fallos ? 1 : 0);
})().catch((e) => {
  console.log("  FALLO: " + e.message);
  apagar();
  process.exit(1);
});
