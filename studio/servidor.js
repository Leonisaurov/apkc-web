// servidor de la pestaña: sin dependencias, para poder cargar apkc.wasm como módulo y usar
// OPFS. No es un servicio ni un backend del build: es lo mínimo para que el navegador trate
// la página como un origen seguro (file:// no deja importar workers ni crear access handles
// sincrónicos) y para que el binario wasm no acabe versionado en el repo.
//
// uso: sh -c 'APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm node servidor.js [puerto]'
//   APKC_WASM   el binario wasm, construido con GOOS=js GOARCH=wasm (obligatorio)
//   GOROOT      de donde se saca wasm_exec.js (por defecto `go env GOROOT`)
//
// lo que sirve y por qué:
//   /                 studio/ (esta carpeta)
//   /apkc.wasm        el binario, con su mime correcto
//   /wasm_exec.js     el del GOROOT de quien corre: no se versiona (regla 2 del repo)
//   /android-jar      el android.jar del SDK local, si está instalado; no se versiona
//   /android-jar-info JSON con versión y tamaño del jar que encuentra (para la pestaña)
const http = require("http");
const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");

const AQUI = __dirname;
const PUERTO = Number(process.argv[2] || process.env.STUDIO_SERVIDOR_PORT || 8787);
const WASM = process.env.APKC_WASM || "";
let GOROOT = process.env.GOROOT || "";
if (!GOROOT) {
  try { GOROOT = execSync("go env GOROOT", { encoding: "utf8" }).trim(); } catch (e) { GOROOT = ""; }
}
const EJECUTA = path.join(GOROOT, "lib", "wasm", "wasm_exec.js");

// El android.jar no se puede versionar (26 MB y licencia de la SDK), pero sí se puede leer del
// SDK que ya está instalado en la máquina. Se busca en ANDROID_JAR, ANDROID_HOME,
// ANDROID_SDK_ROOT y ~/Android, de la plataforma más nueva a la más vieja. La página lo pide a
// su propio origen y lo guarda en OPFS: no sale ni una petición a Google desde la pestaña.
function jarDelSdk() {
  if (process.env.ANDROID_JAR && fs.existsSync(process.env.ANDROID_JAR)) return process.env.ANDROID_JAR;
  const raices = [process.env.ANDROID_HOME, process.env.ANDROID_SDK_ROOT,
    process.env.HOME ? path.join(process.env.HOME, "Android") : ""].filter(Boolean);
  for (const raiz of raices) {
    const plataformas = path.join(raiz, "platforms");
    if (!fs.existsSync(plataformas)) continue;
    let candidatas;
    try {
      candidatas = fs.readdirSync(plataformas)
        .map((nombre) => ({ nombre, version: Number((/^android-(\d+)$/.exec(nombre) || [])[1]) }))
        .filter((x) => Number.isInteger(x.version))
        .sort((a, b) => b.version - a.version);
    } catch (e) { continue; }
    for (const c of candidatas) {
      const jar = path.join(plataformas, c.nombre, "android.jar");
      if (fs.existsSync(jar)) return jar;
    }
  }
  return null;
}

const MIME = {
  ".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8", ".json": "application/json; charset=utf-8",
  ".wasm": "application/wasm", ".apk": "application/vnd.android.package-archive",
  ".png": "image/png", ".ico": "image/x-icon",
};

if (!WASM || !fs.existsSync(WASM)) {
  console.error("falta APKC_WASM apuntando a un binario wasm construido, por ejemplo:");
  console.error("  sh -c 'cd ../apkc && GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o \"$PREFIX/tmp/apkc-wasm.wasm\" .'");
  process.exit(2);
}
if (!fs.existsSync(EJECUTA)) {
  console.error("no hay wasm_exec.js en " + EJECUTA + " (GOROOT mal); con GOROOT= lo señalas tú");
  process.exit(2);
}

