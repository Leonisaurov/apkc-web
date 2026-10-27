// prueba en navegador real (chromium headless, por CDP sin dependencias): lo que node no puede
// ejercitar —el DOM, los manejadores de click, el localStorage del origen file://— que es justo
// donde una pagina de este tipo se cae.
//
// uso: node prueba-navegador.js   (lanza chromium, conduce la pagina y la apaga)
const { spawn } = require("child_process");
const fs = require("fs");
const path = require("path");

const PUERTO = process.env.STUDIO_PORT || "9333";
const URL_PAGINA = "file://" + path.join(__dirname, "index.html");
const muestras = JSON.parse(fs.readFileSync(path.join(__dirname, "muestras.json"), "utf8"));
const app = require(path.join(__dirname, "app.js"));   // el numero de reglas cambia: no se congela

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
      const p = lista.find((t) => t.type === "page" && t.url.startsWith("file://"));
      if (p) return p;
    } catch (e) { /* aun no escucha */ }
    await esperar(500);
  }
  throw new Error("chromium no subio el debugger:\n" + salidaNavegador.slice(0, 400));
}

const conductor = (pegado) => `(async () => {
  const r = {};
  const sa = (f) => { try { return f(); } catch (e) { return "EXCEPCION: " + e.message; } };
  r.titulo = document.title;
  r.reglas = document.getElementById("n-reglas").textContent;
  r.tarjetasGeneradas = document.querySelectorAll("#ficheros details").length;

  // cambiar a Kotlin tiene que reflejarse en el arbol sin recargar
  const kt = document.querySelector('[name="kotlin"]');
  kt.checked = true; form.dispatchEvent(new Event("input", { bubbles: true }));
  r.arbolKotlin = /MainActivity\\.kt/.test(document.getElementById("arbol").textContent);
  r.ficheroKotlin = [...document.querySelectorAll("#ficheros summary")].some((s) => /\\.kt$/.test(s.textContent));
  kt.checked = false; form.dispatchEvent(new Event("input", { bubbles: true }));

  // textos del formulario que antes se caian a "undefined"
  const bl = document.querySelector('[name="buttonLabel"]');
  bl.value = "Pulsar"; bl.dispatchEvent(new Event("input", { bubbles: true }));
  r.botonPulsar = [...document.querySelectorAll("#ficheros details")].some((d) =>
    /Press me|Pulsar/.test(d.textContent));
  r.sinUndefined = !/undefined/.test(document.getElementById("ficheros").textContent);

  // pegar una salida real de apkc y traducirla
  document.getElementById("log").value = ${JSON.stringify(pegado)};
  sa(() => document.getElementById("analizar").click());
  r.analizarExcepcion = typeof document.querySelector("#salida-analisis .nota") === "object"
    ? document.querySelector("#salida-analisis .nota").textContent : null;
  r.tarjetas = document.querySelectorAll("#salida-analisis .regla").length;
  r.capas = [...document.querySelectorAll("#salida-analisis h3")].map((h) => h.textContent);

  // descargar el zip: en file:// esto crea un blob y hace click en un ancla
  r.descarga = sa(() => { document.getElementById("descargar").click();
    return document.getElementById("estado-zip").textContent; });

  // el rastreo local, que es lo que produce el informe de friccion
  r.localStorage = sa(() => { localStorage.setItem("apkc-studio-prueba", "1");
    const v = localStorage.getItem("apkc-studio-prueba"); localStorage.removeItem("apkc-studio-prueba");
    return v === "1" ? "funciona" : "leyo null"; });
  r.informe = sa(() => { document.getElementById("ver-informe").click();
    return document.getElementById("informe-out").textContent.split("\\n")[0]; });
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
  // dejo cargar el javascript de la pagina
  for (let i = 0; i < 20; i++) {
    const probe = await exec("Runtime.evaluate", { expression: "typeof RULES === 'object' && typeof analizarPegado === 'function'", awaitPromise: false });
    if (probe.result && probe.result.value === true) break;
    await esperar(400);
  }

  const pegado = muestras.filter((m) => m.esperada !== "keystore-file-missing").map((m) => m.texto).join("\n")
    + "\n\u001b[31m[build] algo que la pagina no conoce del todo \u001b[0m";
  const r = await exec("Runtime.evaluate", { expression: conductor(pegado), awaitPromise: true, returnByValue: true });
  // CDP envuelve dos veces: {result: {result: {value}}} ; y un fallo dentro de la pagina sale
  // en exceptionDetails, no como excepcion del websocket
  const v = r.result && r.result.result && r.result.result.value;
  if (r.result && r.result.exceptionDetails)
    console.log("  FALLO: excepcion en la pagina:", JSON.stringify(r.result.exceptionDetails.exception || r.result.exceptionDetails).slice(0, 300));
  if (!v) { console.log("  FALLO: el conductor no devolvio nada", JSON.stringify(r).slice(0, 300)); fallos++; }
  else {
    console.log("  pagina:", v.titulo, "| reglas:", v.reglas, "| ficheros mostrados:", v.tarjetasGeneradas);
    console.log("  analisis:", v.tarjetas, "tarjetas en capas:", JSON.stringify(v.capas));
    console.log("  descarga:", v.descarga, "| localStorage:", v.localStorage, "| informe:", v.informe);
    comprueba(v.reglas === String(app.RULES.length),
    "el contador del DOM dice " + v.reglas + " y RULES tiene " + app.RULES.length);
    comprueba(v.tarjetasGeneradas === 5, "se esperaban 5 ficheros mostrados, hay " + v.tarjetasGeneradas);
    comprueba(v.arbolKotlin && v.ficheroKotlin, "marcar Kotlin no cambio el arbol");
    comprueba(v.botonPulsar, "el texto del boton no llego al layout");
    comprueba(v.sinUndefined, "quedo un 'undefined' visible en los ficheros");
    comprueba(v.tarjetas >= 5, "el log pegado produjo solo " + v.tarjetas + " tarjetas");
    comprueba(v.capas.join(" ").includes("que la página no conoce"), "las lineas desconocidas no se listaron");
    comprueba(typeof v.descarga === "string" && v.descarga.startsWith("descargado"), "la descarga no informou estado: " + v.descarga);
    comprueba(v.localStorage === "funciona", "localStorage en file://: " + v.localStorage);
    comprueba(/^# informe/.test(v.informe), "el informe no arranco: " + v.informe);
  }
  ws.close();
  navegador.kill("SIGKILL");
  console.log(fallos ? "navegador: " + fallos + " problemas" : "navegador: todo en orden");
  process.exit(fallos ? 1 : 0);
})().catch((e) => {
  console.log("  FALLO: " + e.message);
  navegador.kill("SIGKILL");
  process.exit(1);
});
