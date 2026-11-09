// Prueba de la pestaña en CHROMIUM REAL, no en node. La diferencia importa: el fs de Go exige OPFS,
// un worker de tipo module y crossOriginIsolated, y nada de eso existe en node. El shim de
// node ya demostró que apkc corre; aquí se demuestra que la página se lo cree.
//
// uso: sh -c 'APKC_WASM=$PREFIX/tmp/apkc-wasm.wasm APK_EJEMPLO=/ruta/app.apk KS_EJEMPLO=$HOME/.android/debug.keystore node prueba-pestana.js'
// levanta servidor.js, lanza chromium headless por CDP, conduce la página y apaga todo.
import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const TMP = process.env.PREFIX ? process.env.PREFIX + "/tmp" : "/tmp";
const WASM = process.env.APKC_WASM || "";
const APK = process.env.APK_EJEMPLO || "";
const KS = process.env.KS_EJEMPLO || "";
// el keystore de release de la prueba: contraseña y alias conocidos, generado por e2e.sh con
// keytool. Es de usar y tirar; no se versiona ninguna clave.
const KS_RELEASE = process.env.KS_RELEASE || "";
// el .aab mínimo de la prueba: un zip con BundleConfig.pb y un módulo, creado por e2e.sh
const AAB = process.env.AAB_EJEMPLO || "";
// el android.jar es opcional: sin él no se puede probar el build completo dentro de la pestaña, y
// la prueba lo dice en la salida en vez de pasar de largo en silencio
const JAR_SRC = process.env.ANDROID_JAR || "";
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
  const envServidor = { ...process.env, APKC_WASM: path.resolve(WASM), APK_EJEMPLO: path.resolve(APK), KS_EJEMPLO: path.resolve(KS) };
  if (KS_RELEASE) envServidor.KS_RELEASE = path.resolve(KS_RELEASE);
  if (AAB) envServidor.AAB_EJEMPLO = path.resolve(AAB);
  if (JAR_SRC) envServidor.ANDROID_JAR = path.resolve(JAR_SRC);
  const servidor = spawn(process.execPath, [path.join(__dirname, "servidor.js"), String(PUERTO)], {
    cwd: __dirname,
    env: envServidor,
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
  const descargas = path.join(TMP, "pestana-descargas");
  fs.rmSync(descargas, { recursive: true, force: true });
  fs.mkdirSync(descargas, { recursive: true });
  await exec("Page.setDownloadBehavior", { behavior: "allow", downloadPath: descargas });

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
  comprueba("el explorador ofrece descargar cada fichero",
    await evaluar("!!document.querySelector('#arbol-pestana a.archivo .arbol-descargar')"));

  const ops = await evaluar(`(async () => {
    await hospedador.escribir("/proyecto/tmp-renombrar.txt", new TextEncoder().encode("hola"));
    const r = await hospedador.renombrar("/proyecto/tmp-renombrar.txt", "/proyecto/tmp-renombrado.txt");
    const texto = await hospedador.texto("/proyecto/tmp-renombrado.txt");
    const viejo = await hospedador.bytes("/proyecto/tmp-renombrar.txt");
    await hospedador.limpiar("/proyecto/tmp-renombrado.txt");
    const borrado = await hospedador.bytes("/proyecto/tmp-renombrado.txt");
    return { tipo: r.tipo, error: r.error, texto, viejo: !!viejo, borrado: !!borrado };
  })()`);
  comprueba("renombrar y borrar ficheros funciona en OPFS",
    ops.tipo === "renombrado" && ops.texto === "hola" && !ops.viejo && !ops.borrado,
    JSON.stringify(ops));

  const menu = await evaluar(`(() => {
    const fila = document.querySelector('#arbol-pestana a.archivo');
    fila.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 20, clientY: 20 }));
    const m = document.getElementById("menu-archivo");
    const salida = { visible: !m.hidden, texto: m.textContent };
    cerrarMenuArchivo();
    return salida;
  })()`);
  comprueba("el menú del fichero ofrece abrir, descargar, renombrar y borrar",
    menu.visible && /Abrir/.test(menu.texto) && /Descargar/.test(menu.texto) &&
    /Renombrar/.test(menu.texto) && /Borrar/.test(menu.texto), JSON.stringify(menu));

  const carga = await evaluar(`(async () => {
    const p = conCarga("probando…", () => new Promise((r) => setTimeout(r, 300)));
    await new Promise((r) => setTimeout(r, 220));
    const visible = !document.getElementById("cargando").hidden;
    const texto = document.getElementById("cargando-texto").textContent;
    await p;
    return { visible, texto, oculto: document.getElementById("cargando").hidden };
  })()`);
  comprueba("la pantalla de carga aparece en las operaciones lentas",
    carga.visible === true && carga.texto === "probando…" && carga.oculto === true,
    JSON.stringify(carga));

  // 2. el manifiesto que escribe apkc es el de sus plantillas, no otro
  const manifiesto = await evaluar(`(async () => {
    return await hospedador.texto("/proyecto/AndroidManifest.xml");
  })()`);
  comprueba("el manifest menciona el package que genera apkc", /package="com\.myapp"/.test(manifiesto),
    manifiesto.split("\n").slice(0, 2).join(" "));

  // 3. el editor escribe en el mismo árbol (guardar -> OPFS -> releer)
  const editado = await evaluar(`(async () => {
    const ruta = "/proyecto/src/com/myapp/Editado.java";
    // la prueba crea el buffer como lo haría «Nuevo fichero»: una pestaña propia
    DOCUMENTOS.lista.push({ ruta, texto: "", sucio: false, cursor: null, scroll: null });
    DOCUMENTOS.activa = ruta;
    estadoPestana.ruta = ruta;
    revelarRuta(ruta);
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

  // 3b. el scroll del editor: las tres capas tienen que moverse juntas, también al pasar de la
  //     primera pantalla (con el transform viejo el <pre> recortaba el contenido y se veía vacío)
  const desplazado = await evaluar(`(async () => {
    document.querySelector('[data-vista="pestana"]').click();
    await new Promise((r) => setTimeout(r, 150));
    const lineas = Array.from({ length: 120 }, (_, i) => "linea " + (i + 1) + " de la prueba de scroll").join("\\n");
    EDITOR.poner(lineas + "\\n" + "x".repeat(400));
    const area = document.getElementById("codigo");
    const resalto = document.getElementById("resalto");
    const numeros = document.getElementById("numeros");
    area.focus();
    area.scrollTop = 500;
    area.scrollLeft = 80;
    await new Promise((r) => setTimeout(r, 80));
    return {
      alto: [area.clientHeight, area.scrollHeight],
      vertical: [area.scrollTop, resalto.scrollTop, numeros.scrollTop],
      horizontal: [area.scrollLeft, resalto.scrollLeft],
      numerosVisibles: numeros.textContent.trim().split("\\n").length,
    };
  })()`);
  comprueba("el editor tiene scroll vertical de verdad", desplazado.alto[1] > desplazado.alto[0] * 3,
    desplazado.alto[0] + " visibles de " + desplazado.alto[1]);
  comprueba("las tres capas del editor comparten el scroll vertical",
    desplazado.vertical[0] > 300 && desplazado.vertical[0] === desplazado.vertical[1] &&
    desplazado.vertical[0] === desplazado.vertical[2], JSON.stringify(desplazado.vertical));
  comprueba("el resaltado comparte también el scroll horizontal",
    desplazado.horizontal[0] > 0 && desplazado.horizontal[0] === desplazado.horizontal[1],
    JSON.stringify(desplazado.horizontal));
  comprueba("los números de línea llegan a la última línea", desplazado.numerosVisibles === 121,
    desplazado.numerosVisibles + " números");

  // 3c. teclado: Tab en el cursor y Shift+Tab sobre un bloque; Enter usa el mismo camino y
  //     entra en el deshacer nativo por EDITOR.cambiar()
  const teclado = await evaluar(`(async () => {
    document.querySelector('[data-vista="pestana"]').click();
    await new Promise((r) => setTimeout(r, 100));
    const area = document.getElementById("codigo");
    EDITOR.poner("abc");
    area.focus();
    area.setSelectionRange(3, 3);
    area.dispatchEvent(new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true }));
    const conTab = area.value;
    const deshacer = document.execCommand("undo");
    const trasDeshacer = area.value;
    EDITOR.poner("uno\\ndos");
    area.focus();
    area.setSelectionRange(0, 7);
    area.dispatchEvent(new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true }));
    const bloque = area.value;
    area.setSelectionRange(0, 15);
    area.dispatchEvent(new KeyboardEvent("keydown", { key: "Tab", shiftKey: true, bubbles: true, cancelable: true }));
    const sinSangria = area.value;
    marcarSucio(false);
    return { conTab, trasDeshacer, bloque, sinSangria, deshacer };
  })()`);
  comprueba("Tab sin selección inserta en el cursor y se puede deshacer",
    teclado.conTab === "abc    " && teclado.trasDeshacer === "abc" && teclado.deshacer === true,
    JSON.stringify(teclado));
  comprueba("Tab y Shift+Tab indenta y desindenta el bloque seleccionado",
    teclado.bloque === "    uno\n    dos" && teclado.sinSangria === "uno\ndos",
    JSON.stringify({ bloque: teclado.bloque, sin: teclado.sinSangria }));

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

  // 5b. un keystore de RELEASE entra por la página: contraseña, alias y certificado. El DN que
  //     se ve aquí es el que apksigner tiene que leer en el apk que sale de firmar.
  if (KS_RELEASE) {
    const ui = await evaluar(`(async () => {
      const k = await (await fetch("/keystore-release")).arrayBuffer();
      await hospedador.escribir("/keystore", new Uint8Array(k));
      document.getElementById("storepass-firmar").value = "saludo";
      await inspeccionarKeystore(KEYSTORES.firmar);
      return {
        estado: document.getElementById("estado-keystore-firmar").textContent,
        alias: document.getElementById("keyalias-firmar").value,
        opciones: [...document.getElementById("keyalias-firmar").options].map((o) => o.value),
      };
    })()`);
    comprueba("la pestaña lista el alias del keystore de release", ui.alias === "lanzable", JSON.stringify(ui));
    comprueba("la pestaña enseña el sujeto y la huella SHA-256 del certificado",
      String(ui.estado).includes("CN=Release de prueba") && /SHA-256 ([0-9A-F]{2}:){31}/.test(String(ui.estado)),
      String(ui.estado).slice(0, 140));

    const release = await evaluar(`(async () => {
      document.getElementById("apk-a-firmar").value = "/in.apk";
      await firmar();
      const u = await hospedador.bytes("/proyecto/build/firmado.apk");
      let b = "";
      if (u) {
        const trozos = [];
        for (let i = 0; i < u.length; i += 8192) trozos.push(String.fromCharCode.apply(null, u.subarray(i, i + 8192)));
        b = btoa(trozos.join(""));
      }
      const id = await hospedador.bytes("/proyecto/build/firmado.apk.idsig");
      let bi = "";
      if (id) {
        const trozos = [];
        for (let i = 0; i < id.length; i += 8192) trozos.push(String.fromCharCode.apply(null, id.subarray(i, i + 8192)));
        bi = btoa(trozos.join(""));
      }
      return { bytes: b, idsig: bi, pass: document.getElementById("storepass-firmar").value,
        estado: document.getElementById("estado-keystore-firmar").textContent,
        nota: document.getElementById("estado-pestana").textContent,
        consola: String(document.getElementById("consola-pestana").textContent).slice(-700) };
    })()`);
    const apkRelease = Buffer.from(release.bytes, "base64");
    const idsigRelease = Buffer.from(release.idsig, "base64");
    fs.writeFileSync(path.join(fuera, "pestanera-release.apk"), apkRelease);
    fs.writeFileSync(path.join(fuera, "pestanera-release.apk.idsig"), idsigRelease);
    comprueba("la pestaña firma con el keystore de release", apkRelease.length > 1000,
      apkRelease.length + " bytes · " + JSON.stringify({ estado: release.estado, nota: release.nota, consola: release.consola }));
    comprueba("la pestaña escribe el .idsig de la v4", idsigRelease.length > 100, idsigRelease.length + " bytes");
    comprueba("la pestaña verifica la v4 de lo que firma", /v4 \(APK Signature Scheme v4\): ok/.test(String(release.consola)),
      String(release.consola).slice(-160));
    comprueba("la contraseña no se queda en el campo", release.pass === "", JSON.stringify(release.pass));
  }

  // 5c. un .aab entra por la misma sección: firma JAR v1, sin bloque v2/v3, y la pestaña lo verifica.
  if (AAB) {
    const bundle = await evaluar(`(async () => {
      const a = await (await fetch("/aab-ejemplo")).arrayBuffer();
      await hospedador.escribir("/in.aab", new Uint8Array(a));
      document.getElementById("storepass-firmar").value = "saludo";
      document.getElementById("apk-a-firmar").value = "/in.aab";
      await firmar();
      const u = await hospedador.bytes("/proyecto/build/firmado.aab");
      let b = "";
      if (u) {
        const trozos = [];
        for (let i = 0; i < u.length; i += 8192) trozos.push(String.fromCharCode.apply(null, u.subarray(i, i + 8192)));
        b = btoa(trozos.join(""));
      }
      return { bytes: b, consola: String(document.getElementById("consola-pestana").textContent).slice(-900) };
    })()`);
    const aab = Buffer.from(bundle.bytes, "base64");
    fs.writeFileSync(path.join(fuera, "pestanera.aab"), aab);
    comprueba("la pestaña firma un .aab con el keystore de release", aab.length > 500, aab.length + " bytes");
    comprueba("el .aab no lleva bloque de firmas de apk", !aab.includes(Buffer.from("APK Sig Block 42")));
    comprueba("la pestaña verifica la v1 del .aab", /v1 \(JAR signing\): ok/.test(String(bundle.consola)),
      String(bundle.consola).slice(-160));
  }

  // 5d. rotación desde la página: el linaje con la clave vieja (debug) y la nueva (release),
  //     y una firma en la que v1/v2 van con la vieja y v3/v3.1 con la nueva.
  if (KS_RELEASE) {
    const rotada = await evaluar(`(async () => {
      const k = await (await fetch("/keystore-ejemplo")).arrayBuffer();
      await hospedador.escribir("/keystore-viejo", new Uint8Array(k));
      document.getElementById("storepass-firmar").value = "saludo";
      document.getElementById("storepass-viejo").value = "android";
      document.getElementById("keyalias-viejo").value = "androiddebugkey";
      await crearLinaje();
      const linaje = await hospedador.bytes("/lineage.bin");
      document.getElementById("usar-rotacion").checked = true;
      document.getElementById("apk-a-firmar").value = "/in.apk";
      await firmar();
      const u = await hospedador.bytes("/proyecto/build/firmado.apk");
      const id = await hospedador.bytes("/proyecto/build/firmado.apk.idsig");
      const b64 = (buf) => {
        if (!buf) return "";
        const t = [];
        for (let i = 0; i < buf.length; i += 8192) t.push(String.fromCharCode.apply(null, buf.subarray(i, i + 8192)));
        return btoa(t.join(""));
      };
      return { apk: b64(u), idsig: b64(id), linaje: !!linaje,
        estado: document.getElementById("estado-linaje").textContent,
        consola: String(document.getElementById("consola-pestana").textContent).slice(-600) };
    })()`);
    const apkRotada = Buffer.from(rotada.apk, "base64");
    const idsigRotada = Buffer.from(rotada.idsig, "base64");
    fs.writeFileSync(path.join(fuera, "pestanera-rotada.apk"), apkRotada);
    fs.writeFileSync(path.join(fuera, "pestanera-rotada.apk.idsig"), idsigRotada);
    comprueba("la pestaña crea el linaje de rotación", rotada.linaje, String(rotada.estado));
    comprueba("la pestaña firma la rotación y escribe el .idsig", apkRotada.length > 1000 && idsigRotada.length > 100,
      apkRotada.length + " + " + idsigRotada.length + " bytes · " + String(rotada.consola).slice(-120));
    comprueba("la pestaña verifica la rotación (v4 de dos firmantes)",
      /v4 \(APK Signature Scheme v4\): ok/.test(String(rotada.consola)), String(rotada.consola).slice(-160));
  }

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
    return { codigo: r.codigo, consola: r.consola, tipo: r.tipo, error: r.error };
  })()`);
  comprueba("apkc build en wasm dice el motivo, no se calla", build.codigo !== 0 &&
    /never started|not implemented on js/.test(String(build.consola)),
    JSON.stringify(build).slice(0, 320));

  // el botón que anuncia el README no puede ser decorativo: pulsa, corre el build clásico y deja
  // el muro en la consola, que es lo que la página promete
  const clasico = await evaluar(`(async () => {
    document.getElementById("probar-build-clasico").click();
    for (let i = 0; i < 80; i++) {
      if (/muro conocido/.test(document.getElementById("consola-pestana").textContent)) break;
      await new Promise((ok) => setTimeout(ok, 200));
    }
    return { texto: String(document.getElementById("consola-pestana").textContent).slice(-600),
      panel: !document.getElementById("panel-inferior").hidden };
  })()`);
  comprueba("el botón del build clásico enseña el muro en la consola",
    clasico.panel && /muro conocido/.test(String(clasico.texto)) && /aapt2/.test(String(clasico.texto)),
    JSON.stringify(clasico).slice(0, 320));

  // 7. el proyecto entero, compilado DENTRO de la pestaña: recursos, Java, dex y firma, sin que
  // el navegador arranque un solo proceso. El jar llega por fetch del propio origen y se guarda
  // en OPFS, que es lo que hace el botón de la página.
  if (JAR_SRC && fs.existsSync(JAR_SRC)) {
    const jarPuesto = await evaluar(`(async () => {
      const n = await ponerJarDeUrl("/android-jar");
      const leido = await hospedador.leer("/sdk/android.jar");
      const arbol = await hospedador.arbol();
      const nodo = (arbol.nodos || []).find(x => x.ruta === "/sdk/android.jar");
      const cabecera = leido.bytes ? Array.from(leido.bytes.slice(0, 4)) : null;
      return { escrito: n, leido: leido.bytes ? leido.bytes.length : null, nodo: nodo ? nodo.bytes : "fuera del arbol", cabecera };
    })()`);
    comprueba("el android.jar entra en OPFS desde el propio origen", jarPuesto.escrito > 1000000, jarPuesto.escrito + " bytes");
    comprueba("el android.jar vuelve a leerse por el fs de la pestaña", jarPuesto.leido === jarPuesto.escrito,
      "leido " + jarPuesto.leido + ", cabecera " + JSON.stringify(jarPuesto.cabecera) + ", arbol " + jarPuesto.nodo);

    const compilado = await evaluar(`(async () => {
      const ks = new Uint8Array(await (await fetch("/keystore-ejemplo")).arrayBuffer());
      await hospedador.escribir("/sdk/keystore.p12", ks);
      document.getElementById("sdk-version").value = "34";
      document.getElementById("codename").value = "14";
      document.getElementById("storepass-compilar").value = "android";
      await compilarDelTodo();
      const datos = await hospedador.bytes("/proyecto/build/app.apk");
      return {
        codigo: document.getElementById("ultimo-codigo").textContent,
        bytes: datos ? Array.from(datos) : null,
        descarga: document.getElementById("descargar-apk-compilada").download || "",
        barra: !document.getElementById("panel-descargas").hidden,
        enlace: document.getElementById("descargar-apk-compilada").textContent,
        colores: /class="consola-fg-/.test(document.getElementById("consola-pestana").innerHTML),
        escape: /\\u001b/.test(document.getElementById("consola-pestana").textContent),
        consola: String(document.getElementById("consola-pestana").textContent).slice(-500),
      };
    })()`);
    const apkPestana = Buffer.from(compilado.bytes || [], "uint8");
    comprueba("apkc build sin procesos sale 0 dentro de chromium", compilado.codigo === "0",
      "codigo " + compilado.codigo + ", consola: " + JSON.stringify(compilado.consola.slice(-200)));
    comprueba("la pestaña produjo un apk", apkPestana.length > 5000, apkPestana.length + " bytes");
    comprueba("la descarga del apk queda lista en la página", compilado.descarga === "app.apk",
      JSON.stringify(compilado.descarga));
    comprueba("la descarga del apk se ve en la barra del panel",
      compilado.barra === true && /app\.apk/.test(compilado.enlace), JSON.stringify(compilado.enlace));
    comprueba("la consola pinta los colores ANSI y no enseña los códigos",
      compilado.colores === true && compilado.escape === false,
      JSON.stringify({ colores: compilado.colores, escape: compilado.escape }));
    const clicDescarga = await evaluar(`(() => {
      const a = document.getElementById("descargar-apk-compilada");
      let error = null;
      try { a.click(); } catch (e) { error = String(e); }
      return { href: a.href, download: a.download, error };
    })()`);
    for (let i = 0; i < 20 && !fs.existsSync(path.join(descargas, "app.apk")); i++) await esperar(250);
    const bajado = fs.existsSync(path.join(descargas, "app.apk"))
      ? fs.statSync(path.join(descargas, "app.apk")).size : 0;
    comprueba("el clic en el enlace descarga de verdad el app.apk",
      /^blob:/.test(clicDescarga.href) && clicDescarga.download === "app.apk" &&
      !clicDescarga.error && bajado > 5000,
      JSON.stringify({ ...clicDescarga, bajado }));
    const destinoPestana = path.join(fuera, "pestanera-completa.apk");
    fs.writeFileSync(destinoPestana, apkPestana);
    console.log("  --- el apk compilado dentro de la pestaña es " + destinoPestana);

    // 7b. sin jar en OPFS no hace falta elegir fichero: compilarDelTodo lo trae del SDK local que
    // sirve el mismo origen y lo guarda en OPFS antes de compilar.
    const auto = await evaluar(`(async () => {
      await hospedador.limpiar("/sdk/android.jar");
      await decirEstadoJar();
      const antes = document.getElementById("estado-jar").textContent;
      document.getElementById("storepass-compilar").value = "android";
      await compilarDelTodo();
      const jar = await hospedador.bytes("/sdk/android.jar");
      const apk = await hospedador.bytes("/proyecto/build/app.apk");
      return { antes, jar: jar ? jar.length : 0, apk: apk ? apk.length : 0,
        codigo: document.getElementById("ultimo-codigo").textContent,
        aviso: document.getElementById("estado-jar").textContent,
        cola: String(document.getElementById("consola-pestana").textContent)
          .replace(/\\u001b\\[[0-9;]*m/g, "").trim().split("\\n").slice(-10).join(" | ") };
    })()`);
    comprueba("sin jar en OPFS, compilar lo trae solo del SDK local",
      auto.jar > 1000000 && auto.apk > 5000 && auto.codigo === "0",
      "jar " + auto.jar + ", apk " + auto.apk + ", código " + auto.codigo + ": " + auto.cola);

    // 8. el proyecto que la PROPIA página genera (con la clase anónima del botón, el `@+id` y el
    // `style`), no la plantilla de create: es el que mide el §42 con javap y el que dexa 6 clases.
    const generado = await evaluar(`(async () => {
      const estado = { name: "Hola apkc", package: "com.hola.apkcstudio", minsdk: "21", targetsdk: "33",
        hello: "Hello World!", button: true, kotlin: false, libs: false, assets: false, native: false,
        buttonLabel: "Press me", toast: "Clicked!" };
      const p = buildProject(estado);
      await hospedador.limpiar("/estudio");
      for (const ruta of Object.keys(p.files)) {
        await hospedador.escribir("/estudio/" + ruta, new TextEncoder().encode(p.files[ruta]));
      }
      await hospedador.cwd("/estudio");
      const r = await correr(["build", "--linker", "go", "--compiler", "go",
        "--android-jar", "/sdk/android.jar", "--sdk-version", "34", "--codename", "14",
        "--keystore", "/sdk/keystore.p12"]);
      const datos = await hospedador.bytes("/estudio/build/app.apk");
      return {
        codigo: r.codigo, ficheros: Object.keys(p.files).length,
        clases: /\\d+ fuentes de Java en Go -> (\\d+) clases/.exec(String(r.consola || "")),
        dexBytes: /-> (\\d+) bytes en build\\/classes\\.dex/.exec(String(r.consola || "")),
        bytes: datos ? Array.from(datos) : null,
        consola: String(r.consola || "").slice(-400),
      };
    })()`);
    const apkEstudio = Buffer.from(generado.bytes || [], "uint8");
    comprueba("la pestaña compila el proyecto que genera la página", generado.codigo === 0,
      "codigo " + generado.codigo + " sobre " + generado.ficheros + " ficheros, consola: " +
      JSON.stringify(generado.consola.slice(-220)));
    comprueba("con la clase anónima son seis clases dexadas", generado.clases && generado.clases[1] === "6",
      generado.clases ? generado.clases[0] : "sin la linea de clases en la consola");
    comprueba("el dex del proyecto generado sale en la pestaña", generado.dexBytes && Number(generado.dexBytes[1]) > 2000,
      generado.dexBytes ? generado.dexBytes[0] : "sin la linea del dex");
    comprueba("el apk del proyecto generado pesa lo de un apk real", apkEstudio.length > 9000, apkEstudio.length + " bytes");
    const destinoEstudio = path.join(fuera, "pestanera-estudio.apk");
    fs.writeFileSync(destinoEstudio, apkEstudio);
    console.log("  --- el apk del proyecto generado, compilado en la pestaña, es " + destinoEstudio);

    // 9. el jar también entra SIN red: el usuario lo elige de su disco y la página lo guarda en OPFS.
    //    Es el camino que reclama el "ni siquiera con conexión" del objetivo; hasta aquí estaba probado
    //    solo el fetch a /android-jar, que es lo que hace el servidor de pruebas.
    const antesDelPicker = peticiones.length;
    const avisoVacio = await evaluar(`(async () => {
      const r = await hospedador.limpiar("/sdk/android.jar");
      await decirEstadoJar();
      return { tipo: r.tipo, error: r.error, aviso: document.getElementById("estado-jar").textContent };
    })()`);
    comprueba("borrado el jar la pagina lo dice, no lo suppose", /sin android\.jar/.test(avisoVacio.aviso),
      JSON.stringify(avisoVacio));
    const doc = await exec("DOM.getDocument");
    const elegido = await exec("DOM.querySelector", { nodeId: doc.result.root.nodeId, selector: "#android-jar" });
    const puesto = await exec("DOM.setFileInputFiles", { files: [JAR_SRC], nodeId: elegido.result.nodeId });
    comprueba("chromium acepta el android.jar en el input de fichero", !puesto.error && !puesto.result.exceptionDetails,
      JSON.stringify(puesto).slice(0, 160));
    const desdeDisco = await evaluar(`(async () => {
      document.getElementById("usar-android-jar").click();
      const aviso = document.getElementById("estado-jar");
      for (let i = 0; i < 160; i++) {
        if (/26361808/.test(aviso.textContent)) break;
        await new Promise((r) => setTimeout(r, 500));
      }
      return { aviso: aviso.textContent, guardado: !!(await hospedador.leer("/sdk/android.jar")).bytes };
    })()`);
    const peticionesNuevo = peticiones.slice(antesDelPicker);
    comprueba("el jar elegido de disco queda en OPFS con su tamano", /26361808 bytes/.test(desdeDisco.aviso) && desdeDisco.guardado,
      JSON.stringify(desdeDisco.aviso));
    comprueba("entrar el jar desde el disco no hace ni una peticion",
      !peticionesNuevo.some((u) => new URL(u).pathname === "/android-jar"),
      peticionesNuevo.length + " peticiones en el tramo: " + peticionesNuevo.slice(0, 3).join(", "));
    const sinConexion = await evaluar(`(async () => {
      await hospedador.limpiar("/proyecto/build");
      document.getElementById("storepass-compilar").value = "android";
      await compilarDelTodo();
      const datos = await hospedador.bytes("/proyecto/build/app.apk");
      return { codigo: document.getElementById("ultimo-codigo").textContent, bytes: datos ? datos.length : 0 };
    })()`);
    comprueba("con el jar del disco la pestaña vuelve a compilar el apk", sinConexion.codigo === "0" && sinConexion.bytes > 5000,
      "codigo " + sinConexion.codigo + ", " + sinConexion.bytes + " bytes");

    // 9b. y el jar se eligió UNA VEZ: recargar la página no lo borra, y se puede volver a compilar
    const antesRecarga = peticiones.length;
    await exec("Page.navigate", { url: URL_PAGINA });
    for (let i = 0; i < 40; i++) {
      try { if (await evaluar("!!document.getElementById('codigo') && typeof correr === 'function'")) break; } catch (e) { /* */ }
      await esperar(300);
    }
    const recargada = await evaluar(`(async () => {
      const datos = await hospedador.bytes("/sdk/android.jar");
      await hospedador.limpiar("/proyecto/build");
      document.getElementById("storepass-compilar").value = "android";
      await compilarDelTodo();
      const apk = await hospedador.bytes("/proyecto/build/app.apk");
      return {
        aviso: document.getElementById("estado-jar").textContent, jar: datos ? datos.length : 0,
        codigo: document.getElementById("ultimo-codigo").textContent, bytes: apk ? apk.length : 0,
      };
    })()`);
    comprueba("tras recargar la página el jar sigue en OPFS sin pedirlo otra vez",
      /26361808 bytes/.test(recargada.aviso) && recargada.jar === 26361808 &&
      !peticiones.slice(antesRecarga).some((u) => new URL(u).pathname === "/android-jar"),
      "aviso " + JSON.stringify(recargada.aviso) + ", jar " + recargada.jar + ", peticiones del tramo " +
      peticiones.slice(antesRecarga).length);
    comprueba("la página recargada compila otra vez con el jar guardado",
      recargada.codigo === "0" && recargada.bytes > 5000,
      "codigo " + recargada.codigo + ", " + recargada.bytes + " bytes");

    // 10. la pestaña dictamina su propio apk: `apkc verify` vuelve a leer el fichero desde OPFS y
    //     comprueba las dos firmas sin apksigner, sin conexión y sin un solo proceso.
    const dictamen = await evaluar(`(async () => {
      await hospedador.cwd("/proyecto");
      const r = await correr(["verify", "build/app.apk"]);
      const texto = String(r.consola || "").replace(/\\u001b\\[[0-9]*m/g, "");
      await verificarLoCompilado();
      return {
        codigo: r.codigo,
        v1: texto.includes("v1 (JAR signing): ok"),
        v2: texto.includes("v2 (APK Signature Scheme v2): ok"),
        aviso: document.getElementById("estado-verificacion").textContent,
        detalle: texto.split("\\n").filter(Boolean).slice(-4).join(" | "),
      };
    })()`);
    comprueba("apkc verify sale 0 dentro de chromium sobre el apk de la pestaña", dictamen.codigo === 0,
      JSON.stringify(dictamen.detalle.slice(-200)));
    comprueba("la pestaña verifica su v1 y su v2 por sí misma", dictamen.v1 && dictamen.v2,
      JSON.stringify(dictamen.detalle.slice(-220)));
    comprueba("el veredicto se lee en la página, no solo en la consola",
      /v1 \(JAR signing\): ok/.test(dictamen.aviso) && /v2 \(APK Signature Scheme v2\): ok/.test(dictamen.aviso),
      JSON.stringify(dictamen.aviso.slice(0, 200)));

    // 10b. y el veredicto discrimina: un byte cambiado dentro de una entrada ALMACENADA (resources.arsc
    //      viaja sin comprimir y alineado) tiene que dar v1 mal y v2 mal, no un "ok" por inercia
    const alterado = await evaluar(`(async () => {
      const datos = await hospedador.bytes("/proyecto/build/app.apk");
      const a = new TextEncoder().encode("resources.arsc");
      const buscar = (aguja, desde) => {
        for (let i = desde; i < datos.length - aguja.length; i++) {
          let ok = true;
          for (let j = 0; j < aguja.length; j++) if (datos[i + j] !== aguja[j]) { ok = false; break; }
          if (ok) return i;
        }
        return -1;
      };
      const nombre = buscar(a, 0);
      if (nombre < 0) return { encontrado: false };
      const ver = nombre - 30;                                  // principio del header local
      const largoNombre = datos[ver + 26] | (datos[ver + 27] << 8);
      const largoExtra = datos[ver + 28] | (datos[ver + 29] << 8);
      const comprimido = datos[ver + 18] | (datos[ver + 19] << 8) | (datos[ver + 20] << 16) | (datos[ver + 21] << 24);
      const datosDe = ver + 30 + largoNombre + largoExtra;
      datos[datosDe + Math.floor(comprimido / 2)] ^= 0x08;
      await hospedador.escribir("/proyecto/build/mal.apk", new Uint8Array(datos));
      await hospedador.cwd("/proyecto");
      const r = await correr(["verify", "build/mal.apk"]);
      const texto = String(r.consola || "").replace(/\\u001b\\[[0-9]*m/g, "");
      return {
        encontrado: true, codigo: r.codigo, tamano: comprimido,
        v1mal: texto.includes("v1 (JAR signing): mal"),
        v2mal: texto.includes("v2 (APK Signature Scheme v2): mal"),
        detalle: texto.split("\\n").filter(Boolean).slice(-3).join(" | "),
      };
    })()`);
    comprueba("el apk alterado se puede preparar en la pestaña", alterado.encontrado === true,
      JSON.stringify(alterado).slice(0, 120));
    comprueba("apkc verify rechaza el apk con un byte cambiado dentro de chromium",
      alterado.codigo !== 0 && alterado.v1mal === true && alterado.v2mal === true,
      "codigo " + alterado.codigo + ", " + JSON.stringify(String(alterado.detalle).slice(-220)));

    // 11. y se puede ESCRIBIR un fuente propio, no solo el que genera la página: campos con
    // modificadores, una cadena de llamadas, cuentas enteras, las branquias del párrafo 52 y la
    // concatenación de cadenas del párrafo 53
    const fuenteEscrita = [
      "package com.myapp;",
      "",
      "import android.app.*;",
      "import android.os.*;",
      "import android.view.*;",
      "import android.widget.*;",
      "",
      "public class MainActivity extends Activity {",
      "    private int pulsos = 0;",
      "    static int total = 5;",
      "",
      "    @Override",
      "    public void onCreate(Bundle savedInstanceState) {",
      "        super.onCreate(savedInstanceState);",
      "        setContentView(R.layout.main);",
      "        View raiz = getWindow().getDecorView();",
      "        pulsos = pulsos + 1;",
      "        if (pulsos > 2) { total = total * 2; } else { total = total + 1; }",
      "        if (raiz != null) { raiz.setVisibility(0); }",
      "        Toast.makeText(this, \"cuenta \" + pulsos, Toast.LENGTH_SHORT).show();",
      "    }",
      "}",
      "",
    ].join("\n");
    const escrito = await evaluar(`(async () => {
      await hospedador.escribir("/proyecto/src/com/myapp/MainActivity.java",
        new TextEncoder().encode(${JSON.stringify(fuenteEscrita)}));
      await hospedador.limpiar("/proyecto/build");
      document.getElementById("storepass-compilar").value = "android";
      await compilarDelTodo();
      const r = await correr(["verify", "build/app.apk"]);
      const texto = String(r.consola || "").replace(/\\u001b\\[[0-9]*m/g, "");
      const datos = await hospedador.bytes("/proyecto/build/app.apk");
      return {
        codigo: document.getElementById("ultimo-codigo").textContent,
        bytes: datos ? Array.from(datos) : null,
        clases: /-> (\\d+) clases/.exec(String(r.consola || "")),
        v1: texto.includes("v1 (JAR signing): ok"), v2: texto.includes("v2 (APK Signature Scheme v2): ok"),
        detalle: texto.split("\\n").filter(Boolean).slice(0, 2).join(" | "),
      };
    })()`);
    const apkEscrita = Buffer.from(escrito.bytes || [], "uint8");
    comprueba("un fuente escrito a mano compila entero en la pestaña", escrito.codigo === "0" && apkEscrita.length > 5000,
      "codigo " + escrito.codigo + ", " + apkEscrita.length + " bytes, consola: " + JSON.stringify(String(escrito.detalle).slice(-180)));
    comprueba("y su apk verifica igual que el generado", escrito.v1 === true && escrito.v2 === true,
      JSON.stringify(String(escrito.detalle).slice(-180)));
    fs.writeFileSync(path.join(fuera, "pestanera-escrita.apk"), apkEscrita);
    console.log("  --- el apk del fuente escrito a mano, compilado en la pestaña, es " +
      path.join(fuera, "pestanera-escrita.apk"));

    // 12. y la app puede CONTAR: una anonima sobre una interfaz del framework que escribe el campo del
    // exterior, decide con `if` y arma el texto con concatenación (párrafo 54). Es el fuente que la
    // página no genera y que, instalado, hace algo.
    const fuenteContador = [
      "package com.myapp;",
      "",
      "import android.app.*;",
      "import android.os.*;",
      "import android.view.*;",
      "import android.widget.*;",
      "",
      "public class MainActivity extends Activity {",
      "    int pulsos = 0;",
      "    static int total = 5;",
      "",
      "    @Override",
      "    public void onCreate(Bundle savedInstanceState) {",
      "        super.onCreate(savedInstanceState);",
      "        setContentView(R.layout.main);",
      "        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {",
      "            public void onClick(View v) {",
      "                pulsos = pulsos + 1;",
      "                total = total + pulsos;",
      "                if (pulsos > 3) { v.setContentDescription(\"muchos \" + pulsos); }",
      "                else { v.setContentDescription(\"pulsos \" + pulsos); }",
      "                Toast.makeText(MainActivity.this, \"llevas \" + pulsos, Toast.LENGTH_SHORT).show();",
      "            }",
      "        });",
      "    }",
      "}",
      "",
    ].join("\n");
    const contador = await evaluar(`(async () => {
      // se le anade un boton con id al layout que creo apkc: es lo que alguien escribiria en el
      // editor, y lo que exige R.id.pulsar para poder enlazar el listener
      const layout = (await hospedador.texto("/proyecto/res/layout/main.xml")) || "";
      const boton = '\\n    <Button\\n        android:id="@+id/pulsar"\\n        android:layout_width="wrap_content"\\n        android:layout_height="wrap_content"\\n        android:text="Press me"/>\\n';
      await hospedador.escribir("/proyecto/res/layout/main.xml",
        new TextEncoder().encode(layout.replace("</LinearLayout>", boton + "</LinearLayout>")));
      await hospedador.escribir("/proyecto/src/com/myapp/MainActivity.java",
        new TextEncoder().encode(${JSON.stringify(fuenteContador)}));
      await hospedador.limpiar("/proyecto/build");
      document.getElementById("storepass-compilar").value = "android";
      await compilarDelTodo();
      const r = await correr(["verify", "build/app.apk"]);
      const texto = String(r.consola || "").replace(/\\u001b\\[[0-9]*m/g, "");
      const datos = await hospedador.bytes("/proyecto/build/app.apk");
      return {
        codigo: document.getElementById("ultimo-codigo").textContent,
        bytes: datos ? Array.from(datos) : null,
        v1: texto.includes("v1 (JAR signing): ok"), v2: texto.includes("v2 (APK Signature Scheme v2): ok"),
        detalle: texto.split("\\n").filter(Boolean).slice(0, 2).join(" | "),
        consola: String(r.consola || "").slice(-260),
      };
    })()`);
    const apkContador = Buffer.from(contador.bytes || [], "uint8");
    comprueba("la app del contador compila entera en la pestaña", contador.codigo === "0" && apkContador.length > 5000,
      "codigo " + contador.codigo + ", " + apkContador.length + " bytes, consola: " + JSON.stringify(String(contador.consola).slice(-180)));
    comprueba("y su apk verifica con la anonima, el if y la concatenación dentro",
      contador.v1 === true && contador.v2 === true,
      JSON.stringify(String(contador.detalle).slice(-180)));
    fs.writeFileSync(path.join(fuera, "pestanera-contador.apk"), apkContador);
    console.log("  --- el apk de la app del contador, compilado en la pestaña, es " +
      path.join(fuera, "pestanera-contador.apk"));

    // 12b. compilar debug no pide keystore: genera la clave típica en OPFS y firma con ella.
    const debug = await evaluar(`(async () => {
      await hospedador.limpiar("/sdk/debug.keystore");
      await hospedador.limpiar("/sdk/keystore.p12");
      document.getElementById("storepass-compilar").value = "";
      await compilarDebug();
      const ks = await hospedador.bytes("/sdk/debug.keystore");
      const apk = await hospedador.bytes("/proyecto/build/app.apk");
      return { ks: ks ? ks.length : 0, apk: apk ? apk.length : 0,
        codigo: document.getElementById("ultimo-codigo").textContent,
        consola: String(document.getElementById("consola-pestana").textContent).slice(-1400),
        veredicto: document.getElementById("estado-verificacion").textContent };
    })()`);
    comprueba("compilar debug crea la clave típica y firma sin keystore de release",
      debug.ks > 1000 && debug.apk > 5000 && debug.codigo === "0" &&
      /certificado Android Debug/.test(debug.consola),
      JSON.stringify({ ks: debug.ks, apk: debug.apk, codigo: debug.codigo, veredicto: debug.veredicto }).slice(0, 300));
  } else {
    console.log("  --- sin ANDROID_JAR en el entorno: no se prueba el build completo en la pestaña");
  }


  // 13. la página servida de forma estática: el paquete oficial de Google es un .zip con
  // android-34/android.jar dentro; el editor lo extrae al elegirlo, sin unzip externo.
  const paquete = await evaluar(`(async () => {
    const interior = "PK" + "x".repeat(2048);
    const zip = makeZip([["android-34/android.jar", interior]]);
    const dt = new DataTransfer();
    dt.items.add(new File([zip], "platform-34.zip", { type: "application/zip" }));
    document.getElementById("android-jar").files = dt.files;
    await usarJarElegido();
    const jar = await hospedador.bytes("/sdk/android.jar");
    return { largo: jar ? jar.length : 0, cabecera: jar ? String.fromCharCode(jar[0], jar[1]) : "",
      aviso: document.getElementById("estado-jar").textContent };
  })()`);
  comprueba("el .zip oficial se descomprime y deja el android.jar",
    paquete.largo === 2050 && paquete.cabecera === "PK", JSON.stringify(paquete));

  const terminos = await evaluar(`(() => {
    const c = document.getElementById("acepto-sdk"), a = document.getElementById("descargar-sdk");
    const antes = a.getAttribute("aria-disabled");
    c.checked = true; c.dispatchEvent(new Event("change", { bubbles: true }));
    const despues = a.getAttribute("aria-disabled");
    c.checked = false; c.dispatchEvent(new Event("change", { bubbles: true }));
    return { antes, despues, href: a.href };
  })()`);
  comprueba("la descarga oficial exige aceptar los términos y apunta a Google",
    terminos.antes === "true" && terminos.despues === "false" &&
    /^https:\/\/dl\.google\.com\//.test(terminos.href), JSON.stringify(terminos));

  // 14. recarga sin red: el service worker sirve el shell y conserva el aislamiento
  const sw = await evaluar(`(async () => {
    if (!("serviceWorker" in navigator)) return { ok: false, motivo: "sin serviceWorker" };
    const listo = await Promise.race([
      navigator.serviceWorker.ready.then(() => true),
      new Promise((ok) => setTimeout(() => ok(false), 5000)),
    ]);
    return { ok: listo, controlado: !!navigator.serviceWorker.controller };
  })()`);
  comprueba("el service worker queda listo", sw.ok === true, JSON.stringify(sw));
  if (sw.ok) {
    await exec("Network.emulateNetworkConditions", {
      offline: true, latency: 0, downloadThroughput: -1, uploadThroughput: -1,
    });
    await exec("Page.navigate", { url: URL_PAGINA });
    let offline = null;
    for (let i = 0; i < 50; i++) {
      try {
        offline = await evaluar(`({
          listo: typeof abrir === "function" && !!document.getElementById("codigo"),
          aislada: crossOriginIsolated,
          controlado: !!navigator.serviceWorker.controller,
        })`);
        if (offline.listo) break;
      } catch (e) { /* aún carga */ }
      await esperar(300);
    }
    comprueba("la página recarga sin red con el service worker",
      offline && offline.listo === true, JSON.stringify(offline));
    comprueba("la recarga offline conserva crossOriginIsolated",
      offline && offline.aislada === true, JSON.stringify(offline));
    await exec("Network.emulateNetworkConditions", {
      offline: false, latency: 0, downloadThroughput: -1, uploadThroughput: -1,
    });
  }

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
