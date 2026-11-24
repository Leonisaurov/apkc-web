#!/usr/bin/env node
// falsificación del cuelgue del hospedador: el shim capitaneaba el exit REAL antes de sustituir
// globalThis.process, así que un Go que salía con 1 dejaba node con rc 0 y, si quedaba algún
// handle vivo (el binario de tests de apkc dejaba uno), node se quedaba en epoll_wait hasta que
// alguien lo mataba con SIGTERM (rc 143). Este guion compila tres sondas fuera del repo, las
// corre por el shim con un timeout corto y exige el código de verdad y el fs de siempre.
//
// uso: node shim/prueba-salida.js        (necesita go y shim/wasm_exec.js de tu GOROOT)
const fs = require("fs");
const os = require("os");
const path = require("path");
const { spawnSync } = require("child_process");

const dirShim = __dirname;
const shim = path.join(dirShim, "navegador-sim.js");
const cache = process.env.XDG_CACHE_HOME
  ? path.join(process.env.XDG_CACHE_HOME, "apkc-web")
  : path.join(os.homedir(), ".cache", "apkc-web");
const tmp = path.join(cache, "prueba-salida");
const LIMITE_MS = 20000;

if (!fs.existsSync(path.join(dirShim, "wasm_exec.js"))) {
  console.error("falta shim/wasm_exec.js: cp \"$(go env GOROOT)/lib/wasm/wasm_exec.js\" shim/");
  process.exit(1);
}

// las tres sondas: sale con el código que le pidan, sale dejando un intervalo vivo en node
// (el síntoma del cuelgue) y copia un fichero montado a otro que luego se exporta
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
const salida = path.join(tmp, "out.txt");
fs.writeFileSync(entrada, "contenido montado\n");
fs.rmSync(salida, { force: true });
const copiado = correr(["--montar", `${entrada}:/in.txt`, "--exportar", `/out.txt:${salida}`, copia]);
comprobar("rc 0", copiado.codigo === 0, `rc ${copiado.codigo}, señal ${copiado.senal}`);
comprobar("el fichero exportado coincide", fs.existsSync(salida) && fs.readFileSync(salida, "utf8") === "contenido montado\n");

if (fallos.length) {
  console.error(`\nprueba-salida: ${fallos.length} fallo(s): ${fallos.join(", ")}`);
  process.exit(1);
}
console.log("\nprueba-salida: el shim propaga el código de salida y conserva el fs simulado");
