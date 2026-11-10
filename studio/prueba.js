// prueba de humo con node: genera el zip con el mismo código que la página y lo escribe en
// disco; después python zipfile y zipcheck.py del harness tienen que estar de acuerdo.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { buildProject, makeZip, analyze, RULES, crc32 } from "./app.js";
import * as pestanaMod from "./pestana.js";
import { Historial } from "./historial.js";
import { fusionar } from "./fusion.js";
import { leerZip, zipDelArbol, paqueteDelManifiesto, nombreDeZip, rutaSegura,
  ajustesDelManifiesto, conAjustes } from "./proyecto.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const a = { buildProject, makeZip, analyze, RULES, crc32 };

const estado = {
  name: "Hola apkc", package: "com.hola.apkcstudio", minsdk: "21", targetsdk: "33",
  hello: "Hello World!", button: true, kotlin: process.env.STUDIO_KOTLIN === "1", libs: true,
  native: true, assets: true, device: true,
};
if (estado.kotlin) estado.package = "com.hola.apkcstudio.kt";

let fallos = 0;
const mal = (msg) => { fallos++; console.log("  FALLO:", msg); };

const p = a.buildProject(estado);
const entradas = Object.entries(p.files).map(([r, t]) => [estado.package + "/" + r, t]);
for (const d of p.dirs) entradas.push([estado.package + "/" + d + "/", ""]);

// 1. lo que se genera es código: un carácter fuera de ascii dentro de una plantilla copia
//    literal de apkc/template/* rompe el XML y aapt2 lo rechaza en el telefono. Los LEEME
//    son prosa para quien descomprime, y esos si pueden llevar tildes.
for (const [ruta, texto] of Object.entries(p.files)) {
  if (!/\.(xml|java|kt)$/.test(ruta)) continue;
  const raros = [...new Set([...texto].filter((c) => c.charCodeAt(0) > 127))];
  if (raros.length) mal(`${ruta} lleva ${raros.length} caracteres no ascii: ${raros.join(" ")}`);
  // un placeholder que no se sustituyo, o un undefined que JS convirtio en texto: el fichero
  // compila y la app sale diciendo "undefined" o mostrando "{{toast}}"
  for (const mala of texto.match(/\{\{\w+\}\}/g) || []) mal(`${ruta} conserva ${mala}`);
  for (const linea of texto.split("\n")) if (/\bundefined\b/.test(linea)) mal(`${ruta}: ${linea.trim()}`);
  const llaves = (texto.match(/\{/g) || []).length, cierres = (texto.match(/\}/g) || []).length;
  if (/\.(java|kt)$/.test(ruta) && llaves !== cierres) mal(`${ruta}: ${llaves} llaves abiertas, ${cierres} cerradas`);
}

// 2. y las partes fijas tienen que ser las del repo, no la versión que uno recuerda: lo que
//    se exige es que todo atributo que apkc escribe esté ahí y bien escrito. La página añade
//    cosas que la plantilla del repo no tiene (un Boton, extractNativeLibs) y eso no es un fallo.
const repo = process.env.APKC_REPO || path.join(process.env.HOME, "Develop/Go/apkc");
const plantillas = { "AndroidManifest.xml": "manifest", "res/layout/main.xml": "layout", "res/values/styles.xml": "styles" };
const token = (s) => [...s.matchAll(/android:[A-Za-z][A-Za-z0-9]*|<[?/]?[A-Za-z][\w.-]*/g)].map((m) => m[0]);
for (const [ruta] of Object.entries(plantillas)) {
  const origen = path.join(repo, "template", path.basename(ruta));
  if (!fs.existsSync(origen)) { console.log("  (sin repo en", origen, ": comparacion omitida)"); continue; }
  const esperado = new Set(token(fs.readFileSync(origen, "utf8")));
  const obtenido = new Set(token(p.files[ruta]));
  // maxSdkVersion se omite a propósito: fijar la API máxima en el manifiesto generado deja la
  // app sin poder instalarse en versiones futuras (docs/plan.md, B1)
  const faltan = [...esperado].filter((t) => !obtenido.has(t) && !t.startsWith("<?") &&
    t !== "</manifest>" && t !== "android:maxSdkVersion");
  if (faltan.length) mal(`${ruta} no tiene ${faltan.join(", ")} como en ${origen}`);
}

