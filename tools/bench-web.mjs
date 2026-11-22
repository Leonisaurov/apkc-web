// Conductor CDP de tools/bench-web.sh. Mide cuatro tiempos y ningún juicio: los números se
// imprimen tal cual. El wasm se instrumenta en el worker con Target.setAutoAttach para envolver
// WebAssembly.instantiate; si el worker no se puede instrumentar, se imprime 0 muestras y el
// tiempo del primer comando, que es la medida sustituta.
// Uso: BENCH_CDP=<puerto> BENCH_URL=<url> ANDROID_JAR=<jar> node tools/bench-web.mjs
import fs from "node:fs";

const CDP = Number(process.env.BENCH_CDP || 9351);
const URL_BASE = process.env.BENCH_URL || "http://127.0.0.1:8793/index.html";
const ORIGEN = new URL(URL_BASE).origin;
const JAR = process.env.ANDROID_JAR || "";
const esperar = (ms) => new Promise((r) => setTimeout(r, ms));

if (!JAR || !fs.existsSync(JAR)) {
  console.error("bench: falta ANDROID_JAR");
  process.exit(2);
}

let objetivo = null;
for (let i = 0; i < 100; i++) {
  try {
    const lista = await (await fetch("http://127.0.0.1:" + CDP + "/json/list")).json();
    objetivo = lista.find((t) => t.type === "page" && (t.url === "about:blank" || t.url.startsWith(ORIGEN)));
    if (objetivo) break;
  } catch (e) { /* el debugger aún no escucha */ }
  await esperar(300);
}
if (!objetivo) { console.error("bench: chromium no subió el debugger"); process.exit(2); }

