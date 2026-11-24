#!/usr/bin/env node
// falsificación del cuelgue del hospedador: el shim capitaneaba el exit REAL antes de sustituir
// globalThis.process, así que un Go que salía con 1 dejaba node con rc 0 y, si quedaba algún
// handle vivo (el binario de tests de apkc dejaba uno), node se quedaba en epoll_wait hasta que
// alguien lo mataba con SIGTERM (rc 143). Tampoco podía cortar la salida al salir: con un pipe
// moroso se perdía el final. Este guion compila cuatro sondas fuera del repo, las corre por el
// shim con un timeout corto y exige el código de verdad, la salida entera y el fs de siempre.
//
// uso: node shim/prueba-salida.js        (necesita go y shim/wasm_exec.js de tu GOROOT)
//      SHIM=<copia>/navegador-sim.js node shim/prueba-salida.js   para correrlo contra otro shim
const fs = require("fs");
const os = require("os");
const path = require("path");
const { spawnSync, spawn } = require("child_process");

const dirShim = __dirname;
const shim = process.env.SHIM ? path.resolve(process.env.SHIM) : path.join(dirShim, "navegador-sim.js");
const cache = process.env.XDG_CACHE_HOME
  ? path.join(process.env.XDG_CACHE_HOME, "apkc-web")
  : path.join(os.homedir(), ".cache", "apkc-web");
const tmp = path.join(cache, "prueba-salida");
const LIMITE_MS = 20000;

if (!fs.existsSync(path.join(dirShim, "wasm_exec.js"))) {
  console.error("falta shim/wasm_exec.js: cp \"$(go env GOROOT)/lib/wasm/wasm_exec.js\" shim/");
  process.exit(1);
}

// las cuatro sondas: sale con el código que le pidan, sale dejando un intervalo vivo en node
// (el síntoma del cuelgue), copia un fichero montado a otro que luego se exporta y escribe
// 200 KB con una marca al final (el pipe moroso)
const SONDAS = {
  sale: `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("sonda: salgo con 1")
	os.Exit(1)
}
`,
  viva: `package main

import (
	"fmt"
	"os"
	"syscall/js"
)

func main() {
	js.Global().Call("setInterval", js.FuncOf(func(this js.Value, args []js.Value) any { return nil }), 60000)
	fmt.Println("sonda: salgo con 1 y un intervalo vivo")
	os.Exit(1)
}
`,
  copia: `package main

import (
	"fmt"
	"os"
)

func main() {
	entrada, err := os.ReadFile("/in.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, "no pude leer /in.txt:", err)
		os.Exit(2)
	}
	if err := os.WriteFile("/out.txt", entrada, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "no pude escribir /out.txt:", err)
		os.Exit(2)
	}
	fmt.Println("sonda: copié", len(entrada), "bytes")
}
`,
  grande: `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	linea := strings.Repeat("a", 200)
	for i := 0; i < 1000; i++ {
		fmt.Println(linea)
	}
	fmt.Println("FIN-200KB")
	os.Exit(1)
}
`,
};

const entorno = { ...process.env, GOOS: "js", GOARCH: "wasm", CGO_ENABLED: "0", GOTMPDIR: path.join(tmp, "gotmp") };
fs.mkdirSync(entorno.GOTMPDIR, { recursive: true });

if (spawnSync("go", ["version"], { encoding: "utf8" }).status !== 0) {
  console.error("esta prueba compila sus sondas con go, y go no está en el PATH");
  process.exit(1);
}

function compilar(nombre) {
  const dir = path.join(tmp, nombre);
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(dir, "main.go"), SONDAS[nombre]);
  const salida = path.join(tmp, nombre + ".wasm");
  const r = spawnSync("go", ["build", "-o", salida, "main.go"], { cwd: dir, env: entorno, encoding: "utf8", timeout: 180000 });
  if (r.status !== 0) {
    console.error(`no compilé la sonda ${nombre}: ${(r.stderr || "") + (r.stdout || "")}`);
    process.exit(1);
  }
  return salida;
}

const fallos = [];
function comprobar(nombre, condicion, detalle) {
  if (condicion) {
    console.log(`  ok   ${nombre}`);
  } else {
    fallos.push(nombre);
    console.log(`  FALLO ${nombre}${detalle ? " — " + detalle : ""}`);
  }
}

function correr(argumentos) {
  const r = spawnSync(process.execPath, [shim, ...argumentos], { cwd: dirShim, encoding: "utf8", timeout: LIMITE_MS });
  return { codigo: r.status, senal: r.signal, salida: (r.stdout || "") + (r.stderr || "") };
}

