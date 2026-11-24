#!/usr/bin/env node
// Falsificación de `apkc wisp-get` en el hospedador con forma de navegador. No
// abre ningún relay: un WebSocket de mentira cuenta las construcciones y falla
// en el primer evento. Comprueba que la petición entra por go.env (no por argv
// ni por OPFS), que una huella inválida no abre nada y que una petición válida
// abre exactamente un socket antes de fallar, sin el PEM en la salida.
//
// uso: node shim/prueba-wisp-get.js   (necesita go, openssl y wasm_exec.js del GOROOT)
const fs = require("fs");
const os = require("os");
const path = require("path");
const crypto = require("crypto");
const { spawnSync } = require("child_process");

const raiz = path.resolve(__dirname, "..");
const cache = process.env.XDG_CACHE_HOME
  ? path.join(process.env.XDG_CACHE_HOME, "apkc-web")
  : path.join(os.homedir(), ".cache", "apkc-web");
const base = path.join(cache, "wisp-get-final");
const tmp = path.join(base, "shim-prueba");
fs.mkdirSync(path.join(base, "gotmp"), { recursive: true });
fs.mkdirSync(path.join(base, "gocache"), { recursive: true });
fs.mkdirSync(tmp, { recursive: true });

const goroot = spawnSync("go", ["env", "GOROOT"], { encoding: "utf8" }).stdout.trim();
const wasmExec = path.join(goroot, "lib", "wasm", "wasm_exec.js");
if (!fs.existsSync(wasmExec)) {
  console.error("falta wasm_exec.js en " + wasmExec);
  process.exit(1);
}
fs.copyFileSync(path.join(__dirname, "navegador-sim.js"), path.join(tmp, "navegador-sim.js"));
fs.copyFileSync(wasmExec, path.join(tmp, "wasm_exec.js"));

const wasm = path.join(tmp, "apkc-wasm.wasm");
const compila = spawnSync("go", ["build", "-o", wasm, "."], {
  cwd: path.join(raiz, "apkc"), encoding: "utf8", timeout: 300000,
  env: {
    ...process.env, GOOS: "js", GOARCH: "wasm", CGO_ENABLED: "0",
    GOTMPDIR: path.join(base, "gotmp"), GOCACHE: path.join(base, "gocache"),
  },
});
if (compila.status !== 0) {
  console.error("no compilé el wasm: " + (compila.stderr || compila.stdout));
  process.exit(1);
}

// El preload corre antes de que el shim sustituya globalThis.process: guardamos
// el proceso real para poder imprimir el recuento al salir. El WebSocket cuenta
// y dispara onerror en el siguiente tick, en cuanto Go le haya puesto el handler.
const stub = path.join(tmp, "ws-stub.cjs");
fs.writeFileSync(stub, `
const proc = process;
globalThis.__wsPrueba = { cuenta: 0, ultima: "" };
globalThis.WebSocket = class WebSocketFalso {
  constructor(url) {
    globalThis.__wsPrueba.cuenta++;
    globalThis.__wsPrueba.ultima = String(url);
    setTimeout(() => { if (typeof this.onerror === "function") this.onerror({}); }, 0);
  }
  send() {}
  close() {}
};
proc.on("exit", () => proc.stderr.write("WS_CUENTA=" + globalThis.__wsPrueba.cuenta + "\\n"));
`);

// Un PEM X.509 real, generado en el temporal: la huella se calcula sobre los
// bytes entregados, igual que `prepararCAS` en la página.
const clave = path.join(tmp, "ca.key");
const cert = path.join(tmp, "ca.pem");
const openssl = spawnSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes",
  "-keyout", clave, "-out", cert, "-days", "1", "-subj", "/CN=wisp-get-prueba"], { encoding: "utf8" });
if (openssl.status !== 0) {
  console.error("no generé el certificado de prueba: " + openssl.stderr);
  process.exit(1);
}
const pem = fs.readFileSync(cert, "utf8");
const huella = crypto.createHash("sha256").update(Buffer.from(pem, "utf8")).digest("hex");

const peticion = {
  version: "apkc-wisp/1", relay: "wss://wisp.mercurywork.shop/wisp/",
  coordenada: "com.ejemplo:artefacto:1.0", destino: "/deps/com/ejemplo/artefacto/1.0/",
  pem, huella,
};

function correr(request) {
  const entorno = { ...process.env, NODE_OPTIONS: "--require " + stub };
  if (request != null) entorno.APKC_WISP_REQ = JSON.stringify(request);
  else delete entorno.APKC_WISP_REQ;
  // la petición solo va en el entorno: argv es el wasm y el nombre del comando
  const argv = [path.join(tmp, "navegador-sim.js"), wasm, "wisp-get"];
  if (argv.some((a) => String(a).includes("BEGIN CERTIFICATE"))) {
    throw new Error("el PEM se coló en argv");
  }
  const r = spawnSync(process.execPath, argv, {
    cwd: tmp, env: entorno, encoding: "utf8", timeout: 60000,
  });
  const salida = (r.stdout || "") + (r.stderr || "");
  const cuenta = /WS_CUENTA=(\d+)/.exec(salida);
  return { codigo: r.status, salida, sockets: cuenta ? Number(cuenta[1]) : -1, argv };
}

const fallos = [];
function comprobar(nombre, condicion, detalle) {
  if (condicion) console.log("  ok   " + nombre);
  else { fallos.push(nombre); console.log("  FALLO " + nombre + (detalle ? " — " + detalle : "")); }
}

const mala = { ...peticion, huella: "0".repeat(64) };
const sinHuella = correr(mala);
comprobar("huella inválida: sale con 1", sinHuella.codigo === 1, "rc " + sinHuella.codigo);
comprobar("huella inválida: el error nombra la huella", /huella/i.test(sinHuella.salida), sinHuella.salida.slice(0, 200));
comprobar("huella inválida: cero WebSockets", sinHuella.sockets === 0, "cuenta " + sinHuella.sockets);
comprobar("huella inválida: sin línea de marca", !sinHuella.salida.includes("APKC_WISP_GET:"));

const sinEnv = correr(null);
comprobar("sin APKC_WISP_REQ: sale con 1 y sin sockets", sinEnv.codigo === 1 && sinEnv.sockets === 0,
  "rc " + sinEnv.codigo + ", cuenta " + sinEnv.sockets);
comprobar("sin APKC_WISP_REQ: el error nombra la variable", /APKC_WISP_REQ/.test(sinEnv.salida),
  sinEnv.salida.slice(0, 200));

const valida = correr(peticion);
comprobar("petición válida: exactamente un WebSocket", valida.sockets === 1, "cuenta " + valida.sockets);
comprobar("petición válida: sale con 1 al fallar el relay", valida.codigo === 1, "rc " + valida.codigo);
comprobar("petición válida: el error es del WebSocket, no un pánico",
  /WebSocket/i.test(valida.salida) && !/panic/i.test(valida.salida), valida.salida.slice(0, 220));
comprobar("petición válida: sin línea de marca ni PEM en la salida",
  !valida.salida.includes("APKC_WISP_GET:") && !valida.salida.includes("BEGIN CERTIFICATE"));
comprobar("la petición nunca va en argv", valida.argv.join(" ").indexOf("BEGIN CERTIFICATE") < 0);

if (fallos.length) {
  console.error("\nprueba-wisp-get: " + fallos.length + " fallo(s): " + fallos.join(", "));
  process.exit(1);
}
console.log("\nprueba-wisp-get: el comando lee go.env, dispara un solo socket y fija la huella antes de conectar");
