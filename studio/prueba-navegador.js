// prueba en navegador real (chromium headless, por CDP sin dependencias): lo que node no puede
// ejercitar —el DOM, los manejadores de click, el localStorage— que es justo donde una pagina de
// este tipo se cae. Se sirve por http sin COOP/COEP para comprobar el aviso de «sin OPFS»; la
// pestaña completa se prueba aparte con servidor.js.
//
// uso: node prueba-navegador.js   (lanza el servidor, conduce la pagina y apaga todo)
import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import http from "node:http";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const PUERTO = process.env.STUDIO_PORT || "9333";
const HTTP_PORT = Number(process.env.STUDIO_NAV_PORT || 8789);
const URL_PAGINA = "http://127.0.0.1:" + HTTP_PORT + "/index.html";

const MIME = {
  ".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8", ".json": "application/json; charset=utf-8",
  ".png": "image/png", ".svg": "image/svg+xml",
};
const servidor = http.createServer((peticion, respuesta) => {
  const ruta = decodeURIComponent((peticion.url || "/").split("?")[0]);
  const fichero = path.normalize(path.join(__dirname, ruta === "/" ? "index.html" : ruta));
  if (!fichero.startsWith(__dirname + path.sep) || !fs.existsSync(fichero) || fs.statSync(fichero).isDirectory()) {
    respuesta.writeHead(404); return void respuesta.end("no existe\n");
  }
  respuesta.writeHead(200, { "Content-Type": MIME[path.extname(fichero)] || "application/octet-stream" });
  respuesta.end(fs.readFileSync(fichero));
});
servidor.listen(HTTP_PORT, "127.0.0.1");

// perfil propio y limpio: chromium deja un SingletonLock cuando muere con SIGKILL, y el
// siguiente arranque con el perfil de ~ no subre el debugger — el fallo parece de la pagina
// y es del perfil. TMPDIR de Termux, nunca /tmp.
const PERFIL = path.join(process.env.PREFIX || "/data/data/com.termux/files/usr", "tmp", "studio-chrome-perfil");
fs.rmSync(PERFIL, { recursive: true, force: true });

const navegador = spawn("chromium-browser", [
  "--headless=new", "--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
  "--no-first-run", "--disable-extensions", "--disable-sync", "--user-data-dir=" + PERFIL,
  "--remote-debugging-port=" + PUERTO, URL_PAGINA,
], { stdio: ["ignore", "pipe", "pipe"] });
let salidaNavegador = "";
navegador.stderr.on("data", (d) => { salidaNavegador += d.toString(); });

const esperar = (ms) => new Promise((r) => setTimeout(r, ms));

async function target() {
  for (let i = 0; i < 60; i++) {
    try {
      const lista = await (await fetch("http://127.0.0.1:" + PUERTO + "/json/list")).json();
      const p = lista.find((t) => t.type === "page" && t.url.startsWith("http://127.0.0.1:" + HTTP_PORT));
      if (p) return p;
    } catch (e) { /* aun no escucha */ }
    await esperar(500);
  }
  throw new Error("chromium no subio el debugger:\n" + salidaNavegador.slice(0, 400));
}

const conductor = `(async () => {
  const r = {};
  const sa = (f) => { try { return f(); } catch (e) { return "EXCEPCION: " + e.message; } };
  r.titulo = document.title;
  r.vistas = [...document.querySelectorAll(".pestana-vista")].map((b) => b.textContent.trim());
  r.actividad = [...document.querySelectorAll(".actividad [data-vista]")].map((b) => b.dataset.vista);

  // el nombre del formulario se refleja en la barra de título y en el explorador
  const nombre = document.querySelector('[name="name"]');
  nombre.value = "Prueba"; nombre.dispatchEvent(new Event("input", { bubbles: true }));
  r.tituloTrasNombre = document.getElementById("titulo-proyecto").textContent;
  const paquete = document.querySelector('[name="package"]');
  paquete.value = "com.prueba.app"; paquete.dispatchEvent(new Event("input", { bubbles: true }));
  r.paqueteLateral = document.getElementById("lateral-proyecto").textContent;
  r.icono = !!document.getElementById("icono");

  // las pestañas cambian la vista sin recargar
  document.querySelector('.pestana-vista[data-vista="pestana"]').click();
  r.editorVisible = document.getElementById("pestana").classList.contains("activa") &&
    !document.getElementById("crear").classList.contains("activa");
  r.estadoBarra = document.getElementById("estado-general").textContent;
  document.querySelector('.pestana-vista[data-vista="crear"]').click();
  r.vueltaInicio = document.getElementById("crear").classList.contains("activa");

  // sin COOP/COEP no hay OPFS sincrónico: el botón tiene que decirlo, no quedarse colgado
  document.getElementById("crear-proyecto").click();
  await new Promise((ok) => setTimeout(ok, 1200));
  r.avisoCrear = document.getElementById("estado-crear").textContent;
  r.sinUndefined = !/undefined/.test(document.body.innerText);

  // el rastreo local, que es lo único que la página guarda
  r.localStorage = sa(() => { localStorage.setItem("apkc-studio-prueba", "1");
    const v = localStorage.getItem("apkc-studio-prueba"); localStorage.removeItem("apkc-studio-prueba");
    return v === "1" ? "funciona" : "leyo null"; });

  // el diagnóstico explica una salida real de apkc sin sacarla del navegador
  document.getElementById("salida-apkc").value =
    "[build] aapt2 compile never started: pipe: not implemented on js";
  document.getElementById("analizar-salida").click();
  await new Promise((ok) => setTimeout(ok, 300));
  const resultado = document.getElementById("resultado-analisis");
  r.diagnosticoVisible = !resultado.hidden;
  r.diagnostico = resultado.textContent;
  r.informe = !document.getElementById("descargar-informe").hidden;

  // temas: el selector cambia data-tema y la paleta del terminal
  const temaUI = document.getElementById("tema-ui");
  const temaTerm = document.getElementById("tema-consola");
  temaUI.value = "catppuccin";
  temaUI.dispatchEvent(new Event("change", { bubbles: true }));
  temaTerm.value = "rose-pine";
  temaTerm.dispatchEvent(new Event("change", { bubbles: true }));
  CONSOLA.escribir("\u001b[31mrojo\\u001b[0m");
  const colorTerm = document.querySelector(".consola .consola-fg-1");
  r.temas = {
    ui: document.documentElement.dataset.tema,
    term: document.documentElement.dataset.temaConsola,
    opcionesUI: temaUI.options.length,
    opcionesTerm: temaTerm.options.length,
    colorTerm: colorTerm ? getComputedStyle(colorTerm).color : "sin color",
  };
  return r;
})()`;