const buf = Buffer.from(a.makeZip(entradas));
const tmp = process.env.PREFIX ? path.join(process.env.PREFIX, "tmp") : process.env.TMPDIR || ".";
const salida = path.join(tmp, process.env.STUDIO_OUT || "studio-test.zip");
fs.writeFileSync(salida, buf);
console.log("zip:", salida, buf.length, "bytes,", entradas.length, "entradas");

// 2b. la plantilla Contador no puede ser un adorno: cambia el layout y la actividad, y conserva
// el package y el manifiesto. Se comprueba en node y e2e la compila de verdad.
const contador = a.buildProject({ ...estado, template: "contador", kotlin: false, libs: false,
  native: false, assets: false, device: false });
const actContador = contador.files["src/" + estado.package.split(".").join("/") + "/MainActivity.java"];
const layContador = contador.files["res/layout/main.xml"];
if (!/TextView total;/.test(actContador) || !/total\.setText\("Pulsos: " \+ pulsos\)/.test(actContador)) {
  mal("la plantilla Contador no escribe el campo y la cuenta");
}
if (!/@\+id\/total/.test(layContador) || !/@\+id\/pulsar/.test(layContador)) {
  mal("la plantilla Contador no declara los ids que usa la actividad");
}
if (/Hello World!/.test(layContador)) mal("la plantilla Contador conserva el texto de Hola mundo");

// 3. el EOCD y los modos: un lector estricto (zipcheck.py) es el que ve lo que zipfile tolera
const eocd = buf.readUInt16LE(buf.lastIndexOf(Buffer.from("PK\x05\x06", "latin1")) + 8);
if (eocd !== entradas.length) mal(`el EOCD anuncia ${eocd} entradas y hay ${entradas.length}`);

// 4. analizador contra salidas reales de apkc, no contra cómo uno se imagina un error
const muestras = JSON.parse(fs.readFileSync(path.join(__dirname, "muestras.json"), "utf8"));
let cubiertas = 0;
for (const m of muestras) {
  const { hits } = a.analyze(m.texto);
  const ok = hits.some((h) => h.id === m.esperada);
  if (ok) cubiertas++;
  else mal(`sin cubrir: ${m.esperada} <- ${m.texto.split("\n")[0].slice(0, 70)}`);
}
console.log("reglas confirmadas contra salidas reales:", cubiertas + "/" + muestras.length);

// 5. y ninguna regla puede estar inventada: la de apkc se busca en el fuente, la de otra
//    herramienta tiene que estar en alguna salida capturada. Sin esto, la página explicaría
//    un mensaje que nadie imprime, que es peor que no explicarlo.
const fuentes = fs.readdirSync(repo).filter((f) => f.endsWith(".go") && !f.endsWith("_test.go"))
  .map((f) => fs.readFileSync(path.join(repo, f), "utf8")).join("\n");
const capturado = muestras.map((m) => m.texto).join("\n");
let sinEvidencia = 0;
for (const r of a.RULES) {
  if (!r.lit) { mal(`${r.id}: sin lit:, no se puede comprobar de donde salio`); continue; }
  if (r.de === "apkc") {
    if (!fuentes.includes(r.lit)) mal(`${r.id}: "${r.lit}" no esta en el fuente de apkc: regla inventada`);
  } else if (!capturado.includes(r.lit)) {
    sinEvidencia++;
    console.log(`  (sin captura: ${r.id} para "${r.lit}", dicha por ${r.de})`);
  }
}
console.log("reglas:", a.RULES.length, "| ancladas al fuente de apkc o a una salida capturada:",
  a.RULES.length - sinEvidencia, "| sin captura propia:", sinEvidencia);