// el mismo shim con un consumidor que drena 512 bytes cada 6 ms: el pipe se llena y los 200 KB
// solo llegan enteros si el hospedador espera a que stdio vacíe antes de salir. Un plazo de
// seguridad que corte con datos en vuelo trunca justo «FIN-200KB» y «(apkc devolvio 1)». Van
// 6 ms y no 4 porque con 4 el margen frente a un plazo de un segundo era demasiado justo: el
// buffer del hijo se vaciaba con el último byte y el defecto no se veía.
// El lector va en otro proceso y recibe el stdout del shim como stdin (stdio con el mismo
// descriptor): si se leyera desde este proceso, sus buffers de 64 KB se quedarían con casi
// toda la salida y taparían el defecto. Con el descriptor compartido el tope es el pipe real.
const LECTOR = `
const fs = require("fs");
const pausa = Number(process.argv[1] || 4);
const bloque = Number(process.argv[2] || 512);
const trozos = [];
const bufer = Buffer.alloc(bloque);
for (;;) {
  let n;
  try { n = fs.readSync(0, bufer, 0, bloque, null); }
  catch (e) { if (e.code === "EAGAIN") { const fin = Date.now() + 1; while (Date.now() < fin); continue; } throw e; }
  if (n === 0) break;
  trozos.push(Buffer.from(bufer.subarray(0, n)));
  const fin = Date.now() + pausa; while (Date.now() < fin);
}
const datos = Buffer.concat(trozos);
process.stdout.write(JSON.stringify({ bytes: datos.length, fin: datos.includes("FIN-200KB"), aviso: datos.includes("(apkc devolvio 1)") }));
`;

function correrDespacio(argumentos, pausaMs, bloque) {
  return new Promise((resolver) => {
    const hijo = spawn(process.execPath, [shim, ...argumentos], { cwd: dirShim, stdio: ["ignore", "pipe", "pipe"] });
    let errores = "";
    hijo.stderr.on("data", (d) => { errores += d.toString(); });
    const lector = spawn(process.execPath, ["-e", LECTOR, String(pausaMs), String(bloque)], { stdio: [hijo.stdout, "pipe", "inherit"] });
    let informe = "";
    lector.stdout.on("data", (d) => { informe += d.toString(); });
    let codigo = null;
    let senal = null;
    const salida = new Promise((listo) => hijo.on("exit", (c, s) => { codigo = c; senal = s; listo(); }));
    const plazo = setTimeout(() => { hijo.kill("SIGTERM"); lector.kill("SIGTERM"); }, 120000);
    lector.on("close", async () => {
      clearTimeout(plazo);
      await salida;                            // el EOF llega después de que el hijo salga
      let datos = {};
      try { datos = JSON.parse(informe); } catch (_) { datos = { roto: informe.slice(0, 200) }; }
      resolver({ codigo, senal, datos, errores });
    });
  });
}

async function principal() {
  const sale = compilar("sale");
  const viva = compilar("viva");
  const copia = compilar("copia");

  console.log("1. un Go que sale con 1 tiene que dar rc 1 (antes: rc 0 y el código perdido)");
  const uno = correr([sale]);
  comprobar("rc 1", uno.codigo === 1, `rc ${uno.codigo}, señal ${uno.senal}`);
  comprobar("avisa del código", uno.salida.includes("(apkc devolvio 1)"));

  console.log("2. el mismo Go con un handle vivo no puede quedarse en epoll_wait (antes: rc 143 al matarlo)");
  const vivo = correr([viva]);
  comprobar("rc 1", vivo.codigo === 1, `rc ${vivo.codigo}, señal ${vivo.senal}`);
  comprobar("sin SIGTERM del timeout", vivo.senal === null, `señal ${vivo.senal}`);

  console.log("3. --montar y --exportar siguen haciendo lo de siempre");
  const entrada = path.join(tmp, "in.txt");
  const exportado = path.join(tmp, "out.txt");
  fs.writeFileSync(entrada, "contenido montado\n");
  fs.rmSync(exportado, { force: true });
  const copiado = correr(["--montar", `${entrada}:/in.txt`, "--exportar", `/out.txt:${exportado}`, copia]);
  comprobar("rc 0", copiado.codigo === 0, `rc ${copiado.codigo}, señal ${copiado.senal}`);
  comprobar("el fichero exportado coincide", fs.existsSync(exportado) && fs.readFileSync(exportado, "utf8") === "contenido montado\n");

  console.log("4. 200 KB por un pipe moroso tienen que llegar enteros (antes: cortados a media frase)");
  const grande = await correrDespacio([compilar("grande")], 6, 512);
  comprobar("rc 1", grande.codigo === 1, `rc ${grande.codigo}, señal ${grande.senal}`);
  comprobar(`los 200 KB llegan (${grande.datos.bytes} bytes)`, (grande.datos.bytes || 0) > 200000, `${grande.datos.bytes} bytes`);
  comprobar("la marca final llega", grande.datos.fin === true);
  comprobar("el aviso final llega", grande.datos.aviso === true);

  if (fallos.length) {
    console.error(`\nprueba-salida: ${fallos.length} fallo(s): ${fallos.join(", ")}`);
    process.exit(1);
  }
  console.log("\nprueba-salida: el shim propaga el código de salida, no corta la salida y conserva el fs simulado");
}

principal().catch((e) => {
  console.error("prueba-salida: " + (e && e.stack ? e.stack : e));
  process.exit(1);
});