const servidor = http.createServer((req, res) => {
  // el isolation es lo que abre OPFS (createSyncAccessHandle) y SharedArrayBuffer; sin estas
  // dos cabeceras la página carga, el worker arranca y el fs se cae en el primer handle
  res.setHeader("Cross-Origin-Opener-Policy", "same-origin");
  res.setHeader("Cross-Origin-Embedder-Policy", "require-corp");
  res.setHeader("Cache-Control", "no-store");

  const ruta = decodeURIComponent((req.url || "/").split("?")[0]);
  let fichero = null, cuerpo = null;
  if (ruta === "/apkc.wasm") {
    fichero = WASM;
  } else if (ruta === "/wasm_exec.js") {
    fichero = EJECUTA;
  } else if (ruta === "/apkc-test.wasm" && process.env.APKC_TEST_WASM && fs.existsSync(process.env.APKC_TEST_WASM)) {
    fichero = process.env.APKC_TEST_WASM;                  // la suite, conducida desde la pestaña
  } else if (ruta === "/apk-ejemplo" && process.env.APK_EJEMPLO && fs.existsSync(process.env.APK_EJEMPLO)) {
    fichero = process.env.APK_EJEMPLO;                     // solo para la prueba de la pestaña
  } else if (ruta === "/aab-ejemplo" && process.env.AAB_EJEMPLO && fs.existsSync(process.env.AAB_EJEMPLO)) {
    fichero = process.env.AAB_EJEMPLO;                     // idem: el .aab de la prueba
  } else if (ruta === "/keystore-ejemplo" && process.env.KS_EJEMPLO && fs.existsSync(process.env.KS_EJEMPLO)) {
    fichero = process.env.KS_EJEMPLO;                      // idem: el keystore de debug del sitio
  } else if (ruta === "/keystore-release" && process.env.KS_RELEASE && fs.existsSync(process.env.KS_RELEASE)) {
    fichero = process.env.KS_RELEASE;                      // el de release, de usar y tirar: contraseña y alias conocidos
  } else if (ruta === "/android-jar-info") {
    const jar = jarDelSdk();
    const version = jar ? Number((/android-(\d+)/.exec(path.basename(path.dirname(jar))) || [])[1]) : null;
    res.writeHead(200, { "Content-Type": "application/json; charset=utf-8" });
    return void res.end(JSON.stringify({
      disponible: !!jar, version, bytes: jar ? fs.statSync(jar).size : 0,
    }));
  } else if (ruta === "/android-jar") {
    const jar = jarDelSdk();
    if (!jar) {
      res.writeHead(404, { "Content-Type": "text/plain; charset=utf-8" });
      return void res.end("no hay android.jar del SDK local; usa el selector de fichero o tools/entorno.sh jar\n");
    }
    fichero = jar;                                          // el jar del SDK, por su origen: la pestaña no habla con nadie más
  } else {
    const dentro = path.normalize(path.join(AQUI, ruta === "/" ? "index.html" : ruta));
    if (!dentro.startsWith(AQUI)) { res.writeHead(403); return void res.end("fuera\n"); }
    if (!fs.existsSync(dentro) || fs.statSync(dentro).isDirectory()) { res.writeHead(404); return void res.end("no existe\n"); }
    fichero = dentro;
  }
  cuerpo = fs.readFileSync(fichero);
  res.writeHead(200, { "Content-Type": MIME[path.extname(fichero)] || "application/octet-stream" });
  res.end(cuerpo);
});

servidor.listen(PUERTO, "127.0.0.1", () => {
  const jar = jarDelSdk();
  console.log(`apkc-studio en http://127.0.0.1:${PUERTO}/ (wasm: ${WASM})`);
  console.log(jar
    ? `android.jar del SDK local: ${jar} (${fs.statSync(jar).size} bytes)`
    : "sin android.jar local: usa el selector de fichero o tools/entorno.sh jar");
});
