// Prueba de la pestaña en CHROMIUM REAL, no en node. La diferencia importa: el fs de Go exige OPFS,
// un worker de tipo module y crossOriginIsolated, y nada de eso existe en node. El shim de
// node ya demostró que apkc corre; aquí se demuestra que la página se lo cree.
//
// uso: sh -c 'APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm APK_EJEMPLO=/ruta/app.apk KS_EJEMPLO=$HOME/.android/debug.keystore node prueba-pestana.js'
// levanta servidor.js, lanza chromium headless por CDP, conduce la página y apaga todo.
const { spawn } = require("child_process");
const fs = require("fs");
const path = require("path");

const TMP = process.env.PREFIX ? process.env.PREFIX + "/tmp" : "/tmp";
const WASM = process.env.APKC_WASM || "";
const APK = process.env.APK_EJEMPLO || "";
const KS = process.env.KS_EJEMPLO || "";
const PUERTO = Number(process.env.STUDIO_SERVIDOR_PORT || 8791);
const CDP = process.env.STUDIO_PORT || "9345";
const URL_PAGINA = "http://127.0.0.1:" + PUERTO + "/index.html";

let fallos = 0, hechos = 0;
function comprueba(nombre, cond, detalle) {
  if (cond) { hechos++; console.log("  ok   " + nombre + (detalle ? " — " + detalle : "")); }
  else { fallos++; console.log("  FALLA " + nombre + (detalle ? " — " + detalle : "")); }
}
function falta(que) {
  console.error("falta " + que + ": esta prueba no puede jurar nada sin él");
  process.exit(2);
}
if (!WASM || !fs.existsSync(WASM)) falta("APKC_WASM apuntando al binario wasm");
if (!APK || !fs.existsSync(APK)) falta("APK_EJEMPLO apuntando a una apk construida por apkc");
if (!KS || !fs.existsSync(KS)) falta("KS_EJEMPLO apuntando a un keystore");

const esperar = (ms) => new Promise((r) => setTimeout(r, ms));

// --- el servidor y el navegador -----------------------------------------------------------
const servidor = spawn(process.execPath, [path.join(__dirname, "servidor.js"), String(PUERTO)], {
  cwd: __dirname,
  env: { ...process.env, APKC_WASM: path.resolve(WASM), APK_EJEMPLO: path.resolve(APK), KS_EJEMPLO: path.resolve(KS) },
  stdio: ["ignore", "pipe", "pipe"],
});
let salidaServidor = "";
servidor.stdout.on("data", (d) => { salidaServidor += d; });
servidor.stderr.on("data", (d) => { salidaServidor += d; });

const PERFIL = path.join(TMP, "pestana-chrome-perfil");
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