const ws = new WebSocket(objetivo.webSocketDebuggerUrl);
await new Promise((res, rej) => { ws.onopen = res; ws.onerror = () => rej(new Error("websocket falló")); });
let id = 0;
const colas = new Map();
const eventos = [];
ws.addEventListener("message", (ev) => {
  const m = JSON.parse(ev.data);
  if (m.id && colas.has(m.id)) { colas.get(m.id)(m); colas.delete(m.id); return; }
  if (m.method) eventos.push(m);
});
function exec(method, params, sessionId) {
  const n = ++id;
  const mensaje = { id: n, method, params: params || {} };
  if (sessionId) mensaje.sessionId = sessionId;
  return new Promise((res) => { colas.set(n, res); ws.send(JSON.stringify(mensaje)); });
}
async function evaluar(expression, sessionId) {
  const r = await exec("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true }, sessionId);
  if (r.result && r.result.exceptionDetails) {
    throw new Error(String((r.result.exceptionDetails.exception || {}).description || r.result.exceptionDetails.text).slice(0, 300));
  }
  return r.result && r.result.result ? r.result.result.value : undefined;
}

await exec("Page.enable");
await exec("Runtime.enable");
await exec("Network.enable");
await exec("Target.setAutoAttach", { autoAttach: true, waitForDebuggerOnStart: false, flatten: true });

const PARCHE = `(() => {
  if (globalThis.__benchWasm) return "ya";
  const original = WebAssembly.instantiate;
  globalThis.__benchWasm = { muestras: 0, total: 0, ultimo: 0 };
  WebAssembly.instantiate = function (...args) {
    const t = performance.now();
    return original.apply(this, args).then((r) => {
      const ms = performance.now() - t;
      globalThis.__benchWasm.muestras += 1;
      globalThis.__benchWasm.total += ms;
      globalThis.__benchWasm.ultimo = ms;
      return r;
    });
  };
  return "parcheado";
})()`;

let sesionWorker = null;
ws.addEventListener("message", (ev) => {
  const m = JSON.parse(ev.data);
  if (m.method !== "Target.attachedToTarget") return;
  const info = m.params && m.params.targetInfo;
  const sesion = m.params && m.params.sessionId;
  if (!sesion || !info) return;
  if (info.type === "worker" && /hospedador\.js/.test(info.url || "")) {
    sesionWorker = sesion;
    exec("Runtime.enable", {}, sesion).then(() => evaluar(PARCHE, sesion)).catch(() => {});
  } else {
    exec("Runtime.runIfWaitingForDebugger", {}, sesion).catch(() => {});
  }
});

const t0 = Date.now();
await exec("Page.navigate", { url: URL_BASE });

let aislada = false;
let tAislamiento = 0;
for (let i = 0; i < 160; i++) {
  try {
    aislada = await evaluar("!!(crossOriginIsolated && navigator.serviceWorker.controller)");
    if (aislada) { tAislamiento = Date.now() - t0; break; }
  } catch (e) { /* la recarga del SW puede caer en mitad de la lectura */ }
  await esperar(250);
}

let listo = false;
for (let i = 0; i < 160; i++) {
  try {
    listo = await evaluar(`typeof correr === "function" && typeof usarJarElegido === "function" &&
      typeof compilarDebug === "function" && typeof hospedador !== "undefined"`);
    if (listo) break;
  } catch (e) { /* aún carga */ }
  await esperar(250);
}

const doc = await exec("DOM.getDocument");
const entrada = await exec("DOM.querySelector", { nodeId: doc.result.root.nodeId, selector: "#android-jar" });
await exec("DOM.setFileInputFiles", { files: [JAR], nodeId: entrada.result.nodeId });

const jar = await evaluar(`(async () => {
  const t = performance.now();
  await usarJarElegido();
  const aviso = document.getElementById("estado-jar");
  for (let i = 0; i < 160; i++) {
    if (/android\\.jar listo/.test(aviso.textContent)) break;
    await new Promise((r) => setTimeout(r, 250));
  }
  const b = await hospedador.bytes("/sdk/android.jar");
  return { ms: performance.now() - t, bytes: b ? b.length : 0 };
})()`);

// El primer comando instancia el wasm. Se mide aparte para que el primer build no mezcle arranque.
const caliente = await evaluar(`(async () => {
  const t = performance.now();
  await correr(["version"]);
  return performance.now() - t;
})()`);

await evaluar(`(async () => { await hospedador.limpiar("/proyecto"); await correr(["create", "/proyecto"]); })()`);
const build = await evaluar(`(async () => {
  const t = performance.now();
  await compilarDebug();
  for (let i = 0; i < 200; i++) {
    const apk = await hospedador.bytes("/proyecto/build/app.apk");
    if (apk && apk.length > 5000) return { ms: performance.now() - t, bytes: apk.length };
    await new Promise((r) => setTimeout(r, 250));
  }
  return { ms: performance.now() - t, bytes: 0 };
})()`);

let wasm = { muestras: 0, total: 0, ultimo: 0 };
if (sesionWorker) {
  try { wasm = await evaluar("globalThis.__benchWasm || { muestras: 0, total: 0, ultimo: 0 }", sesionWorker); }
  catch (e) { wasm = { muestras: 0, total: 0, ultimo: 0 }; }
}
const red = eventos.filter((e) => e.method === "Network.requestWillBeSent").length;

const numero = (x) => Math.round(Number(x) || 0);
console.log("aislamiento_ms " + numero(tAislamiento));
console.log("aislamiento " + numero(aislada ? 1 : 0));
console.log("modulo_listo " + numero(listo ? 1 : 0));
console.log("wasm_instantiate_muestras " + numero(wasm.muestras));
console.log("wasm_instantiate_total_ms " + numero(wasm.total));
console.log("wasm_instantiate_ultimo_ms " + numero(wasm.ultimo));
console.log("wasm_primer_comando_ms " + numero(caliente));
console.log("android_jar_opfs_ms " + numero(jar.ms));
console.log("android_jar_bytes " + numero(jar.bytes));
console.log("primer_build_ms " + numero(build.ms));
console.log("primer_build_bytes " + numero(build.bytes));
console.log("peticiones_red " + numero(red));
ws.close();
process.exit(0);