// 6. el cableado del DOM, que es lo unico que prueba.js no puede ejecutar: todo id que vistas.js
//    pide tiene que estar en index.html, y todo name del formulario lo tiene que leer el creador
const html = fs.readFileSync(path.join(__dirname, "index.html"), "utf8");
const js = fs.readFileSync(path.join(__dirname, "vistas.js"), "utf8");
const ids = new Set([...html.matchAll(/id="([^"]+)"/g)].map((m) => m[1]));
for (const m of js.matchAll(/getElementById\("([^"]+)"\)/g))
  if (!ids.has(m[1])) mal(`vistas.js pide #${m[1]} y index.html no lo tiene`);
// lo mismo para la pestaña: es el otro fichero que toca el DOM, y un id que no existe se traduce
// en un `addEventListener` sobre null que solo se ve cuando alguien pulsa el boton
const pestana = fs.readFileSync(path.join(__dirname, "pestana.js"), "utf8");
for (const m of pestana.matchAll(/getElementById\("([^"]+)"\)/g))
  if (!ids.has(m[1])) mal(`pestana.js pide #${m[1]} y index.html no lo tiene`);
for (const fichero of ["diagnostico.js", "temas.js", "historial.js"]) {
  const fuente = fs.readFileSync(path.join(__dirname, fichero), "utf8");
  for (const m of fuente.matchAll(/getElementById\("([^"]+)"\)/g))
    if (!ids.has(m[1])) mal(`${fichero} pide #${m[1]} y index.html no lo tiene`);
}
for (const m of html.matchAll(/<input[^>]*name="([^"]+)"/g))
  if (!new RegExp(`"(f\\.get|f\\.has)\\(\"${m[1]}\"`).test(js) && !js.includes(`"${m[1]}"`))
    mal(`el formulario tiene ${m[1]} y estado() no lo lee`);

// 7. el resaltador del editor es puro y se puede probar sin DOM; si se rompe, el código se
// queda sin color o, peor, el HTML del resaltado se come el texto
const java = pestanaMod.resaltar("package x;\n// hola\npublic class X {}\n", "java");
if (!/<span class="palabra">package<\/span>/.test(java)) mal("el resaltador no pinta package");
if (!/<span class="comentario">\/\/ hola<\/span>/.test(java)) mal("el resaltador no pinta el comentario");
const anotado = pestanaMod.resaltar("@Override int x = 42; char c = 'a';", "java");
if (!/class="anotacion">@Override</.test(anotado)) mal("el resaltador no pinta la anotación");
if (!/class="numero">42</.test(anotado)) mal("el resaltador no pinta el número");
if (!/class="cadena">'a'</.test(anotado)) mal("el resaltador no pinta el char");
const xml = pestanaMod.resaltar('<manifest package="x">', "xml");
if (!/class="etiqueta">manifest</.test(xml)) mal("el resaltador no pinta la etiqueta XML");
if (!/class="atributo">package</.test(xml)) mal("el resaltador no pinta el atributo XML");
if (pestanaMod.esXml("package x;") || !pestanaMod.esXml('<?xml version="1.0"?>')) mal("esXml no distingue Java de XML");
if (pestanaMod.esXml("package x; import java.util.List; class X { List<String> xs; }"))
  mal("esXml confunde los genéricos de Java con etiquetas XML");
if (!pestanaMod.esXml('<!-- c --><resources><string name="x">y</string></resources>'))
  mal("esXml no reconoce un recurso que empieza por comentario");

// los códigos ANSI de apkc no se enseñan crudos: se convierten en spans con su color
const color = pestanaMod.consolaHtml("\u001b[31mrojo\u001b[0m y \u001b[1;34mazul\u001b[0m");
if (!/class="consola-fg-1">rojo</.test(color) || !/class="consola-fuerte consola-fg-4">azul</.test(color))
  mal("consolaHtml no traduce los colores ANSI");
if (/\u001b/.test(color)) mal("consolaHtml deja códigos de escape en la consola");
if (!/&lt;b&gt;/.test(pestanaMod.consolaHtml("<b>"))) mal("consolaHtml no escapa el HTML");
// una salida `ruta:línea` se convierte en un enlace que abre el fichero
const conSalto = pestanaMod.consolaHtml("src/com/x/Main.java:7: error: falta ;");
if (!/<a href="#" class="salto" data-ruta="src\/com\/x\/Main\.java" data-linea="7">src\/com\/x\/Main\.java:7<\/a>/.test(conSalto)) {
  mal("consolaHtml no enlaza ruta:línea: " + conSalto);
}
if (/class="salto"[^>]*>.*&lt;/.test(conSalto)) mal("el enlace de la consola se come el texto escapado");

// 8. el árbol de deshacer: ramas, reconstrucción y poda sin tocar la rama actual
const historial = Historial;
const h = historial.crear("/proyecto/A.java", "uno");
historial.agregar(h, "uno\ndos");
historial.agregar(h, "uno\ndos\ntres");
if (historial.textoDe(h, h.actual) !== "uno\ndos\ntres") mal("el árbol no reconstruye el texto actual");
historial.deshacer(h);
historial.agregar(h, "uno\nRAMA");
const raiz = h.nodos[h.raiz];
const rama = raiz.h[0] ? h.nodos[raiz.h[0]] : null;
if (!rama || rama.h.length !== 2) mal("deshacer + escribir no creó una rama");
if (historial.textoDe(h, h.actual) !== "uno\nRAMA") mal("la rama nueva no se lee");
historial.restaurar(h, h.nodos[raiz.h[0]].h[0]);
if (historial.textoDe(h, h.actual) !== "uno\ndos\ntres") mal("la rama vieja no se conserva");
const recorrido = historial.recorrer(h);
if (recorrido.length !== 4 || recorrido[0].nivel !== 0) mal("recorrer no devuelve el árbol con niveles");
for (let i = 0; i < 300; i++) historial.agregar(h, "v" + i);
const nodos = Object.keys(h.nodos).length;
if (nodos > historial.MAX_NODOS) mal("la poda no respeta MAX_NODOS: " + nodos);
if (!h.nodos[h.actual]) mal("la poda borró el nodo actual");

// 9. fusión de tres vías: cambios separados se combinan; los que se pisan, conflicto
const baseFusion = "uno\ndos\ntres\n";
const auto = fusionar(baseFusion, "uno\nDOS\ntres\n", "uno\ndos\nTRES\n");
if (auto.estado !== "auto" || auto.texto !== "uno\nDOS\nTRES\n") mal("la fusión automática no combina cambios separados");
const choque = fusionar(baseFusion, "uno\nMIO\ntres\n", "uno\nSUYO\ntres\n");
if (choque.estado !== "conflicto") mal("la fusión no detecta el cambio en la misma línea");
if (fusionar(baseFusion, baseFusion, "uno\ndos\nTRES\n").estado !== "suyo") mal("la fusión no toma el remoto si el local no cambió");
if (fusionar(baseFusion, "uno\nDOS\ntres\n", baseFusion).estado !== "nuestro") mal("la fusión no conserva el local si el remoto no cambió");

// 9. el paquete oficial de la plataforma es un zip con android-34/android.jar: el editor tiene
// que sacarlo sin depender de un unzip externo. Se prueba con el mismo makeZip de la página.
(async () => {
  const interior = "PK" + "x".repeat(2048);
  const zip = a.makeZip([["android-34/android.jar", interior]]);
  const jar = await pestanaMod.jarDeSeleccion(zip);
  if (new TextDecoder().decode(jar) !== interior) mal("jarDeSeleccion no extrae android.jar del .zip oficial");

  // 10. exportar e importar el proyecto: binarios intactos, carpetas, y rutas que no salen
  const icono = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0, 255, 1, 254]);
  const zipProyecto = zipDelArbol([
    ["AndroidManifest.xml", '<manifest package="com.hola.apkcstudio"/>'],
    ["res/drawable/icono.png", icono],
    ["res/drawable/", new Uint8Array(0)],
  ]);
  const vuelta = await leerZip(zipProyecto);
  const png = vuelta.find((e) => e.ruta === "res/drawable/icono.png");
  if (!png || png.datos.length !== icono.length || png.datos.some((b, i) => b !== icono[i])) {
    mal("leerZip no devuelve el PNG byte a byte");
  }
  if (!vuelta.some((e) => e.ruta === "res/drawable/" && e.dir)) mal("leerZip pierde las carpetas");

  // un zip hecho con DEFLATE (lo normal fuera de esta página) también entra
  const texto = "package x;\nclass X {}\n";
  const crudo = Buffer.from(texto);
  const comprimido = new Uint8Array(await new Response(
    new Blob([crudo]).stream().pipeThrough(new CompressionStream("deflate-raw"))).arrayBuffer());
  const nombre = Buffer.from("src/x/X.java");
  const local = Buffer.alloc(30);
  local.writeUInt32LE(0x04034b50, 0); local.writeUInt16LE(20, 4); local.writeUInt16LE(0x0800, 6);
  local.writeUInt16LE(8, 8); local.writeUInt32LE(a.crc32(crudo), 14);
  local.writeUInt32LE(comprimido.length, 18); local.writeUInt32LE(crudo.length, 22);
  local.writeUInt16LE(nombre.length, 26);
  const centralD = Buffer.alloc(46);
  centralD.writeUInt32LE(0x02014b50, 0); centralD.writeUInt16LE(20, 4); centralD.writeUInt16LE(20, 6);
  centralD.writeUInt16LE(0x0800, 8); centralD.writeUInt16LE(8, 10);
  centralD.writeUInt32LE(a.crc32(crudo), 16); centralD.writeUInt32LE(comprimido.length, 20);
  centralD.writeUInt32LE(crudo.length, 24); centralD.writeUInt16LE(nombre.length, 28);
  centralD.writeUInt32LE(0, 42);
  const fin = Buffer.alloc(22);
  fin.writeUInt32LE(0x06054b50, 0); fin.writeUInt16LE(1, 8); fin.writeUInt16LE(1, 10);
  fin.writeUInt32LE(46 + nombre.length, 12); fin.writeUInt32LE(30 + nombre.length + comprimido.length, 16);
  const zipDeflate = Buffer.concat([local, nombre, comprimido, centralD, nombre, fin]);
  const deflate = await leerZip(new Uint8Array(zipDeflate));
  if (deflate.length !== 1 || new TextDecoder().decode(deflate[0].datos) !== texto) {
    mal("leerZip no infla una entrada DEFLATE");
  }

  if (rutaSegura("../fuera.txt") || rutaSegura("/abs.txt") || rutaSegura("a\\b.txt") || !rutaSegura("a/b.txt")) {
    mal("rutaSegura no filtra las rutas que salen del proyecto");
  }
  if (paqueteDelManifiesto('<manifest package="com.x.y">') !== "com.x.y") mal("no lee el paquete del manifiesto");
  if (nombreDeZip("com.x.y") !== "com.x.y.zip" || nombreDeZip("") !== "proyecto.zip") mal("nombreDeZip no nombra el zip");

  // 11. los ajustes se escriben en el manifiesto y no en un fichero paralelo
  const manifiesto = p.files["AndroidManifest.xml"];
  if (/maxSdkVersion/.test(manifiesto)) mal("la plantilla sigue fijando maxSdkVersion");
  const ajustado = conAjustes(manifiesto, { minSdk: 24, targetSdk: 35, versionCode: 7, versionName: "1.0" });
  const leidos = ajustesDelManifiesto(ajustado);
  if (leidos.minSdk !== "24" || leidos.targetSdk !== "35" || leidos.versionCode !== "7" ||
      leidos.versionName !== "1.0") mal("conAjustes no escribe los cuatro valores: " + JSON.stringify(leidos));
  if (/maxSdkVersion/.test(ajustado)) mal("conAjustes no quita maxSdkVersion");
  if ((ajustado.match(/<uses-sdk/g) || []).length !== 1) mal("conAjustes duplica el bloque uses-sdk");
  // sin uses-sdk se añade, y los atributos que ya estaban no se pierden
  const sinUsos = conAjustes('<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.x">\n</manifest>',
    { minSdk: 21, targetSdk: 34, versionCode: 1, versionName: "0.1" });
  if (!/<uses-sdk[^>]*android:minSdkVersion="21"/.test(sinUsos) || !/package="com.x"/.test(sinUsos)) {
    mal("conAjustes no añade uses-sdk cuando falta");
  }
  let rechazo = "";
  try { conAjustes(manifiesto, { minSdk: 34, targetSdk: 21, versionCode: 1, versionName: "1" }); }
  catch (e) { rechazo = e.message; }
  if (!/targetSdk no puede ser menor/.test(rechazo)) mal("conAjustes acepta targetSdk menor que minSdk");
  let rechazoNombre = "";
  try { conAjustes(manifiesto, { minSdk: 21, targetSdk: 34, versionCode: 1, versionName: "1<2" }); }
  catch (e) { rechazoNombre = e.message; }
  if (!/versionName/.test(rechazoNombre)) mal("conAjustes acepta un versionName con <");

  console.log(fallos ? `PRUEBA FALLA: ${fallos} problemas` : "prueba: todo en orden");
  process.exit(fallos ? 1 : 0);
})();