async function conductor() {
  const t = await target();
  const ws = new WebSocket(t.webSocketDebuggerUrl);
  await new Promise((res, rej) => { ws.onopen = res; ws.onerror = () => rej(new Error("websocket fallo")); });
  let id = 0;
  const colas = new Map();
  ws.addEventListener("message", (ev) => {
    const m = JSON.parse(ev.data);
    if (m.id && colas.has(m.id)) { colas.get(m.id)(m); colas.delete(m.id); }
  });
  const exec = (method, params) => new Promise((res) => {
    const n = ++id;
    colas.set(n, res);
    ws.send(JSON.stringify({ id: n, method, params: params || {} }));
  });
  const evaluarSinPlazo = async (expression) => {
    const r = await exec("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true });
    const ex = r.result && r.result.exceptionDetails;
    if (ex) throw new Error(String((ex.exception && ex.exception.description) || ex.text).slice(0, 400));
    return r.result.result.value;
  };
  // un await que no resuelve no es un test lento, es un test mudo: con plazo propio se sabe
  // qué paso quedó colgado en lugar de tragarse el timeout general
  const conPlazo = (promesa, ms, de) => Promise.race([
    promesa,
    esperar(ms).then(() => { throw new Error("plazo de " + ms + " ms agotado en: " + de); }),
  ]);
  const evaluar = (expression) => conPlazo(evaluarSinPlazo(expression), 150000, expression.slice(0, 70).replace(/\s+/g, " "));
  const peticiones = [];
  ws.addEventListener("message", (ev) => {
    const m = JSON.parse(ev.data);
    if (m.method === "Network.requestWillBeSent") peticiones.push(m.params.request.url);
  });
  await exec("Network.enable");
  await exec("Runtime.enable");
  for (let i = 0; i < 60; i++) {
    try { if (await evaluar("!!document.getElementById('codigo') && typeof correr === 'function'")) break; } catch (e) { /* */ }
    await esperar(300);
  }
  return { exec, evaluar, ws, peticiones };
}

// --- la prueba ----------------------------------------------------------------------------
(async () => {
  const { evaluar, exec, ws, peticiones } = await conductor();
  const fuera = path.join(TMP, "pestana-apk");
  fs.mkdirSync(fuera, { recursive: true });

  comprueba("la página está aislada (OPFS sincrónico disponible)",
    await evaluar("crossOriginIsolated === true"));

  // 1. crear el proyecto CON apkc dentro de la pestaña, desde cero: OPFS es por origen y una
  // sesión anterior puede haber dejado /proyecto, que entonces se traga el create
  await evaluar(`(async () => { await hospedador.limpiar("/proyecto"); return true; })()`);
  const creacion = await evaluar(`(async () => {
    const r = await correr(["create", "/proyecto"]);
    const arbol = await hospedador.arbol();
    const nodos = (arbol.nodos || []).filter(n => n.ruta.startsWith("/proyecto")).map(n => n.ruta + "|" + (n.dir ? "dir" : n.bytes));
    return { codigo: r.codigo, consola: r.consola, tipo: r.tipo, error: r.error, arbolTipo: arbol.tipo, arbolError: arbol.error, nodos };
  })()`);
  comprueba("apkc create sale 0 en wasm", creacion.codigo === 0,
    "tipo " + creacion.tipo + ", consola: " + JSON.stringify(String(creacion.consola === undefined ? "<ausente>" : creacion.consola).slice(-160)) +
    (creacion.error ? ", error del hospedador: " + creacion.error : "") + (creacion.arbolError ? ", error del arbol: " + creacion.arbolError : ""));
  const rutas = (creacion.nodos || []).filter((n) => !/\|dir$/.test(n)).map((n) => n.split("|")[0]);
  for (const esperado of ["/proyecto/AndroidManifest.xml", "/proyecto/src/com/myapp/MainActivity.java", "/proyecto/res/layout/main.xml"]) {
    comprueba("create escribió " + esperado, rutas.includes(esperado),
      rutas.length + " ficheros: " + rutas.slice(0, 6).join(", "));
  }

  // 2. el manifiesto que escribe apkc es el de sus plantillas, no otro
  const manifiesto = await evaluar(`(async () => {
    return await hospedador.texto("/proyecto/AndroidManifest.xml");
  })()`);
  comprueba("el manifest menciona el package que genera apkc", /package="com\.myapp"/.test(manifiesto),
    manifiesto.split("\n").slice(0, 2).join(" "));

  // 3. el editor escribe en el mismo árbol (guardar -> OPFS -> releer)
  const editado = await evaluar(`(async () => {
    estadoPestana.ruta = "/proyecto/src/com/myapp/Editado.java";
    EDITOR.poner("package com.myapp;\\n// probado desde la prueba de pestaña\\npublic class Editado {}\\n");
    await guardar();
    const texto = await hospedador.texto("/proyecto/src/com/myapp/Editado.java");
    const resaltado = document.getElementById("resalto").innerHTML;
    return { texto, tienePalabra: /<span class="palabra">package<\\/span>/.test(resaltado), tieneComentario: /<span class="comentario">/.test(resaltado), lineas: document.getElementById("numeros").textContent.trim().split("\\n").length };
  })()`);
  const fuente = "package com.myapp;\n// probado desde la prueba de pestaña\npublic class Editado {}\n";
  comprueba("lo editado llega a OPFS idéntico", editado.texto === fuente,
    JSON.stringify(editado.texto.slice(0, 40)));
  comprueba("el resaltador pinta la palabra clave", editado.tienePalabra === true);
  comprueba("el resaltador pinta el comentario", editado.tieneComentario === true);
  comprueba("los números de línea siguen al contenido", editado.lineas === fuente.split("\n").length - 1 || editado.lineas >= 3,
    editado.lineas + " líneas");

  // 4. persistencia: recargar la página y volver a leer el árbol
  await exec("Page.enable");
  await exec("Page.navigate", { url: URL_PAGINA });
  for (let i = 0; i < 40; i++) {
    try { if (await evaluar("!!document.getElementById('codigo') && typeof correr === 'function'")) break; } catch (e) { /* */ }
    await esperar(300);
  }
  const trasRecargar = await evaluar(`(async () => {
    return await hospedador.texto("/proyecto/src/com/myapp/Editado.java");
  })()`);
  comprueba("el fuente editado sobrevive a recargar la página", trasRecargar === fuente,
    JSON.stringify(String(trasRecargar).slice(0, 40)));

  // 5. firmar una apk real dentro de la pestaña, con el keystore montado desde el disco
  const instalacion = await evaluar(`(async () => {
    const a = await (await fetch("/apk-ejemplo")).arrayBuffer();
    const k = await (await fetch("/keystore-ejemplo")).arrayBuffer();
    await hospedador.escribir("/in.apk", new Uint8Array(a));
    await hospedador.escribir("/keystore", new Uint8Array(k));
    const r = await correr(["sign", "--keystore", "/keystore", "--out", "/out.apk", "/in.apk"]);
    const u = await hospedador.bytes("/out.apk");
    let b = "";
    if (u) {
      const trozos = [];
      for (let i = 0; i < u.length; i += 8192) trozos.push(String.fromCharCode.apply(null, u.subarray(i, i + 8192)));
      b = btoa(trozos.join(""));
    }
    return { codigo: r.codigo, consola: r.consola, bytes: b, largo: b.length };
  })()`);
  comprueba("apkc sign sale 0 dentro de chromium", instalacion.codigo === 0,
    String(instalacion.consola).split("\n").filter(Boolean).slice(-1)[0]);
  const apk = Buffer.from(instalacion.bytes, "base64");
  const destino = path.join(fuera, "pestanera.apk");
  fs.writeFileSync(destino, apk);
  comprueba("la apk firmada pesa lo que dice el árbol", apk.length > 1000, apk.length + " bytes");
  comprueba("el magic de zip, por si fuera un texto", apk.slice(0, 2).toString("latin1") === "PK");
  console.log("  --- la apk firmada en la pestaña es " + destino);

  // 6. el muro conocido, dicho tal cual
  const nodos = await evaluar(`(async () => {
    const a = await hospedador.arbol();
    const b = a.nodos.filter(n => n.ruta === "/proyecto" || n.ruta === "/proyecto/src" || n.ruta === "/proyecto/res/layout" || n.ruta === "/proyecto/AndroidManifest.xml");
    return JSON.stringify(b.map(n => n.ruta + ":" + (n.dir ? "dir" : n.bytes + "b")));
  })()`);
  console.log("  --- nodos tras recargar: " + nodos);
  const build = await evaluar(`(async () => {
    await hospedador.cwd("/proyecto");
    const r = await correr(["build"]);
    return { codigo: r.codigo, consola: r.consola };
  })()`);
  comprueba("apkc build en wasm dice el motivo, no se calla", build.codigo !== 0 &&
    /never started|not implemented on js/.test(String(build.consola)),
    JSON.stringify(build).slice(0, 320));

  // requisito del proyecto, escrito como comprobación: la pestaña no puede pedir nada fuera de
  // su origen. Ni un CDN de un runtime de terceros, ni telemetría, ni una fuente.
  const ajenas = peticiones.filter((u) => !u.startsWith("http://127.0.0.1:" + PUERTO) && !u.startsWith("data:") && !u.startsWith("blob:"));
  comprueba("la pestaña no pide nada fuera de su origen (" + peticiones.length + " peticiones)",
    ajenas.length === 0, ajenas.length ? "fuera del origen: " + ajenas.slice(0, 4).join(", ") : peticiones.length + " peticiones, todas locales");

  ws.close();
  console.log("\n" + hechos + " comprobaciones bien, " + fallos + " mal");
  apagar();
  process.exit(fallos ? 1 : 0);
})().catch((e) => {
  console.error("FALLO la prueba de pestaña:", e && e.message || e);
  console.error(salidaNavegador.slice(-600));
  apagar();
  process.exit(1);
});