let fallos = 0;
function comprueba(cond, msg) { if (!cond) { fallos++; console.log("  FALLO:", msg); } }

(async () => {
  const t = await target();
  const ws = new WebSocket(t.webSocketDebuggerUrl);
  await new Promise((res, rej) => { ws.onopen = res; ws.onerror = () => rej(new Error("websocket fallo")); });
  let id = 0;
  const esperando = new Map();
  ws.addEventListener("message", (ev) => {
    const m = JSON.parse(ev.data);
    if (m.id && esperando.has(m.id)) { const { res } = esperando.get(m.id); esperando.delete(m.id); res(m); }
  });
  const exec = (method, params) => new Promise((res) => {
    const n = ++id;
    esperando.set(n, { res });
    ws.send(JSON.stringify({ id: n, method, params: params || {} }));
  });

  await exec("Runtime.enable");
  for (let i = 0; i < 20; i++) {
    const probe = await exec("Runtime.evaluate", { expression: "typeof buildProject === 'function' && typeof RULES === 'object'" });
    if (probe.result && probe.result.value === true) break;
    await esperar(400);
  }

  const r = await exec("Runtime.evaluate", { expression: conductor, awaitPromise: true, returnByValue: true });
  const v = r.result && r.result.result && r.result.result.value;
  if (r.result && r.result.exceptionDetails)
    console.log("  FALLO: excepcion en la pagina:", JSON.stringify(r.result.exceptionDetails.exception || r.result.exceptionDetails).slice(0, 300));
  if (!v) { console.log("  FALLO: el conductor no devolvio nada", JSON.stringify(r).slice(0, 300)); fallos++; }
  else {
    console.log("  ui:", v.titulo, "| pestañas:", JSON.stringify(v.vistas), "| actividad:", JSON.stringify(v.actividad));
    console.log("  inicio:", v.tituloTrasNombre, "|", v.paqueteLateral, "| editor:", v.editorVisible,
      "| aviso:", String(v.avisoCrear).slice(0, 120));
    comprueba(/apkc studio/.test(v.titulo), "el título no nombra el studio: " + v.titulo);
    comprueba(v.vistas.join(",") === "Inicio,Editor", "las pestañas no son las dos medidas: " + v.vistas);
    comprueba(v.actividad.join(",") === "crear,pestana", "la barra de actividad no tiene las dos vistas: " + v.actividad);
    comprueba(v.tituloTrasNombre === "Prueba", "el nombre no llegó a la barra de título: " + v.tituloTrasNombre);
    comprueba(v.paqueteLateral === "com.prueba.app", "el package no llegó al explorador: " + v.paqueteLateral);
    comprueba(v.editorVisible, "la pestaña Editor no se abrió");
    comprueba(v.estadoBarra === "Editor", "la barra de estado no dijo Editor: " + v.estadoBarra);
    comprueba(v.vueltaInicio, "no se pudo volver a Inicio");
    comprueba(v.icono, "falta el campo del icono");
    comprueba(/OPFS|no se pudo crear/.test(String(v.avisoCrear)), "sin COOP/COEP no avisó de que no hay OPFS: " + v.avisoCrear);
    comprueba(v.sinUndefined, "quedó un 'undefined' visible en la página");
    comprueba(v.localStorage === "funciona", "localStorage en http: " + v.localStorage);
    comprueba(v.diagnosticoVisible && /herramienta-no-arranca/.test(String(v.diagnostico)),
      "el diagnóstico no explicó el muro de aapt2: " + String(v.diagnostico).slice(0, 160));
    comprueba(/navegador no puede hablar con un proceso/.test(String(v.diagnostico)),
      "el diagnóstico no dice por qué el build clásico no corre en wasm: " + String(v.diagnostico).slice(0, 160));
    comprueba(v.informe, "el diagnóstico no ofreció el informe descargable");
    comprueba(v.temas && v.temas.ui === "catppuccin" && v.temas.term === "rose-pine",
      "los temas no se aplican: " + JSON.stringify(v.temas));
    comprueba(v.temas && v.temas.opcionesUI >= 4 && v.temas.opcionesTerm >= 4,
      "faltan temas en los selectores: " + JSON.stringify(v.temas));
    comprueba(/235, 111, 146/.test(String(v.temas && v.temas.colorTerm)),
      "el tema de terminal no cambia la paleta ANSI: " + String(v.temas && v.temas.colorTerm));
  }
  ws.close();
  navegador.kill("SIGKILL");
  servidor.close();
  console.log(fallos ? "navegador: " + fallos + " problemas" : "navegador: todo en orden");
  process.exit(fallos ? 1 : 0);
})().catch((e) => {
  console.log("  FALLO: " + e.message);
  navegador.kill("SIGKILL");
  servidor.close();
  process.exit(1);
});
