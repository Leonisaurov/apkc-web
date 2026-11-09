// prueba de humo con node: genera el zip con el mismo código que la página y lo escribe en
// disco; después python zipfile y zipcheck.py del harness tienen que estar de acuerdo.
const fs = require("fs");
const path = require("path");
const a = require(path.join(__dirname, "app.js"));

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
  const faltan = [...esperado].filter((t) => !obtenido.has(t) && !t.startsWith("<?") && t !== "</manifest>");
  if (faltan.length) mal(`${ruta} no tiene ${faltan.join(", ")} como en ${origen}`);
}

const buf = Buffer.from(a.makeZip(entradas));
const tmp = process.env.PREFIX ? path.join(process.env.PREFIX, "tmp") : process.env.TMPDIR || ".";
const salida = path.join(tmp, process.env.STUDIO_OUT || "studio-test.zip");
fs.writeFileSync(salida, buf);
console.log("zip:", salida, buf.length, "bytes,", entradas.length, "entradas");

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
for (const m of html.matchAll(/<input[^>]*name="([^"]+)"/g))
  if (!new RegExp(`"(f\\.get|f\\.has)\\(\"${m[1]}\"`).test(js) && !js.includes(`"${m[1]}"`))
    mal(`el formulario tiene ${m[1]} y estado() no lo lee`);

// 7. el resaltador del editor es puro y se puede probar sin DOM; si se rompe, el código se
// queda sin color o, peor, el HTML del resaltado se come el texto
const pestanaMod = require(path.join(__dirname, "pestana.js"));
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

// 8. el paquete oficial de la plataforma es un zip con android-34/android.jar: el editor tiene
// que sacarlo sin depender de un unzip externo. Se prueba con el mismo makeZip de la página.
(async () => {
  const interior = "PK" + "x".repeat(2048);
  const zip = a.makeZip([["android-34/android.jar", interior]]);
  const jar = await pestanaMod.jarDeSeleccion(zip);
  if (new TextDecoder().decode(jar) !== interior) mal("jarDeSeleccion no extrae android.jar del .zip oficial");
  console.log(fallos ? `PRUEBA FALLA: ${fallos} problemas` : "prueba: todo en orden");
  process.exit(fallos ? 1 : 0);
})();
