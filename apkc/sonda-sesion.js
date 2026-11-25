#!/usr/bin/env node
// Sonda de la sesión viva antes del CDP. Levanta el wasm en un worker, entrega LA ORDEN 2 desde
// un setTimeout posterior al arranque y exige dos `apkc:fin-orden`. Si el bucle Go no cede al
// event loop, ese temporizador nunca corre y el padre imprime TIMEOUT (la falsación).
//
//   node apkc/sonda-sesion.js "$PREFIX/tmp/apkc-sesion.wasm" "$APK_EJEMPLO"
//   node apkc/sonda-sesion.js "$PREFIX/tmp/apkc-sesion.wasm" "$APK_EJEMPLO" --ocupado

const { Worker, workerData } = require("worker_threads");

function totalDe(linea) {
  try {
    const modelo = JSON.parse(linea);
    const capturas = (modelo.ejecucion && modelo.ejecucion.capturas) || [];
    for (let i = capturas.length - 1; i >= 0; i--) {
      if (capturas[i].id === "id/total") return String(capturas[i].texto).replace("Pulsos: ", "");
    }
  } catch (e) { /* una línea que no es JSON no cuenta */ }
  return "?";
}

if (!workerData) {
  const wasm = process.argv[2];
  const apk = process.argv[3];
  const ocupado = process.argv.includes("--ocupado");
  if (!wasm || !apk) {
    console.error("uso: node sonda-sesion.js <apkc.wasm> <app.apk> [--ocupado]");
    process.exit(2);
  }
  worker = new Worker(__filename, { workerData: { wasm, apk, ocupado } });
  const fines = [];
  const lineas = [];
  let terminado = false;
  const acabar = (codigo, texto) => {
    if (terminado) return;
    terminado = true;
    console.log(texto);
    worker.terminate();
    process.exit(codigo);
  };
  worker.on("error", (e) => console.error("sonda worker:", e && e.message || e));
  worker.on("message", (m) => {
    if (m.kind === "linea" && m.texto.startsWith("{")) lineas.push(m.texto);
    if (m.kind === "pm" && m.m.tipo === "apkc:fin-orden") {
      fines.push(m.m.id);
      if (fines.length === 2) {
        const timeline = ["0", ...lineas.map(totalDe)].join(",");
        setTimeout(() => acabar(0, `ordenes: 1,2; fin-orden: [${fines.join(",")}]; instancias: 1; timeline: ${timeline}`), 40);
      }
    }
    if (m.kind === "error") acabar(1, "ERROR: " + m.error);
  });
  setTimeout(() => acabar(1, "TIMEOUT: sin fin-orden para la orden 2"), 6000);
} else {
  const fs = require("fs");
  const os = require("os");
  const path = require("path");
  // Go necesita un fs real para leer la apk; las líneas del modelo se recogen por el gancho
  // opcional `apkcSesion.linea` del adaptador, no interceptando stdout.
  globalThis.fs = fs;
  if (!globalThis.crypto) globalThis.crypto = require("crypto").webcrypto;
  const { parentPort } = require("worker_threads");
  const goroot = process.env.APKC_GOROOT || path.join(os.homedir(), "sdk", "go1.24.0");
  require(path.join(goroot, "lib", "wasm", "wasm_exec.js"));
  const go = new Go();
  go.argv = ["apkc", "preview", "--sesion", "s1", workerData.apk];
  go.env = {
    HOME: "/home/usuario", ANDROID_HOME: "/android", JAVA_HOME: "/jvm",
    APKC_SESION_ID: "s1",
    ...(workerData.ocupado ? { APKC_SESION_OCUPADO: "1" } : {}),
  };
  go.exit = () => {};

  const cola = [{ id: 1, argv: ["--pulsar", "id/pulsar"] }];
  globalThis.apkcSesion = {
    id: "s1",
    siguiente: () => (cola.length ? JSON.stringify(cola.shift()) : ""),
    linea: (texto) => parentPort.postMessage({ kind: "linea", texto }),
  };
  globalThis.postMessage = (mensaje) => {
    parentPort.postMessage({ kind: "pm", m: mensaje });
    if (mensaje.tipo === "apkc:fin-orden" && mensaje.id === 1) {
      setTimeout(() => cola.push({ id: 2, argv: ["--pulsar", "id/pulsar"] }), 0);
    }
  };
  WebAssembly.instantiate(fs.readFileSync(workerData.wasm), go.importObject)
    .then(({ instance }) => go.run(instance))
    .then(() => parentPort.postMessage({ kind: "error", error: "go.run terminó" }))
    .catch((e) => parentPort.postMessage({ kind: "error", error: String(e && e.message || e) }));
}
